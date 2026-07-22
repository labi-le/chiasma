package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/labi-le/chiasma/internal/fs"
	"github.com/labi-le/chiasma/pkg/api/searcher"
	"github.com/labi-le/chiasma/pkg/wallpaper/execute"
	"github.com/rs/zerolog"
)

type QuerySource interface {
	GetLastSearch() (string, error)
}

// FileSaver persists an image to disk and reports whether an identical file
// already existed (cache hit). It is the seam that lets Update run without a
// real filesystem in tests; when nil, Update falls back to fs.SaveFile.
type FileSaver func(ctx context.Context, data io.Reader, dir string, tags []string) (string, bool, error)

type WallpaperService struct {
	Log     zerolog.Logger
	API     searcher.Searcher
	History QuerySource
	Setter  execute.Provider
	// Save is an optional filesystem seam; nil means use fs.SaveFile.
	Save FileSaver
}

type UpdateParams struct {
	Phrase     string
	Resolution searcher.Resolution
	SaveDir    string
	OutputID   string
	RetryCount int
}

func (p UpdateParams) validate() error {
	if strings.TrimSpace(p.SaveDir) == "" {
		return errors.New("save directory is empty")
	}
	if p.RetryCount <= 0 {
		return fmt.Errorf("retry count must be positive, got %d", p.RetryCount)
	}
	if p.Resolution.Width <= 0 || p.Resolution.Height <= 0 {
		return fmt.Errorf("resolution must be positive, got %dx%d", p.Resolution.Width, p.Resolution.Height)
	}
	return nil
}

func (s *WallpaperService) Update(ctx context.Context, params UpdateParams) error {
	log := s.Log.With().Str("op", "Update").Logger()

	if err := params.validate(); err != nil {
		return fmt.Errorf("invalid update parameters: %w", err)
	}

	phrase := strings.TrimSpace(params.Phrase)
	if phrase == "" {
		if s.History == nil {
			return errors.New("search phrase is empty and no history source provided")
		}
		var err error
		phrase, err = s.History.GetLastSearch()
		if err != nil {
			return fmt.Errorf("failed to get search phrase from history: %w", err)
		}
		phrase = strings.TrimSpace(phrase)
		log.Info().Msgf("using phrase from history: %s", phrase)
	}

	img, err := s.fetchImageWithRetry(ctx, phrase, params.Resolution, params.RetryCount)
	if err != nil {
		return err
	}
	defer img.Close()

	tags := strings.Fields(phrase)

	save := s.Save
	if save == nil {
		save = func(ctx context.Context, data io.Reader, dir string, tags []string) (string, bool, error) {
			return fs.SaveFile(ctx, s.Log, data, dir, tags)
		}
	}

	path, skipped, err := save(ctx, img, params.SaveDir, tags)
	if err != nil {
		return fmt.Errorf("failed to save image: %w", err)
	}

	if skipped {
		log.Debug().Str("path", path).Msg("image reused (cached)")
	} else {
		log.Debug().Str("path", path).Msg("image saved")
	}

	if changeErr := s.Setter.Change(ctx, path, params.OutputID); changeErr != nil {
		return fmt.Errorf("failed to set wallpaper: %w", changeErr)
	}

	return nil
}

//nolint:ireturn // returns the searcher.Image interface produced by s.API.Search; the concrete type is backend-specific and intentionally hidden behind the seam.
func (s *WallpaperService) fetchImageWithRetry(ctx context.Context, phrase string, res searcher.Resolution, retries int) (searcher.Image, error) {
	if retries <= 0 {
		return nil, fmt.Errorf("retry count must be positive, got %d", retries)
	}

	var lastErr error
	for range retries {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("image fetch cancelled: %w", err)
		}

		img, err := s.API.Search(ctx, phrase, res)
		if err != nil {
			lastErr = err
			continue
		}

		w, h := img.Size()
		if w < res.Width || h < res.Height {
			_ = img.Close()
			lastErr = fmt.Errorf("image too small: %dx%d < %dx%d", w, h, res.Width, res.Height)
			continue
		}

		return img, nil
	}
	return nil, fmt.Errorf("failed to find suitable image after %d attempts: %w", retries, lastErr)
}
