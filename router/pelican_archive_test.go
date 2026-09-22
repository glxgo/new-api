package router

import (
	"bytes"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/pelicanarchive"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
)

// Real production route/middleware registration, isolated storage and synthetic
// sessions. No upstream calls, external credentials or production mutation.
func TestPelicanHTTPVisibilityPermissionsAndEvidence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldDB, oldRedis, oldMaster := model.DB, common.RedisEnabled, common.IsMasterNode
	oldRatio := ratio_setting.GroupRatio2JSONString()
	oldUsable, err := common.Marshal(setting.GetUserUsableGroupsCopy())
	require.NoError(t, err)
	oldPath := common.SQLitePath
	oldSQLite, oldMySQL, oldPostgres := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	t.Setenv("SQL_DSN", "local")
	common.SQLitePath = fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	common.IsMasterNode, common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = false, true, false, false
	require.NoError(t, model.InitDB())
	db := model.DB
	sql, err := db.DB()
	require.NoError(t, err)
	sql.SetMaxOpenConns(1)
	model.DB, common.RedisEnabled, common.IsMasterNode = db, false, true
	t.Cleanup(func() {
		common.SQLitePath = oldPath
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = oldSQLite, oldMySQL, oldPostgres
		model.DB, common.RedisEnabled, common.IsMasterNode = oldDB, oldRedis, oldMaster
		_ = ratio_setting.UpdateGroupRatioByJSONString(oldRatio)
		_ = setting.UpdateUserUsableGroupsByJSONString(string(oldUsable))
		_ = sql.Close()
	})
	require.NoError(t, model.MigratePelicanSchema(db))
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Ability{}, &model.UserSubscription{}, &model.UserVirtualMembership{}))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"public":1.5,"private":2}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"public":"Public"}`))
	require.NoError(t, db.Create(&model.User{Id: 71, Username: "fixture", Group: "public", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, model.ReconcileCapabilityGroups())
	var pub, priv model.CapabilityGroupPresentation
	require.NoError(t, db.Where("routing_key = ?", "public").First(&pub).Error)
	require.NoError(t, db.Where("routing_key = ?", "private").First(&priv).Error)
	require.NoError(t, db.Create(&model.Channel{Id: 3, Key: "fixture-key-never-disclose", Name: "internal-provider", Group: "public", Models: "m", Status: common.ChannelStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.Ability{ChannelId: 3, Group: "public", Model: "m", Enabled: true}).Error)
	// Ordinary business models without external associations must not occupy
	// the test model selector or hide the actual available test by sorting first.
	require.NoError(t, db.Create(&model.Channel{Id: 4, Key: "unused", Group: "public", Models: "aaa-untested", Status: common.ChannelStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.Ability{ChannelId: 4, Group: "public", Model: "aaa-untested", Enabled: true}).Error)
	control, err := model.UpdatePelicanControl(db, 0, 71, "resume", func(_ *gorm.DB, c *model.PelicanControl) error { c.SyncEnabled = true; c.Visible = true; return nil })
	require.NoError(t, err)
	now := time.Now().UTC()
	snapshot := pelicanarchive.Snapshot{SourceID: "fixture", CapturedAt: now.Format(time.RFC3339Nano), Config: pelicanarchive.Config{Prompt: "fixture question", PromptHash: pelicanarchive.PromptHash("fixture question"), ExpectedAnswer: 21}, Targets: []pelicanarchive.Target{{ProviderID: "internal-provider-id", Name: "internal-provider", Model: "m", Enabled: 1}}}
	for i := int64(1); i <= 2; i++ {
		snapshot.Runs = append(snapshot.Runs, pelicanarchive.Run{ID: i, ProviderID: "internal-provider-id", Name: "internal-provider", Model: "m", Grade: "wrong", Expected: 21, Attempts: 1, CreatedAt: now.Format(time.RFC3339Nano), PromptHash: snapshot.Config.PromptHash, SVG: `<svg xmlns="http://www.w3.org/2000/svg"><text>21</text></svg>`, RawText: "private-raw-response", Error: "private-diagnostic"})
	}
	_, err = model.ImportPelicanSnapshot(db, snapshot, control.Revision)
	require.NoError(t, err)
	targetID := model.PelicanTargetID("fixture", "internal-provider-id", "m")
	require.NoError(t, model.SavePelicanMapping(db, targetID, 3, "m", false))
	var records []model.PelicanRecord
	require.NoError(t, db.Order("external_id").Find(&records).Error)
	engine := gin.New()
	engine.Use(sessions.Sessions("test-session", cookie.NewStore([]byte("pelican-fixture-cookie-key"))))
	engine.Use(func(c *gin.Context) { common.SetContextKey(c, constant.ContextKeyAuditLogged, true); c.Next() })
	engine.GET("/fixture/:role", func(c *gin.Context) {
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
	request := func(role, method, path string, body any) *httptest.ResponseRecorder {
		raw, err := common.Marshal(body)
		require.NoError(t, err)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		if role != "" {
			login := httptest.NewRecorder()
			engine.ServeHTTP(login, httptest.NewRequest("GET", "/fixture/"+role, nil))
			for _, c := range login.Result().Cookies() {
				req.AddCookie(c)
			}
			req.Header.Set("New-Api-User", "71")
		}
		out := httptest.NewRecorder()
		engine.ServeHTTP(out, req)
		return out
	}
	base := "/api/pelican-archive"
	require.Equal(t, 404, request("root", "GET", "/api/capability/groups", nil).Code)
	require.Equal(t, 401, request("", "GET", base+"/groups", nil).Code)
	require.False(t, gjson.Get(request("user", "GET", base+"/admin/control", nil).Body.String(), "success").Bool())
	require.True(t, gjson.Get(request("admin", "GET", base+"/admin/control", nil).Body.String(), "success").Bool())
	adminReport := request("admin", "GET", base+"/admin/control", nil).Body.String()
	require.Equal(t, targetID, gjson.Get(adminReport, "data.mapping_reports.0.target_id").String())
	require.Equal(t, pub.GroupUID, gjson.Get(adminReport, "data.mapping_reports.0.groups.0.group_uid").String())
	require.True(t, gjson.Get(adminReport, "data.mapping_reports.0.groups.0.eligible").Bool())
	require.NotContains(t, adminReport, "fixture-key-never-disclose")
	for _, role := range []string{"user", "admin"} {
		require.False(t, gjson.Get(request(role, "PUT", base+"/admin/control", map[string]any{"revision": 1, "action": "pause"}).Body.String(), "success").Bool())
		require.False(t, gjson.Get(request(role, "POST", base+"/admin/sync", nil).Body.String(), "success").Bool())
	}
	update := func(action string, payload map[string]any) *httptest.ResponseRecorder {
		c, err := model.GetPelicanControl(db)
		require.NoError(t, err)
		if payload == nil {
			payload = map[string]any{}
		}
		payload["revision"] = c.Revision
		payload["action"] = action
		return request("root", "PUT", base+"/admin/control", payload)
	}
	v := model.DefaultCapabilityPresentation()
	privateLabel := "private-name-secret"
	v.Groups[priv.GroupUID] = model.CapabilityGroupOverride{Name: &privateLabel, DescriptionMode: "inherit"}
	v.Order = []string{priv.GroupUID, pub.GroupUID}
	require.Equal(t, 200, update("presentation", map[string]any{"presentation": v}).Code)
	out := request("user", "GET", base+"/groups", nil)
	require.Len(t, gjson.Get(out.Body.String(), "data").Array(), 1)
	require.Len(t, gjson.Get(out.Body.String(), "data.0.models").Array(), 1)
	require.Equal(t, "m", gjson.Get(out.Body.String(), "data.0.models.0").String())
	require.NotContains(t, out.Body.String(), privateLabel)
	require.NotContains(t, out.Body.String(), priv.GroupUID)
	require.Equal(t, 404, request("user", "GET", base+"/groups/"+priv.GroupUID+"/results?model=m", nil).Code)
	out = request("user", "GET", base+"/groups/"+pub.GroupUID+"/results?model=m", nil)
	require.Len(t, gjson.Get(out.Body.String(), "data.gallery").Array(), 1)
	require.Len(t, gjson.Get(out.Body.String(), "data.history").Array(), 2)
	path := base + "/records/" + records[1].ID
	out = request("user", "GET", path, nil)
	require.Equal(t, 200, out.Code)
	require.EqualValues(t, 21, gjson.Get(out.Body.String(), "data.expected_answer").Int())
	require.Equal(t, "m", gjson.Get(out.Body.String(), "data.model").String())
	for _, secret := range []string{"internal-provider", "private-raw-response", "private-diagnostic", "fixture-key-never-disclose", "source_record"} {
		require.NotContains(t, out.Body.String(), secret)
	}
	out = request("user", "GET", path+"/artwork", nil)
	require.Equal(t, 200, out.Code)
	require.Contains(t, out.Header().Get("Content-Security-Policy"), "sandbox")
	require.Equal(t, "no-store", out.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", out.Header().Get("X-Content-Type-Options"))
	require.Equal(t, 404, request("user", "GET", path+"/arbitrary", nil).Code)
	v.ShowHistory = false
	require.Equal(t, 200, update("presentation", map[string]any{"presentation": v}).Code)
	require.Equal(t, 404, request("user", "GET", base+"/records/"+records[0].ID, nil).Code)
	require.Equal(t, 200, request("user", "GET", path, nil).Code)
	require.Equal(t, 200, update("hide_and_pause", nil).Code)
	c, err := model.GetPelicanControl(db)
	require.NoError(t, err)
	require.False(t, c.SyncEnabled)
	require.Equal(t, 404, request("user", "GET", path, nil).Code)
	require.Equal(t, 404, request("user", "GET", path+"/artwork", nil).Code)
	require.Equal(t, 200, request("admin", "GET", base+"/admin/records/"+records[1].ID, nil).Code)
	require.True(t, gjson.Get(request("admin", "GET", base+"/admin/preview", nil).Body.String(), "visible").Bool())
	require.Equal(t, 200, update("show", nil).Code)
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 3).Update("status", common.ChannelStatusManuallyDisabled).Error)
	require.NoError(t, db.Model(&model.Ability{}).Where("channel_id = ?", 3).Update("enabled", false).Error)
	require.Equal(t, 200, request("user", "GET", path+"/artwork", nil).Code)
	require.Equal(t, 200, request("user", "GET", path, nil).Code)
	out = request("user", "GET", base+"/groups", nil)
	require.Equal(t, "m", gjson.Get(out.Body.String(), "data.0.models.0").String())
	out = request("admin", "GET", base+"/admin/control", nil)
	require.True(t, gjson.Get(out.Body.String(), "data.mapping_reports.0.channel_disabled").Bool())
	require.True(t, gjson.Get(out.Body.String(), "data.mapping_reports.0.groups.0.eligible").Bool())
	require.NoError(t, model.SavePelicanMapping(db, targetID, 3, "m", true))
	out = request("user", "GET", base+"/groups", nil)
	require.Empty(t, gjson.Get(out.Body.String(), "data.0.models").Array())
	require.NoError(t, model.SavePelicanMapping(db, targetID, 3, "m", false))
	// Revoking the real membership still revokes both listing and direct links.
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 3).Update("group", "private").Error)
	require.Equal(t, 404, request("user", "GET", path+"/artwork", nil).Code)
	out = request("user", "GET", base+"/groups", nil)
	require.Empty(t, gjson.Get(out.Body.String(), "data.0.models").Array())
	common.IsMasterNode = false
	require.False(t, gjson.Get(update("resume", nil).Body.String(), "success").Bool())
	require.False(t, gjson.Get(request("root", "POST", base+"/admin/sync", nil).Body.String(), "success").Bool())
}
