package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/draw"
	"image/png"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const CapabilityAnimationVersion = "chromium-animation-1"
const CapabilityAnimationBrowser = "149.0.7827.55"
const capabilityAnimationFrames = 48
const capabilityAnimationLimit = 12 << 20

// Only pixels leave the isolated renderer. HTML is retained as text in the
// generation ledger and is never served as an executable public artifact.
type CapabilityAnimation struct {
	Artifact      string `json:"artifact"`
	Evidence      string `json:"evidence"`
	SourceSHA256  string `json:"source_sha256"`
	Renderer      string `json:"renderer"`
	Browser       string `json:"browser"`
	Frames        int    `json:"frames"`
	StepMS        int    `json:"step_ms"`
	EvidenceMS    []int  `json:"evidence_ms"`
	ChangedFrames int    `json:"changed_frames"`
}

type capabilityAnimationResponse struct {
	Version string   `json:"version"`
	Browser string   `json:"browser"`
	StepMS  int      `json:"step_ms"`
	Frames  [][]byte `json:"frames"`
}

func RenderCapabilityAnimation(ctx context.Context, html string) (string, *CapabilityAnimation, []byte, error) {
	if len(html) > 400<<10 {
		return "", nil, nil, errors.New("animation_input_too_large")
	}
	select {
	case capabilityRenderQueue <- struct{}{}:
		defer func() { <-capabilityRenderQueue }()
	case <-ctx.Done():
		return "", nil, nil, ctx.Err()
	}
	socket := os.Getenv("CAPABILITY_RENDERER_SOCKET")
	if socket == "" {
		return "", nil, nil, errors.New("renderer_not_configured")
	}
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	defer transport.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, "POST", "http://renderer/animation", bytes.NewBufferString(html))
	if err != nil {
		return "", nil, nil, err
	}
	resp, err := (&http.Client{Transport: transport}).Do(req)
	if err != nil {
		return "", nil, nil, errors.New("renderer_unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", nil, nil, errors.New("animation_render_failed")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 17<<20+1))
	if err != nil || len(raw) > 17<<20 {
		return "", nil, nil, errors.New("invalid_animation_output")
	}
	var result capabilityAnimationResponse
	if common.Unmarshal(raw, &result) != nil {
		return "", nil, nil, errors.New("invalid_animation_output")
	}
	poster, animation, evidence, meta, err := capabilityAssembleAnimation(result)
	if err != nil {
		return "", nil, nil, err
	}
	meta.SourceSHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte(html)))
	posterID, err := StoreCapabilityPNG(poster)
	if err != nil {
		return "", nil, nil, err
	}
	meta.Artifact, err = storeCapabilityAnimationAsset(animation, "animation")
	if err != nil {
		return "", nil, nil, err
	}
	meta.Evidence, err = storeCapabilityAnimationAsset(evidence, "evidence")
	return posterID, meta, evidence, err
}

