package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTokenMetadataDoesNotQueryLogs(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.Create(&model.Token{UserId: 7, Name: "visible", Key: "test-key"}).Error)
	for _, tc := range []struct {
		path    string
		handler gin.HandlerFunc
	}{
		{"/api/token/?include_usage=false", GetAllTokens},
		{"/api/token/search?include_usage=false&keyword=visible", SearchTokens},
	} {
		t.Run(tc.path, func(t *testing.T) {
			logQueries := 0
			require.NoError(t, db.Callback().Query().Before("gorm:query").Register("track_logs", func(tx *gorm.DB) {
				if tx.Statement.Table == "logs" {
					logQueries++
				}
			}))
			defer db.Callback().Query().Remove("track_logs")
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Set("id", 7)
			c.Request = httptest.NewRequest("GET", tc.path, nil)
			tc.handler(c)
			var response struct {
				Success bool
				Data    struct{ Items []model.Token }
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.True(t, response.Success)
			require.Len(t, response.Data.Items, 1)
			require.Zero(t, logQueries, "key management must not wait for log aggregates")
		})
	}
}

func TestCursorLogPagesDoNotCountAndKeepUserScope(t *testing.T) {
	for _, admin := range []bool{false, true} {
		t.Run(fmt.Sprint(admin), func(t *testing.T) {
			db := setupTokenControllerTestDB(t)
			require.NoError(t, db.AutoMigrate(&model.Log{}))
			rows := []model.Log{
				{Id: 101, UserId: 7, CreatedAt: 200, ModelName: "a", Other: `{"admin_info":{"secret":true}}`},
				{Id: 102, UserId: 7, CreatedAt: 200, ModelName: "b"},
				{Id: 103, UserId: 7, CreatedAt: 200, ModelName: "c"},
				{Id: 104, UserId: 8, CreatedAt: 200, ModelName: "foreign"},
			}
			require.NoError(t, db.Create(&rows).Error)
			var statements []string
			require.NoError(t, db.Callback().Query().After("gorm:query").Register("track_count", func(tx *gorm.DB) {
				statements = append(statements, tx.Statement.SQL.String())
			}))
			defer db.Callback().Query().Remove("track_count")
			cursor := ""
			var got []string
			for page := 1; page < 5; page++ {
				c, recorder := newAuthenticatedContext(t, http.MethodGet, fmt.Sprintf("/api/log/?pagination=cursor&page_size=2&p=%d&cursor=%s", page, cursor), nil, 7)
				if admin {
					GetAllLogs(c)
				} else {
					GetUserLogs(c)
				}
				var response struct {
					Success bool
					Data    struct {
						Items      []model.Log
						HasMore    bool   `json:"has_more"`
						NextCursor string `json:"next_cursor"`
						Total      int
					}
				}
				require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
				require.True(t, response.Success, recorder.Body.String())
				require.Equal(t, -1, response.Data.Total)
				for _, row := range response.Data.Items {
					got = append(got, row.ModelName)
					if !admin {
						require.Equal(t, 7, row.UserId)
						require.NotContains(t, row.Other, "admin_info")
					}
				}
				if !response.Data.HasMore {
					break
				}
				require.NotEmpty(t, response.Data.NextCursor)
				cursor = response.Data.NextCursor
				// New rows at the front must not shift the next page.
				require.NoError(t, db.Create(&model.Log{UserId: 7, CreatedAt: 201, ModelName: "new"}).Error)
			}
			want := []string{"c", "b", "a"}
			if admin {
				want = append([]string{"foreign"}, want...)
			}
			require.Equal(t, want, got)
			for _, statement := range statements {
				require.NotContains(t, strings.ToLower(statement), "count(")
				require.NotContains(t, strings.ToLower(statement), "offset")
			}
		})
	}
}

func TestTokenUsageBatchChecksOwnershipAndCachesSnapshot(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.UsageLogDailyAggregate{}))
	require.NoError(t, db.Create([]model.Token{
		{Id: 1, UserId: 7, Name: "own", Key: "a"},
		{Id: 2, UserId: 8, Name: "foreign", Key: "b"},
	}).Error)
	require.NoError(t, db.Create(&model.Log{TokenId: 1, UserId: 7, Type: model.LogTypeConsume, Settled: true, CreatedAt: time.Now().Add(-time.Minute).Unix(), Quota: 123}).Error)
	for _, ids := range [][]int{{2}, {1, 2}, {999}} {
		c, rec := newAuthenticatedContext(t, "POST", "/api/token/usage-stats", map[string]any{"ids": ids}, 7)
		GetTokenUsageStats(c)
		require.Equal(t, http.StatusForbidden, rec.Code)
	}
	for i := 0; i < 2; i++ {
		c, rec := newAuthenticatedContext(t, "POST", "/api/token/usage-stats", map[string]any{"ids": []int{1, 1}}, 7)
		GetTokenUsageStats(c)
		var response struct {
			Success bool
			Data    struct {
				Items []struct {
					ID       int
					Lifetime int `json:"lifetime_used_quota"`
				}
				AsOf int64 `json:"as_of"`
			}
		}
		require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &response))
		require.True(t, response.Success, rec.Body.String())
		require.Len(t, response.Data.Items, 1)
		require.Equal(t, 1, response.Data.Items[0].ID)
		require.Equal(t, 123, response.Data.Items[0].Lifetime)
		require.Zero(t, response.Data.AsOf%30)
		if i == 0 {
			require.NoError(t, db.Exec("UPDATE logs SET quota = 999").Error)
		}
	}
	// Ownership must be rechecked even if a previous request filled the cache.
	require.NoError(t, db.Delete(&model.Token{}, 1).Error)
	c, rec := newAuthenticatedContext(t, "POST", "/api/token/usage-stats", map[string]any{"ids": []int{1}}, 7)
	GetTokenUsageStats(c)
	require.Equal(t, http.StatusForbidden, rec.Code)
}
