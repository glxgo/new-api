package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeResponsesInputItemIDsRepairsKnownGenericIDs(t *testing.T) {
	body := []byte(`{
		"model":"gpt-5.6-sol",
		"input":[
			{"type":"reasoning","id":"item_cde38a42cc60f24afb9e1dc2","encrypted_content":"opaque","summary":[]},
			{"type":"message","id":"item_0123456789abcdef01234567","role":"assistant","content":[]},
			{"id":"item_89abcdef0123456789abcdef","role":"user","content":[]}
		],
		"custom":{"preserved":true}
	}`)

	got, report, err := NormalizeResponsesInputItemIDs(body)
	require.NoError(t, err)
	require.Equal(t, ResponsesInputItemIDNormalizationReport{
		Reasoning: 1,
		Message:   2,
	}, report)
	require.Equal(t, "rs_cde38a42cc60f24afb9e1dc2", gjson.GetBytes(got, "input.0.id").String())
	require.Equal(t, "msg_0123456789abcdef01234567", gjson.GetBytes(got, "input.1.id").String())
	require.Equal(t, "msg_89abcdef0123456789abcdef", gjson.GetBytes(got, "input.2.id").String())
	require.Equal(t, "opaque", gjson.GetBytes(got, "input.0.encrypted_content").String())
	require.True(t, gjson.GetBytes(got, "custom.preserved").Bool())
}

func TestNormalizeResponsesInputItemIDsRepairsProviderResponseMessageIDs(t *testing.T) {
	body := []byte(`{
		"input":[
			{"type":"message","id":"resp_0bfb033d1ff7a0ee016a72d0389fc4819bbb6ec2557d139e37_msg","role":"assistant","content":[]},
			{"type":"function_call","id":"resp_0bfb033d1ff7a0ee016a72d0389fc4819bbb6ec2557d139e37_msg"}
		]
	}`)

	got, report, err := NormalizeResponsesInputItemIDs(body)
	require.NoError(t, err)
	require.Equal(t, ResponsesInputItemIDNormalizationReport{Message: 1}, report)
	require.Equal(t, "msg_0bfb033d1ff7a0ee016a72d0389fc4819bbb6ec2557d139e37_msg", gjson.GetBytes(got, "input.0.id").String())
	require.Equal(t, "resp_0bfb033d1ff7a0ee016a72d0389fc4819bbb6ec2557d139e37_msg", gjson.GetBytes(got, "input.1.id").String())
}

func TestNormalizeResponsesInputItemIDsRepairsCustomToolCallPrefixByType(t *testing.T) {
	body := []byte(`{
		"input":[
			{"type":"custom_tool_call","id":"fc_0a0f49100a7769fb016a87a42e7e108194b945110ab7f5a06d","call_id":"call_1","name":"lookup","input":"{}"},
			{"type":"custom_tool_call","id":"ctc_already_valid","call_id":"call_2","name":"lookup","input":"{}"},
			{"type":"function_call","id":"fc_function","call_id":"call_3","name":"lookup","arguments":"{}"},
			{"type":"function_call","id":"ctc_wrong_type","call_id":"call_4","name":"lookup","arguments":"{}"},
			{"type":"custom_tool_call_output","id":"fco_output","call_id":"call_5","output":"ok"},
			{"type":"function_call_output","id":"ctco_wrong_type","call_id":"call_6","output":"ok"}
		]
	}`)

	got, report, err := NormalizeResponsesInputItemIDs(body)
	require.NoError(t, err)
	require.Equal(t, ResponsesInputItemIDNormalizationReport{
		CustomToolCall:     1,
		FunctionCall:       1,
		FunctionCallOutput: 1,
		CustomToolOutput:   1,
	}, report)
	require.Equal(t, "ctc_fc_0a0f49100a7769fb016a87a42e7e108194b945110ab7f5a06d", gjson.GetBytes(got, "input.0.id").String())
	require.Equal(t, "ctc_already_valid", gjson.GetBytes(got, "input.1.id").String())
	require.Equal(t, "fc_function", gjson.GetBytes(got, "input.2.id").String())
	require.Equal(t, "fc_ctc_wrong_type", gjson.GetBytes(got, "input.3.id").String())
	require.Equal(t, "ctco_fco_output", gjson.GetBytes(got, "input.4.id").String())
	require.Equal(t, "fco_ctco_wrong_type", gjson.GetBytes(got, "input.5.id").String())
	require.Equal(t, "call_1", gjson.GetBytes(got, "input.0.call_id").String())
}

func TestNormalizeResponsesInputItemIDsLeavesUnknownAndCanonicalIDsUntouched(t *testing.T) {
	body := []byte(`{
		"input":[
			{"type":"reasoning","id":"rs_valid"},
			{"type":"message","id":"msg_valid","role":"assistant"},
			{"type":"function_call","id":"item_unknown"},
			{"type":"reasoning","id":"other_invalid"},
			{"type":"reasoning"}
		]
	}`)

	got, report, err := NormalizeResponsesInputItemIDs(body)
	require.NoError(t, err)
	require.Equal(t, ResponsesInputItemIDNormalizationReport{}, report)
	require.Equal(t, string(body), string(got))
}

func TestNormalizeResponsesInputItemIDsLeavesNonArrayInputUntouched(t *testing.T) {
	body := []byte(`{"model":"gpt-5.6-sol","input":"hello"}`)

	got, report, err := NormalizeResponsesInputItemIDs(body)
	require.NoError(t, err)
	require.Equal(t, ResponsesInputItemIDNormalizationReport{}, report)
	require.Equal(t, string(body), string(got))
}
