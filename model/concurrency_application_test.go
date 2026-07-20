package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestConcurrencyApplicationLifecycle(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &ConcurrencyApplication{}))
	oldDB := DB
	DB = db
	defer func() { DB = oldDB }()

	user := User{Username: "concurrency-user", Password: "hashed-password", ConcurrencyLimit: 8}
	require.NoError(t, db.Create(&user).Error)
	application, err := CreateConcurrencyApplication(user.Id, 16, "团队多人同时使用，需要提升并发容量", "contact@example.com")
	require.NoError(t, err)
	require.Equal(t, 8, application.CurrentLimit)
	applications, total, err := ListConcurrencyApplications(0, ConcurrencyApplicationPending, 0, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, applications, 1)
	require.Equal(t, "concurrency-user", applications[0].Username)
	_, err = CreateConcurrencyApplication(user.Id, 20, "第二个同时提交的申请应被拒绝", "contact@example.com")
	require.Error(t, err)

	reviewed, err := ReviewConcurrencyApplication(application.Id, 99, true, 12, "approved")
	require.NoError(t, err)
	require.Equal(t, ConcurrencyApplicationApproved, reviewed.Status)
	var refreshed User
	require.NoError(t, db.First(&refreshed, user.Id).Error)
	require.Equal(t, 12, refreshed.ConcurrencyLimit)
}
