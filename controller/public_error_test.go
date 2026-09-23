package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPublicErrorTaskResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	err := &dto.TaskError{StatusCode: 502, Code: "provider_error", Message: "https://private.example/task", Data: map[string]any{"endpoint": "https://private.example/debug"}}
	respondTaskError(c, err)
	require.Equal(t, 502, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "private.example")
	require.Contains(t, err.Message, "private.example")
}

func TestPublicErrorProbeHistory(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:public_error_probe?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB; _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.ChannelProbeState{}))
	require.NoError(t, db.Create(&model.Channel{Id: 1, Name: "test", Group: "default"}).Error)
	original := &model.ChannelProbeState{ChannelId: 1, LastErrorCode: "https://private.example/code", LastErrorMessage: "502, url: https://private.example/probe"}
	require.NoError(t, db.Create(original).Error)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	GetChannelProbeStatus(c)
	require.Equal(t, 200, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "private.example")
	require.Contains(t, recorder.Body.String(), `"last_error_message":"502"`)
	public := publicProbeState(original)
	require.Equal(t, "502", public.LastErrorMessage)
	require.NotContains(t, public.LastErrorCode, "private.example")
	var saved model.ChannelProbeState
	require.NoError(t, db.First(&saved).Error)
	require.Equal(t, original.LastErrorMessage, saved.LastErrorMessage)
	require.Contains(t, original.LastErrorMessage, "private.example")
	require.Nil(t, publicProbeState(nil))
}
