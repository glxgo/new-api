package model

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestCapabilityRecoveryClaimStopFenceAndImmutableRevision(t *testing.T) {
	db := capabilityTestDB(t)
	row, err := UpdateCapabilityControl(0, 1, "resume", func(c *CapabilityControl) error { c.Running = true; c.ExecutionRevision = 1; return nil })
	require.NoError(t, err)
	require.NoError(t, db.Create(&CapabilityWorker{ID: 1, Owner: "one", Heartbeat: time.Now().Unix()}).Error)
	require.NoError(t, db.Create(&CapabilityRun{ID: "source", Identity: "source", Status: "complete", Result: "original-public-result"}).Error)
	require.NoError(t, EnqueueCapabilityRecovery(db, "source", "geometry", 1))
	require.NoError(t, EnqueueCapabilityRecovery(db, "source", "geometry", 1))
	var claimed atomic.Int32
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			job, e := ClaimCapabilityRecovery("one", 1)
			if e == nil && job != nil {
				claimed.Add(1)
			}
		}()
	}
	group.Wait()
	require.EqualValues(t, 1, claimed.Load())
	var old CapabilityRecoveryJob
	require.NoError(t, db.First(&old).Error)
	// Redis lease takeover is reflected by the worker row; old owners cannot commit.
	require.NoError(t, db.Model(&CapabilityWorker{}).Where("id = ?", 1).Update("owner", "two").Error)
	job, err := ClaimCapabilityRecovery("two", 1)
	require.NoError(t, err)
	require.NotNil(t, job)
	require.Error(t, FinishCapabilityRecovery(old, "complete", "", "stale"))
	require.NoError(t, FinishCapabilityRecovery(*job, "pending", "renderer_unavailable", "first-evaluation"))
	// An unchanged infrastructure retry increments checks, not evidence copies.
	require.NoError(t, db.Model(&CapabilityRecoveryJob{}).Where("id = ?", job.ID).Update("next_at", 0).Error)
	job, err = ClaimCapabilityRecovery("two", 1)
	require.NoError(t, err)
	require.NotNil(t, job)
	require.NoError(t, FinishCapabilityRecovery(*job, "pending", "renderer_unavailable", "first-evaluation"))
	require.NoError(t, db.Model(&CapabilityRecoveryJob{}).Where("id = ?", job.ID).Update("next_at", 0).Error)
	job, err = ClaimCapabilityRecovery("two", 1)
	require.NoError(t, err)
	require.NotNil(t, job)
	require.NoError(t, FinishCapabilityRecovery(*job, "complete", "", "second-evaluation"))
	var source CapabilityRun
	require.NoError(t, db.First(&source, "id = ?", "source").Error)
	require.Equal(t, "original-public-result", source.Result)
	var revisions []CapabilityEvaluation
	require.NoError(t, db.Order("revision").Find(&revisions).Error)
	require.Len(t, revisions, 2)
	require.Equal(t, 1, revisions[0].Revision)
	require.Equal(t, 2, revisions[1].Revision)
	require.Equal(t, "first-evaluation", revisions[0].Result)
	require.NoError(t, EnqueueCapabilityRecovery(db, "source", "scene", 1))
	job, err = ClaimCapabilityRecovery("two", 1)
	require.NoError(t, err)
	require.NotNil(t, job)
	row, err = UpdateCapabilityControl(row.Revision, 1, "stop", func(c *CapabilityControl) error { c.Running = false; c.ExecutionRevision++; return nil })
	require.NoError(t, err)
	require.Error(t, FinishCapabilityRecovery(*job, "complete", "", "after-stop"))
	_, err = ClaimCapabilityRecovery("two", 1)
	require.Error(t, err)
	row, err = UpdateCapabilityControl(row.Revision, 1, "resume", func(c *CapabilityControl) error { c.Running = true; c.ExecutionRevision++; return nil })
	require.NoError(t, err)
	job, err = ClaimCapabilityRecovery("two", row.ExecutionRevision)
	require.NoError(t, err)
	require.Nil(t, job)
	var stopped CapabilityRecoveryJob
	require.NoError(t, db.First(&stopped, "kind = ?", "scene").Error)
	require.Equal(t, "cancelled", stopped.Status)
}

func TestCapabilityRecoveryJudgeBudgetAndDuplicateDispatch(t *testing.T) {
	db := capabilityTestDB(t)
	config := DefaultCapabilityConfig()
	config.DailyBudgetMicros = 100
	config.CallReserveMicros = 50
	raw, err := common.Marshal(config)
	require.NoError(t, err)
	_, err = UpdateCapabilityControl(0, 1, "resume", func(c *CapabilityControl) error { c.Running = true; c.Settings = string(raw); return nil })
	require.NoError(t, err)
	require.NoError(t, db.Create(&CapabilityWorker{ID: 1, Owner: "worker", Heartbeat: time.Now().Unix()}).Error)
	run := CapabilityRun{ID: "source", Identity: "source", Status: "complete", Owner: "old-worker"}
	require.NoError(t, db.Create(&run).Error)
	require.NoError(t, EnqueueCapabilityRecovery(db, run.ID, "scene", 0))
	job, err := ClaimCapabilityRecovery("worker", 0)
	require.NoError(t, err)
	require.NotNil(t, job)
	run.Owner = "worker"
	require.Error(t, DispatchCapabilityRecoveryAttempt(run, &CapabilityAttempt{Kind: "scene"}, *job, 100, 50))
	attempt := CapabilityAttempt{Kind: "judge"}
	require.NoError(t, DispatchCapabilityRecoveryAttempt(run, &attempt, *job, 100, 50))
	require.ErrorContains(t, DispatchCapabilityRecoveryAttempt(run, &CapabilityAttempt{Kind: "judge"}, *job, 100, 50), "already_dispatched")
	var budget CapabilityBudgetDay
	require.NoError(t, db.First(&budget).Error)
	require.EqualValues(t, 50, budget.ReservedMicros)
	require.NoError(t, db.Model(&CapabilityControl{}).Where("id = ?", 1).Update("running", false).Error)
	require.Error(t, DispatchCapabilityRecoveryAttempt(run, &CapabilityAttempt{Kind: "judge"}, *job, 100, 50))
}
