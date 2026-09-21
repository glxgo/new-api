package controller

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestClientFamilyHTTPFilterRunsBeforePagination(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:client-http-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.Channel{}))
	oldDB, oldLogs := model.DB, model.LOG_DB
	model.DB, model.LOG_DB = db, db
	t.Cleanup(func() { model.DB, model.LOG_DB = oldDB, oldLogs; sqlDB, _ := db.DB(); _ = sqlDB.Close() })
	for _, input := range []struct {
		user int
		ua   string
	}{{1, "codex_cli_rs/1.0"}, {1, "curl/8.0"}, {2, "codex_cli_rs/1.0"}, {1, "codex_cli_rs/2.0"}} {
		require.NoError(t, db.Create(&model.Log{UserId: input.user, CreatedAt: 100, Type: model.LogTypeConsume, Other: common.MapToJsonStr(map[string]interface{}{"client": common.IdentifyClient(input.ua)})}).Error)
	}
	for _, cursor := range []bool{false, true} {
		for _, admin := range []bool{false, true} {
			url := "/api/log?client_family=codex&page_size=1"
			if cursor {
				url += "&pagination=cursor"
			}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", url, nil)
			c.Set("id", 1)
			if admin {
				GetAllLogs(c)
			} else {
				GetUserLogs(c)
			}
			var result struct {
				Success bool `json:"success"`
				Data    struct {
					Items   []model.Log `json:"items"`
					Total   int64       `json:"total"`
					HasMore bool        `json:"has_more"`
				} `json:"data"`
			}
			require.NoError(t, common.Unmarshal(w.Body.Bytes(), &result))
			require.True(t, result.Success, w.Body.String())
			require.Len(t, result.Data.Items, 1)
			require.Equal(t, "codex", result.Data.Items[0].ClientFamily)
			if !admin {
				require.Equal(t, 1, result.Data.Items[0].UserId)
			}
			if cursor {
				require.True(t, result.Data.HasMore)
			} else if admin {
				require.EqualValues(t, 3, result.Data.Total)
			} else {
				require.EqualValues(t, 2, result.Data.Total)
			}
		}
	}
}
