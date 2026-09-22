package router

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/png"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// Retain regression coverage of the retired local executor using test-only
// routes. Production no longer registers these endpoints or starts its worker.
// Exercise the handlers and auth middleware against an
// isolated database. Only the session issuer is a test fixture; no upstream
// provider, production data or production credentials are used.
func TestCapabilityHTTPControlAndPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldDB, oldRedis := model.DB, common.RedisEnabled
	oldPath, oldMaster := common.SQLitePath, common.IsMasterNode
	oldSQLite, oldMySQL, oldPostgres := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	t.Setenv("SQL_DSN", "local")
	common.SQLitePath = fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	common.IsMasterNode, common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = false, true, false, false
	require.NoError(t, model.InitDB())
	db := model.DB
	sql, err := db.DB()
	require.NoError(t, err)
	sql.SetMaxOpenConns(1)
	oldRatio := ratio_setting.GroupRatio2JSONString()
	oldUsable, err := common.Marshal(setting.GetUserUsableGroupsCopy())
	require.NoError(t, err)
	model.DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = oldDB
		common.RedisEnabled = oldRedis
		common.SQLitePath, common.IsMasterNode = oldPath, oldMaster
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = oldSQLite, oldMySQL, oldPostgres
		_ = ratio_setting.UpdateGroupRatioByJSONString(oldRatio)
		_ = setting.UpdateUserUsableGroupsByJSONString(string(oldUsable))
		_ = sql.Close()
	})
	require.NoError(t, model.MigrateCapabilitySchema(db))
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSubscription{}, &model.UserVirtualMembership{}, &model.Channel{}, &model.Ability{}))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"public":0,"private":2}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"public":"Public group"}`))
	require.NoError(t, db.Create(&model.User{Id: 71, Username: "fixture", Group: "public", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, model.ReconcileCapabilityGroups())
	var public, private model.CapabilityGroupPresentation
	require.NoError(t, db.Where("routing_key = ?", "public").First(&public).Error)
	require.NoError(t, db.Where("routing_key = ?", "private").First(&private).Error)
	engine := gin.New()
	engine.Use(sessions.Sessions("test-session", cookie.NewStore([]byte("capability-local-session-fixture"))))
	engine.Use(func(c *gin.Context) {
		// Suppress unrelated asynchronous log writes; capability events are
		// still persisted and asserted below.
		common.SetContextKey(c, constant.ContextKeyAuditLogged, true)
		c.Next()
	})
	engine.GET("/fixture/session/:role", func(c *gin.Context) {
		role := common.RoleCommonUser
		if c.Param("role") == "admin" {
			role = common.RoleAdminUser
		}
		if c.Param("role") == "root" {
			role = common.RoleRootUser
		}
		s := sessions.Default(c)
		s.Set("username", "fixture")
		s.Set("id", 71)
		s.Set("role", role)
		s.Set("status", common.UserStatusEnabled)
		s.Set("group", "public")
		require.NoError(t, s.Save())
		c.Status(204)
	})
	SetApiRouter(engine)
	legacy := engine.Group("/api/capability")
	legacy.Use(func(c *gin.Context) { c.Header("Cache-Control", "no-store"); c.Next() })
	legacy.GET("/groups", middleware.UserAuth(), controller.GetCapabilityGroups)
	legacy.GET("/groups/:group_uid/results", middleware.UserAuth(), controller.GetCapabilityResults)
	legacy.GET("/groups/:group_uid/timeline", middleware.UserAuth(), controller.GetCapabilityTimeline)
	legacy.GET("/runs/:id", middleware.UserAuth(), controller.GetCapabilityPublicRun)
	legacy.GET("/runs/:id/artifacts/:kind", middleware.UserAuth(), controller.GetCapabilityPublicRun)
	legacyAdmin := legacy.Group("/admin", middleware.AdminAuth())
	legacyAdmin.GET("/control", controller.GetCapabilityAdminControl)
	legacyAdmin.PUT("/control", middleware.RootAuth(), controller.UpdateCapabilityAdminControl)
	legacyAdmin.GET("/runs/:id", controller.GetCapabilityAdminRun)
	legacyAdmin.GET("/runs/:id/evaluations/:evaluation_id/artifact", controller.GetCapabilityEvaluationArtifact)
	request := func(role, method, path string, body any) *httptest.ResponseRecorder {
		var raw []byte
		if body != nil {
			raw, err = common.Marshal(body)
			require.NoError(t, err)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		if role != "" {
			login := httptest.NewRecorder()
			engine.ServeHTTP(login, httptest.NewRequest("GET", "/fixture/session/"+role, nil))
			for _, cookie := range login.Result().Cookies() {
				req.AddCookie(cookie)
			}
			req.Header.Set("New-Api-User", "71")
		}
		out := httptest.NewRecorder()
		engine.ServeHTTP(out, req)
		return out
	}
	require.Equal(t, 401, request("", "GET", "/api/capability/groups", nil).Code)
	for _, role := range []string{"user", "admin"} {
		out := request(role, "PUT", "/api/capability/admin/control", map[string]any{"revision": 0, "action": "show_only"})
		require.False(t, gjson.Get(out.Body.String(), "success").Bool())
	}
	require.False(t, gjson.Get(request("user", "GET", "/api/capability/admin/control", nil).Body.String(), "success").Bool())
	require.True(t, gjson.Get(request("admin", "GET", "/api/capability/admin/control", nil).Body.String(), "success").Bool())
	update := func(revision int64, action string, payload map[string]any) *httptest.ResponseRecorder {
		if payload == nil {
			payload = map[string]any{}
		}
		payload["revision"], payload["action"] = revision, action
		return request("root", "PUT", "/api/capability/admin/control", payload)
	}
	out := update(0, "show_only", nil)
	require.Equal(t, 200, out.Code, out.Body.String())
	require.False(t, gjson.Get(out.Body.String(), "data.running").Bool())
	require.Equal(t, 409, update(0, "stop", nil).Code)
	view := model.DefaultCapabilityPresentation()
	name := "精选线路"
	view.Groups[public.GroupUID] = model.CapabilityGroupOverride{Name: &name, DescriptionMode: "custom", Description: "本地验证描述"}
	view.Copy["page_title"] = "本地验证标题"
	view.Order = []string{private.GroupUID, public.GroupUID}
	out = update(1, "publish", map[string]any{"presentation": view})
	require.Equal(t, 200, out.Code, out.Body.String())
	out = request("user", "GET", "/api/capability/groups", nil)
	require.Equal(t, 200, out.Code, out.Body.String())
	require.Equal(t, "no-store", out.Header().Get("Cache-Control"))
	require.Len(t, gjson.Get(out.Body.String(), "data").Array(), 1)
	require.Equal(t, name, gjson.Get(out.Body.String(), "data.0.display_name").String())
	require.Equal(t, float64(0), gjson.Get(out.Body.String(), "data.0.ratio").Float())
	require.Equal(t, "本地验证标题", gjson.Get(out.Body.String(), "copy.page_title").String())
	require.Equal(t, 404, request("user", "GET", "/api/capability/groups/"+private.GroupUID+"/results", nil).Code)
	// Business ratio changes are inherited without overwriting the alias.
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"public":1.25,"private":2}`))
	out = request("user", "GET", "/api/capability/groups", nil)
	require.Equal(t, name, gjson.Get(out.Body.String(), "data.0.display_name").String())
	require.Equal(t, 1.25, gjson.Get(out.Body.String(), "data.0.ratio").Float())
	// A hidden page is unavailable through direct results and detail URLs.
	require.Equal(t, 200, update(2, "hide_and_stop", nil).Code)
	out = request("user", "GET", "/api/capability/groups", nil)
	require.False(t, gjson.Get(out.Body.String(), "visible").Bool())
	require.Equal(t, 404, request("user", "GET", "/api/capability/groups/"+public.GroupUID+"/results", nil).Code)
	out = update(3, "show_only", nil)
	require.False(t, gjson.Get(out.Body.String(), "data.running").Bool())
	var events []model.CapabilityEvent
	require.NoError(t, db.Order("id").Find(&events).Error)
	require.Len(t, events, 4)
	require.Contains(t, events[1].After, "精选线路")
	// History hiding applies to direct evidence URLs, not just the button.
	round := model.CapabilityRound{ID: "finished", Slot: time.Now().Unix(), Status: "complete"}
	require.NoError(t, db.Create(&round).Error)
	for _, id := range []string{"old", "current"} {
		require.NoError(t, db.Create(&model.CapabilityRun{ID: id, Identity: id, RoundID: round.ID, Model: "m", Status: "complete", Result: "[]"}).Error)
		require.NoError(t, db.Create(&model.CapabilityBinding{PublicID: id, RunID: id, GroupUID: public.GroupUID, DispatchedAt: 1}).Error)
	}
	require.NoError(t, db.Create(&model.CapabilitySnapshot{ID: "snapshot", GroupUID: public.GroupUID, Model: "m", RoundID: round.ID, Bindings: `["current"]`}).Error)
	// Timeline cells resolve to actual evidence and ignore unfinished rounds.
	require.NoError(t, db.Model(&model.CapabilityRun{}).Where("id = ?", "old").Update("result", `[{"kind":"logic","status":"graded","checks":[true,true],"question":{"prompt":"original logic question"}}]`).Error)
	require.NoError(t, db.Model(&model.CapabilityRun{}).Where("id = ?", "current").Update("result", `[{"kind":"logic","status":"graded","checks":[false,false],"question":{"prompt":"original logic question"}}]`).Error)
	running := model.CapabilityRound{ID: "running", Slot: round.Slot + 60, Status: "running"}
	require.NoError(t, db.Create(&running).Error)
	require.NoError(t, db.Create(&model.CapabilityRun{ID: "pending", Identity: "pending", RoundID: running.ID, Model: "m", Status: "complete", Result: `[]`}).Error)
	require.NoError(t, db.Create(&model.CapabilityBinding{PublicID: "pending", RunID: "pending", GroupUID: public.GroupUID, DispatchedAt: 1}).Error)
	out = request("user", "GET", "/api/capability/groups/"+public.GroupUID+"/timeline?model=m", nil)
	require.Equal(t, 200, out.Code, out.Body.String())
	require.Len(t, gjson.Get(out.Body.String(), "data").Array(), 1)
	require.EqualValues(t, 2, gjson.Get(out.Body.String(), "data.0.items.0.score").Int())
	require.Equal(t, "old", gjson.Get(out.Body.String(), "data.0.items.0.public_id").String())
	require.EqualValues(t, 2, gjson.Get(out.Body.String(), "data.0.samples").Int())
	require.Equal(t, "original logic question", gjson.Get(request("user", "GET", "/api/capability/runs/old", nil).Body.String(), "data.items.0.question.prompt").String())
	require.NoError(t, db.Create(&model.CapabilityAttempt{ID: "metric", RunID: "old", Kind: "logic", Status: "complete", StartedAt: 100, CompletedAt: 142, UsageSource: "provider_reported", Usage: `{"input_tokens":123,"output_tokens":0,"private":"internal"}`, Endpoint: "private-host", KeySlot: 7}).Error)
	out = request("user", "GET", "/api/capability/runs/old", nil)
	require.Equal(t, 200, out.Code)
	require.EqualValues(t, 42, gjson.Get(out.Body.String(), "data.metrics.logic.duration_seconds").Int())
	require.True(t, gjson.Get(out.Body.String(), "data.metrics.logic.output_tokens").Exists())
	require.Zero(t, gjson.Get(out.Body.String(), "data.metrics.logic.output_tokens").Int())
	require.NotContains(t, out.Body.String(), "private-host")
	require.NotContains(t, out.Body.String(), "internal")
	require.NotContains(t, out.Body.String(), "key_slot")
	require.Equal(t, 404, request("user", "GET", "/api/capability/groups/"+private.GroupUID+"/timeline?model=m", nil).Code)
	view.ShowHistory = false
	require.Equal(t, 200, update(4, "publish", map[string]any{"presentation": view}).Code)
	require.Equal(t, 404, request("user", "GET", "/api/capability/runs/old", nil).Code)
	require.Equal(t, 200, request("user", "GET", "/api/capability/runs/current", nil).Code)
	require.Equal(t, 404, request("user", "GET", "/api/capability/groups/"+public.GroupUID+"/timeline?model=m", nil).Code)
	// Recovery evidence remains administrator-only, including PNG direct links.
	t.Setenv("CAPABILITY_ARTIFACT_DIR", t.TempDir())
	var imageData bytes.Buffer
	require.NoError(t, png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 600, 400))))
	hash, err := service.StoreCapabilityPNG(imageData.Bytes())
	require.NoError(t, err)
	var evidenceData bytes.Buffer
	require.NoError(t, png.Encode(&evidenceData, image.NewRGBA(image.Rect(0, 0, 1800, 800))))
	evidenceHash := fmt.Sprintf("%x", sha256.Sum256(evidenceData.Bytes()))
	require.NoError(t, os.WriteFile(filepath.Join(service.CapabilityArtifactRoot(), hash+".animation.png"), imageData.Bytes(), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(service.CapabilityArtifactRoot(), evidenceHash+".evidence.png"), evidenceData.Bytes(), 0600))
	item, err := common.Marshal(map[string]any{"kind": "scene", "status": "graded", "artifact": hash, "animation": map[string]any{"artifact": hash, "evidence": evidenceHash, "frames": 48, "step_ms": 100}, "question": map[string]string{"prompt": "original scene"}})
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.CapabilityRun{}).Where("id = ?", "current").Update("result", "["+string(item)+"]").Error)
	for _, variant := range []string{"animation", "evidence"} {
		publicPath := "/api/capability/runs/current/artifacts/scene?view=" + variant
		require.Equal(t, 401, request("", "GET", publicPath, nil).Code)
		require.Equal(t, 200, request("user", "GET", publicPath, nil).Code)
		require.Equal(t, 404, request("user", "GET", "/api/capability/runs/old/artifacts/scene?view="+variant, nil).Code)
	}
	out = request("user", "GET", "/api/capability/runs/current", nil)
	require.Equal(t, "/api/capability/runs/current/artifacts/scene?view=animation", gjson.Get(out.Body.String(), "data.items.0.animation.artifact").String())
	require.NoError(t, db.Create(&model.CapabilityEvaluation{ID: "evaluation", RunID: "old", RecoveryID: "recovery", Revision: 1, Result: string(item)}).Error)
	path := "/api/capability/admin/runs/old/evaluations/evaluation/artifact"
	require.Equal(t, 401, request("", "GET", path, nil).Code)
	require.NotEqual(t, "image/png", request("user", "GET", path, nil).Header().Get("Content-Type"))
	require.False(t, gjson.Get(request("user", "GET", "/api/capability/admin/runs/old", nil).Body.String(), "success").Bool())
	// Hiding the public page must not hide administrator audit records.
	require.Equal(t, 200, update(5, "hide_and_stop", nil).Code)
	for _, variant := range []string{"animation", "evidence"} {
		require.Equal(t, 404, request("user", "GET", "/api/capability/runs/current/artifacts/scene?view="+variant, nil).Code)
		require.Equal(t, 200, request("admin", "GET", path+"?view="+variant, nil).Code)
		require.NotEqual(t, "image/png", request("user", "GET", path+"?view="+variant, nil).Header().Get("Content-Type"))
		require.Equal(t, 404, request("admin", "GET", "/api/capability/admin/runs/current/evaluations/evaluation/artifact?view="+variant, nil).Code)
	}
	for _, role := range []string{"admin", "root"} {
		out = request(role, "GET", path, nil)
		require.Equal(t, 200, out.Code)
		require.Equal(t, imageData.Bytes(), out.Body.Bytes())
		require.Equal(t, "no-store", out.Header().Get("Cache-Control"))
		require.Equal(t, "nosniff", out.Header().Get("X-Content-Type-Options"))
		require.Contains(t, out.Header().Get("Content-Security-Policy"), "sandbox")
		out = request(role, "GET", "/api/capability/admin/runs/old", nil)
		require.Equal(t, path, gjson.Get(out.Body.String(), "data.evaluations.0.item.artifact").String())
		require.Equal(t, "original logic question", gjson.Get(out.Body.String(), "data.items.0.question.prompt").String())
	}
	require.Equal(t, 404, request("admin", "GET", "/api/capability/admin/runs/current/evaluations/evaluation/artifact", nil).Code)
	require.Equal(t, 404, request("admin", "GET", "/api/capability/admin/runs/old/evaluations/missing/artifact", nil).Code)
}
