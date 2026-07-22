package local

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/labi-le/chiasma/pkg/api/searcher"
	"github.com/rs/zerolog"
)

const Name = "local"

type Local struct {
	dir string
	log zerolog.Logger
}

func NewLocal(log zerolog.Logger, dir string) *Local {
	return &Local{
		dir: dir,
		log: log.With().Str("component", "local_fs").Logger(),
	}
}

//nolint:ireturn // seam: the local provider satisfies searcher.Searcher and yields searcher.Image.
func (l *Local) Search(_ context.Context, q string, _ searcher.Resolution) (searcher.Image, error) {
	log := l.log.With().Str("op", "Search").Str("query", q).Logger()

	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", l.dir, err)
	}

	candidates := l.matchingFiles(entries, q)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no local images found for query: %s", q)
	}

	rand.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	return l.openFirstValid(log, candidates)
}

func (l *Local) matchingFiles(entries []os.DirEntry, q string) []string {
	queryTerms := strings.Fields(strings.ToLower(q))
	candidates := make([]string, 0, len(entries))

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := strings.ToLower(e.Name())
		if matchesAllTerms(name, queryTerms) {
			candidates = append(candidates, filepath.Join(l.dir, e.Name()))
		}
	}
	return candidates
}

func matchesAllTerms(name string, terms []string) bool {
	for _, term := range terms {
		if !strings.Contains(name, term) {
			return false
		}
	}
	return true
}

//nolint:ireturn // seam: the local provider satisfies searcher.Searcher and yields searcher.Image.
func (l *Local) openFirstValid(log zerolog.Logger, candidates []string) (searcher.Image, error) {
	for _, path := range candidates {
		if err := l.validateImage(path); err != nil {
			continue
		}

		f, err := os.Open(path)
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("failed to open candidate")
			continue
		}

		img, err := searcher.DetectSize(f)
		if err != nil {
			_ = f.Close()
			log.Warn().Err(err).Str("path", path).Msg("failed to detect image size")
			continue
		}

		return img, nil
	}

	return nil, errors.New("no valid images found among candidates")
}

func (l *Local) validateImage(path string) error {
	ext := filepath.Ext(path)
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png", ".webp", ".bmp":
		return nil
	}

	mtype, err := mimetype.DetectFile(path)
	if err != nil {
		return fmt.Errorf("detect mime type: %w", err)
	}
	if !strings.HasPrefix(mtype.String(), "image/") {
		return errors.New("not an image")
	}
	return nil
}
