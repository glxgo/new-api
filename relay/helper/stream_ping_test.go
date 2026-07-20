package helper

import (
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestResolveStreamPingResponsesHasSafeDefault(t *testing.T) {
	info := &relaycommon.RelayInfo{IsStream: true, RelayMode: relayconstant.RelayModeResponses}
	setting := &operation_setting.GeneralSetting{PingIntervalEnabled: false, PingIntervalSeconds: 60}

	enabled, interval := ResolveStreamPing(info, setting)

	require.True(t, enabled)
	require.Equal(t, DefaultPingInterval, interval)
}

func TestResolveStreamPingResponsesClampsUnsafeConfiguredInterval(t *testing.T) {
	info := &relaycommon.RelayInfo{IsStream: true, RelayMode: relayconstant.RelayModeResponsesCompact}
	setting := &operation_setting.GeneralSetting{PingIntervalEnabled: true, PingIntervalSeconds: 60}

	enabled, interval := ResolveStreamPing(info, setting)

	require.True(t, enabled)
	require.Equal(t, DefaultPingInterval, interval)
}

func TestResolveStreamPingHonorsGlobalSettingForOtherStreams(t *testing.T) {
	// StreamScannerHandler itself establishes that this is a stream. Keep this
	// compatible with adaptors that do not redundantly populate IsStream.
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeChatCompletions}
	setting := &operation_setting.GeneralSetting{PingIntervalEnabled: true, PingIntervalSeconds: 7}

	enabled, interval := ResolveStreamPing(info, setting)

	require.True(t, enabled)
	require.Equal(t, 7*time.Second, interval)
}

func TestResolveStreamPingOtherStreamsRemainDisabledByDefault(t *testing.T) {
	info := &relaycommon.RelayInfo{IsStream: true, RelayMode: relayconstant.RelayModeChatCompletions}
	setting := &operation_setting.GeneralSetting{PingIntervalEnabled: false, PingIntervalSeconds: 60}

	enabled, interval := ResolveStreamPing(info, setting)

	require.False(t, enabled)
	require.Zero(t, interval)
}

func TestResolveStreamPingHonorsPerRequestDisable(t *testing.T) {
	info := &relaycommon.RelayInfo{
		IsStream:    true,
		DisablePing: true,
		RelayMode:   relayconstant.RelayModeResponses,
	}

	enabled, interval := ResolveStreamPing(info, &operation_setting.GeneralSetting{})

	require.False(t, enabled)
	require.Zero(t, interval)
}
