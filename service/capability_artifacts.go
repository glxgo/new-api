package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

var capabilityArtifactID = regexp.MustCompile(`^[0-9a-f]{64}$`)

// The renderer intentionally accepts one job at a time. Wait locally rather
// than discarding already-paid model output when several targets finish.
var capabilityRenderQueue = make(chan struct{}, 1)
var capabilityArtifactMu sync.Mutex

func CapabilityArtifactRoot() string { return os.Getenv("CAPABILITY_ARTIFACT_DIR") }
func RenderCapabilitySVG(ctx context.Context, svg []byte) ([]byte, image.Image, error) {
	select {
	case capabilityRenderQueue <- struct{}{}:
		defer func() { <-capabilityRenderQueue }()
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
	socket := os.Getenv("CAPABILITY_RENDERER_SOCKET")
	if socket == "" {
		return nil, nil, errors.New("renderer_not_configured")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	defer transport.CloseIdleConnections()
	client := http.Client{Transport: transport, Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", "http://renderer/render", bytes.NewReader(svg))
	if err != nil {
		return nil, nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, errors.New("renderer_unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, nil, errors.New("render_failed")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20+1))
	if err != nil || len(raw) > 2<<20 {
		return nil, nil, errors.New("invalid_render_output")
	}
	config, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil || config.Width != 600 || config.Height != 400 {
		return nil, nil, errors.New("invalid_render_output")
	}
	img, err := png.Decode(bytes.NewReader(raw))
	return raw, img, err
}
func StoreCapabilityPNG(raw []byte) (string, error) {
	capabilityArtifactMu.Lock()
	defer capabilityArtifactMu.Unlock()
	if len(raw) > 2<<20 {
		return "", errors.New("invalid_render_output")
	}
	config, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil || config.Width != 600 || config.Height != 400 {
		return "", errors.New("invalid_render_output")
	}
	root := CapabilityArtifactRoot()
	if !filepath.IsAbs(root) {
		return "", errors.New("artifact_directory_not_configured")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", errors.New("artifact_storage_unavailable")
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(raw))
	path := filepath.Join(root, hash+".png")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		if _, e := ReadCapabilityPNG(hash); e != nil {
			return "", e
		}
		return hash, nil
	}
	if err != nil {
		return "", errors.New("artifact_storage_unavailable")
	}
	_, err = f.Write(raw)
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(path)
		return "", errors.New("artifact_storage_failed")
	}
	return hash, nil
}
func ReadCapabilityPNG(id string) ([]byte, error) {
	if !capabilityArtifactID.MatchString(id) || !filepath.IsAbs(CapabilityArtifactRoot()) {
		return nil, errors.New("artifact_not_found")
	}
	path := filepath.Join(CapabilityArtifactRoot(), id+".png")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 2<<20 {
		return nil, errors.New("artifact_not_found")
	}
	raw, err := os.ReadFile(path)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(raw)) != id {
		return nil, errors.New("artifact_integrity_failed")
	}
	return raw, nil
}
