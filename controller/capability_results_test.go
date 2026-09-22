package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/capabilitytest"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCapabilityMissingGradeIsNotZero(t *testing.T) {
	items := []capabilityItem{{Kind: "logic", Status: "graded", Checks: []bool{false, false}}, {Kind: "geometry", Status: "pending_render"}, {Kind: "scene", Status: "pending_review"}}
	raw, err := common.Marshal(items)
	require.NoError(t, err)
	run := model.CapabilityRun{Result: string(raw)}
	dto := capabilityRunDTO(model.CapabilityBinding{}, run, model.CapabilityRound{}, false)
	require.NotNil(t, dto.Score[0])
	require.Zero(t, *dto.Score[0])
	require.Nil(t, dto.Score[1])
	require.Nil(t, dto.Score[2])
	require.Nil(t, dto.Score[3])
	require.False(t, dto.Ranked)
	judgment, err := capabilitytest.ParseJudgment(`{"items":[{"status":"fail","evidence":"a"},{"status":"pass","evidence":"b"},{"status":"pass","evidence":"c"},{"status":"pass","evidence":"d"},{"status":"pass","evidence":"e"},{"status":"pass","evidence":"f"}],"aesthetic":4,"confidence":0.9}`)
	require.NoError(t, err)
	items[1] = capabilityItem{Kind: "geometry", Status: "graded", Checks: []bool{true, true, true, true, false}}
	items[2] = capabilityItem{Kind: "scene", Status: "graded", Judgment: &judgment}
	raw, err = common.Marshal(items)
	require.NoError(t, err)
	run.Result = string(raw)
	dto = capabilityRunDTO(model.CapabilityBinding{}, run, model.CapabilityRound{}, false)
	require.True(t, dto.Ranked)
	require.Equal(t, 4, *dto.Score[1])
	require.Equal(t, 5, *dto.Score[2])
}

func TestCapabilityCoverageBeforeSecondTarget(t *testing.T) {
	job := func(group string) *capabilityWork {
		return &capabilityWork{targets: []service.CapabilityTarget{{GroupUID: group}}}
	}
	ordered := capabilityCoverageOrder([]*capabilityWork{job("a"), job("a"), job("a"), job("b"), job("c")})
	for i, want := range []string{"a", "b", "c", "a", "a"} {
		require.Equal(t, want, ordered[i].targets[0].GroupUID)
	}
}
