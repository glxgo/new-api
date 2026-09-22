package service

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCapabilityArtifactsVerifyContentAndRejectLinks(t *testing.T){
	root:=t.TempDir();t.Setenv("CAPABILITY_ARTIFACT_DIR",root)
	var data bytes.Buffer;require.NoError(t,png.Encode(&data,image.NewRGBA(image.Rect(0,0,600,400))))
	id,err:=StoreCapabilityPNG(data.Bytes());require.NoError(t,err)
	got,err:=ReadCapabilityPNG(id);require.NoError(t,err);require.Equal(t,data.Bytes(),got)
	_,err=StoreCapabilityPNG(data.Bytes());require.NoError(t,err)
	path:=filepath.Join(root,id+".png");require.NoError(t,os.WriteFile(path,[]byte("corrupted"),0600))
	_,err=ReadCapabilityPNG(id);require.Error(t,err)
	_,err=StoreCapabilityPNG(data.Bytes());require.Error(t,err)
	require.NoError(t,os.Remove(path));require.NoError(t,os.Symlink(filepath.Join(root,"missing"),path))
	_,err=ReadCapabilityPNG(id);require.Error(t,err)
	_,err=ReadCapabilityPNG("../outside");require.Error(t,err)
}
