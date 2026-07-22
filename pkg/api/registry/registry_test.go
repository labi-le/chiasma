package registry_test

import (
	"testing"

	"github.com/labi-le/chiasma/pkg/api/local"
	"github.com/labi-le/chiasma/pkg/api/nasa"
	"github.com/labi-le/chiasma/pkg/api/registry"
	"github.com/labi-le/chiasma/pkg/api/unsplash"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSearcher(t *testing.T) {
	log := zerolog.Nop()

	for _, name := range []string{nasa.Name, unsplash.Name, local.Name} {
		t.Run(name, func(t *testing.T) {
			s, err := registry.NewSearcher(log, name, "/tmp")
			require.NoError(t, err)
			assert.NotNil(t, s)
		})
	}

	t.Run("unknown", func(t *testing.T) {
		s, err := registry.NewSearcher(log, "does-not-exist", "/tmp")
		require.ErrorIs(t, err, registry.ErrUnknownSearcher)
		assert.Nil(t, s)
	})
}
