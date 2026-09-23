package common

import (
	"strings"
	"testing"
)

func TestSanitizePublicError(t *testing.T) {
	cases := []struct{ input, secret string }{
		{`502 url: https://api.private.example/v1/responses, cf-ray: trace-NRT`, "api.private.example"},
		{`Post HTTPS://localhost:18096/%zz?key=secret: EOF`, "localhost"},
		{`wss://user:pass@provider.internal:443/ws`, "provider.internal"},
		{`https:\/\/provider.example\/v1`, "provider.example"},
		{`https%3A%2F%2Fprovider.example%2Fv1`, "provider.example"},
		{`dial tcp [2001:db8::7]:443: timeout`, "2001:db8"},
		{`connect ::1 refused`, "::1"},
		{`connect 10.0.0.1:18096 refused`, "18096"},
		{`server private.example:18096 failed`, "18096"},
		{`server localhost:18096 failed`, "localhost"},
		{`lookup backend on 10.0.0.1:53: no such host`, "backend"},
		{`upstream api.provider.com failed`, "api.provider.com"},
		{`Authorization: Bearer secret-token`, "secret-token"},
		{`https://用户:密码@上游.中国:443/v1?key=secret`, "上游"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := SanitizePublicError(tc.input)
			if strings.Contains(got, tc.secret) {
				t.Fatalf("leaked %s", got)
			}
			if SanitizePublicError(got) != got {
				t.Fatalf("not idempotent: %s", got)
			}
		})
	}
	for _, text := range []string{"Invalid input[42].id: expected fc_ prefix", "insufficient_quota", "response.failed", "maximum context length exceeded"} {
		if got := SanitizePublicError(text); got != text {
			t.Errorf("changed useful error: %q => %q", text, got)
		}
	}
}

func TestSanitizeErrorJSONPreservesSuccessfulOutput(t *testing.T) {
	for _, data := range []string{
		`{"type":"response.output_text.delta","delta":"https://docs.example/guide"}`,
		`{"type":"response.completed","response":{"error":null,"output":[{"text":"https://docs.example/error"}],"usage":{"total_tokens":9007199254740993}}}`,
		`{"data":[{"url":"https://images.example/error.png"}]}`,
		`[DONE]`,
		`{"status":"SUCCESS","fail_reason":"https://images.example/legacy.mp4"}`,
	} {
		if got := string(SanitizeErrorJSON([]byte(data))); got != data {
			t.Fatalf("normal output changed: %s", got)
		}
	}
}

func TestSanitizeErrorJSONProtocols(t *testing.T) {
	for _, data := range []string{
		`{"error":{"message":"https://private.example/fail","metadata":{"url":"https://private.example/meta","api_key":"secret"}}}`,
		`{"type":"error","code":"server_error","message":"https://private.example/fail","sequence_number":9007199254740993}`,
		`{"type":"response.failed","response":{"error":{"message":"https://private.example/fail"},"output":[{"text":"https://docs.example/guide"}]}}`,
		`{"type":"response.done","response":{"status_details":{"error":{"message":"https://private.example/fail"}}}}`,
		`{"err\u006fr":{"message":"https://private.example/fail"}}`,
		`{"type":"response.incomplete","response":{"incomplete_details":{"reason":"https://private.example/fail"}}}`,
		`{"status":"failed","metadata":{"url":"https://private.example/fail"},"error":{"message":"failed"}}`,
	} {
		got := string(SanitizeErrorJSON([]byte(data)))
		if strings.Contains(got, "private.example") || strings.Contains(got, "secret") {
			t.Fatalf("leak: %s", got)
		}
		if strings.Contains(data, "9007199254740993") && !strings.Contains(got, "9007199254740993") {
			t.Fatalf("sequence changed: %s", got)
		}
		if strings.Contains(data, "https://docs.example/guide") && !strings.Contains(got, "https://docs.example/guide") {
			t.Fatalf("output changed: %s", got)
		}
	}
}

