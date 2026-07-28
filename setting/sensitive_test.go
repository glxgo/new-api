package setting

import (
	"strings"
	"testing"
)

func TestDefaultJailbreakWordsAreNormalizedAndComplete(t *testing.T) {
	if got := len(DefaultJailbreakWords); got != 70 {
		t.Fatalf("default jailbreak word count = %d, want 70", got)
	}
	for _, word := range DefaultJailbreakWords {
		if word == "" || word != strings.TrimSpace(word) || word != strings.ToLower(word) {
			t.Fatalf("default jailbreak word is not normalized: %q", word)
		}
	}
}

func TestSensitiveWordsFromStringNormalizesAndDeduplicates(t *testing.T) {
	original := SensitiveWords
	t.Cleanup(func() {
		SensitiveWords = original
	})

	SensitiveWordsFromString("  Reveal System Prompt  \r\nreveal system prompt\n开启越狱模式\n")

	if got := len(SensitiveWords); got != 2 {
		t.Fatalf("normalized word count = %d, want 2: %#v", got, SensitiveWords)
	}
	if SensitiveWords[0] != "reveal system prompt" {
		t.Fatalf("first normalized word = %q", SensitiveWords[0])
	}
	if SensitiveWords[1] != "开启越狱模式" {
		t.Fatalf("second normalized word = %q", SensitiveWords[1])
	}
}
