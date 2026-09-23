package types

import (
	"errors"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestPublicErrorAllFieldsAreSanitized(t *testing.T) {
	raw := "Post https://private-upstream.example:8443/v1/responses?key=secret: dial tcp [2001:db8::7]:443: refused"
	err := WithOpenAIError(OpenAIError{Message: raw, Type: "https://private-upstream.example/type", Param: "https://private-upstream.example/param", Code: map[string]any{"url": "https://private-upstream.example/code"}, Metadata: []byte(`{"provider":{"url":"https://private-upstream.example/meta"}}`)}, 502)
	for _, output := range []any{err.ToOpenAIError(), err.ToClaudeError(), NewError(errors.New(raw), ErrorCodeCountTokenFailed).ToOpenAIError()} {
		data, marshalErr := common.Marshal(output)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		for _, secret := range []string{"private-upstream", "2001:db8", "key=secret"} {
			if strings.Contains(string(data), secret) {
				t.Fatalf("public output leaked %q: %s", secret, data)
			}
		}
	}
	if !strings.Contains(err.Error(), "private-upstream") {
		t.Fatal("internal diagnostic was changed")
	}
}
