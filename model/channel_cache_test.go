package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/types"
)

func TestGetRandomSatisfiedChannelExcludesPreviouslyUsedChannels(t *testing.T) {
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalGroupChannels := group2model2channels
	originalChannels := channelsIDM
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		channelSyncLock.Lock()
		group2model2channels = originalGroupChannels
		channelsIDM = originalChannels
		channelSyncLock.Unlock()
	})

	priority := int64(0)
	weight := uint(100)
	common.MemoryCacheEnabled = true
	channelSyncLock.Lock()
	group2model2channels = map[string]map[string][]int{
		"plan": {"gpt-test": {1, 2}},
	}
	channelsIDM = map[int]*Channel{
		1: {Id: 1, Name: "kedaya", Type: constant.ChannelTypeOpenAI, Priority: &priority, Weight: &weight},
		2: {Id: 2, Name: "krill", Type: constant.ChannelTypeAnthropic, Priority: &priority, Weight: &weight},
	}
	channelSyncLock.Unlock()

	channel, err := GetRandomSatisfiedChannel(
		"plan",
		"gpt-test",
		1,
		types.RelayFormatOpenAI,
		map[int]struct{}{1: {}},
	)
	if err != nil {
		t.Fatalf("select fallback channel: %v", err)
	}
	if channel == nil {
		t.Fatal("expected an untried fallback channel, got nil")
	}
	if channel.Id != 2 {
		t.Fatalf("expected retry to exclude kedaya (#1) and select krill (#2), got #%d", channel.Id)
	}
}

func TestGetRandomSatisfiedChannelUsesPositiveWeightInsteadOfFirstChannel(t *testing.T) {
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalGroupChannels := group2model2channels
	originalChannels := channelsIDM
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		channelSyncLock.Lock()
		group2model2channels = originalGroupChannels
		channelsIDM = originalChannels
		channelSyncLock.Unlock()
	})

	priority := int64(80)
	firstWeight := uint(0)
	secondWeight := uint(100)
	common.MemoryCacheEnabled = true
	channelSyncLock.Lock()
	group2model2channels = map[string]map[string][]int{
		"plan": {"gpt-test": {8, 24}},
	}
	channelsIDM = map[int]*Channel{
		8:  {Id: 8, Name: "krill", Type: constant.ChannelTypeOpenAI, Priority: &priority, Weight: &firstWeight},
		24: {Id: 24, Name: "kedaya", Type: constant.ChannelTypeOpenAI, Priority: &priority, Weight: &secondWeight},
	}
	channelSyncLock.Unlock()

	channel, err := GetRandomSatisfiedChannel(
		"plan",
		"gpt-test",
		0,
		types.RelayFormatOpenAI,
		nil,
	)
	if err != nil {
		t.Fatalf("select first configured channel: %v", err)
	}
	if channel == nil {
		t.Fatal("expected the first configured channel, got nil")
	}
	if channel.Id != 24 {
		t.Fatalf("expected positive-weight channel #24 instead of zero-weight channel #8, got #%d", channel.Id)
	}
}

func TestGetRandomSatisfiedChannelDropsToNextPriorityAfterErrorRetry(t *testing.T) {
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalGroupChannels := group2model2channels
	originalChannels := channelsIDM
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		channelSyncLock.Lock()
		group2model2channels = originalGroupChannels
		channelsIDM = originalChannels
		channelSyncLock.Unlock()
	})

	highPriority := int64(80)
	lowPriority := int64(70)
	weight := uint(1)
	common.MemoryCacheEnabled = true
	channelSyncLock.Lock()
	group2model2channels = map[string]map[string][]int{
		"plan": {"gpt-test": {8, 31, 32}},
	}
	channelsIDM = map[int]*Channel{
		8:  {Id: 8, Priority: &highPriority, Weight: &weight},
		31: {Id: 31, Priority: &highPriority, Weight: &weight},
		32: {Id: 32, Priority: &lowPriority, Weight: &weight},
	}
	channelSyncLock.Unlock()

	channel, err := GetRandomSatisfiedChannel("plan", "gpt-test", 1, types.RelayFormatOpenAI, map[int]struct{}{8: {}})
	if err != nil {
		t.Fatalf("select lower-priority retry channel: %v", err)
	}
	if channel == nil || channel.Id != 32 {
		t.Fatalf("expected retry after high-priority failure to use lower-priority channel #32, got %#v", channel)
	}
}
