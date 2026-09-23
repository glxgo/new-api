package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublicErrorHistoricalUserLog(t *testing.T) {
	logs := []*Log{{Type: LogTypeError, Content: "502 https://private.example/v1/responses", Other: `{"admin_info":{"origin":"https://private.example"},"stream_status":{"error":"https://private.example"}}`}, {Type: LogTypeConsume, Content: "https://docs.example"}}
	formatUserLogs(logs, 0)
	require.NotContains(t, logs[0].Content, "private.example")
	require.NotContains(t, logs[0].Other, "private.example")
	require.Equal(t, "https://docs.example", logs[1].Content)
}
