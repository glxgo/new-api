package operation_setting

import (
	"os"
	"strconv"

	"github.com/QuantumNous/new-api/setting/config"
)

type MonitorSetting struct {
	AutoTestChannelEnabled         bool    `json:"auto_test_channel_enabled"`
	AutoTestChannelMinutes         float64 `json:"auto_test_channel_minutes"`
	AutoTestChannelIds             []int   `json:"auto_test_channel_ids"`
	ChannelCanaryEnabled           bool    `json:"channel_canary_enabled"`
	ChannelCanaryMinutes           int     `json:"channel_canary_minutes"`
	ChannelCanaryChannelIds        []int   `json:"channel_canary_channel_ids"`
	ChannelCanaryFailureThreshold  int     `json:"channel_canary_failure_threshold"`
	ChannelCanaryRecoveryThreshold int     `json:"channel_canary_recovery_threshold"`
	ChannelCanaryTimeoutSeconds    int     `json:"channel_canary_timeout_seconds"`
}

// 默认配置
var monitorSetting = MonitorSetting{
	AutoTestChannelEnabled:         false,
	AutoTestChannelMinutes:         10,
	ChannelCanaryEnabled:           false,
	ChannelCanaryMinutes:           5,
	ChannelCanaryFailureThreshold:  3,
	ChannelCanaryRecoveryThreshold: 2,
	ChannelCanaryTimeoutSeconds:    30,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("monitor_setting", &monitorSetting)
}

func GetMonitorSetting() *MonitorSetting {
	if os.Getenv("CHANNEL_TEST_FREQUENCY") != "" {
		frequency, err := strconv.Atoi(os.Getenv("CHANNEL_TEST_FREQUENCY"))
		if err == nil && frequency > 0 {
			monitorSetting.AutoTestChannelEnabled = true
			monitorSetting.AutoTestChannelMinutes = float64(frequency)
		}
	}
	if monitorSetting.ChannelCanaryMinutes < 1 {
		monitorSetting.ChannelCanaryMinutes = 5
	}
	if monitorSetting.ChannelCanaryFailureThreshold < 1 {
		monitorSetting.ChannelCanaryFailureThreshold = 3
	}
	if monitorSetting.ChannelCanaryRecoveryThreshold < 1 {
		monitorSetting.ChannelCanaryRecoveryThreshold = 2
	}
	if monitorSetting.ChannelCanaryTimeoutSeconds < 1 || monitorSetting.ChannelCanaryTimeoutSeconds > 120 {
		monitorSetting.ChannelCanaryTimeoutSeconds = 30
	}
	return &monitorSetting
}
