package searcher

import (
	"errors"
	"os/exec"

	"github.com/vcraescu/go-xrandr"
)

var (
	ErrMonitorNotFound            = errors.New("monitor not found")
	ErrDetectCurrentMode          = errors.New("failed to detect current mode")
	ErrAutoResolutionNotSupported = errors.New("xrandr not found. Please install xrandr or set resolution manually")
)

type Monitor struct {
	ID                string
	CurrentResolution Resolution
}

// MonitorDetector reports the connected screens. It defaults to xrandr.GetScreens
// in production; tests inject a fake to exercise monitor detection.
type MonitorDetector func() (xrandr.Screens, error)

func NewByIDXrandr(id string) (Monitor, error) {
	return newByIDXrandr(xrandr.GetScreens, id)
}

func (m *Monitor) String() string {
	return m.ID
}

func (m *Monitor) Set(s string) error {
	mon, err := NewByIDXrandr(s)
	if err != nil {
		return err
	}

	*m = mon
	return nil
}

func (m *Monitor) Type() string {
	return "monitor"
}

func newByIDXrandr(detect MonitorDetector, id string) (Monitor, error) {
	var mon Monitor
	screens, err := detect()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return mon, ErrAutoResolutionNotSupported
		}
		return mon, err
	}

	monitor, ok := getMonitorByIDOrPrimary(screens, id)
	if !ok {
		return mon, ErrMonitorNotFound
	}
	mode, ok := monitor.CurrentMode()
	if !ok {
		return mon, ErrDetectCurrentMode
	}

	return Monitor{
		ID: id,
		CurrentResolution: Resolution{
			Width:  int(mode.Resolution.Width),
			Height: int(mode.Resolution.Height),
		},
	}, nil
}

func getMonitorByIDOrPrimary(screens xrandr.Screens, id string) (xrandr.Monitor, bool) {
	if id != "" {
		return screens.MonitorByID(id)
	}

	for _, screen := range screens {
		for _, monitor := range screen.Monitors {
			if _, ok := monitor.CurrentMode(); ok {
				return monitor, true
			}
		}
	}

	return xrandr.Monitor{}, false
}
