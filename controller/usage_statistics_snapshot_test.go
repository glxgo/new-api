package controller

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUsageStatisticsExplicitSnapshotKeepsQueryAndResponseAligned(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.UsageLogDailyAggregate{}))
	now := time.Now().Unix()
	require.NoError(t, db.Create([]model.Log{
		{UserId: 7, Type: model.LogTypeConsume, CreatedAt: now - 120, Quota: 700},
		{UserId: 8, Type: model.LogTypeConsume, CreatedAt: now - 120, Quota: 900},
		{UserId: 7, Type: model.LogTypeConsume, CreatedAt: now + 60, Quota: 1000},
	}).Error)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("id", 7)
	c.Request = httptest.NewRequest("GET", "/api/usage-statistics/self?range=24h&snapshot=true", nil)
	GetUsageStatisticsSelf(c)
	var response struct {
		Success bool
		Data    struct {
			Start   int64 `json:"start_timestamp"`
			End     int64 `json:"end_timestamp"`
			Summary struct{ Quota int64 }
		}
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &response))
	require.True(t, response.Success, rec.Body.String())
	require.Zero(t, response.Data.End%30)
	require.EqualValues(t, 86400, response.Data.End-response.Data.Start)
	require.EqualValues(t, 700, response.Data.Summary.Quota)
}
