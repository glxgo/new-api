// Local-only acceptance harness: real archive data, isolated SQLite and clearly
// labelled local channel/group associations. Never built into the server.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/pelicanarchive"
	"github.com/QuantumNous/new-api/router"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func main() {
	archive := flag.String("archive", "", "private local archive snapshot")
	source := flag.String("source", "bench-primary", "expected source identity")
	topology := flag.String("topology", "", "private read-only production topology snapshot")
	mappings := flag.String("mappings", "", "confirmed mappings; requires --topology")
	userGroup := flag.String("user-group", "default", "local test account group; no production users are copied")
	checkOnly := flag.Bool("check-only", false, "validate and import twice without starting a listener")
	bootstrap := flag.Bool("bootstrap", false, "exercise production initializer in this isolated local database")
	flag.Parse()
	if *archive == "" {
		panic("--archive is required")
	}
	if (*topology == "") != (*mappings == "") {
		panic("--topology and --mappings must be supplied together")
	}
	if *bootstrap && *topology == "" {
		panic("--bootstrap requires --topology and --mappings")
	}
	// In-memory database cannot accidentally open production via SQL_DSN.
	must(os.Setenv("SQL_DSN", "local"))
	common.SQLitePath = "file:pelican-preview?mode=memory&cache=shared"
	common.RedisEnabled = false
	common.IsMasterNode = false
	common.UsingSQLite = true
	must(model.InitDB()) // also initializes dialect-specific column names
	db := model.DB
	pool, err := db.DB()
	must(err)
	pool.SetMaxOpenConns(1)
	pool.SetMaxIdleConns(1)
	pool.SetConnMaxLifetime(0) // keep the isolated in-memory database alive
	common.IsMasterNode = true
	must(model.MigratePelicanSchema(db))
	must(db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Ability{}, &model.Option{}, &model.UserSubscription{}, &model.UserVirtualMembership{}))
	mode, capturedAt := "synthetic", ""
	if *topology != "" {
		captured, err := loadPreviewTopology(db, *topology)
		must(err)
		mode, capturedAt = "production_snapshot", captured.Format(time.RFC3339)
	} else {
		must(ratio_setting.UpdateGroupRatioByJSONString(`{"验收映射 A":1,"验收映射 B":2}`))
		must(setting.UpdateUserUsableGroupsByJSONString(`{"验收映射 A":"本地关联演示，非生产分组","验收映射 B":"共享同一份真实外部记录"}`))
		*userGroup = "验收映射 A"
	}
	must(db.Create(&model.User{Id: 98701, Username: "local_archive_acceptance", Group: *userGroup, Status: common.UserStatusEnabled, Role: common.RoleRootUser}).Error)
	bootstrappedMappings := 0
	if *bootstrap {
		f, err := os.Open(*archive)
		must(err)
		snapshot, err := pelicanarchive.Decode(f, *source, time.Now())
		must(f.Close())
		must(err)
		var bindings service.PelicanBindings
		must(readPreviewJSON(*mappings, &bindings))
		plan, err := service.BootstrapPelicanArchive(db, snapshot, bindings, 98701, false, "")
		must(err)
		applied, err := service.BootstrapPelicanArchive(db, snapshot, bindings, 98701, true, plan.PlanHash)
		must(err)
		bootstrappedMappings = applied.Mappings
		control, err := model.GetPelicanControl(db)
		must(err)
		if control.Visible || control.SyncEnabled {
			panic("bootstrap must leave page hidden and synchronization paused")
		}
		raw, err := common.Marshal(applied)
		must(err)
		fmt.Printf("Local production initializer: %s\n", raw)
		if *checkOnly {
			return
		}
	} else {
		must(model.ReconcileCapabilityGroups())
	}
	control, err := model.GetPelicanControl(db)
	must(err)
	_, err = model.UpdatePelicanControl(db, control.Revision, 98701, "resume", func(_ *gorm.DB, c *model.PelicanControl) error { c.SyncEnabled = true; c.Visible = true; return nil })
	must(err)
	must(os.Setenv("PELICAN_ARCHIVE_FILE", *archive))
	must(os.Setenv("PELICAN_SOURCE_ID", *source))
	n, err := service.SyncPelicanArchive(context.Background())
	must(err)
	var targets []model.PelicanTarget
	must(db.Find(&targets).Error)
	mapped := 0
	if *bootstrap {
		mapped = bootstrappedMappings
	} else if *topology != "" {
		mapped, err = applyPreviewMappings(db, *mappings, *source)
		must(err)
	} else {
		for i, t := range targets {
			id := 10001 + i
			must(db.Create(&model.Channel{Id: id, Name: fmt.Sprintf("本地映射 %d", i+1), Key: "local-placeholder-not-a-credential", Models: t.ModelName, Group: "验收映射 A,验收映射 B", Status: common.ChannelStatusEnabled}).Error)
			for _, g := range []string{"验收映射 A", "验收映射 B"} {
				must(db.Create(&model.Ability{Group: g, Model: t.ModelName, ChannelId: id, Enabled: true}).Error)
			}
			must(model.SavePelicanMapping(db, t.ID, id, t.ModelName, false))
			mapped++
		}
	}
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(sessions.Sessions("pelican-local", cookie.NewStore([]byte(uuid.NewString()))))
	engine.Use(func(c *gin.Context) { common.SetContextKey(c, constant.ContextKeyAuditLogged, true); c.Next() })
	engine.GET("/api/local-acceptance-session", func(c *gin.Context) {
		if origin := c.GetHeader("Origin"); origin != "" && !strings.HasPrefix(origin, "http://127.0.0.1:") {
			c.Status(403)
			return
		}
		session := sessions.Default(c)
		session.Set("username", "local_archive_acceptance")
		session.Set("id", 98701)
		session.Set("role", common.RoleRootUser)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", *userGroup)
		must(session.Save())
		c.JSON(200, gin.H{"id": 98701, "username": "local_archive_acceptance", "role": common.RoleRootUser, "preview_mode": mode, "topology_captured_at": capturedAt, "local_user_group": *userGroup})
	})
	router.SetApiRouter(engine)
	var previewable int64
	must(db.Model(&model.PelicanRecord{}).Where("preview = ?", true).Count(&previewable).Error)
	fmt.Printf("Local acceptance: %d imported records, %d previewable; %s, %d mappings; 127.0.0.1:4198\n", n, previewable, mode, mapped)
	if *checkOnly {
		repeated, err := service.SyncPelicanArchive(context.Background())
		must(err)
		if repeated != 0 {
			panic("duplicate import added records")
		}
		fmt.Println("Repeated import: 0 new records; no listener started")
		return
	}
	// Exercise the real importer against the local private snapshot. This never
	// invokes the external test executor; a separate read-only pull refreshes it.
	service.StartPelicanArchiveWorker()
	must(http.ListenAndServe("127.0.0.1:4198", engine))
}
