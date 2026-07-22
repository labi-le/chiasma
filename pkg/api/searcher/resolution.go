package searcher

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	ErrInvalidResolution = errors.New("invalid resolution")
	resolutionRe         = regexp.MustCompile(`\d+x\d+`)
)

type Resolution struct {
	Width  int
	Height int
}

func (r *Resolution) String() string {
	return fmt.Sprintf("%dx%d", r.Width, r.Height)
}

func (r *Resolution) Set(s string) error {
	if !resolutionRe.MatchString(s) {
		return ErrInvalidResolution
	}

	var w, h int
	if _, err := fmt.Sscanf(s, "%dx%d", &w, &h); err != nil {
		return fmt.Errorf("parse resolution %q: %w", s, err)
	}
	if w <= 0 || h <= 0 {
		return ErrInvalidResolution
	}

	r.Width = w
	r.Height = h
	return nil
}

func (r *Resolution) Type() string {
	return "resolution"
}
