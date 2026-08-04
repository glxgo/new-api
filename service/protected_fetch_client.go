package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

type ssrfResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type protectedFetchDialer struct {
	resolver      ssrfResolver
	dialContext   func(context.Context, string, string) (net.Conn, error)
	getProtection func() (*common.SSRFProtection, bool, error)
}

type ssrfProtectedRoundTripper struct {
	resolver      ssrfResolver
	dialContext   func(context.Context, string, string) (net.Conn, error)
	getProtection func() (*common.SSRFProtection, bool, error)
	proxy         func(*http.Request) (*url.URL, error)

	mutex      sync.Mutex
	transports map[string]*http.Transport
}

func currentFetchProtection() (*common.SSRFProtection, bool, error) {
	setting := system_setting.GetFetchSetting()
	if !setting.EnableSSRFProtection {
		return nil, false, nil
	}
	protection, err := common.NewSSRFProtectionFromFetchSetting(setting.AllowPrivateIp, setting.DomainFilterMode, setting.IpFilterMode, setting.DomainList, setting.IpList, setting.AllowedPorts, setting.ApplyIPFilterForDomain)
	if err != nil {
		return nil, true, err
	}
	return protection, true, nil
}

func newProtectedFetchHTTPClient() *http.Client {
	return newProtectedFetchHTTPClientWithProxy(nil, nil, nil, http.ProxyFromEnvironment)
}

func newProtectedFetchHTTPClientWithProxy(resolver ssrfResolver, dialContext func(context.Context, string, string) (net.Conn, error), getProtection func() (*common.SSRFProtection, bool, error), proxyFunc func(*http.Request) (*url.URL, error)) *http.Client {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	if dialContext == nil {
		dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
		dialContext = dialer.DialContext
	}
	if getProtection == nil {
		getProtection = currentFetchProtection
	}
	if proxyFunc == nil {
		proxyFunc = http.ProxyFromEnvironment
	}
	client := &http.Client{
		Transport:     &ssrfProtectedRoundTripper{resolver: resolver, dialContext: dialContext, getProtection: getProtection, proxy: proxyFunc, transports: make(map[string]*http.Transport)},
		CheckRedirect: checkProtectedFetchRedirect,
	}
	if common.RelayTimeout != 0 {
		client.Timeout = time.Duration(common.RelayTimeout) * time.Second
	}
	return client
}

func (t *ssrfProtectedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, fmt.Errorf("invalid request")
	}
	if err := ValidateSSRFProtectedFetchURL(req.URL.String()); err != nil {
		return nil, err
	}
	proxyURL, err := t.proxy(req)
	if err != nil {
		return nil, err
	}
	return t.transportFor(proxyURL).RoundTrip(req)
}

func (t *ssrfProtectedRoundTripper) CloseIdleConnections() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	for _, transport := range t.transports {
		transport.CloseIdleConnections()
	}
}

func (t *ssrfProtectedRoundTripper) transportFor(proxyURL *url.URL) *http.Transport {
	key := "direct"
	if proxyURL != nil {
		key = proxyURL.String()
	}
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if transport, ok := t.transports[key]; ok {
		return transport
	}
	transport := t.newTransport(proxyURL)
	t.transports[key] = transport
	return transport
}

func (t *ssrfProtectedRoundTripper) newTransport(proxyURL *url.URL) *http.Transport {
	dialContext := t.dialContext
	proxyFunc := http.ProxyURL(proxyURL)
	if proxyURL == nil {
		dialContext = (&protectedFetchDialer{resolver: t.resolver, dialContext: t.dialContext, getProtection: t.getProtection}).DialContext
		proxyFunc = nil
	}
	transport := &http.Transport{
		MaxIdleConns:          common.RelayMaxIdleConns,
		MaxIdleConnsPerHost:   common.RelayMaxIdleConnsPerHost,
		IdleConnTimeout:       time.Duration(common.RelayIdleConnTimeout) * time.Second,
		ResponseHeaderTimeout: time.Duration(common.RelayResponseHeaderTimeout) * time.Second,
		ForceAttemptHTTP2:     true,
		Proxy:                 proxyFunc,
		DialContext:           dialContext,
	}
	if common.TLSInsecureSkipVerify {
		transport.TLSClientConfig = common.InsecureTLSConfig.Clone()
	}
	return transport
}

func (d *protectedFetchDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	protection, enabled, err := d.getProtection()
	if err != nil {
		return nil, err
	}
	if !enabled {
		return d.dialContext(ctx, network, address)
	}
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid dial address %s: %w", address, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %s", portText)
	}
	if err := protection.ValidateNetworkTarget(host, port); err != nil {
		return nil, err
	}
	if ip := net.ParseIP(host); ip != nil {
		return d.dialContext(ctx, network, net.JoinHostPort(ip.String(), portText))
	}
	if !protection.ApplyIPFilterForDomain {
		return d.dialContext(ctx, network, address)
	}
	resolved, err := d.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("DNS resolution failed for %s: %v", host, err)
	}
	var candidates []net.IP
	for _, ipAddr := range resolved {
		ip := ipAddr.IP
		if ip == nil || !networkAllowsIP(network, ip) {
			continue
		}
		if err := protection.ValidateResolvedIP(host, ip); err != nil {
			return nil, err
		}
		candidates = append(candidates, ip)
	}
	var lastErr error
	for _, ip := range candidates {
		conn, err := d.dialContext(ctx, network, net.JoinHostPort(ip.String(), portText))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("DNS resolution for %s returned no usable IP addresses", host)
}

func networkAllowsIP(network string, ip net.IP) bool {
	switch network {
	case "tcp4":
		return ip.To4() != nil
	case "tcp6":
		return ip.To4() == nil
	default:
		return true
	}
}
