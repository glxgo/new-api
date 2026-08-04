package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"golang.org/x/net/proxy"
)

var (
	httpClient              *http.Client
	ssrfProtectedHTTPClient *http.Client
	proxyClientLock         sync.Mutex
	proxyClients            = make(map[string]*http.Client)
)

func checkRedirect(req *http.Request, via []*http.Request) error {
	fetchSetting := system_setting.GetFetchSetting()
	if err := common.ValidateURLWithFetchSetting(req.URL.String(), fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		return fmt.Errorf("redirect to %s blocked: %v", req.URL.String(), err)
	}
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	return nil
}

func relayHTTPTransport() *http.Transport {
	transport := &http.Transport{
		MaxIdleConns:          common.RelayMaxIdleConns,
		MaxIdleConnsPerHost:   common.RelayMaxIdleConnsPerHost,
		IdleConnTimeout:       time.Duration(common.RelayIdleConnTimeout) * time.Second,
		ResponseHeaderTimeout: time.Duration(common.RelayResponseHeaderTimeout) * time.Second,
		ForceAttemptHTTP2:     true,
		Proxy:                 http.ProxyFromEnvironment,
	}
	if common.TLSInsecureSkipVerify {
		transport.TLSClientConfig = common.InsecureTLSConfig.Clone()
	}
	return transport
}

func relayHTTPClient(roundTripper http.RoundTripper) *http.Client {
	client := &http.Client{Transport: roundTripper, CheckRedirect: checkRedirect}
	if common.RelayTimeout != 0 {
		client.Timeout = time.Duration(common.RelayTimeout) * time.Second
	}
	return client
}

func InitHttpClient() {
	policy := defaultHTTPTransportPolicy()
	transport := relayHTTPTransport()
	applyHTTPTransportPolicy(transport, policy)
	httpClient = relayHTTPClient(transport)
	ssrfProtectedHTTPClient = newProtectedFetchHTTPClient()
}

func GetHttpClient() *http.Client {
	if httpClient == nil {
		InitHttpClient()
	}
	return httpClient
}

// GetSSRFProtectedHTTPClient is for operator/user-controlled URL fetches. It
// validates both the URL and the resolved destination immediately before dial.
func GetSSRFProtectedHTTPClient() *http.Client {
	if ssrfProtectedHTTPClient == nil {
		InitHttpClient()
	}
	return ssrfProtectedHTTPClient
}

// GetHttpClientWithProxy returns the default client or a proxy-enabled client.
func GetHttpClientWithProxy(proxyURL string) (*http.Client, error) {
	return GetHttpClientWithProxySettings(proxyURL, dto.ChannelSettings{})
}

func GetHttpClientWithProxySettings(proxyURL string, settings dto.ChannelSettings) (*http.Client, error) {
	proxyURL = strings.TrimSpace(proxyURL)
	policy := NormalizeHTTPTransportPolicy(settings)
	if proxyURL == "" && policy == defaultHTTPTransportPolicy() {
		return GetHttpClient(), nil
	}
	key := proxyURL + "\x00" + policy.cacheKeyPart()
	proxyClientLock.Lock()
	if client, ok := proxyClients[key]; ok {
		proxyClientLock.Unlock()
		return client, nil
	}
	proxyClientLock.Unlock()

	var parsedURL *url.URL
	var err error
	if proxyURL != "" {
		parsedURL, err = url.Parse(proxyURL)
		if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
			return nil, fmt.Errorf("invalid proxy URL: %s", proxyURL)
		}
	}
	transport, err := newRelayTransportForProxy(parsedURL, policy)
	if err != nil {
		return nil, err
	}
	var roundTripper http.RoundTripper = transport
	if policy.Shards > 1 && policy.Protocol != dto.HTTPProtocolHTTP1 {
		roundTripper = newShardedRoundTripper(policy, func() *http.Transport {
			t, factoryErr := newRelayTransportForProxy(parsedURL, policy)
			if factoryErr != nil {
				return relayHTTPTransport()
			}
			return t
		})
	}
	client := relayHTTPClient(roundTripper)
	proxyClientLock.Lock()
	if old, ok := proxyClients[key]; ok {
		proxyClientLock.Unlock()
		if closer, ok := roundTripper.(interface{ CloseIdleConnections() }); ok {
			closer.CloseIdleConnections()
		}
		return old, nil
	}
	proxyClients[key] = client
	proxyClientLock.Unlock()
	return client, nil
}

func newRelayTransportForProxy(proxyURL *url.URL, policy HTTPTransportPolicy) (*http.Transport, error) {
	transport := relayHTTPTransport()
	if proxyURL == nil {
		applyHTTPTransportPolicy(transport, policy)
		return transport, nil
	}
	switch strings.ToLower(proxyURL.Scheme) {
	case "http", "https":
		transport.Proxy = http.ProxyURL(proxyURL)
		applyHTTPTransportPolicy(transport, policy)
		return transport, nil
	case "socks5", "socks5h":
		var auth *proxy.Auth
		if proxyURL.User != nil {
			auth = &proxy.Auth{User: proxyURL.User.Username()}
			if password, ok := proxyURL.User.Password(); ok {
				auth.Password = password
			}
		}
		dialer, err := proxy.SOCKS5("tcp", proxyURL.Host, auth, proxy.Direct)
		if err != nil {
			return nil, err
		}
		transport.Proxy = nil
		if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
			transport.DialContext = contextDialer.DialContext
		} else {
			transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
				type result struct {
					conn net.Conn
					err  error
				}
				ch := make(chan result, 1)
				go func() {
					conn, err := dialer.Dial(network, address)
					ch <- result{conn: conn, err: err}
				}()
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case result := <-ch:
					return result.conn, result.err
				}
			}
		}
		applyHTTPTransportPolicy(transport, policy)
		return transport, nil
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s, must be http, https, socks5 or socks5h", proxyURL.Scheme)
	}
}

// ResetProxyClientCache clears cached channel clients so changed settings take
// effect without requiring a process restart.
func ResetProxyClientCache() {
	proxyClientLock.Lock()
	defer proxyClientLock.Unlock()
	for _, client := range proxyClients {
		if closer, ok := client.Transport.(interface{ CloseIdleConnections() }); ok {
			closer.CloseIdleConnections()
		}
	}
	proxyClients = make(map[string]*http.Client)
}

func NewProxyHttpClient(proxyURL string) (*http.Client, error) {
	return GetHttpClientWithProxySettings(proxyURL, dto.ChannelSettings{})
}

func ValidateSSRFProtectedFetchURL(rawURL string) error {
	setting := system_setting.GetFetchSetting()
	return common.ValidateURLWithFetchSetting(rawURL, setting.EnableSSRFProtection, setting.AllowPrivateIp, setting.DomainFilterMode, setting.IpFilterMode, setting.DomainList, setting.IpList, setting.AllowedPorts, setting.ApplyIPFilterForDomain)
}

func checkProtectedFetchRedirect(req *http.Request, via []*http.Request) error {
	if err := ValidateSSRFProtectedFetchURL(req.URL.String()); err != nil {
		return fmt.Errorf("redirect to %s blocked: %v", req.URL.String(), err)
	}
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	return nil
}
