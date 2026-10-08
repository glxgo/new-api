package middleware

import (
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
)

func dashboardFixture(t *testing.T, role int) string {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("%s/auth.db", t.TempDir())), &gorm.Config{})
	require.NoError(t, err)
	previous, redis := model.DB, common.RedisEnabled
	model.DB = db
	common.RedisEnabled = false
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.DashboardSession{}))
	user := model.User{Id: 1, Username: "fixture", Role: role, Status: common.UserStatusEnabled, Group: "default"}
	require.NoError(t, db.Create(&user).Error)
	token, err := model.SecurityToken()
	require.NoError(t, err)
	require.NoError(t, model.CreateDashboardSession(&user, token))
	t.Cleanup(func() { model.DB = previous; common.RedisEnabled = redis; sql, _ := db.DB(); sql.Close() })
	return token
}
