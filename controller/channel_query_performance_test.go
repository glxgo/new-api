package controller

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAdminListQueriesHonorCancellation(t *testing.T) {
	initModelListColumnNames(t)
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := model.GetAllUsersWithContext(ctx, &common.PageInfo{Page: 1, PageSize: 10})
	require.ErrorIs(t, err, context.Canceled)
	_, _, err = model.SearchUsersWithContext(ctx, "", "", nil, nil, 0, 10)
	require.ErrorIs(t, err, context.Canceled)
	for _, handler := range []gin.HandlerFunc{GetAllChannels, SearchChannels} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest("GET", "/?keyword=match", nil).WithContext(ctx)
		handler(c)
		var response channelPageResponse
		require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &response))
		require.False(t, response.Success)
	}
}

type channelPageResponse struct {
	Success bool
	Data    struct {
		Items      []*model.Channel
		Total      int64
		TypeCounts map[int]int64 `json:"type_counts"`
	}
}

func readChannelPage(t *testing.T, path string, handler gin.HandlerFunc) channelPageResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("GET", path, nil)
	handler(c)
	var result channelPageResponse
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &result))
	require.True(t, result.Success, rec.Body.String())
	for _, channel := range result.Data.Items {
		require.Empty(t, channel.Key)
	}
	return result
}

func TestChannelSearchReadsOnlyRequestedPage(t *testing.T) {
	initModelListColumnNames(t)
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	for i := 1; i <= 120; i++ {
		require.NoError(t, db.Create(&model.Channel{Id: i, Name: "match", Models: "gpt", Key: "secret", Type: i%2 + 1, Status: 1, Group: "default,vip"}).Error)
	}
	var loaded int64
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("track_channel_rows", func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*[]*model.Channel); ok {
			loaded += tx.RowsAffected
		}
	}))
	defer db.Callback().Query().Remove("track_channel_rows")
	result := readChannelPage(t, "/api/channel/search?keyword=match&group=vip&model=gpt&type=1&status=enabled&id_sort=true&p=2&page_size=10", SearchChannels)
	require.EqualValues(t, 60, result.Data.Total)
	require.Equal(t, map[int]int64{1: 60, 2: 60}, result.Data.TypeCounts)
	require.Len(t, result.Data.Items, 10)
	require.Equal(t, 100, result.Data.Items[0].Id)
	require.EqualValues(t, 10, loaded, "search must not materialize every matching channel before paging")
	for _, tc := range []struct {
		params string
		count  int
	}{
		{"", 20}, {"&page_size=-1&p=-2", 20}, {"&page_size=100000", 100}, {"&p=1000&page_size=10", 0},
	} {
		page := readChannelPage(t, "/api/channel/search?keyword=match"+tc.params, SearchChannels)
		require.Len(t, page.Data.Items, tc.count)
		require.EqualValues(t, 120, page.Data.Total)
	}
}

func TestChannelTagListUsesOneBatchQuery(t *testing.T) {
	initModelListColumnNames(t)
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	for i := 0; i < 24; i++ {
		tag := fmt.Sprintf("tag-%02d", i)
		for j := 0; j < 2; j++ {
			require.NoError(t, db.Create(&model.Channel{Name: "match", Models: "gpt", Key: "secret", Tag: &tag, Status: 1, Type: 1}).Error)
		}
	}
	queries := 0
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("track_channel_queries", func(tx *gorm.DB) { queries++ }))
	defer db.Callback().Query().Remove("track_channel_queries")
	result := readChannelPage(t, "/api/channel/?tag_mode=true&page_size=20&id_sort=true", GetAllChannels)
	require.EqualValues(t, 24, result.Data.Total)
	require.Len(t, result.Data.Items, 40)
	require.LessOrEqual(t, queries, 4, "tag pages must not run one query per tag")
	require.Equal(t, "tag-00", *result.Data.Items[0].Tag)
	require.Greater(t, result.Data.Items[0].Id, result.Data.Items[1].Id)
}

func TestChannelTagSearchKeepsSiblingAndFilterSemantics(t *testing.T) {
	initModelListColumnNames(t)
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	tag := "matching-tag"
	rows := []model.Channel{
		{Id: 1, Name: "needle", Models: "gpt", Key: "secret", Tag: &tag, Type: 1, Status: 2, Group: "vip"},
		{Id: 2, Name: "sibling", Models: "other", Key: "secret", Tag: &tag, Type: 2, Status: 1, Group: "vip"},
		{Id: 3, Name: "other-group", Key: "secret", Tag: &tag, Type: 2, Status: 1, Group: "default"},
		{Id: 4, Name: "untagged needle", Models: "gpt", Key: "secret", Type: 2, Status: 1, Group: "vip"},
	}
	require.NoError(t, db.Create(&rows).Error)
	result := readChannelPage(t, "/api/channel/search?keyword=needle&model=gpt&tag_mode=true&group=vip&status=enabled&type=2&page_size=1", SearchChannels)
	require.EqualValues(t, 1, result.Data.Total)
	require.Equal(t, map[int]int64{2: 1}, result.Data.TypeCounts)
	require.Len(t, result.Data.Items, 1)
	require.Equal(t, 2, result.Data.Items[0].Id)
}
