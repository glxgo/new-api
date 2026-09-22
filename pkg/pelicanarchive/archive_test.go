package pelicanarchive

import (
	"bytes"
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func TestPelicanArchiveRejectsUnsafeAndPartialInput(t *testing.T) {
	now := time.Now()
	s := Snapshot{Schema: 1, SourceID: "fixture", CapturedAt: now.Format(time.RFC3339Nano), Config: Config{IntervalMinutes: 60, Prompt: "测试🦤", PromptHash: PromptHash("测试🦤")}, Targets: []Target{{ProviderID: "p", Model: "m", Enabled: 1}}, Runs: []Run{{ID: 1, ProviderID: "p", Model: "m", Grade: "wrong", Attempts: 1, CreatedAt: now.Format(time.RFC3339Nano)}}}
	raw, err := common.Marshal(s)
	require.NoError(t, err)
	got, err := Decode(bytes.NewReader(raw), "fixture", now)
	require.NoError(t, err)
	require.Equal(t, s.Runs, got.Runs)
	_, err = Decode(bytes.NewReader(raw), "other", now)
	require.Error(t, err)
	_, err = Decode(bytes.NewReader(raw[:len(raw)-1]), "fixture", now)
	require.Error(t, err)
	_, err = Decode(bytes.NewReader(raw), "fixture", now.Add(25*time.Hour))
	require.Error(t, err)
	s.Runs = append(s.Runs, s.Runs[0])
	raw, _ = common.Marshal(s)
	_, err = Decode(bytes.NewReader(raw), "fixture", now)
	require.Error(t, err)
}
func TestPelicanSVGImageBoundary(t *testing.T) {
	require.True(t, SafeSVG(`<svg xmlns="http://www.w3.org/2000/svg"><defs><g id="Star"><path d="M0 0L1 1"/></g></defs><use href="#Star"/></svg>`))
	require.True(t, SafeSVG(`<svg><g id="x"><use href="#y"/></g><path id="y"/><use href="#x"/></svg>`))
	for _, s := range []string{`<svg><use href="https://external/x.svg#x"/></svg>`, `<svg><g id="x"><use href="#x"/></g></svg>`, `<svg><g id="x"><use href="#y"/></g><g id="y"><use href="#x"/></g></svg>`, `<svg><path id="x"/><path id="x"/></svg>`} {
		require.False(t, SafeSVG(s), s)
	}
	var bomb strings.Builder
	bomb.WriteString(`<svg><path id="a0"/>`)
	for i := 1; i < 25; i++ {
		fmt.Fprintf(&bomb, `<g id="a%d"><use href="#a%d"/><use href="#a%d"/></g>`, i, i-1, i-1)
	}
	bomb.WriteString(`<use href="#a24"/></svg>`)
	require.False(t, SafeSVG(bomb.String()))
	require.True(t, SafeSVG(`<svg xmlns="http://www.w3.org/2000/svg"><defs><linearGradient id="a"/></defs><rect fill="url(#a)"/><text>21</text></svg>`))
	for _, s := range []string{`<svg><script>alert(1)</script></svg>`, `<svg onload="x()"/>`, `<svg><foreignObject/></svg>`, `<svg><style>@import 'https://x';</style></svg>`, `<svg><rect fill="url(https://x)"/></svg>`, `<svg><image href="data:image/svg+xml;base64,x"/></svg>`, `<!DOCTYPE svg><svg/>`, `<svg/><svg/>`, `<svg><text>`, `<svg><use href="#x"/></svg>`} {
		require.False(t, SafeSVG(s), s)
	}
}
