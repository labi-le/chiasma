package browser_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/labi-le/chiasma/pkg/browser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

// newTempDB creates a writable SQLite database in a temp dir, applies schema,
// and returns an open handle.
func newTempDB(t *testing.T, schema string) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "history.sqlite")
	db, err := sql.Open("sqlite", "file:"+path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(schema)
	require.NoError(t, err)
	return db
}

const chromiumSchema = `CREATE TABLE urls (
	id INTEGER PRIMARY KEY,
	url TEXT,
	last_visit_time INTEGER
);`

const firefoxSchema = `CREATE TABLE moz_formhistory (
	id INTEGER PRIMARY KEY,
	fieldname TEXT,
	value TEXT,
	lastUsed INTEGER
);`

func TestChromiumHistory_GetLastSearch(t *testing.T) {
	tests := []struct {
		name    string
		rows    [][2]any // url, last_visit_time
		want    string
		wantErr error
	}{
		{
			name: "picks most recent search query",
			rows: [][2]any{
				{"https://www.google.com/search?q=old+query", 100},
				{"https://www.google.com/search?q=fresh+query", 300},
				{"https://example.com/page", 400}, // non-search, ignored
			},
			want: "fresh query",
		},
		{
			name:    "empty db returns ErrHistoryIsEmpty",
			rows:    nil,
			wantErr: browser.ErrHistoryIsEmpty,
		},
		{
			name: "no matching search rows returns ErrHistoryIsEmpty",
			rows: [][2]any{
				{"https://example.com/page", 100},
			},
			wantErr: browser.ErrHistoryIsEmpty,
		},
		{
			name: "malformed row without q param yields empty string",
			rows: [][2]any{
				{"https://www.google.com/search?foo=bar", 100},
			},
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := newTempDB(t, chromiumSchema)
			for _, r := range tc.rows {
				_, err := db.Exec("INSERT INTO urls (url, last_visit_time) VALUES (?, ?)", r[0], r[1])
				require.NoError(t, err)
			}

			h := browser.NewChromiumHistory(db)
			got, err := h.GetLastSearch()
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestFirefoxHistory_GetLastSearch(t *testing.T) {
	tests := []struct {
		name    string
		rows    [][3]any // fieldname, value, lastUsed
		want    string
		wantErr error
	}{
		{
			name: "picks most recent searchbar value",
			rows: [][3]any{
				{"searchbar-history", "old term", 100},
				{"searchbar-history", "new term", 300},
				{"email", "nope@example.com", 400}, // other field, ignored
			},
			want: "new term",
		},
		{
			name:    "empty db returns ErrHistoryIsEmpty",
			rows:    nil,
			wantErr: browser.ErrHistoryIsEmpty,
		},
		{
			name: "no searchbar rows returns ErrHistoryIsEmpty",
			rows: [][3]any{
				{"email", "nope@example.com", 100},
			},
			wantErr: browser.ErrHistoryIsEmpty,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := newTempDB(t, firefoxSchema)
			for _, r := range tc.rows {
				_, err := db.Exec("INSERT INTO moz_formhistory (fieldname, value, lastUsed) VALUES (?, ?, ?)", r[0], r[1], r[2])
				require.NoError(t, err)
			}

			h := browser.NewFirefoxHistory(db)
			got, err := h.GetLastSearch()
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestNoopFallback(t *testing.T) {
	h, err := browser.NewHistoryProvider(browser.NoopBrowser, "")
	require.NoError(t, err)

	got, err := h.GetLastSearch()
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.NoError(t, h.Close())
}
