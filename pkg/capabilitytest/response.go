package capabilitytest

import (
	"errors"
	"strings"

	"github.com/tidwall/gjson"
)

func ParseResponse(raw []byte, responses bool) (string, string, error) {
	text := strings.TrimSpace(string(raw))
	usage := ""
	if len(raw) > 2<<20 {
		return "", "", errors.New("response_too_large")
	}
	if gjson.Valid(text) {
		return parseCompletedJSON(text, responses)
	}
	var answer strings.Builder
	finished, terminal := false, false
	events, err := streamEvents(text)
	if err != nil {
		return "", "", err
	}
	for _, data := range events {
		if terminal {
			return "", "", errors.New("data_after_terminal")
		}
		if data == "[DONE]" {
			terminal = true
			continue
		}
		if !gjson.Valid(data) {
			return "", "", errors.New("invalid_stream_event")
		}
		obj := gjson.Parse(data)
		if obj.Get("error").Exists() && obj.Get("error").Type != gjson.Null {
			return "", "", errors.New("upstream_error")
		}
		if responses {
			switch obj.Get("type").String() {
			case "response.output_text.delta":
				answer.WriteString(obj.Get("delta").String())
			case "response.failed", "response.incomplete", "error":
				return "", "", errors.New("response_incomplete")
			case "response.completed":
				a, u, err := parseCompletedJSON(obj.Get("response").Raw, true)
				if err != nil {
					return "", "", err
				}
				if a != "" {
					answer.Reset()
					answer.WriteString(a)
				}
				usage = u
				finished = true
				terminal = true
			}
		} else {
			choices := obj.Get("choices").Array()
			if len(choices) > 1 {
				return "", "", errors.New("multiple_completions")
			}
			if len(choices) == 1 {
				choice := choices[0]
				if finished && choice.Get("delta.content").String() != "" {
					return "", "", errors.New("data_after_finish")
				}
				if choice.Get("index").Int() != 0 {
					return "", "", errors.New("multiple_completions")
				}
				answer.WriteString(choice.Get("delta.content").String())
				reason := choice.Get("finish_reason").String()
				if reason != "" {
					if reason != "stop" {
						return "", "", errors.New("response_incomplete")
					}
					finished = true
				}
			}
			if obj.Get("usage").IsObject() {
				usage = obj.Get("usage").Raw
			}
		}
		if answer.Len() > MaxAnswerBytes {
			return "", "", errors.New("answer_too_large")
		}
	}
	if !finished || !terminal || strings.TrimSpace(answer.String()) == "" {
		return "", "", errors.New("response_incomplete")
	}
	return answer.String(), usage, nil
}
func parseCompletedJSON(text string, responses bool) (string, string, error) {
	if !gjson.Valid(text) {
		return "", "", errors.New("invalid_response_json")
	}
	obj := gjson.Parse(text)
	if obj.Get("error").Exists() && obj.Get("error").Type != gjson.Null {
		return "", "", errors.New("upstream_error")
	}
	var answer strings.Builder
	if responses {
		if obj.Get("status").String() != "completed" {
			return "", "", errors.New("response_incomplete")
		}
		for _, item := range obj.Get("output").Array() {
			if item.Get("type").String() != "message" {
				continue
			}
			if item.Get("status").String() != "completed" {
				return "", "", errors.New("response_incomplete")
			}
			for _, part := range item.Get("content").Array() {
				if part.Get("type").String() == "output_text" {
					answer.WriteString(part.Get("text").String())
				}
			}
		}
	} else {
		choices := obj.Get("choices").Array()
		if len(choices) != 1 || choices[0].Get("finish_reason").String() != "stop" {
			return "", "", errors.New("response_incomplete")
		}
		content := choices[0].Get("message.content")
		if content.Type != gjson.String {
			return "", "", errors.New("invalid_response_content")
		}
		answer.WriteString(content.String())
	}
	if answer.Len() > MaxAnswerBytes {
		return "", "", errors.New("answer_too_large")
	}
	if strings.TrimSpace(answer.String()) == "" {
		return "", "", errors.New("empty_response")
	}
	usage := ""
	if obj.Get("usage").IsObject() {
		usage = obj.Get("usage").Raw
	}
	return answer.String(), usage, nil
}

// SSE joins all data fields in an event with newlines. An unterminated final
// event is discarded according to the wire protocol, never counted complete.
func streamEvents(text string) ([]string, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var events []string
	var data []string
	flush := func() {
		if len(data) > 0 {
			events = append(events, strings.Join(data, "\n"))
			data = nil
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			value := strings.TrimPrefix(line, "data:")
			value = strings.TrimPrefix(value, " ")
			data = append(data, value)
		}
	}
	// Normalized adaptor output may omit the final blank line, but only a
	// complete explicit terminal event can be accepted in that position.
	if len(data) > 0 {
		value := strings.Join(data, "\n")
		if value != "[DONE]" && gjson.Get(value, "type").String() != "response.completed" {
			return nil, errors.New("unterminated_stream")
		}
		flush()
	}
	return events, nil
}
func HasReportedUsage(raw []byte) bool {
	return ReportedUsage(raw) != ""
}

func ReportedUsage(raw []byte) string {
	if gjson.ValidBytes(raw) {
		v := gjson.ParseBytes(raw)
		for _, key := range []string{"usage", "usageMetadata"} {
			if u := v.Get(key); u.IsObject() {
				return u.Raw
			}
		}
		return ""
	}
	events, err := streamEvents(strings.TrimSpace(string(raw)))
	if err != nil {
		return ""
	}
	usages := []string{}
	for _, event := range events {
		v := gjson.Parse(event)
		for _, key := range []string{"usage", "response.usage", "message.usage", "usageMetadata"} {
			if u := v.Get(key); u.IsObject() {
				usages = append(usages, u.Raw)
				break
			}
		}
	}
	// Preserve incremental provider reports separately; adding them would
	// double-count cumulative usage on several protocols.
	if len(usages) == 0 {
		return ""
	}
	if len(usages) == 1 {
		return usages[0]
	}
	return "[" + strings.Join(usages, ",") + "]"
}
