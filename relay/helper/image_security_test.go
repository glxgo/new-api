package helper

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/i18n"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSecurityImageCountBounds(t *testing.T) {
	require.NoError(t, i18n.Init())
	for _, form := range []bool{false, true} {
		for _, n := range []string{"128", "129", "18446744073709551615", "-1"} {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			mode := relayconstant.RelayModeImagesGenerations
			if form {
				var body bytes.Buffer
				writer := multipart.NewWriter(&body)
				require.NoError(t, writer.WriteField("model", "image-model"))
				require.NoError(t, writer.WriteField("n", n))
				require.NoError(t, writer.Close())
				c.Request = httptest.NewRequest("POST", "/v1/images/edits", &body)
				c.Request.Header.Set("Content-Type", writer.FormDataContentType())
				mode = relayconstant.RelayModeImagesEdits
			} else {
				c.Request = httptest.NewRequest("POST", "/v1/images/generations", strings.NewReader(fmt.Sprintf(`{"model":"image-model","n":%s}`, n)))
				c.Request.Header.Set("Content-Type", "application/json")
			}
			_, err := GetAndValidOpenAIImageRequest(c, mode)
			if n == "128" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		}
	}
}
