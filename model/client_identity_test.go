package model

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupClientTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB, oldLogs := DB, LOG_DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:clients-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ClientIdentity{}, &ClientReview{}, &ClientGroupPolicy{}, &ClientGroupPolicyReview{}, &Log{}, &Task{}, &Midjourney{}))
	DB, LOG_DB = db, db
	t.Cleanup(func() { DB, LOG_DB = oldDB, oldLogs; sqlDB, _ := db.DB(); _ = sqlDB.Close() })
	return db
}

func TestClientApprovalLifecycleAndGroupPolicy(t *testing.T) {
	db := setupClientTestDB(t)
	ctx := context.Background()
	s := common.IdentifyClient("codex_cli_rs/1.0")
	require.NoError(t, ObserveClient(ctx, s))
	coding, err := CheckClientGroupAccess(ctx, s, "default")
	require.NoError(t, err)
	require.False(t, coding)
	coding, err = CheckClientGroupAccess(ctx, s, "Coding")
	require.Error(t, err)
	require.True(t, coding)
	require.Error(t, ReviewClient(ctx, s.ClientKey, "approved", " ", 1, 0))
	require.NoError(t, ReviewClient(ctx, s.ClientKey, "approved", "verified prefix", 1, 0))
	_, err = CheckClientGroupAccess(ctx, s, "Coding")
	require.Error(t, err, "approval alone must not authorize a group")
	p := ClientGroupPolicy{GroupName: "Coding", IsCoding: true, SupportedClients: []string{s.ClientKey}}
	require.Error(t, SaveClientGroupPolicy(ctx, p, "", 1))
	require.NoError(t, SaveClientGroupPolicy(ctx, p, "enable tested client", 1))
	_, err = CheckClientGroupAccess(ctx, s, "Coding")
	require.NoError(t, err)
	require.NoError(t, ObserveClient(ctx, common.IdentifyClient("codex_cli_rs/2.0")))
	var identity ClientIdentity
	require.NoError(t, db.First(&identity, "client_key = ?", s.ClientKey).Error)
	require.EqualValues(t, 2, identity.RequestCount)
	require.Equal(t, "approved", identity.Status)
	require.EqualValues(t, 1, identity.Revision)
	require.Error(t, ReviewClient(ctx, s.ClientKey, "rejected", "stale", 2, 0))
	require.NoError(t, ReviewClient(ctx, s.ClientKey, "pending", "revoke immediately", 2, 1))
	_, err = CheckClientGroupAccess(ctx, s, "Coding")
	require.Error(t, err, "next attempt must reread approval")
	require.NoError(t, ReviewClient(ctx, s.ClientKey, "approved", "restore", 2, 2))
	p.Revision, p.SupportedClients = 1, nil
	require.NoError(t, SaveClientGroupPolicy(ctx, p, "remove group support", 2))
	_, err = CheckClientGroupAccess(ctx, s, "Coding")
	require.Error(t, err, "next attempt must reread supported clients")
	require.Error(t, SaveClientGroupPolicy(ctx, p, "stale revision", 2))
	var reviews []ClientReview
	require.NoError(t, db.Order("id").Find(&reviews).Error)
	require.Len(t, reviews, 3)
	require.Equal(t, "approved", reviews[1].PreviousStatus)
	require.Equal(t, "revoke immediately", reviews[1].Reason)
	var policyReviews []ClientGroupPolicyReview
	require.NoError(t, db.Order("id").Find(&policyReviews).Error)
	require.Len(t, policyReviews, 2)
	require.Contains(t, policyReviews[1].PreviousPolicy, s.ClientKey)
	unknown := common.IdentifyClient("custom/1")
	require.NoError(t, ObserveClient(ctx, unknown))
	require.Error(t, ReviewClient(ctx, unknown.ClientKey, "approved", "cannot approve unidentified UA", 1, 0))
	_, err = CheckClientGroupAccess(ctx, unknown, "default")
	require.NoError(t, err)
}

