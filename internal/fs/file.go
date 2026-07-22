package fs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/rs/zerolog"
)

const maxTagLen = 20

var (
	ErrUnknownExtension = errors.New("unknown extension")
	unsafeChars         = regexp.MustCompile(`[^a-zA-Z0-9а-яА-Я]+`)
)

func SaveFile(ctx context.Context, log zerolog.Logger, data io.Reader, dir string, tags []string) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, fmt.Errorf("save file: %w", err)
	}

	img, err := io.ReadAll(data)
	if err != nil {
		return "", false, fmt.Errorf("read image data: %w", err)
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", false, fmt.Errorf("save file: %w", ctxErr)
	}

	ext := mimetype.Detect(img).Extension()
	if ext == "" {
		return "", false, ErrUnknownExtension
	}

	if ioErr := os.MkdirAll(dir, 0750); ioErr != nil {
		return "", false, fmt.Errorf("create dir %q: %w", dir, ioErr)
	}

	sum := sha256.Sum256(img)
	short := hex.EncodeToString(sum[:])[:7]

	tagSuffix := buildTagSuffix(log, tags)
	gen := fmt.Sprintf("%s/sw-%s%s%s", dir, short, tagSuffix, ext)

	if _, statErr := os.Stat(gen); statErr == nil {
		return gen, true, nil
	}

	if writeErr := writeFileAtomic(ctx, gen, dir, img); writeErr != nil {
		return "", false, writeErr
	}

	return gen, false, nil
}

// writeFileAtomic writes data to a temp file in dir and renames it into place,
// so a crash mid-write never leaves a partial file at the target path.
func writeFileAtomic(ctx context.Context, path, dir string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("write file atomic: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "sw-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	defer func() {
		if tmpName != "" {
			_ = os.Remove(tmpName)
		}
	}()

	if _, writeErr := tmp.Write(data); writeErr != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", writeErr)
	}
	if closeErr := tmp.Close(); closeErr != nil {
		return fmt.Errorf("close temp file: %w", closeErr)
	}
	if chmodErr := os.Chmod(tmpName, 0600); chmodErr != nil {
		return fmt.Errorf("chmod temp file: %w", chmodErr)
	}
	if renameErr := os.Rename(tmpName, path); renameErr != nil {
		return fmt.Errorf("rename temp file: %w", renameErr)
	}

	tmpName = "" // renamed successfully; skip cleanup
	return nil
}

func buildTagSuffix(log zerolog.Logger, tags []string) string {
	if len(tags) == 0 {
		return ""
	}

	var validTags []string
	seen := make(map[string]struct{})

	for _, t := range tags {
		clean := unsafeChars.ReplaceAllString(t, "")
		clean = strings.ToLower(clean)

		if clean == "" {
			continue
		}
		if len(clean) > maxTagLen {
			log.Debug().Str("tag", t).Int("limit", maxTagLen).Msg("dropping tag: exceeds length limit")
			continue
		}

		if _, exists := seen[clean]; !exists {
			validTags = append(validTags, clean)
			seen[clean] = struct{}{}
		}
	}

	if len(validTags) == 0 {
		return ""
	}

	return "__" + strings.Join(validTags, "_")
}
