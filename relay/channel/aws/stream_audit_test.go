package aws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestStreamAuditBedrockTransportErrorIsNotSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		_, _ = w.Write([]byte("truncated event"))
	}))
	defer server.Close()
	client := bedrockruntime.New(bedrockruntime.Options{Region: "us-east-1", BaseEndpoint: aws.String(server.URL), Credentials: credentials.NewStaticCredentialsProvider("test", "test", "")})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	info := &relaycommon.RelayInfo{IsStream: true, RelayFormat: types.RelayFormatOpenAI, ChannelMeta: &relaycommon.ChannelMeta{}}
	apiErr, _ := awsStreamHandler(c, info, &Adaptor{AwsClient: client, AwsReq: &bedrockruntime.InvokeModelWithResponseStreamInput{ModelId: aws.String("test"), Body: []byte("{}")}})
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.NotContains(t, w.Body.String(), "[DONE]")
}

func TestStreamAuditBedrockRespectsCanceledRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := bedrockruntime.New(bedrockruntime.Options{Region: "us-east-1", BaseEndpoint: aws.String("http://127.0.0.1:1"), Credentials: credentials.NewStaticCredentialsProvider("test", "test", "")})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
	apiErr, _ := awsStreamHandler(c, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}, &Adaptor{AwsClient: client, AwsReq: &bedrockruntime.InvokeModelWithResponseStreamInput{ModelId: aws.String("test"), Body: []byte("{}")}})
	require.NotNil(t, apiErr)
	require.Equal(t, 499, apiErr.StatusCode)
}