func TestClientLogFilterPaginationAndUserIsolation(t *testing.T) {
	db := setupClientTestDB(t)
	for _, input := range []struct {
		user  int
		ua    string
		quota int
	}{{1, "codex_cli_rs/1.0", 10}, {2, "codex_cli_rs/1.0", 900}, {1, "curl/8.0", 20}, {1, "codex_cli_rs/2.0", 30}, {1, "", 40}, {2, "", 500}} {
		other := ""
		if input.ua != "" {
			other = common.MapToJsonStr(map[string]interface{}{"client": common.IdentifyClient(input.ua)})
		}
		require.NoError(t, db.Create(&Log{UserId: input.user, Username: fmt.Sprintf("client-test-%d", input.user), CreatedAt: 100, Type: LogTypeConsume, Quota: input.quota, Other: other}).Error)
	}
	ctx := WithClientLogFilter(context.Background(), "codex")
	rows, total, err := GetUserLogsWithContext(ctx, 1, LogTypeUnknown, 0, 200, "", "", 0, 1, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, rows, 1)
	require.Equal(t, 1, rows[0].UserId)
	require.Contains(t, rows[0].Other, "codex_cli_rs/2.0")
	rows, _, err = GetUserLogsWithContext(ctx, 1, LogTypeUnknown, 0, 200, "", "", 1, 1, "", "", "")
	require.NoError(t, err)
	require.Contains(t, rows[0].Other, "codex_cli_rs/1.0")
	rows, total, err = GetUserLogsWithContext(WithClientLogFilter(ctx, "unrecorded"), 1, LogTypeUnknown, 0, 200, "", "", 0, 20, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, rows, 1)
	require.NotContains(t, rows[0].Other, "client")
	for _, family := range []string{"codex", "curl", "unrecorded"} {
		stat, err := SumUsedQuotaWithContext(WithClientLogFilter(ctx, family), LogTypeConsume, 0, 200, "", "client-test-1", "", 0, "")
		require.NoError(t, err)
		want := map[string]int{"codex": 40, "curl": 20, "unrecorded": 40}[family]
		require.EqualValues(t, want, stat.Quota, "stat caches must include client family")
	}
}

func TestClientTaskFiltersPreserveOwnerAndSnapshot(t *testing.T) {
	db := setupClientTestDB(t)
	s := common.IdentifyClient("deepseek-harness/1.0")
	for _, user := range []int{1, 2} {
		require.NoError(t, db.Create(&Task{UserId: user, TaskID: fmt.Sprintf("client-task-%d", user), Client: &s, ClientFamily: s.Family, CodingGroup: true}).Error)
		require.NoError(t, db.Create(&Midjourney{UserId: user, MjId: fmt.Sprintf("client-mj-%d", user), Client: &s, ClientFamily: s.Family, CodingGroup: true}).Error)
	}
	require.NoError(t, db.Create(&Task{UserId: 1, TaskID: "legacy-client-task"}).Error)
	require.NoError(t, db.Create(&Midjourney{UserId: 1, MjId: "legacy-client-mj"}).Error)
	tp := SyncTaskQueryParams{ClientFamily: s.Family}
	require.EqualValues(t, 1, TaskCountAllUserTask(1, tp))
	tasks := TaskGetAllUserTask(1, 0, 1, tp)
	require.Len(t, tasks, 1)
	require.Equal(t, s, *tasks[0].Client)
	require.Equal(t, 1, tasks[0].UserId)
	mp := TaskQueryParams{ClientFamily: s.Family}
	require.EqualValues(t, 1, CountAllUserTask(1, mp))
	mjs := GetAllUserTask(1, 0, 1, mp)
	require.Len(t, mjs, 1)
	require.Equal(t, s, *mjs[0].Client)
	require.Equal(t, 1, mjs[0].UserId)
	require.EqualValues(t, 1, TaskCountAllUserTask(1, SyncTaskQueryParams{ClientFamily: "unrecorded"}))
	require.EqualValues(t, 1, CountAllUserTask(1, TaskQueryParams{ClientFamily: "unrecorded"}))
}
