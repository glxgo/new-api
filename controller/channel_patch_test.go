package controller

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPatchChannelCapacityFieldsTrackOmissionAndExplicitZero(t *testing.T) {
	var omitted PatchChannel
	require.NoError(t, json.Unmarshal([]byte(`{"id":8,"name":"channel"}`), &omitted))
	require.Nil(t, omitted.ConcurrencyLimit)
	require.Nil(t, omitted.RPMLimit)

	var explicit PatchChannel
	require.NoError(t, json.Unmarshal([]byte(`{"id":8,"concurrency_limit":0,"rpm_limit":0}`), &explicit))
	require.NotNil(t, explicit.ConcurrencyLimit)
	require.NotNil(t, explicit.RPMLimit)
	require.Zero(t, *explicit.ConcurrencyLimit)
	require.Zero(t, *explicit.RPMLimit)
}
