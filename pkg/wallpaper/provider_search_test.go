//nolint:testpackage // white-box: stubs unexported builders/probeOrder vars.
package wallpaper

import (
	"context"
	"errors"
	"testing"

	"github.com/labi-le/chiasma/pkg/wallpaper/execute"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubProvider struct{ id string }

func (stubProvider) Change(context.Context, string, string) error { return nil }
func (stubProvider) Close(context.Context) error                  { return nil }

// stubProbe replaces builders/probeOrder for the duration of a test.
func stubProbe(t *testing.T, b map[string]providerFactory, order []string) {
	t.Helper()
	origB, origOrder := builders, probeOrder
	builders, probeOrder = b, order
	t.Cleanup(func() { builders, probeOrder = origB, origOrder })
}

func ok(id string) providerFactory {
	return func() (execute.Provider, error) { return stubProvider{id: id}, nil }
}

func fail() providerFactory {
	return func() (execute.Provider, error) { return nil, execute.ErrUtilityNotFound }
}

func TestByNameSelectsRequestedBackend(t *testing.T) {
	stubProbe(t, map[string]providerFactory{
		"swaybg": ok("swaybg"),
		"swww":   ok("swww"),
	}, []string{"swaybg", "swww"})

	p, err := ByNameOrAvailable("swww")
	require.NoError(t, err)
	assert.Equal(t, "swww", p.(stubProvider).id)

	p, err = ByNameOrAvailable("swaybg")
	require.NoError(t, err)
	assert.Equal(t, "swaybg", p.(stubProvider).id)
}

func TestByNameUnknownReturnsError(t *testing.T) {
	stubProbe(t, map[string]providerFactory{"swww": ok("swww")}, []string{"swww"})

	_, err := ByNameOrAvailable("nonsense")
	assert.ErrorIs(t, err, execute.ErrUtilityNotFound)
}

func TestByNamePropagatesConstructorError(t *testing.T) {
	wantErr := errors.New("no binary")
	stubProbe(t, map[string]providerFactory{
		"swww": func() (execute.Provider, error) { return nil, wantErr },
	}, []string{"swww"})

	_, err := ByNameOrAvailable("swww")
	assert.ErrorIs(t, err, wantErr)
}

func TestAutoPicksFirstAvailableInProbeOrder(t *testing.T) {
	stubProbe(t, map[string]providerFactory{
		"swaybg": fail(),
		"swww":   ok("swww"),
	}, []string{"swaybg", "swww"})

	p, err := ByNameOrAvailable("")
	require.NoError(t, err)
	assert.Equal(t, "swww", p.(stubProvider).id, "must fall back past unavailable swaybg")
}

func TestAutoPrefersEarlierProbeEntry(t *testing.T) {
	stubProbe(t, map[string]providerFactory{
		"swaybg": ok("swaybg"),
		"swww":   ok("swww"),
	}, []string{"swaybg", "swww"})

	p, err := ByNameOrAvailable("")
	require.NoError(t, err)
	assert.Equal(t, "swaybg", p.(stubProvider).id)
}

func TestAutoNoneAvailableReturnsError(t *testing.T) {
	stubProbe(t, map[string]providerFactory{
		"swaybg": fail(),
		"swww":   fail(),
	}, []string{"swaybg", "swww"})

	_, err := ByNameOrAvailable("")
	assert.ErrorIs(t, err, execute.ErrUtilityNotFound)
}
