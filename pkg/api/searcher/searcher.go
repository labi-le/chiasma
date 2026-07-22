package searcher

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg" // register the JPEG decoder for image.DecodeConfig
	_ "image/png"  // register the PNG decoder for image.DecodeConfig
	"io"
)

type Image interface {
	io.ReadCloser
	Size() (int, int)
}

type Searcher interface {
	Search(ctx context.Context, q string, resolution Resolution) (Image, error)
}

type detectedImage struct {
	io.Reader
	closer io.Closer
	w, h   int
}

func (d *detectedImage) Size() (int, int) { return d.w, d.h }
func (d *detectedImage) Close() error {
	if err := d.closer.Close(); err != nil {
		return fmt.Errorf("close image: %w", err)
	}
	return nil
}

// DetectSize decodes the image header to report its real dimensions while
// preserving the full byte stream for the returned reader.
//
//nolint:ireturn // seam: callers consume the searcher.Image abstraction, not a concrete type.
func DetectSize(img io.Reader) (Image, error) {
	var header bytes.Buffer
	tee := io.TeeReader(img, &header)

	config, _, err := image.DecodeConfig(tee)
	if err != nil {
		return nil, fmt.Errorf("decode image config: %w", err)
	}

	var closer io.Closer
	if c, ok := img.(io.Closer); ok {
		closer = c
	} else {
		closer = io.NopCloser(nil)
	}

	return &detectedImage{
		Reader: io.MultiReader(&header, img),
		closer: closer,
		w:      config.Width,
		h:      config.Height,
	}, nil
}
