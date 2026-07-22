package searcher //nolint:testpackage // white-box: exercises unexported newByIDXrandr via injected MonitorDetector.

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vcraescu/go-xrandr"
)

func monitor(id string, w, h float32, withMode bool) xrandr.Monitor {
	m := xrandr.Monitor{ID: id, Connected: true}
	if withMode {
		m.Modes = []xrandr.Mode{{
			Resolution:   xrandr.Size{Width: w, Height: h},
			RefreshRates: []xrandr.RefreshRate{{Current: true}},
		}}
	}
	return m
}

func fakeDetector(screens xrandr.Screens, err error) MonitorDetector {
	return func() (xrandr.Screens, error) { return screens, err }
}

func TestNewByIDXrandr(t *testing.T) {
	screens := xrandr.Screens{{
		No:       0,
		Monitors: []xrandr.Monitor{monitor("eDP-1", 2560, 1440, true), monitor("HDMI-1", 1920, 1080, true)},
	}}

	t.Run("found by id", func(t *testing.T) {
		mon, err := newByIDXrandr(fakeDetector(screens, nil), "HDMI-1")
		require.NoError(t, err)
		assert.Equal(t, "HDMI-1", mon.ID)
		assert.Equal(t, 1920, mon.CurrentResolution.Width)
		assert.Equal(t, 1080, mon.CurrentResolution.Height)
	})

	t.Run("primary fallback for empty id", func(t *testing.T) {
		mon, err := newByIDXrandr(fakeDetector(screens, nil), "")
		require.NoError(t, err)
		assert.Equal(t, 2560, mon.CurrentResolution.Width)
	})

	t.Run("unknown monitor", func(t *testing.T) {
		_, err := newByIDXrandr(fakeDetector(screens, nil), "DP-9")
		require.ErrorIs(t, err, ErrMonitorNotFound)
	})

	t.Run("xrandr not installed", func(t *testing.T) {
		_, err := newByIDXrandr(fakeDetector(nil, exec.ErrNotFound), "eDP-1")
		require.ErrorIs(t, err, ErrAutoResolutionNotSupported)
	})

	t.Run("detector error propagates", func(t *testing.T) {
		sentinel := errors.New("boom")
		_, err := newByIDXrandr(fakeDetector(nil, sentinel), "eDP-1")
		require.ErrorIs(t, err, sentinel)
	})

	t.Run("no current mode", func(t *testing.T) {
		noMode := xrandr.Screens{{Monitors: []xrandr.Monitor{monitor("eDP-1", 0, 0, false)}}}
		_, err := newByIDXrandr(fakeDetector(noMode, nil), "eDP-1")
		require.ErrorIs(t, err, ErrDetectCurrentMode)
	})
}