// Re-encode each frame to a common PNG color model before constructing APNG.
// This avoids trusting Chromium chunk layout, ancillary data or color profiles.
func capabilityAssembleAnimation(result capabilityAnimationResponse) ([]byte, []byte, []byte, *CapabilityAnimation, error) {
	invalid := errors.New("invalid_animation_output")
	if result.Version != CapabilityAnimationVersion || result.Browser != CapabilityAnimationBrowser || result.StepMS != 100 || len(result.Frames) != capabilityAnimationFrames {
		return nil, nil, nil, nil, invalid
	}
	meta := &CapabilityAnimation{Renderer: result.Version, Browser: result.Browser, Frames: len(result.Frames), StepMS: result.StepMS, EvidenceMS: []int{0, 900, 1900, 2800, 3800, 4700}}
	montage := image.NewNRGBA(image.Rect(0, 0, 1800, 800))
	var output bytes.Buffer
	output.Write([]byte("\x89PNG\r\n\x1a\n"))
	var poster, header, previous []byte
	var sequence uint32
	inputSize, tile := 0, 0
	for i, raw := range result.Frames {
		inputSize += len(raw)
		if len(raw) > 2<<20 || inputSize > capabilityAnimationLimit {
			return nil, nil, nil, nil, invalid
		}
		config, err := png.DecodeConfig(bytes.NewReader(raw))
		if err != nil || config.Width != 600 || config.Height != 400 {
			return nil, nil, nil, nil, invalid
		}
		img, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			return nil, nil, nil, nil, invalid
		}
		frame := image.NewNRGBA(image.Rect(0, 0, 600, 400))
		draw.Draw(frame, frame.Bounds(), img, image.Point{}, draw.Src)
		if i > 0 && !bytes.Equal(previous, frame.Pix) {
			meta.ChangedFrames++
		}
		previous = append(previous[:0], frame.Pix...)
		if tile < len(meta.EvidenceMS) && i*100 == meta.EvidenceMS[tile] {
			draw.Draw(montage, image.Rect((tile%3)*600, (tile/3)*400, (tile%3+1)*600, (tile/3+1)*400), frame, image.Point{}, draw.Src)
			tile++
		}
		// Force RGBA, including opaque frames, so all IHDRs match.
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, capabilityNonOpaque{frame}); err != nil {
			return nil, nil, nil, nil, err
		}
		data := encoded.Bytes()
		if i == 0 {
			poster = append([]byte(nil), data...)
			header = append([]byte(nil), data[16:29]...)
			capabilityPNGChunk(&output, "IHDR", header)
			control := make([]byte, 8)
			binary.BigEndian.PutUint32(control, uint32(len(result.Frames)))
			capabilityPNGChunk(&output, "acTL", control)
		} else if !bytes.Equal(header, data[16:29]) {
			return nil, nil, nil, nil, invalid
		}
		control := make([]byte, 26)
		binary.BigEndian.PutUint32(control, sequence)
		sequence++
		binary.BigEndian.PutUint32(control[4:], 600)
		binary.BigEndian.PutUint32(control[8:], 400)
		binary.BigEndian.PutUint16(control[20:], 100)
		binary.BigEndian.PutUint16(control[22:], 1000)
		capabilityPNGChunk(&output, "fcTL", control)
		for offset := 8; offset+12 <= len(data); {
			n := int(binary.BigEndian.Uint32(data[offset:]))
			if offset+12+n > len(data) {
				return nil, nil, nil, nil, invalid
			}
			if string(data[offset+4:offset+8]) == "IDAT" {
				payload := data[offset+8 : offset+8+n]
				if i == 0 {
					capabilityPNGChunk(&output, "IDAT", payload)
				} else {
					fd := make([]byte, 4+n)
					binary.BigEndian.PutUint32(fd, sequence)
					sequence++
					copy(fd[4:], payload)
					capabilityPNGChunk(&output, "fdAT", fd)
				}
			}
			offset += 12 + n
		}
		if output.Len() > capabilityAnimationLimit {
			return nil, nil, nil, nil, invalid
		}
	}
	capabilityPNGChunk(&output, "IEND", nil)
	var evidence bytes.Buffer
	if err := png.Encode(&evidence, montage); err != nil {
		return nil, nil, nil, nil, err
	}
	return poster, output.Bytes(), evidence.Bytes(), meta, nil
}

type capabilityNonOpaque struct{ *image.NRGBA }

func (capabilityNonOpaque) Opaque() bool { return false }
func capabilityPNGChunk(out *bytes.Buffer, kind string, payload []byte) {
	_ = binary.Write(out, binary.BigEndian, uint32(len(payload)))
	out.WriteString(kind)
	out.Write(payload)
	crc := crc32.NewIEEE()
	_, _ = crc.Write([]byte(kind))
	_, _ = crc.Write(payload)
	_ = binary.Write(out, binary.BigEndian, crc.Sum32())
}

func storeCapabilityAnimationAsset(raw []byte, kind string) (string, error) {
	if err := validateCapabilityAnimationAsset(raw, kind); err != nil {
		return "", err
	}
	capabilityArtifactMu.Lock()
	defer capabilityArtifactMu.Unlock()
	root := CapabilityArtifactRoot()
	if !filepath.IsAbs(root) {
		return "", errors.New("artifact_directory_not_configured")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", errors.New("artifact_storage_unavailable")
	}
	id := fmt.Sprintf("%x", sha256.Sum256(raw))
	path := filepath.Join(root, id+"."+kind+".png")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		_, err = ReadCapabilityAnimationAsset(id, kind)
		return id, err
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
	return id, nil
}

func validateCapabilityAnimationAsset(raw []byte, kind string) error {
	width, height := 600, 400
	if kind == "evidence" {
		width, height = 1800, 800
	} else if kind != "animation" {
		return errors.New("invalid_artifact_kind")
	}
	if len(raw) > capabilityAnimationLimit {
		return errors.New("invalid_animation_output")
	}
	config, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil || config.Width != width || config.Height != height {
		return errors.New("invalid_animation_output")
	}
	return nil
}

func ReadCapabilityAnimationAsset(id, kind string) ([]byte, error) {
	if !capabilityArtifactID.MatchString(id) || (kind != "animation" && kind != "evidence") || !filepath.IsAbs(CapabilityArtifactRoot()) {
		return nil, errors.New("artifact_not_found")
	}
	path := filepath.Join(CapabilityArtifactRoot(), id+"."+kind+".png")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > capabilityAnimationLimit {
		return nil, errors.New("artifact_not_found")
	}
	raw, err := os.ReadFile(path)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(raw)) != id {
		return nil, errors.New("artifact_integrity_failed")
	}
	if err = validateCapabilityAnimationAsset(raw, kind); err != nil {
		return nil, err
	}
	return raw, nil
}
