package common

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

// DiskSpaceInfo 磁盘空间信息
type DiskSpaceInfo struct {
	// 总空间（字节）
	Total uint64 `json:"total"`
	// 可用空间（字节）
	Free uint64 `json:"free"`
	// 已用空间（字节）
	Used uint64 `json:"used"`
	// 使用百分比
	UsedPercent float64 `json:"used_percent"`
}

// SystemStatus 系统状态信息
type SystemStatus struct {
	CPUUsage    float64
	MemoryUsage float64
	DiskUsage   float64
	// CPUEWMA is a smoothed CPU signal used by admission control. It avoids
	// rejecting traffic on one noisy five-second sample.
	CPUEWMA float64
	// SampleAtUnix is the wall-clock time of the last valid sample.
	SampleAtUnix int64
	// ProtectionLevel is 0..3 and is shared by relay/background admission.
	ProtectionLevel    int32
	CPUPressureSome    float64
	MemoryPressureSome float64
}

var latestSystemStatus atomic.Value
var cpuHighSamples atomic.Int32
var cpuLowSamples atomic.Int32
var protectionLevel atomic.Int32

func init() {
	latestSystemStatus.Store(SystemStatus{})
}

// StartSystemMonitor 启动系统监控
func StartSystemMonitor() {
	go func() {
		for {
			config := GetPerformanceMonitorConfig()
			if !config.Enabled {
				time.Sleep(30 * time.Second)
				continue
			}

			updateSystemStatus()
			time.Sleep(5 * time.Second)
		}
	}()
}

func updateSystemStatus() {
	var status SystemStatus

	// CPU
	// 注意：cpu.Percent(0, false) 返回自上次调用以来的 CPU 使用率
	// 如果是第一次调用，可能会返回错误或不准确的值，但在循环中会逐渐正常
	percents, err := cpu.Percent(0, false)
	if err == nil && len(percents) > 0 {
		status.CPUUsage = percents[0]
	}
	previous := latestSystemStatus.Load().(SystemStatus)
	if previous.SampleAtUnix > 0 {
		// alpha=0.25 gives a stable ~15-20 second signal with the current
		// five-second sampling interval.
		status.CPUEWMA = previous.CPUEWMA*0.75 + status.CPUUsage*0.25
	} else {
		status.CPUEWMA = status.CPUUsage
	}

	// Memory
	memInfo, err := mem.VirtualMemory()
	if err == nil {
		status.MemoryUsage = memInfo.UsedPercent
	}

	// Disk
	diskInfo := GetDiskSpaceInfo()
	if diskInfo.Total > 0 {
		status.DiskUsage = diskInfo.UsedPercent
	}
	status.CPUPressureSome = readPressureAverage("cpu")
	status.MemoryPressureSome = readPressureAverage("memory")
	status.SampleAtUnix = time.Now().Unix()
	level := updateProtectionLevel(status)
	status.ProtectionLevel = level

	latestSystemStatus.Store(status)
}

func updateProtectionLevel(status SystemStatus) int32 {
	config := GetPerformanceMonitorConfig()
	enterAt := float64(config.CPUThreshold)
	if enterAt <= 0 {
		enterAt = 90
	}
	if status.MemoryPressureSome >= 50 {
		cpuHighSamples.Add(1)
		cpuLowSamples.Store(0)
	} else if status.CPUEWMA >= enterAt || status.CPUPressureSome >= 20 || status.MemoryPressureSome >= 20 {
		cpuHighSamples.Add(1)
		cpuLowSamples.Store(0)
	} else if status.CPUEWMA <= enterAt-12 && status.CPUPressureSome < 10 && status.MemoryPressureSome < 10 {
		cpuLowSamples.Add(1)
		cpuHighSamples.Store(0)
	} else {
		cpuHighSamples.Store(0)
		cpuLowSamples.Store(0)
	}
	level := protectionLevel.Load()
	if level == 0 && status.CPUEWMA >= enterAt-10 && cpuHighSamples.Load() >= 2 {
		level = 1
		protectionLevel.Store(level)
	}
	if level < 2 && cpuHighSamples.Load() >= 3 {
		level = 2
		if status.MemoryPressureSome >= 50 {
			level = 3
		}
		protectionLevel.Store(level)
	} else if level == 2 && status.MemoryPressureSome >= 50 && cpuHighSamples.Load() >= 3 {
		level = 3
		protectionLevel.Store(level)
	} else if level > 0 && cpuLowSamples.Load() >= 3 {
		level = 0
		protectionLevel.Store(level)
	}
	return level
}

func readPressureAverage(resource string) float64 {
	if runtime.GOOS != "linux" {
		return 0
	}
	data, err := os.ReadFile("/proc/pressure/" + resource)
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "some ") {
			continue
		}
		for _, field := range strings.Fields(line) {
			if strings.HasPrefix(field, "avg10=") {
				value, _ := strconv.ParseFloat(strings.TrimPrefix(field, "avg10="), 64)
				return value
			}
		}
	}
	return 0
}

// GetSystemStatus 获取当前系统状态
func GetSystemStatus() SystemStatus {
	return latestSystemStatus.Load().(SystemStatus)
}

// SystemProtectionActive reports the debounced overload state. A single
// transient sample never flips this value.
func SystemProtectionActive() bool {
	return protectionLevel.Load() >= 2
}

// SystemProtectionLevel returns the shared debounced overload level (0..3).
// Level 1 is a warning used to defer optional work; levels 2/3 are admission
// protection for new expensive relay requests.
func SystemProtectionLevel() int32 { return protectionLevel.Load() }

// BackgroundWorkAllowed is the common gate for optional collectors, reports,
// probes and compaction tasks. It intentionally keeps billing/relay work out
// of this gate; callers should pause and retry on their next scheduled tick.
func BackgroundWorkAllowed() bool { return protectionLevel.Load() == 0 }

func SystemSampleStale(maxAge time.Duration) bool {
	status := GetSystemStatus()
	if status.SampleAtUnix <= 0 {
		return false
	}
	return time.Since(time.Unix(status.SampleAtUnix, 0)) > maxAge
}
