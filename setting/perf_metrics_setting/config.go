package perf_metrics_setting

import "github.com/QuantumNous/new-api/setting/config"

// ModelStatusBucketSeconds is the fixed cadence used by the public model
// status timeline. It intentionally remains independent of the selected
// reporting range.
const ModelStatusBucketSeconds int64 = 30 * 60

type PerfMetricsSetting struct {
	Enabled       bool   `json:"enabled"`
	FlushInterval int    `json:"flush_interval"`
	BucketTime    string `json:"bucket_time"`
	RetentionDays int    `json:"retention_days"`
}

var perfMetricsSetting = PerfMetricsSetting{
	Enabled:       true,
	FlushInterval: 5,
	BucketTime:    "30min",
	RetentionDays: 0,
}

func init() {
	config.GlobalConfig.Register("perf_metrics_setting", &perfMetricsSetting)
}

func GetSetting() PerfMetricsSetting {
	return perfMetricsSetting
}

func GetBucketSeconds() int64 {
	switch perfMetricsSetting.BucketTime {
	case "minute":
		return 60
	case "5min":
		return 300
	case "15min":
		return 900
	case "20min":
		return ModelStatusBucketSeconds
	case "30min":
		return ModelStatusBucketSeconds
	case "hour":
		return 3600
	default:
		return 3600
	}
}

func GetFlushIntervalMinutes() int {
	if perfMetricsSetting.FlushInterval < 1 {
		return 1
	}
	return perfMetricsSetting.FlushInterval
}
