package channel

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

func TestApplyUpstreamBodyMetadataBindsIndependentReplayBody(t *testing.T) {
	payload := []byte(`{"model":"gpt-test","input":"replay-me"}`)
	info := &relaycommon.RelayInfo{
		UpstreamRequestBodySize: int64(len(payload)),
		UpstreamRequestGetBody: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(payload)), nil
		},
	}
	request, err := http.NewRequest(http.MethodPost, "https://example.com/v1/responses", struct{ io.Reader }{bytes.NewReader(payload)})
	require.NoError(t, err)
	require.Nil(t, request.GetBody)

	ApplyUpstreamBodyMetadata(request, info)

	require.Equal(t, int64(len(payload)), request.ContentLength)
	require.NotNil(t, request.GetBody)
	first, err := request.GetBody()
	require.NoError(t, err)
	second, err := request.GetBody()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = first.Close()
		_ = second.Close()
	})
	firstBytes, err := io.ReadAll(first)
	require.NoError(t, err)
	secondBytes, err := io.ReadAll(second)
	require.NoError(t, err)
	require.Equal(t, payload, firstBytes)
	require.Equal(t, payload, secondBytes)
}

type localH2ReplayResult struct {
	err    error
	bodies [][]byte
}

func acceptLocalH2Connection(listener net.Listener) (net.Conn, *http2.Framer, error) {
	connection, err := listener.Accept()
	if err != nil {
		return nil, nil, err
	}
	_ = connection.SetDeadline(time.Now().Add(10 * time.Second))
	preface := make([]byte, len(http2.ClientPreface))
	if _, err := io.ReadFull(connection, preface); err != nil {
		_ = connection.Close()
		return nil, nil, err
	}
	if !bytes.Equal(preface, []byte(http2.ClientPreface)) {
		_ = connection.Close()
		return nil, nil, fmt.Errorf("unexpected HTTP/2 preface")
	}
	framer := http2.NewFramer(connection, connection)
	framer.ReadMetaHeaders = hpack.NewDecoder(4096, nil)
	if err := framer.WriteSettings(); err != nil {
		_ = connection.Close()
		return nil, nil, err
	}
	return connection, framer, nil
}

func readLocalH2Request(framer *http2.Framer) (uint32, []byte, error) {
	var streamID uint32
	var body []byte
	for {
		frame, err := framer.ReadFrame()
		if err != nil {
			return 0, nil, err
		}
		switch typed := frame.(type) {
		case *http2.SettingsFrame:
			if !typed.IsAck() {
				if err := framer.WriteSettingsAck(); err != nil {
					return 0, nil, err
				}
			}
		case *http2.MetaHeadersFrame:
			streamID = typed.Header().StreamID
			if typed.StreamEnded() {
				return streamID, body, nil
			}
		case *http2.DataFrame:
			if streamID == 0 {
				streamID = typed.Header().StreamID
			}
			if typed.Header().StreamID != streamID {
				continue
			}
			body = append(body, typed.Data()...)
			if typed.StreamEnded() {
				return streamID, body, nil
			}
		}
	}
}

func writeLocalH2Response(framer *http2.Framer, streamID uint32) error {
	var header bytes.Buffer
	encoder := hpack.NewEncoder(&header)
	if err := encoder.WriteField(hpack.HeaderField{Name: ":status", Value: "200"}); err != nil {
		return err
	}
	if err := framer.WriteHeaders(http2.HeadersFrameParam{
		StreamID:      streamID,
		BlockFragment: header.Bytes(),
		EndHeaders:    true,
	}); err != nil {
		return err
	}
	return framer.WriteData(streamID, true, []byte(`{}`))
}

func runLocalH2ResetServer(listener net.Listener) <-chan localH2ReplayResult {
	resultCh := make(chan localH2ReplayResult, 1)
	go func() {
		result := localH2ReplayResult{}
		defer func() { resultCh <- result }()
		connection, framer, err := acceptLocalH2Connection(listener)
		if err != nil {
			result.err = err
			return
		}
		defer connection.Close()
		for attempt := 0; attempt < 2; attempt++ {
			streamID, body, err := readLocalH2Request(framer)
			if err != nil {
				result.err = err
				return
			}
			result.bodies = append(result.bodies, body)
			if attempt == 0 {
				if err := framer.WriteRSTStream(streamID, http2.ErrCodeRefusedStream); err != nil {
					result.err = err
					return
				}
				continue
			}
			result.err = writeLocalH2Response(framer, streamID)
			return
		}
	}()
	return resultCh
}

func TestUpstreamGetBodyHTTP2RetriesFullBodyAfterRefusedStream(t *testing.T) {
	payload := []byte(`{"model":"gpt-test","input":"retry-me"}`)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	resultCh := runLocalH2ResetServer(listener)

	transport := &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, network, _ string, _ *tls.Config) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, network, listener.Addr().String())
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}

	body, size, getBody, closer, err := relaycommon.NewOutboundJSONBody(payload)
	require.NoError(t, err)
	defer closer.Close()
	request, err := http.NewRequest(http.MethodPost, "http://upstream.test/v1/responses", body)
	require.NoError(t, err)
	ApplyUpstreamBodyMetadata(request, &relaycommon.RelayInfo{
		UpstreamRequestBodySize: size,
		UpstreamRequestGetBody:  getBody,
	})

	response, err := client.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)

	select {
	case result := <-resultCh:
		require.NoError(t, result.err)
		require.Len(t, result.bodies, 2)
		require.Equal(t, payload, result.bodies[0])
		require.Equal(t, payload, result.bodies[1])
	case <-time.After(12 * time.Second):
		t.Fatal("timed out waiting for HTTP/2 replay server")
	}
}
