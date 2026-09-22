// Package pelicanarchive describes the external site's saved facts. It never
// calls a model or grades an answer.
package pelicanarchive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/QuantumNous/new-api/common"
)

const MaxSnapshotBytes = 32 << 20
const Schema = 1

type Config struct {
	AutoRun         bool   `json:"auto_run"`
	IntervalMinutes int    `json:"interval_minutes"`
	ExpectedAnswer  int    `json:"expected_answer"`
	Prompt          string `json:"prompt"`
	PromptHash      string `json:"prompt_hash"`
	BuiltinExpected *int   `json:"builtin_expected,omitempty"`
	UsingBuiltin    bool   `json:"using_builtin"`
}
type Target struct {
	ProviderID      string `json:"provider_id"`
	Model           string `json:"model_name"`
	Name            string `json:"provider_name"`
	IntervalMinutes int    `json:"interval_minutes"`
	Enabled         int    `json:"enabled"`
}
type Run struct {
	ID           int64  `json:"id"`
	ProviderID   string `json:"provider_id"`
	Model        string `json:"model_name"`
	Name         string `json:"provider_name"`
	Grade        string `json:"grade"`
	Expected     int    `json:"expected_answer"`
	Reported     *int   `json:"reported_answer"`
	AnswerInSVG  int    `json:"answer_in_svg"`
	AnswerInText int    `json:"answer_in_text"`
	SVG          string `json:"svg"`
	SVGBytes     int    `json:"svg_bytes"`
	Truncated    int    `json:"svg_truncated"`
	RawText      string `json:"raw_text"`
	LatencyMS    int64  `json:"latency_ms"`
	TTFTMS       int64  `json:"ttft_ms"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	Attempts     int    `json:"attempts"`
	Error        string `json:"error"`
	PromptHash   string `json:"prompt_hash"`
	CreatedAt    string `json:"created_at"`
}
type Snapshot struct {
	Schema     int      `json:"schema_version"`
	SourceID   string   `json:"source_id"`
	CapturedAt string   `json:"captured_at"`
	Config     Config   `json:"config"`
	Targets    []Target `json:"targets"`
	Runs       []Run    `json:"runs"`
}

func Hash(raw []byte) string { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }

// Source uses FNV-1a over JavaScript UTF-16 code units. This is a version hint,
// NOT cryptographic proof that an old prompt was archived.
func PromptHash(prompt string) string {
	h := uint32(0x811c9dc5)
	for _, c := range utf16.Encode([]rune(prompt)) {
		h ^= uint32(c)
		h *= 0x01000193
	}
	return fmt.Sprintf("%08x", h)
}
func Decode(r io.Reader, source string, now time.Time) (Snapshot, error) {
	var s Snapshot
	raw, err := io.ReadAll(io.LimitReader(r, MaxSnapshotBytes+1))
	if err != nil {
		return s, err
	}
	if len(raw) > MaxSnapshotBytes {
		return s, errors.New("snapshot_too_large")
	}
	if err = common.Unmarshal(raw, &s); err != nil {
		return s, errors.New("invalid_snapshot_json")
	}
	if s.Schema != Schema || s.SourceID == "" || s.SourceID != source {
		return s, errors.New("source_or_schema_mismatch")
	}
	captured, err := time.Parse(time.RFC3339Nano, s.CapturedAt)
	if err != nil || captured.After(now.Add(5*time.Minute)) || now.Sub(captured) > 24*time.Hour {
		return s, errors.New("snapshot_time_invalid_or_expired")
	}
	if len(s.Targets) > 2000 || len(s.Runs) > 10000 || len(s.Config.Prompt) > 128000 || s.Config.IntervalMinutes < 1 || s.Config.IntervalMinutes > 10080 {
		return s, errors.New("snapshot_limits_exceeded")
	}
	if s.Config.PromptHash != PromptHash(s.Config.Prompt) {
		return s, errors.New("current_prompt_hash_mismatch")
	}
	seen := map[int64]bool{}
	targets := map[string]bool{}
	for _, t := range s.Targets {
		k := t.ProviderID + "\x00" + t.Model
		if !identity(t.ProviderID, 191) || !identity(t.Model, 255) || len(t.Name) > 1000 || targets[k] || t.IntervalMinutes < 0 || t.IntervalMinutes > 10080 || (t.Enabled != 0 && t.Enabled != 1) {
			return s, errors.New("invalid_target")
		}
		targets[k] = true
	}
	for _, v := range s.Runs {
		if v.ID <= 0 || seen[v.ID] || !identity(v.ProviderID, 191) || !identity(v.Model, 255) || len(v.Name) > 1000 || len(v.SVG) > 800000 || len(v.RawText) > 100000 || len(v.Error) > 16000 || len(v.PromptHash) > 64 {
			return s, errors.New("invalid_record")
		}
		seen[v.ID] = true
		if _, err = time.Parse(time.RFC3339Nano, v.CreatedAt); err != nil {
			return s, errors.New("invalid_record_time")
		}
		switch v.Grade {
		case "correct", "wrong", "no_svg", "no_answer", "error":
		default:
			return s, errors.New("unsupported_grade")
		}
		if v.LatencyMS < 0 || v.TTFTMS < 0 || v.InputTokens < 0 || v.OutputTokens < 0 || v.Attempts < 1 || v.Attempts > 100 {
			return s, errors.New("invalid_record_metrics")
		}
	}
	return s, nil
}
func identity(v string, max int) bool {
	return strings.TrimSpace(v) != "" && len(v) <= max && !strings.ContainsAny(v, "\x00\r\n")
}

// Defense in depth: artifacts are served as images with a restrictive CSP and
// never inserted as HTML. Reject active XML instead of silently altering facts.
func SafeSVG(s string) bool {
	if s == "" || len(s) > 800000 {
		return false
	}
	d := xml.NewDecoder(strings.NewReader(s))
	depth, roots, tokens := 0, 0, 0
	type frame struct {
		id    string
		start int
	}
	var stack []frame
	var references []string
	sizes, children := map[string]int{}, map[string][]string{}
	for {
		tok, err := d.Token()
		if err == io.EOF {
			// Allow bounded local SVG reuse, but reject cycles and
			// expansion bombs. No external resource is fetched or rewritten.
			memo, visiting := map[string]int{}, map[string]bool{}
			var expand func(string, int) int
			expand = func(id string, depth int) int {
				if depth > 100 || visiting[id] {
					return 100001
				}
				if n, ok := memo[id]; ok {
					return n
				}
				n, ok := sizes[id]
				if !ok {
					return 100001
				}
				visiting[id] = true
				for _, child := range children[id] {
					n += expand(child, depth+1)
					if n > 100000 {
						return 100001
					}
				}
				visiting[id] = false
				memo[id] = n
				return n
			}
			expanded := tokens
			for _, ref := range references {
				expanded += expand(ref, 0)
				if expanded > 100000 {
					return false
				}
			}
			return depth == 0 && roots == 1
		}
		if err != nil {
			return false
		}
		tokens++
		if tokens > 80000 {
			return false
		}
		switch v := tok.(type) {
		case xml.Directive, xml.ProcInst:
			return false
		case xml.StartElement:
			name := strings.ToLower(v.Name.Local)
			if depth == 0 {
				roots++
				if name != "svg" || roots != 1 {
					return false
				}
			}
			depth++
			if depth > 100 {
				return false
			}
			switch name {
			case "script", "foreignobject", "iframe", "embed", "object", "a", "image", "audio", "video", "set", "animate", "animatemotion", "animatetransform":
				return false
			}
			id := ""
			for _, a := range v.Attr {
				if a.Name.Local == "id" {
					id = a.Value
					if _, exists := sizes[id]; exists {
						return false
					}
					sizes[id] = 0
				}
			}
			stack = append(stack, frame{id: id, start: tokens})
			for _, a := range v.Attr {
				n := strings.ToLower(a.Name.Local)
				value := strings.ToLower(strings.TrimSpace(a.Value))
				if n == "href" {
					ref := strings.TrimSpace(a.Value)
					if name != "use" || !strings.HasPrefix(ref, "#") || len(ref) < 2 || len(ref) > 192 || strings.ContainsAny(ref, "\t\r\n ") {
						return false
					}
					references = append(references, ref[1:])
					for _, f := range stack {
						if f.id != "" {
							children[f.id] = append(children[f.id], ref[1:])
						}
					}
				}
				if strings.HasPrefix(n, "on") || n == "src" || n == "base" {
					return false
				}
				if n == "style" && unsafeCSS(value) {
					return false
				}
				if strings.Contains(value, "url(") && unsafeCSS(value) {
					return false
				}
			}
		case xml.EndElement:
			f := stack[len(stack)-1]
			if f.id != "" {
				sizes[f.id] = tokens - f.start + 1
			}
			stack = stack[:len(stack)-1]
			depth--
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(v)) != "" {
				return false
			}
			if unsafeCSS(strings.ToLower(string(v))) {
				return false
			}
		}
	}
}
func unsafeCSS(v string) bool {
	// Reject escapes/imports and all non-fragment CSS URLs. Image rendering is
	// also isolated by browser image mode, even if a future CSS construct exists.
	if strings.ContainsAny(v, "\\") || strings.Contains(v, "@import") {
		return true
	}
	for {
		i := strings.Index(v, "url(")
		if i < 0 {
			return false
		}
		v = v[i+4:]
		end := strings.IndexByte(v, ')')
		if end < 0 {
			return true
		}
		url := strings.Trim(strings.TrimSpace(v[:end]), "\"'")
		if !strings.HasPrefix(url, "#") {
			return true
		}
		v = v[end+1:]
	}
}
