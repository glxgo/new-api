package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestBuildPerfMetricChannelScopesUsesEnabledSelectedAbilities(t *testing.T) {
	channels := []*model.Channel{
		{Id: 8, Status: common.ChannelStatusEnabled, Group: "gpt-pro,套餐专用分组"},
		{Id: 31, Status: common.ChannelStatusEnabled, Group: "套餐专用分组"},
		{Id: 32, Status: common.ChannelStatusEnabled, Group: "gpt-pro"},
		{Id: 34, Status: common.ChannelStatusManuallyDisabled, Group: "gpt-pro"},
	}
	abilities := []model.Ability{
		{Group: "gpt-pro", Model: "gpt-5", ChannelId: 8, Enabled: true},
		{Group: "套餐专用分组", Model: "gpt-5", ChannelId: 8, Enabled: true},
		{Group: "套餐专用分组", Model: "gpt-5", ChannelId: 31, Enabled: false},
		{Group: "gpt-pro", Model: "gpt-5", ChannelId: 32, Enabled: true},
		{Group: "gpt-pro", Model: "gpt-5", ChannelId: 34, Enabled: true},
	}
	settings := &operation_setting.MonitorSetting{ChannelCanaryChannelIds: []int{8, 31, 34}}

	scopes := buildPerfMetricChannelScopes(
		[]string{"gpt-pro", "套餐专用分组"},
		channels,
		abilities,
		settings,
	)

	require.Len(t, scopes, 2)
	byGroup := make(map[string]map[string][]int, len(scopes))
	for _, scope := range scopes {
		byGroup[scope.Group] = scope.ModelChannels
	}
	require.Equal(t, []int{8}, byGroup["gpt-pro"]["gpt-5"])
	require.Equal(t, []int{8}, byGroup["套餐专用分组"]["gpt-5"])
}
