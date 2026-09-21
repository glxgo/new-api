package common

import (
	"net/http"
	"sync"
	"time"
)

// TrafficMetric is a deliberately low-cardinality request bucket. It is used
// for capacity decisions only; it never stores API keys, cookies, prompts, or
// response bodies.
type TrafficMetric struct {
	Requests           uint64 `json:"requests"`
	Success            uint64 `json:"success"`
	ClientErrors       uint64 `json:"client_errors"`
	ServerErrors       uint64 `json:"server_errors"`
	Cancelled          uint64 `json:"cancelled"`
	InFlight           int64  `json:"in_flight"`
	TotalLatencyMicros uint64 `json:"total_latency_us"`
	TotalRequestBytes  uint64 `json:"total_request_bytes"`
	TotalResponseBytes uint64 `json:"total_response_bytes"`
}

type trafficMetricCounter struct {
	TrafficMetric
}

var trafficMetricState = struct {
	sync.Mutex
	buckets map[string]*trafficMetricCounter
}{buckets: make(map[string]*trafficMetricCounter)}

func trafficBucket(routeClass string) *trafficMetricCounter {
	if routeClass == "" {
		routeClass = "other"
	}
	if bucket, ok := trafficMetricState.buckets[routeClass]; ok {
		return bucket
	}
	bucket := &trafficMetricCounter{}
	trafficMetricState.buckets[routeClass] = bucket
	return bucket
}

func RecordTrafficStart(routeClass string) {
	trafficMetricState.Lock()
	trafficBucket(routeClass).InFlight++
	trafficMetricState.Unlock()
}

func RecordTrafficEnd(routeClass string, status int, started time.Time, requestBytes, responseBytes int64, cancelled bool) {
	trafficMetricState.Lock()
	bucket := trafficBucket(routeClass)
	bucket.InFlight--
	bucket.Requests++
	if status >= 200 && status < 400 {
		bucket.Success++
	} else if status >= 400 && status < 500 {
		bucket.ClientErrors++
	} else if status >= http.StatusInternalServerError {
		bucket.ServerErrors++
	}
	if cancelled {
		bucket.Cancelled++
	}
	if elapsed := time.Since(started); elapsed > 0 {
		bucket.TotalLatencyMicros += uint64(elapsed / time.Microsecond)
	}
	if requestBytes > 0 {
		bucket.TotalRequestBytes += uint64(requestBytes)
	}
	if responseBytes > 0 {
		bucket.TotalResponseBytes += uint64(responseBytes)
	}
	trafficMetricState.Unlock()
}

func SnapshotTrafficMetrics() map[string]TrafficMetric {
	trafficMetricState.Lock()
	defer trafficMetricState.Unlock()
	result := make(map[string]TrafficMetric, len(trafficMetricState.buckets))
	for key, bucket := range trafficMetricState.buckets {
		result[key] = bucket.TrafficMetric
	}
	return result
}