func TestPublicErrorPlainGatewayBody(t *testing.T) {
	for _, input := range []string{
		`<html>502: upstream https://private.example failed</html>`,
		`502 url: https://private.example/v1/responses`,
		`{"message":"https://private.example/v1/responses","code":502}`,
	} {
		got := SanitizeHTTPErrorBody([]byte(input))
		if strings.Contains(string(got), "private.example") {
			t.Fatalf("leak: %s", got)
		}
		var payload map[string]any
		if err := Unmarshal(got, &payload); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPublicErrorOmitsURLDiagnosticSuffix(t *testing.T) {
	cases := []struct{ input, want string }{
		{`unexpected status 502 Bad Gateway: error code: 502, url: https://private.example/v1/responses, cf-ray: trace-NRT`, `unexpected status 502 Bad Gateway: error code: 502`},
		{`502 Bad Gateway, url: [redacted]`, `502 Bad Gateway`},
		{`请求失败，URL：https://private.example/v1，其他诊断信息`, `请求失败`},
		{`timeout; URL = "https://private.example/v1"; retry=5`, `timeout`},
		{`502 url: https%3A%2F%2Fprivate.example%2Fv1`, `502`},
		{`url: https://private.example/v1`, `Upstream request failed`},
		{`Invalid url: expected an absolute URL`, `Invalid url: expected an absolute URL`},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			if got := SanitizePublicError(tc.input); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
	var output struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	data := SanitizeErrorJSON([]byte(`{"error":{"message":"502 Bad Gateway, url: https://private.example/v1, cf-ray: trace-NRT"}}`))
	if err := Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Error.Message != "502 Bad Gateway" {
		t.Fatalf("unexpected public JSON: %s", data)
	}
}

func TestSanitizePublicErrorEncodedAddresses(t *testing.T) {
	for _, input := range []string{
		`failed: %68%74%74%70%73%3a%2f%2fprivate%2eexample%2fapi malformed%zz`,
		`failed: https:\u002f\u002fprivate\u002eexample/v1`,
		`failed: https:\x2f\x2fprivate.example/v1`,
		`failed: https&#58;&#47;&#47;private.example/v1`,
		"failed: https://pri\u200bvate.example/v1",
		`failed: 上游.中国:443`,
		`failed: https%25252525253a%25252525252f%25252525252fprivate%25252525252eexample`,
	} {
		clean := SanitizePublicError(input)
		if strings.Contains(clean, "private") || strings.Contains(clean, "上游") {
			t.Fatalf("encoded address leaked: %s", clean)
		}
		if SanitizePublicError(clean) != clean {
			t.Fatalf("not idempotent: %s", clean)
		}
	}
}

func TestSanitizeErrorJSONNestedBusinessFailures(t *testing.T) {
	for _, input := range []string{
		`{"success":false,"message":"502, url: https://private.example/fail","request_url":"https://private.example/fail","data":{"trace":"private.example:443"}}`,
		`{"data":[{"error":{"message":"https://private.example/fail","url":"https://private.example/fail"}}]}`,
		`{"data":{"data":{"status":"failed","code":"https://private.example/fail","debug":"private.example:443"}}}`,
		`{"base_resp":{"status_code":1000,"status_msg":"https://private.example/fail"}}`,
		`{"error":{"message":"failed"},"upstream_url":"https://private.example/fail","debug":{"address":"10.0.0.1:443"}}`,
	} {
		clean := string(SanitizeErrorJSON([]byte(input)))
		if strings.Contains(clean, "private.example") || strings.Contains(clean, "10.0.0.1") || strings.Contains(clean, "request_url") || strings.Contains(clean, "upstream_url") {
			t.Fatalf("diagnostic leaked: %s", clean)
		}
		if string(SanitizeErrorJSON([]byte(clean))) != clean {
			t.Fatalf("not idempotent: %s", clean)
		}
	}
}
