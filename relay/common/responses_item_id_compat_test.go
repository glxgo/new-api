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
