package searcher_test

import (
	"testing"

	"github.com/labi-le/chiasma/pkg/api/searcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolutionSet(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
		wantW   int
		wantH   int
	}{
		{name: "valid", in: "1920x1080", wantW: 1920, wantH: 1080},
		{name: "valid small", in: "1x1", wantW: 1, wantH: 1},
		{name: "invalid non-numeric", in: "abc", wantErr: true},
		{name: "invalid empty", in: "", wantErr: true},
		{name: "invalid missing sep", in: "1920", wantErr: true},
		{name: "zero", in: "0x0", wantErr: true},
		{name: "zero width", in: "0x1080", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r searcher.Resolution
			err := r.Set(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantW, r.Width)
			assert.Equal(t, tt.wantH, r.Height)
		})
	}
}

// TestResolutionSetNoDirtyState ensures an invalid input never mutates the
// receiver's previous valid value.
func TestResolutionSetNoDirtyState(t *testing.T) {
	r := searcher.Resolution{Width: 1920, Height: 1080}

	err := r.Set("0x0")
	require.ErrorIs(t, err, searcher.ErrInvalidResolution)
	assert.Equal(t, 1920, r.Width, "width must be unchanged after invalid Set")
	assert.Equal(t, 1080, r.Height, "height must be unchanged after invalid Set")

	err = r.Set("garbage")
	require.Error(t, err)
	assert.Equal(t, 1920, r.Width)
	assert.Equal(t, 1080, r.Height)
}

func TestResolutionSetErrIs(t *testing.T) {
	var r searcher.Resolution
	assert.ErrorIs(t, r.Set("nope"), searcher.ErrInvalidResolution)
}

func TestResolutionString(t *testing.T) {
	r := searcher.Resolution{Width: 800, Height: 600}
	assert.Equal(t, "800x600", r.String())
	assert.Equal(t, "resolution", r.Type())
}
