package resizer

import (
	"bytes"
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResize_ValidPNG_ResizeToSmaller(t *testing.T) {
	pngPath := filepath.Join("tests", "test.png")
	data, err := os.ReadFile(pngPath)
	require.NoError(t, err)

	resized, err := Resize(data, 256, 256)
	require.NoError(t, err)

	img, _, err := image.Decode(bytes.NewReader(resized))
	require.NoError(t, err)

	bounds := img.Bounds()
	assert.Equal(t, 256, bounds.Dx())
	assert.Equal(t, 256, bounds.Dy())
}

func TestResize_ValidPNG_ResizeToLarger(t *testing.T) {
	pngPath := filepath.Join("tests", "test.png")
	data, err := os.ReadFile(pngPath)
	require.NoError(t, err)

	resized, err := Resize(data, 1024, 1024)
	require.NoError(t, err)

	img, _, err := image.Decode(bytes.NewReader(resized))
	require.NoError(t, err)

	bounds := img.Bounds()
	assert.Equal(t, 1024, bounds.Dx())
	assert.Equal(t, 1024, bounds.Dy())
}

func TestResize_ValidPNG_ResizeToSameSize(t *testing.T) {
	pngPath := filepath.Join("tests", "test.png")
	data, err := os.ReadFile(pngPath)
	require.NoError(t, err)

	resized, err := Resize(data, 512, 512)
	require.NoError(t, err)

	img, _, err := image.Decode(bytes.NewReader(resized))
	require.NoError(t, err)

	bounds := img.Bounds()
	assert.Equal(t, 512, bounds.Dx())
	assert.Equal(t, 512, bounds.Dy())
}

func TestResize_InvalidBytes(t *testing.T) {
	invalidData := []byte("not an image")

	_, err := Resize(invalidData, 256, 256)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}

func TestResize_NilBytes(t *testing.T) {
	_, err := Resize(nil, 256, 256)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}
