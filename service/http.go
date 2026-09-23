package service

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"github.com/gin-gonic/gin"
)

func CloseResponseBodyGracefully(httpResponse *http.Response) {
	if httpResponse == nil || httpResponse.Body == nil {
		return
	}
	err := httpResponse.Body.Close()
	if err != nil {
		common.SysError("failed to close response body: " + err.Error())
	}
}

// ShouldCopyUpstreamHeader checks whether a given upstream response header
// should be copied to the client response. It returns false for Content-Length
// (managed separately) and X-Oneapi-Request-Id (to preserve the local instance
// ID). When the upstream header is X-Oneapi-Request-Id, the value is captured
// into the Gin context for later logging.
func ShouldCopyUpstreamHeader(c *gin.Context, k string, v []string) bool {
	if strings.EqualFold(k, "Content-Length") {
		return false
	}
	if strings.EqualFold(k, common.RequestIdKey) {
		if c != nil && len(v) > 0 {
			c.Set(common.UpstreamRequestIdKey, v[0])
		}
		return false
	}
	// Only protocol and quota headers are public. Provider diagnostics, routing,
	// cookies and redirects may disclose the origin even on HTTP 200 responses.
	for _, value := range v {
		if common.SanitizePublicError(value) != value {
			return false
		}
	}
	key := strings.ToLower(k)
	return key == "content-type" || key == "content-disposition" || key == "content-range" || key == "accept-ranges" || key == "content-encoding" ||
		key == "cache-control" || key == "etag" || key == "expires" || key == "last-modified" || key == "openai-version" ||
		key == "retry-after" || key == "request-id" || key == "x-request-id" ||
		key == "openai-processing-ms" || strings.HasPrefix(key, "x-ratelimit-") ||
		strings.HasPrefix(key, "anthropic-ratelimit-")
}

func IOCopyBytesGracefully(c *gin.Context, src *http.Response, data []byte) {
	if c.Writer == nil {
		return
	}

	originalData := data
	if src != nil && src.StatusCode >= http.StatusBadRequest {
		data = common.SanitizeHTTPErrorBody(data)
	} else {
		data = common.SanitizeErrorJSON(data)
	}
	body := io.NopCloser(bytes.NewBuffer(data))

	// We shouldn't set the header before we parse the response body, because the parse part may fail.
	// And then we will have to send an error response, but in this case, the header has already been set.
	// So the httpClient will be confused by the response.
	// For example, Postman will report error, and we cannot check the response at all.
	if src != nil {
		for k, v := range src.Header {
			if !ShouldCopyUpstreamHeader(c, k, v) || len(v) == 0 {
				continue
			}
			c.Writer.Header().Set(k, v[0])
		}
		if !bytes.Equal(originalData, data) {
			c.Writer.Header().Del("Content-Encoding")
		}
		if src.StatusCode >= http.StatusBadRequest {
			c.Writer.Header().Set("Content-Type", "application/json")
			c.Writer.Header().Del("Content-Encoding")
		}
	}

	// set Content-Length header manually BEFORE calling WriteHeader
	c.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))

	// Write header with status code (this sends the headers)
	if src != nil {
		c.Writer.WriteHeader(src.StatusCode)
	} else {
		c.Writer.WriteHeader(http.StatusOK)
	}

	_, err := io.Copy(c.Writer, body)
	if err != nil {
		logger.LogError(c, fmt.Sprintf("failed to copy response body: %s", err.Error()))
	}
	c.Writer.Flush()
}
