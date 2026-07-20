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

func TestGetRandomSatisfiedChannelStartsWithFirstConfiguredChannel(t *testing.T) {
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
	if channel.Id != 8 {
		t.Fatalf("expected first configured channel #8 regardless of weight, got #%d", channel.Id)
	}
}
