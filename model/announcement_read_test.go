package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAnnouncementReadTrackingIsPerUserAndIdempotent(t *testing.T) {
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:announcement-read-%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&UserAnnouncementRead{}))
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

	require.NoError(t, MarkAnnouncementsRead(12, []int64{101, 102, 101, 0}))
	require.NoError(t, MarkAnnouncementsRead(12, []int64{101}))
	require.NoError(t, MarkAnnouncementsRead(13, []int64{101}))

	read12, err := GetReadAnnouncementIds(12)
	require.NoError(t, err)
	require.Len(t, read12, 2)
	_, has101 := read12[101]
	_, has102 := read12[102]
	require.True(t, has101)
	require.True(t, has102)
	read13, err := GetReadAnnouncementIds(13)
	require.NoError(t, err)
	require.Len(t, read13, 1)
}
