package wallpaper

import (
	"github.com/labi-le/chiasma/pkg/wallpaper/execute"
	"github.com/labi-le/chiasma/pkg/wallpaper/swaybg"
	"github.com/labi-le/chiasma/pkg/wallpaper/swww"
)

type providerFactory func() (execute.Provider, error)

// builders maps a backend name to its constructor and probeOrder is the order
// in which the "" (auto) case probes availability. Both are package vars so
// availability is probed at call time (never cached at init) and can be
// stubbed in tests.
var (
	builders = map[string]providerFactory{
		swaybg.Name: func() (execute.Provider, error) { return swaybg.NewSwayBG() },
		swww.Name:   func() (execute.Provider, error) { return swww.NewSWWW() },
	}
	probeOrder = []string{swaybg.Name, swww.Name}
)

// ByNameOrAvailable returns the Provider interface: this is the backend
// factory seam, so an interface return is intentional.
//
//nolint:ireturn // factory seam: callers select a backend by interface.
func ByNameOrAvailable(tool string) (execute.Provider, error) {
	if tool == "" {
		return getAvailableProvider()
	}

	build, ok := builders[tool]
	if !ok {
		return nil, execute.ErrUtilityNotFound
	}

	p, err := build()
	if err != nil {
		return nil, err
	}
	return p, nil
}

//nolint:ireturn // factory helper: returns the selected backend interface.
func getAvailableProvider() (execute.Provider, error) {
	for _, name := range probeOrder {
		if p, err := builders[name](); err == nil {
			return p, nil
		}
	}

	return nil, execute.ErrUtilityNotFound
}
