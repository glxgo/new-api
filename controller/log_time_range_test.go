package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// The legacy log API leaves the time range entirely to the caller. In
// particular, a missing range must not be silently replaced with a rolling
// two-hour window; older users rely on the database query's unbounded
// behavior when no timestamps are supplied.
func TestGetAllLogsKeepsLegacyUnboundedTimeParameters(t *testing.T) {
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
	})

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:legacy-log-range-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	model.DB = db
	model.LOG_DB = db
	if err := db.AutoMigrate(&model.Log{}); err != nil {
		t.Fatalf("migrate logs: %v", err)
	}

	if err := db.Create([]*model.Log{
		{UserId: 1, Username: "legacy-range", Type: model.LogTypeConsume, CreatedAt: 1},
		{UserId: 1, Username: "legacy-range", Type: model.LogTypeConsume, CreatedAt: 2},
	}).Error; err != nil {
		t.Fatalf("seed logs: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/log/?p=1&page_size=20", nil)
	GetAllLogs(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Total int         `json:"total"`
			Items []model.Log `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Success {
		t.Fatalf("legacy log query failed: %s", recorder.Body.String())
	}
	if response.Data.Total != 2 || len(response.Data.Items) != 2 {
		t.Fatalf("missing time range should include all rows, got total=%d items=%d", response.Data.Total, len(response.Data.Items))
	}
}
