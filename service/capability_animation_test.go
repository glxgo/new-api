package service

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCapabilityAnimationPixelsAndAPNG(t *testing.T) {
	response := capabilityAnimationResponse{Version: CapabilityAnimationVersion, Browser: CapabilityAnimationBrowser, StepMS: 100}
	for i := 0; i < 48; i++ {
		img := image.NewNRGBA(image.Rect(0, 0, 600, 400))
		img.SetNRGBA(i, 20, color.NRGBA{R: 255, A: 255})
		var raw bytes.Buffer
		require.NoError(t, png.Encode(&raw, img))
		response.Frames = append(response.Frames, raw.Bytes())
	}
	poster, animation, evidence, meta, err := capabilityAssembleAnimation(response)
	require.NoError(t, err)
	require.Equal(t, 47, meta.ChangedFrames)
	_, err = png.Decode(bytes.NewReader(poster))
	require.NoError(t, err)
	img, err := png.Decode(bytes.NewReader(evidence))
	require.NoError(t, err)
	require.Equal(t, image.Rect(0, 0, 1800, 800), img.Bounds())
	for tile, ms := range meta.EvidenceMS {
		r, _, _, a := img.At(tile%3*600+ms/100, tile/3*400+20).RGBA()
		require.EqualValues(t, 65535, r)
		require.EqualValues(t, 65535, a)
	}
	// Independently reconstruct every PNG frame, verify CRC/sequence/timing,
	// and compare each decoded pixel with the corresponding original frame.
	var header []byte
	var frame bytes.Buffer
	frameIndex := -1
	sequence := uint32(0)
	finish := func() {
		if frameIndex < 0 {
			return
		}
		capabilityPNGChunk(&frame, "IEND", nil)
		decoded, e := png.Decode(bytes.NewReader(frame.Bytes()))
		require.NoError(t, e)
		original, e := png.Decode(bytes.NewReader(response.Frames[frameIndex]))
		require.NoError(t, e)
		for y := 0; y < 400; y++ {
			for x := 0; x < 600; x++ {
				require.Equal(t, color.NRGBAModel.Convert(original.At(x, y)), color.NRGBAModel.Convert(decoded.At(x, y)))
			}
		}
	}
	for offset := 8; offset < len(animation); {
		n := int(binary.BigEndian.Uint32(animation[offset:]))
		kind := string(animation[offset+4 : offset+8])
		payload := animation[offset+8 : offset+8+n]
		require.Equal(t, crc32.ChecksumIEEE(animation[offset+4:offset+8+n]), binary.BigEndian.Uint32(animation[offset+8+n:]))
		switch kind {
		case "IHDR":
			header = payload
		case "acTL":
			require.EqualValues(t, 48, binary.BigEndian.Uint32(payload))
			require.Zero(t, binary.BigEndian.Uint32(payload[4:]))
		case "fcTL":
			finish()
			frameIndex++
			frame.Reset()
			frame.Write([]byte("\x89PNG\r\n\x1a\n"))
			capabilityPNGChunk(&frame, "IHDR", header)
			require.Equal(t, sequence, binary.BigEndian.Uint32(payload))
			sequence++
			require.EqualValues(t, 100, binary.BigEndian.Uint16(payload[20:]))
			require.EqualValues(t, 1000, binary.BigEndian.Uint16(payload[22:]))
			require.Zero(t, payload[25])
		case "IDAT":
			capabilityPNGChunk(&frame, "IDAT", payload)
		case "fdAT":
			require.Equal(t, sequence, binary.BigEndian.Uint32(payload))
			sequence++
			capabilityPNGChunk(&frame, "IDAT", payload[4:])
		case "IEND":
			finish()
		}
		offset += 12 + n
	}
	require.Equal(t, 47, frameIndex)
	t.Setenv("CAPABILITY_ARTIFACT_DIR", t.TempDir())
	for kind, raw := range map[string][]byte{"animation": animation, "evidence": evidence} {
		id, e := storeCapabilityAnimationAsset(raw, kind)
		require.NoError(t, e)
		got, e := ReadCapabilityAnimationAsset(id, kind)
		require.NoError(t, e)
		require.Equal(t, raw, got)
		_, e = ReadCapabilityPNG(id)
		require.Error(t, e)
	}
	response.Frames = response.Frames[:47]
	_, _, _, _, err = capabilityAssembleAnimation(response)
	require.Error(t, err)
}

func TestCapabilityAnimationStaticAndInvalid(t *testing.T) {
	var raw bytes.Buffer
	require.NoError(t, png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 600, 400))))
	response := capabilityAnimationResponse{Version: CapabilityAnimationVersion, Browser: CapabilityAnimationBrowser, StepMS: 100}
	for i := 0; i < 48; i++ {
		response.Frames = append(response.Frames, raw.Bytes())
	}
	_, _, _, meta, err := capabilityAssembleAnimation(response)
	require.NoError(t, err)
	require.Zero(t, meta.ChangedFrames)
	response.StepMS = 0
	_, _, _, _, err = capabilityAssembleAnimation(response)
	require.Error(t, err)
	response.StepMS = 100
	response.Frames[20] = []byte("<html>bad</html>")
	_, _, _, _, err = capabilityAssembleAnimation(response)
	require.Error(t, err)
	_, err = ReadCapabilityAnimationAsset("../outside", "animation")
	require.Error(t, err)
}
