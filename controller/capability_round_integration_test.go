package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/capabilitytest"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
)

// This is a local wire-level fixture, not evidence of real model performance.
// It uses the actual native adaptor, SQLite ledger, Unix renderer and publisher.
func TestCapabilityLocalRoundWithRealRenderer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	node := os.Getenv("CAPABILITY_TEST_NODE")
	if node == "" {
		t.Skip("set CAPABILITY_TEST_NODE to run local renderer integration")
	}
	renderDir, err := filepath.Abs("../tools/capability-renderer")
	require.NoError(t, err)
	// Unix socket paths on macOS are limited to 104 bytes.
	scratch, err := os.MkdirTemp("/tmp", "iq-round-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(scratch) })
	socket := filepath.Join(scratch, "render.sock")
	cmd := exec.Command(node, "server.mjs")
	cmd.Dir = renderDir
	animationFixture := `<!doctype html><html><body style="margin:0"><svg width="600" height="400"><rect width="600" height="400" fill="white"/><circle cx="60" cy="200" r="30" fill="red"><animate attributeName="cx" from="60" to="540" dur="5s" repeatCount="indefinite"/></circle></svg></body></html>`
	cmd.Env = append(os.Environ(), "RENDERER_SOCKET="+socket, fmt.Sprintf("CAPABILITY_RENDERER_FIXTURE_HASHES=%x", sha256.Sum256([]byte(animationFixture))))
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	ready := false
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		conn, e := net.DialTimeout("unix", socket, 100*time.Millisecond)
		if e == nil {
			_ = conn.Close()
			ready = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.True(t, ready, "renderer did not start")
	t.Setenv("CAPABILITY_RENDERER_SOCKET", socket)
	t.Setenv("CAPABILITY_ARTIFACT_DIR", filepath.Join(scratch, "artifacts"))
	t.Setenv("CAPABILITY_FINGERPRINT_SECRET", "local-only-fixture-secret-32-characters")
	db, err := gorm.Open(sqlite.Open(filepath.Join(scratch, "test.db")), &gorm.Config{})
	require.NoError(t, err)
	sql, err := db.DB()
	require.NoError(t, err)
	sql.SetMaxOpenConns(1)
	oldDB, oldRedis := model.DB, common.RedisEnabled
	model.DB = db
	common.RedisEnabled = false
	t.Cleanup(func() { model.DB = oldDB; common.RedisEnabled = oldRedis; _ = sql.Close() })
	require.NoError(t, model.MigrateCapabilitySchema(db))
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	var generationCalls, judgeCalls atomic.Int32
	parameters := regexp.MustCompile(`但圆形苹果糖(\d+)颗、圆形桃子糖(\d+)颗、星形苹果糖(\d+)颗、星形桃子糖(\d+)颗，西瓜糖(\d+)颗`)
	label := regexp.MustCompile(`JX-[0-9A-F]{4}`)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		prompt := gjson.GetBytes(raw, "messages.0.content").String()
		answer := ""
		if gjson.GetBytes(raw, "model").String() == "vision-fixture" {
			judgeCalls.Add(1)
			if !strings.Contains(string(raw), "data:image/png;base64,") {
				t.Error("judge did not receive rendered image")
			}
			answer = `{"items":[{"status":"pass","evidence":"local fixture"},{"status":"pass","evidence":"local fixture"},{"status":"pass","evidence":"local fixture"},{"status":"pass","evidence":"local fixture"},{"status":"pass","evidence":"local fixture"},{"status":"pass","evidence":"local fixture"}],"aesthetic":3,"confidence":1}`
		} else {
			generationCalls.Add(1)
			switch {
			case strings.Contains(prompt, "此题id为anchor"):
				matches := parameters.FindStringSubmatch(prompt)
				if len(matches) != 6 {
					t.Error("unknown logic prompt")
					w.WriteHeader(500)
					return
				}
				v := make([]int, 5)
				for i := range v {
					v[i], _ = strconv.Atoi(matches[i+1])
				}
				answer = fmt.Sprintf(`{"answers":[{"id":"anchor","value":29},{"id":"random","value":%d}]}`, v[4]+max(v[0], v[2])+max(v[1], v[3])+1)
			case strings.Contains(prompt, "左半区"):
				answer = fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="600" height="400" viewBox="0 0 600 400"><rect x="0" y="0" width="600" height="400" fill="#ffffff"/><circle cx="70" cy="70" r="25" fill="#ef4444"/><circle cx="150" cy="70" r="25" fill="#ef4444"/><circle cx="230" cy="70" r="25" fill="#ef4444"/><rect x="350" y="50" width="50" height="50" fill="#3b82f6"/><rect x="450" y="50" width="50" height="50" fill="#3b82f6"/><text x="30" y="380" font-size="20" fill="#000000">%s</text></svg>`, label.FindString(prompt))
			default:
				answer = animationFixture
			}
		}
		response, _ := common.Marshal(map[string]any{"model": gjson.GetBytes(raw, "model").String(), "choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": answer}, "finish_reason": "stop"}}, "usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30}})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(response)
	}))
	defer upstream.Close()
	base := upstream.URL
	channels := []model.Channel{{Id: 801, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "local-fixture", BaseURL: &base, Group: "A,B", Models: "text-fixture"}, {Id: 802, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "judge-fixture", BaseURL: &base, Group: "A,B", Models: "vision-fixture"}}
	require.NoError(t, db.Create(&channels).Error)
	a, b := "A", "B"
	require.NoError(t, db.Create(&[]model.CapabilityGroupPresentation{{GroupUID: "a", RoutingKey: &a}, {GroupUID: "b", RoutingKey: &b}}).Error)
	require.NoError(t, db.Create(&[]model.Ability{{Group: "A", Model: "text-fixture", ChannelId: 801, Enabled: true}, {Group: "B", Model: "text-fixture", ChannelId: 801, Enabled: true}}).Error)
	config := model.DefaultCapabilityConfig()
	config.Anchor = time.Now().Add(-time.Minute).Unix()
	config.Models = []model.CapabilityProfile{{Model: "text-fixture", Protocol: "chat", MaxTokens: 4096, Enabled: true}}
	config.DailyBudgetMicros = 100000
	config.CallReserveMicros = 100
	config.JudgeChannelID = 802
	config.JudgeModel = "vision-fixture"
	raw, err := common.Marshal(config)
	require.NoError(t, err)
	row, err := model.UpdateCapabilityControl(0, 1, "resume", func(row *model.CapabilityControl) error {
		row.Running = true
		row.ExecutionRevision = 1
		row.Settings = string(raw)
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.CapabilityWorker{ID: 1, Owner: "local-owner", Heartbeat: time.Now().Unix()}).Error)
	firstSlot := time.Now().Unix()
	capabilityExecuteRound(context.Background(), row, config, firstSlot, "local-owner")
	if generationCalls.Load() == 0 {
		var failed []model.CapabilityRun
		_ = db.Find(&failed).Error
		for _, run := range failed {
			t.Logf("run channel=%d model=%s status=%s reason=%s result=%s", run.ChannelID, run.Model, run.Status, run.Reason, run.Result)
		}
	}
	require.EqualValues(t, 3, generationCalls.Load())
	require.EqualValues(t, 1, judgeCalls.Load())
	var attempts []model.CapabilityAttempt
	require.NoError(t, db.Find(&attempts).Error)
	require.Len(t, attempts, 4)
	for _, attempt := range attempts {
		require.Equal(t, "complete", attempt.Status)
		require.NotEmpty(t, attempt.Answer)
		require.NotEmpty(t, attempt.RequestHash)
		require.NotEmpty(t, attempt.Usage)
	}
	var snapshots []model.CapabilitySnapshot
	require.NoError(t, db.Find(&snapshots).Error)
	require.Len(t, snapshots, 2)
	require.Equal(t, snapshots[0].RoundID, snapshots[1].RoundID)
	var firstRun model.CapabilityRun
	require.NoError(t, db.First(&firstRun, "round_id = ? AND channel_id = ?", snapshots[0].RoundID, 801).Error)
	var firstItems []capabilityItem
	require.NoError(t, common.UnmarshalJsonStr(firstRun.Result, &firstItems))
	scene := firstItems[2]
	var firstRound model.CapabilityRound
	require.NoError(t, db.First(&firstRound, "id = ?", firstRun.RoundID).Error)
	var frozenQuestions []capabilitytest.Question
	require.NoError(t, common.UnmarshalJsonStr(firstRound.Questions, &frozenQuestions))
	require.Equal(t, capabilitytest.Generate(firstRound.Seed), frozenQuestions)
	require.Equal(t, frozenQuestions[2], scene.Question)
	require.NotEmpty(t, scene.Question.TemplateID)
	require.NotNil(t, scene.Animation)
	require.Equal(t, 48, scene.Animation.Frames)
	require.Greater(t, scene.Animation.ChangedFrames, 0)
	require.NotEqual(t, scene.Artifact, scene.Animation.Evidence)
	for _, attempt := range attempts {
		if attempt.Kind == "judge" {
			require.Equal(t, scene.Animation.Evidence, attempt.ImageSHA256)
			require.Equal(t, capabilitytest.JudgePrompt(scene.Question), attempt.Prompt)
		}
		if attempt.Kind == "scene" {
			require.Equal(t, scene.Question.Prompt, attempt.Prompt)
		}
	}
	// Optional export of this explicitly synthetic wire fixture for UI QA.
	// No production credentials or model responses are involved.
	if output := os.Getenv("CAPABILITY_TEST_EXPORT"); output != "" {
		require.True(t, filepath.IsAbs(output))
		require.NoError(t, os.MkdirAll(output, 0700))
		poster, e := service.ReadCapabilityPNG(scene.Artifact)
		require.NoError(t, e)
		animation, e := service.ReadCapabilityAnimationAsset(scene.Animation.Artifact, "animation")
		require.NoError(t, e)
		evidence, e := service.ReadCapabilityAnimationAsset(scene.Animation.Evidence, "evidence")
		require.NoError(t, e)
		metadata, e := common.Marshal(scene.Animation)
		require.NoError(t, e)
		for name, data := range map[string][]byte{"poster.png": poster, "animation.png": animation, "evidence.png": evidence, "metadata.json": metadata} {
			require.NoError(t, os.WriteFile(filepath.Join(output, name), data, 0600))
		}
	}
	var budget model.CapabilityBudgetDay
	require.NoError(t, db.First(&budget).Error)
	require.EqualValues(t, 400, budget.ReservedMicros)
	// Re-entering the same scheduled slot never sends a duplicate request.
	capabilityExecuteRound(context.Background(), row, config, firstSlot, "local-owner")
	require.EqualValues(t, 3, generationCalls.Load())
	// Excluding A keeps B's shared target running without disabling channel 801.
	config.Excluded = []model.CapabilityExclusion{{GroupUID: "a"}}
	raw, err = common.Marshal(config)
	require.NoError(t, err)
	row, err = model.UpdateCapabilityControl(row.Revision, 1, "save_config", func(row *model.CapabilityControl) error { row.Settings = string(raw); return nil })
	require.NoError(t, err)
	capabilityExecuteRound(context.Background(), row, config, firstSlot+1, "local-owner")
	require.EqualValues(t, 6, generationCalls.Load())
	require.EqualValues(t, 2, judgeCalls.Load())
	var aSnapshot, bSnapshot model.CapabilitySnapshot
	require.NoError(t, db.Where("group_uid = ?", "a").First(&aSnapshot).Error)
	require.NoError(t, db.Where("group_uid = ?", "b").First(&bSnapshot).Error)
	require.Equal(t, firstSlot, aSnapshot.Slot)
	require.Equal(t, firstSlot+1, bSnapshot.Slot)
	// A new C membership receives only the next round, never A's history.
	c := "C"
	require.NoError(t, db.Create(&model.CapabilityGroupPresentation{GroupUID: "c", RoutingKey: &c}).Error)
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 801).Update("group", "B,C").Error)
	require.NoError(t, db.Create(&model.Ability{Group: "C", Model: "text-fixture", ChannelId: 801, Enabled: true}).Error)
	capabilityExecuteRound(context.Background(), row, config, firstSlot+2, "local-owner")
	require.EqualValues(t, 9, generationCalls.Load())
	require.EqualValues(t, 3, judgeCalls.Load())
	var cSnapshot model.CapabilitySnapshot
	require.NoError(t, db.Where("group_uid = ?", "c").First(&cSnapshot).Error)
	require.Equal(t, firstSlot+2, cSnapshot.Slot)
	var cBindings int64
	require.NoError(t, db.Model(&model.CapabilityBinding{}).Where("group_uid = ?", "c").Count(&cBindings).Error)
	require.EqualValues(t, 1, cBindings)
	// Disabled business channels send no further test calls.
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 801).Update("status", common.ChannelStatusManuallyDisabled).Error)
	capabilityExecuteRound(context.Background(), row, config, firstSlot+3, "local-owner")
	require.EqualValues(t, 9, generationCalls.Load())
	require.EqualValues(t, 3, judgeCalls.Load())
	require.NoError(t, db.Where("group_uid = ?", "c").First(&cSnapshot).Error)
	require.Equal(t, firstSlot+2, cSnapshot.Slot)

	// Renderer outage: keep paid answers and the previous publication. Restart
	// then recovers only rendering/review, without regenerating any answer.
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 801).Update("status", common.ChannelStatusEnabled).Error)
	t.Setenv("CAPABILITY_RENDERER_SOCKET", filepath.Join(scratch, "offline.sock"))
	capabilityExecuteRound(context.Background(), row, config, firstSlot+4, "local-owner")
	require.EqualValues(t, 12, generationCalls.Load())
	require.EqualValues(t, 3, judgeCalls.Load())
	var outageRound model.CapabilityRound
	require.NoError(t, db.First(&outageRound, "slot = ?", firstSlot+4).Error)
	var source model.CapabilityRun
	require.NoError(t, db.First(&source, "round_id = ? AND channel_id = ? AND status = ?", outageRound.ID, 801, "complete").Error)
	originalResult := source.Result
	require.Contains(t, originalResult, "pending_render")
	require.NoError(t, db.First(&cSnapshot, "group_uid = ?", "c").Error)
	require.Equal(t, firstSlot+2, cSnapshot.Slot)
	// Recovery outage adds bounded backoff; no calls are sent to the tested model.
	for i := 0; i < 12; i++ {
		capabilityRecoverOne(context.Background(), row, config, "local-owner")
	}
	var queued []model.CapabilityRecoveryJob
	require.NoError(t, db.Where("run_id = ? AND status = ?", source.ID, "pending").Find(&queued).Error)
	require.Len(t, queued, 2)
	for _, q := range queued {
		require.Greater(t, q.NextAt, time.Now().Unix())
	}
	t.Setenv("CAPABILITY_RENDERER_SOCKET", socket)
	// Custom short intervals must not starve the first pending review forever.
	config.IntervalMinutes = 1
	config.Anchor = time.Now().Add(-time.Second).Unix()
	raw, err = common.Marshal(config)
	require.NoError(t, err)
	row, err = model.UpdateCapabilityControl(row.Revision, 1, "save_config", func(row *model.CapabilityControl) error { row.Settings = string(raw); return nil })
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.CapabilityWorker{}).Where("id = ?", 1).Updates(map[string]any{"owner": "restarted", "heartbeat": time.Now().Unix()}).Error)
	require.NoError(t, model.RecoverCapabilityRuns("restarted"))
	require.NoError(t, db.Model(&model.CapabilityRecoveryJob{}).Where("run_id = ?", source.ID).Update("next_at", 0).Error)
	for i := 0; i < 3; i++ {
		capabilityRecoverOne(context.Background(), row, config, "restarted")
	}
	require.EqualValues(t, 12, generationCalls.Load())
	require.EqualValues(t, 4, judgeCalls.Load())
	require.NoError(t, db.First(&source, "id = ?", source.ID).Error)
	require.Equal(t, originalResult, source.Result)
	require.NoError(t, db.First(&cSnapshot, "group_uid = ?", "c").Error)
	require.Equal(t, firstSlot+2, cSnapshot.Slot)
	var revisions []model.CapabilityEvaluation
	require.NoError(t, db.Where("run_id = ?", source.ID).Order("revision desc").Find(&revisions).Error)
	require.GreaterOrEqual(t, len(revisions), 4)
	graded := map[string]bool{}
	for _, revision := range revisions {
		if strings.Contains(revision.Result, `"status":"graded"`) {
			graded[revision.Kind] = true
		}
	}
	require.True(t, graded["geometry"])
	require.True(t, graded["scene"])
	for i := 0; i < 3; i++ {
		capabilityRecoverOne(context.Background(), row, config, "restarted")
	}
	require.EqualValues(t, 12, generationCalls.Load())
	require.EqualValues(t, 4, judgeCalls.Load())
	// Recovery of the saved generation/unsaved result crash window is local.
	require.NoError(t, db.Model(&model.CapabilityRun{}).Where("id = ?", source.ID).Update("result", "").Error)
	logicJob := model.CapabilityRecoveryJob{RunID: source.ID, Kind: "logic", Owner: "restarted"}
	item, status, _ := capabilityProcessRecovery(context.Background(), row, config, logicJob)
	require.Equal(t, "complete", status)
	require.NotNil(t, item)
	require.Equal(t, "graded", item.Status)
	require.Equal(t, []bool{true, true}, item.Checks)
	require.NoError(t, db.Model(&model.CapabilityRun{}).Where("id = ?", source.ID).Update("result", originalResult).Error)
	var sceneJob model.CapabilityRecoveryJob
	require.NoError(t, db.First(&sceneJob, "run_id = ? AND kind = ?", source.ID, "scene").Error)
	var judgeAttempt model.CapabilityAttempt
	require.NoError(t, db.First(&judgeAttempt, "run_id = ? AND kind = ?", source.ID, "judge").Error)
	require.Len(t, judgeAttempt.ImageSHA256, 64)
	// Never reuse a judge response against another PNG or resend unknown calls.
	for _, scenario := range []struct{ status, hash, reason string }{
		{"outcome_unknown", judgeAttempt.ImageSHA256, "judge_outcome_unknown"},
		{"complete", strings.Repeat("0", 64), "judge_evidence_mismatch"},
	} {
		require.NoError(t, db.Model(&model.CapabilityAttempt{}).Where("id = ?", judgeAttempt.ID).Updates(map[string]any{"status": scenario.status, "image_sha256": scenario.hash}).Error)
		_, status, reason := capabilityProcessRecovery(context.Background(), row, config, sceneJob)
		require.Equal(t, "blocked", status)
		require.Equal(t, scenario.reason, reason)
	}
	require.NoError(t, db.Model(&model.CapabilityAttempt{}).Where("id = ?", judgeAttempt.ID).Updates(map[string]any{"status": judgeAttempt.Status, "image_sha256": judgeAttempt.ImageSHA256}).Error)
	// A temporary read failure at either guard or answer lookup remains retryable.
	for _, table := range []string{"channels", "capability_attempts"} {
		require.NoError(t, db.Callback().Query().Before("gorm:query").Register("fixture:recovery_read_outage", func(tx *gorm.DB) {
			if tx.Statement.Table == table {
				tx.AddError(errors.New("temporary database outage"))
			}
		}))
		_, status, _ := capabilityProcessRecovery(context.Background(), row, config, sceneJob)
		require.Equal(t, "pending", status)
		require.NoError(t, db.Callback().Query().Remove("fixture:recovery_read_outage"))
	}
	require.EqualValues(t, 12, generationCalls.Load())
	require.EqualValues(t, 4, judgeCalls.Load())
}
