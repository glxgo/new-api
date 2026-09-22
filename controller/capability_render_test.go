package controller

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/capabilitytest"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/require"
)

func TestCapabilityAnimationMotionIsNotInferredFromAppearance(t *testing.T) {
	answer := `{"items":[{"status":"pass","evidence":"fixture"},{"status":"pass","evidence":"fixture"},{"status":"pass","evidence":"fixture"},{"status":"pass","evidence":"fixture"},{"status":"pass","evidence":"fixture"},{"status":"pass","evidence":"fixture"}],"aesthetic":5,"confidence":1}`
	j, err := capabilitytest.ParseJudgment(answer)
	require.NoError(t, err)
	item := capabilityItem{Judgment: &j, Animation: &service.CapabilityAnimation{ChangedFrames: 0}}
	capabilityApplyMotionEvidence(&item)
	require.Equal(t, "pass", j.Items[0].Status)
	require.Equal(t, "fail", j.Items[4].Status)
	require.Equal(t, "fail", j.Items[5].Status)
	item.Animation.ChangedFrames = 47
	capabilityApplyMotionEvidence(&item)
	require.Equal(t, "fail", j.Items[5].Status, "pixel changes cannot promote a failed semantic judgment")
}

func TestCapabilityAnimationCalibrationRejectsStaleVersions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test-only-calibration.json")
	t.Setenv("CAPABILITY_CALIBRATION_FILE", path)
	config := model.CapabilityConfig{JudgeChannelID: 1, JudgeModel: "fixture"}
	samples := []any{}
	for _, template := range capabilitytest.AnimationTemplateIDs() {
		for _, category := range []string{"correct", "missing_requirement", "text_substitution", "attractive_incorrect", "blank_truncated", "prompt_injection"} {
			for i := 0; i < 10; i++ {
				correct := category == "correct"
				labels := []bool{correct, correct, correct, correct, correct, correct}
				samples = append(samples, map[string]any{"template_id": template, "category": category, "expected": labels, "observed": labels, "image_sha256": fmt.Sprintf("%064x", len(samples)+1)})
			}
		}
	}
	report := map[string]any{"suite": capabilitytest.SuiteVersion, "renderer": service.CapabilityAnimationVersion, "browser": service.CapabilityAnimationBrowser, "judge_channel_id": 1, "judge_model": "fixture", "reviewer": "test-only", "samples": samples}
	write := func() {
		raw, err := common.Marshal(report)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, raw, 0600))
	}
	write()
	require.True(t, capabilityCalibrationValid(config))
	for _, key := range []string{"suite", "renderer", "browser"} {
		saved := report[key]
		report[key] = "previous-version"
		write()
		require.False(t, capabilityCalibrationValid(config))
		report[key] = saved
	}
	for _, key := range []string{"template_id", "category", "image_sha256"} {
		sample := samples[0].(map[string]any)
		saved := sample[key]
		sample[key] = "unknown"
		write()
		require.False(t, capabilityCalibrationValid(config), key)
		sample[key] = saved
	}
	// A pool-wide report cannot mask missing coverage or low agreement in one
	// newly introduced template, even when every other template is perfect.
	report["samples"] = samples[60:]
	write()
	require.False(t, capabilityCalibrationValid(config), "missing template")
	report["samples"] = samples
	for i := 0; i < 10; i++ {
		samples[i].(map[string]any)["observed"] = []bool{false, false, false, false, false, false}
	}
	write()
	require.False(t, capabilityCalibrationValid(config), "per-template agreement")
	for i := 0; i < 10; i++ {
		samples[i].(map[string]any)["observed"] = []bool{true, true, true, true, true, true}
	}
	samples[10].(map[string]any)["observed"] = []bool{true, false, false, false, false, false}
	write()
	require.False(t, capabilityCalibrationValid(config), "false positive")
}
