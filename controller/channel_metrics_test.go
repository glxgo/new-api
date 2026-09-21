package controller

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChannelMetricsWindow(t *testing.T) {
	now := time.Date(2026, 9, 21, 0, 0, 35, 0, channelMetricsZone)
	start, end, err := channelMetricsWindow("day", "", now)
	require.NoError(t, err)
	require.Equal(t, end, start, "first incomplete minute is not counted")
	start, end, err = channelMetricsWindow("hour", "", now)
	require.NoError(t, err)
	require.EqualValues(t, 3600, end-start)
	require.EqualValues(t, 0, end%60)
	start, end, err = channelMetricsWindow("day", "2026-09-20", now)
	require.NoError(t, err)
	require.EqualValues(t, 86400, end-start)
	require.Equal(t, "2026-09-20 00:00", time.Unix(start, 0).In(channelMetricsZone).Format("2006-01-02 15:04"))
	for _, tc := range []struct{ period, date string }{{"week", ""}, {"day", "2026-09-22"}, {"day", "2026-02-30"}, {"day", "2026-9-20"}} {
		_, _, err := channelMetricsWindow(tc.period, tc.date, now)
		require.Error(t, err)
	}
}

func TestChannelMetricsAdminPaginationAndPrivacy(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.ChannelPerfMetric{}))
	for i := 1; i <= 25; i++ {
		require.NoError(t, db.Create(&model.Channel{Id: i, Name: fmt.Sprintf("channel-%d", i), Key: "NEVER_RETURN_THIS_KEY", Models: "test"}).Error)
	}
	require.NoError(t, db.Create(&model.Channel{Id: 26, Name: "literal%_name", Key: "NEVER_RETURN_THIS_KEY"}).Error)
	r := gin.New()
	r.Use(sessions.Sessions("channel-test", cookie.NewStore([]byte("test-session-key"))))
	r.Use(func(c *gin.Context) {
		if role := c.GetHeader("Test-Role"); role != "" {
			s := sessions.Default(c)
			s.Set("username", "test-admin")
			s.Set("id", 1)
			s.Set("status", common.UserStatusEnabled)
			value := common.RoleCommonUser
			if role == "admin" {
				value = common.RoleAdminUser
			}
			s.Set("role", value)
		}
		c.Next()
	})
	r.GET("/api/channel/metrics", middleware.AdminAuth(), GetChannelMetrics)
	for _, role := range []string{"", "user", "admin"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/channel/metrics?keyword=channel-&p=2&page_size=10", nil)
		req.Header.Set("Test-Role", role)
		req.Header.Set("New-Api-User", "1")
		r.ServeHTTP(w, req)
		var result struct {
			Success bool
			Data    struct {
				Items []adminChannelMetric
				Total int64
			}
		}
		require.NoError(t, common.Unmarshal(w.Body.Bytes(), &result))
		require.Equal(t, role == "admin", result.Success)
		require.NotContains(t, w.Body.String(), "NEVER_RETURN_THIS_KEY")
		require.NotContains(t, w.Body.String(), `"key"`)
		require.NotContains(t, w.Body.String(), "base_url")
		if role == "admin" {
			require.Len(t, result.Data.Items, 10)
			require.EqualValues(t, 25, result.Data.Total)
			require.Equal(t, 15, result.Data.Items[0].Id)
			require.Nil(t, result.Data.Items[0].SuccessRate)
			require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
		}
	}
	for _, query := range []string{"keyword=literal%25_", "keyword=26"} {
		rows, total, err := model.GetChannelMetricPage(t.Context(), map[string]string{"keyword=literal%25_": "literal%_", "keyword=26": "26"}[query], 0, 20)
		require.NoError(t, err)
		require.EqualValues(t, 1, total)
		require.Equal(t, 26, rows[0].Id)
	}
}
