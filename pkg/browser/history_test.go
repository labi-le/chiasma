//nolint:testpackage // white-box tests exercise unexported resolveHistoryPath/openReadOnlyDB
package browser

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func TestResolveHistoryPath(t *testing.T) {
	t.Setenv("HOME", "/home/tester")

	tests := []struct {
		name        string
		browserName string
		fullPath    string
		want        string
		wantErr     bool
	}{
		{
			name:        "explicit path is returned verbatim",
			browserName: firefoxBrowser,
			fullPath:    "/custom/places.sqlite",
			want:        "/custom/places.sqlite",
		},
		{
			name:        "chromium default path from HOME",
			browserName: "google-chrome",
			fullPath:    "",
			want:        "/home/tester/.config/google-chrome/Default/History",
		},
		{
			name:        "firefox auto-detect is unsupported",
			browserName: firefoxBrowser,
			fullPath:    "",
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveHistoryPath(tc.browserName, tc.fullPath)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "--history-file")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestOpenReadOnlyDB_MissingFileFailsEarly(t *testing.T) {
	_, err := openReadOnlyDB(filepath.Join(t.TempDir(), "does-not-exist.sqlite"))
	require.Error(t, err)
}
