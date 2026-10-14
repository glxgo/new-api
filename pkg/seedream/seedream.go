package seedream

import (
	"context"
	"fmt"

	doubaoplugin "github.com/QuantumNous/new-api/plugins/tasks/doubao"
)

type Request struct {
	Kind   string         `json:"kind"`
	Model  string         `json:"model"`
	Action string         `json:"action"`
	Body   map[string]any `json:"requestBody"`
}

func Decode(ctx context.Context, model, operation string, body map[string]any) (*Request, error) {
	engine, err := doubaoplugin.Runtime()
	if err != nil {
		return nil, err
	}
	var request Request
	err = engine.CallPathInto(ctx, &request, "protocols", []string{"openai_image", "decodeRequest"}, map[string]any{"model": model, "operation": operation, "body": body})
	return &request, err
}

func (r *Request) Context(upstream string) map[string]any {
	return map[string]any{"model": r.Model, "upstreamModel": upstream, "action": r.Action, "requestBody": r.Body}
}

func (r *Request) Build(ctx context.Context, upstream string) (map[string]any, map[string]float64, error) {
	engine, err := doubaoplugin.Runtime()
	if err != nil {
		return nil, nil, err
	}
	var result struct {
		Body map[string]any `json:"body"`
	}
	input := r.Context(upstream)
	if err = engine.CallInto(ctx, &result, "buildSubmitRequest", input); err != nil {
		return nil, nil, err
	}
	var facts map[string]any
	err = engine.CallInto(ctx, &facts, "extractUsage", input)
	if err != nil {
		return nil, nil, err
	}
	usage, err := numericFacts(facts)
	return result.Body, usage, err
}

func (r *Request) ActualUsage(ctx context.Context, upstream string, body map[string]any) (map[string]float64, error) {
	engine, err := doubaoplugin.Runtime()
	if err != nil {
		return nil, err
	}
	var facts map[string]any
	err = engine.CallInto(ctx, &facts, "extractUsageOnComplete", r.Context(upstream), map[string]any{}, body)
	if err != nil {
		return nil, err
	}
	return numericFacts(facts)
}

func numericFacts(facts map[string]any) (map[string]float64, error) {
	values := map[string]float64{}
	for key, value := range facts {
		switch value := value.(type) {
		case float64:
			values[key] = value
		case bool:
			if value {
				values[key] = 1
			} else {
				values[key] = 0
			}
		default:
			return nil, fmt.Errorf("invalid Seedream usage fact %s", key)
		}
	}
	return values, nil
}
