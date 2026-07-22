package fs_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labi-le/chiasma/internal/fs"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pngBytes(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestSaveFileAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	data := pngBytes(t, 4, 4, color.RGBA{R: 255, A: 255})

	path, skipped, err := fs.SaveFile(context.Background(), zerolog.Nop(), bytes.NewReader(data), dir, []string{"red"})
	require.NoError(t, err)
	assert.False(t, skipped)
	assert.True(t, strings.HasPrefix(filepath.Base(path), "sw-"))
	assert.Contains(t, path, "__red")

	// File exists with expected contents and no leftover temp files.
	onDisk, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, data, onDisk)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temp file should remain")
}

func TestSaveFileDedupBySha(t *testing.T) {
	dir := t.TempDir()
	data := pngBytes(t, 4, 4, color.RGBA{G: 255, A: 255})

	path1, skipped1, err := fs.SaveFile(context.Background(), zerolog.Nop(), bytes.NewReader(data), dir, []string{"green"})
	require.NoError(t, err)
	assert.False(t, skipped1)

	// Same bytes + same tags -> same path, reported as cached, not rewritten.
	path2, skipped2, err := fs.SaveFile(context.Background(), zerolog.Nop(), bytes.NewReader(data), dir, []string{"green"})
	require.NoError(t, err)
	assert.True(t, skipped2)
	assert.Equal(t, path1, path2)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestSaveFileContextCancelled(t *testing.T) {
	dir := t.TempDir()
	data := pngBytes(t, 4, 4, color.RGBA{B: 255, A: 255})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := fs.SaveFile(ctx, zerolog.Nop(), bytes.NewReader(data), dir, []string{"blue"})
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "nothing should be written when cancelled")
}

func TestSaveFileUnknownExtension(t *testing.T) {
	dir := t.TempDir()
	_, _, err := fs.SaveFile(context.Background(), zerolog.Nop(), bytes.NewReader([]byte{0x00, 0x01, 0x02, 0x03, 0xff}), dir, nil)
	assert.ErrorIs(t, err, fs.ErrUnknownExtension)
}
