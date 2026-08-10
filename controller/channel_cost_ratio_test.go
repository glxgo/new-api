package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestValidateChannelAllowsEnabledChannelWithoutCostObservation(t *testing.T) {
	channel := &model.Channel{Status: common.ChannelStatusEnabled, Key: "test-key", Name: "test-channel"}
	require.NoError(t, validateChannel(channel, true))
}
