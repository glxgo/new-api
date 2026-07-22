package service

type ChannelCapacityReason string

const (
	ChannelCapacityAvailable       ChannelCapacityReason = ""
	ChannelCapacityConcurrencyFull ChannelCapacityReason = "concurrency"
	ChannelCapacityRPMFull         ChannelCapacityReason = "rpm"
)

// AcquireChannelCapacity reserves both capacity dimensions. Concurrency is
// acquired first; if RPM is already full, its lease is immediately released
// so a skipped channel never leaks an active slot.
func AcquireChannelCapacity(channelId, concurrencyLimit, rpmLimit int) (*ConcurrencyLease, bool, ChannelCapacityReason) {
	lease, acquired := AcquireChannelConcurrency(channelId, concurrencyLimit)
	if !acquired {
		return nil, false, ChannelCapacityConcurrencyFull
	}
	rpmAcquired, _ := AcquireChannelRPM(channelId, rpmLimit)
	if !rpmAcquired {
		lease.Release()
		return nil, false, ChannelCapacityRPMFull
	}
	return lease, true, ChannelCapacityAvailable
}
