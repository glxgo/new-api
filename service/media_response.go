package service

import (
	"bufio"
	"io"
	"net/http"
	"strings"
)

// IsMediaDiagnostic detects text/JSON gateway errors returned as HTTP 200 by
// binary media endpoints, including bodies mislabeled as audio or video.
func IsMediaDiagnostic(contentType string, prefix []byte) bool {
	for _, value := range []string{contentType, http.DetectContentType(prefix)} {
		value = strings.ToLower(strings.TrimSpace(strings.SplitN(value, ";", 2)[0]))
		if value == "text/html" || value == "text/plain" || value == "application/json" || strings.HasSuffix(value, "+json") {
			return true
		}
	}
	return false
}

type bufferedMediaBody struct {
	io.Reader
	io.Closer
}

// HasMediaDiagnostic peeks at a bounded prefix and preserves the original body
// and closer for the normal streaming path.
func HasMediaDiagnostic(resp *http.Response) bool {
	reader := bufio.NewReader(resp.Body)
	prefix, _ := reader.Peek(512)
	resp.Body = bufferedMediaBody{Reader: reader, Closer: resp.Body}
	return IsMediaDiagnostic(resp.Header.Get("Content-Type"), prefix)
}
