package common

import (
	"sync/atomic"
	"time"
)

// ConsumeLogMetrics keeps the billing-log hot path measurable without
// retaining request content or high-cardinality labels.
type ConsumeLogMetrics struct {
	Calls              uint64 `json:"calls"`
	JSONNanos          uint64 `json:"json_nanos"`
	BalanceNanos       uint64 `json:"balance_nanos"`
	LogWriteNanos      uint64 `json:"log_write_nanos"`
	LogWriteErrors     uint64 `json:"log_write_errors"`
	QuotaDataEnqueued  uint64 `json:"quota_data_enqueued"`
	QuotaDataInline    uint64 `json:"quota_data_inline"`
	QuotaDataQueueFull uint64 `json:"quota_data_queue_full"`
}

var consumeMetrics struct {
	calls, jsonNanos, balanceNanos, logWriteNanos                          atomic.Uint64
	logWriteErrors, quotaDataEnqueued, quotaDataInline, quotaDataQueueFull atomic.Uint64
}

func RecordConsumeLogMetrics(jsonDuration, balanceDuration, logWriteDuration time.Duration, writeError bool) {
	consumeMetrics.calls.Add(1)
	consumeMetrics.jsonNanos.Add(uint64(jsonDuration))
	consumeMetrics.balanceNanos.Add(uint64(balanceDuration))
	consumeMetrics.logWriteNanos.Add(uint64(logWriteDuration))
	if writeError {
		consumeMetrics.logWriteErrors.Add(1)
	}
}

func RecordQuotaDataEnqueued()  { consumeMetrics.quotaDataEnqueued.Add(1) }
func RecordQuotaDataInline()    { consumeMetrics.quotaDataInline.Add(1) }
func RecordQuotaDataQueueFull() { consumeMetrics.quotaDataQueueFull.Add(1) }

func SnapshotConsumeLogMetrics() ConsumeLogMetrics {
	return ConsumeLogMetrics{
		Calls: consumeMetrics.calls.Load(), JSONNanos: consumeMetrics.jsonNanos.Load(),
		BalanceNanos: consumeMetrics.balanceNanos.Load(), LogWriteNanos: consumeMetrics.logWriteNanos.Load(),
		LogWriteErrors: consumeMetrics.logWriteErrors.Load(), QuotaDataEnqueued: consumeMetrics.quotaDataEnqueued.Load(),
		QuotaDataInline: consumeMetrics.quotaDataInline.Load(), QuotaDataQueueFull: consumeMetrics.quotaDataQueueFull.Load(),
	}
}
