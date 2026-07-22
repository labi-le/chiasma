//nolint:testpackage // white-box test: exercises the unexported buildTagSuffix helper.
package fs

import (
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestBuildTagSuffix(t *testing.T) {
	log := zerolog.Nop()
	tests := []struct {
		name string
		tags []string
		want string
	}{
		{"no tags", nil, ""},
		{"empty slice", []string{}, ""},
		{"single", []string{"forest"}, "__forest"},
		{"multiple", []string{"deep", "forest"}, "__deep_forest"},
		{"lowercased", []string{"FOREST"}, "__forest"},
		{"unsafe chars stripped", []string{"fo!re@st#"}, "__forest"},
		{"dedup by clean form", []string{"Forest", "forest", "FOREST"}, "__forest"},
		{"only unsafe -> dropped", []string{"!!!", "@#$"}, ""},
		{"too long dropped", []string{strings.Repeat("a", 21)}, ""},
		{"exactly max kept", []string{strings.Repeat("a", 20)}, "__" + strings.Repeat("a", 20)},
		{"mix keep and drop", []string{"ok", strings.Repeat("b", 21), "fine"}, "__ok_fine"},
		{"cyrillic preserved", []string{"лес"}, "__лес"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, buildTagSuffix(log, tc.tags))
		})
	}
}
