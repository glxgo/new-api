package service

import (
	"context"
	"net"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

type staticSSRFResolver struct {
	addresses []net.IPAddr
}

func (r staticSSRFResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return r.addresses, nil
}

func TestProtectedFetchDialerRejectsPrivateDNSAnswer(t *testing.T) {
	dialCalled := false
	dialer := &protectedFetchDialer{
		resolver: staticSSRFResolver{addresses: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}},
		dialContext: func(context.Context, string, string) (net.Conn, error) {
			dialCalled = true
			return nil, nil
		},
		getProtection: func() (*common.SSRFProtection, bool, error) {
			return &common.SSRFProtection{
				DomainFilterMode:       false,
				IpFilterMode:           false,
				AllowedPorts:           []int{80},
				ApplyIPFilterForDomain: true,
			}, true, nil
		},
	}
	_, err := dialer.DialContext(context.Background(), "tcp", "public.example:80")
	require.Error(t, err)
	require.Contains(t, err.Error(), "private IP")
	require.False(t, dialCalled)
}
