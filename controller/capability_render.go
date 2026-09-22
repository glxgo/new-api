package controller

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/QuantumNous/new-api/pkg/capabilitytest"
	"github.com/QuantumNous/new-api/service"
)

// Shared by initial execution and durable recovery; returns the exact image
// used by the reviewer (the six-frame montage for animation, never its poster).
func capabilityRenderItem(ctx context.Context, item *capabilityItem) ([]byte, error) {
	if item.Question.Format == "html-svg-animation" {
		html := capabilitytest.Unfence(item.Answer)
		if item.Animation != nil && item.Animation.SourceSHA256 == fmt.Sprintf("%x", sha256.Sum256([]byte(html))) && item.Animation.Renderer == service.CapabilityAnimationVersion && item.Animation.Browser == service.CapabilityAnimationBrowser {
			if _, err := service.ReadCapabilityPNG(item.Artifact); err == nil {
				if _, err = service.ReadCapabilityAnimationAsset(item.Animation.Artifact, "animation"); err == nil {
					if evidence, e := service.ReadCapabilityAnimationAsset(item.Animation.Evidence, "evidence"); e == nil {
						return evidence, nil
					}
				}
			}
		}
		poster, animation, evidence, err := service.RenderCapabilityAnimation(ctx, html)
		if err != nil {
			item.Status = "pending_render"
			return nil, err
		}
		item.Artifact, item.Animation = poster, animation
		return evidence, nil
	}
	svg, shapes, err := capabilitytest.ValidateSVG(item.Answer, item.Kind == "geometry")
	if err != nil {
		item.Status = "ungraded"
		return nil, err
	}
	if item.Kind == "scene" && item.Artifact != "" {
		if raw, e := service.ReadCapabilityPNG(item.Artifact); e == nil {
			return raw, nil
		}
	}
	raw, img, err := service.RenderCapabilitySVG(ctx, svg)
	if err != nil {
		item.Status = "pending_render"
		return nil, err
	}
	item.Artifact, err = service.StoreCapabilityPNG(raw)
	if err != nil {
		item.Status = "pending_render"
		return nil, err
	}
	if item.Kind == "geometry" {
		item.Checks = capabilitytest.GradeGeometry(shapes, img, item.Question.Label)
		item.Status = "graded"
	}
	return raw, nil
}

func capabilityJudgeArtifact(item capabilityItem) string {
	if item.Animation != nil {
		return item.Animation.Evidence
	}
	return item.Artifact
}

// Absence of any pixel change disproves observed motion. The converse is NOT
// true: background animation alone cannot mechanically pass a riding task.
func capabilityApplyMotionEvidence(item *capabilityItem) {
	if item.Animation == nil || item.Animation.ChangedFrames != 0 || item.Judgment == nil || len(item.Judgment.Items) != 6 {
		return
	}
	for _, i := range []int{4, 5} {
		item.Judgment.Items[i].Status = "fail"
		item.Judgment.Items[i].Evidence = "在0至4700毫秒的48个采样帧中未观察到画面变化。"
	}
}
