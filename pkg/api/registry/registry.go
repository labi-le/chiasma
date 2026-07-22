// Package registry maps an --api name to a constructed searcher.Searcher.
//
// It lives in a separate package from searcher because the concrete providers
// (nasa, unsplash, local) already import searcher; wiring the factory here
// avoids an import cycle.
package registry

import (
	"errors"

	"github.com/labi-le/chiasma/pkg/api/local"
	"github.com/labi-le/chiasma/pkg/api/nasa"
	"github.com/labi-le/chiasma/pkg/api/searcher"
	"github.com/labi-le/chiasma/pkg/api/unsplash"
	"github.com/rs/zerolog"
)

// ErrUnknownSearcher is returned when name does not match a known provider.
var ErrUnknownSearcher = errors.New("unknown searcher")

// NewSearcher constructs the searcher.Searcher selected by name. dir is only
// used by the local provider. Providers use a default HTTP client.
//
//nolint:ireturn // registry factory: selects a provider by name behind the searcher.Searcher seam.
func NewSearcher(log zerolog.Logger, name string, dir string) (searcher.Searcher, error) {
	switch name {
	case unsplash.Name:
		return unsplash.NewUnsplash(log, nil), nil
	case nasa.Name:
		return nasa.NewNasa(log, nil), nil
	case local.Name:
		return local.NewLocal(log, dir), nil
	default:
		return nil, ErrUnknownSearcher
	}
}
