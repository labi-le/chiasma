package browser

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"

	// Registers the pure-Go sqlite driver used to read browser history databases.
	_ "modernc.org/sqlite"
)

var ErrHistoryIsEmpty = errors.New("browser history is empty")

type History interface {
	GetLastSearch() (string, error)
	Close() error
}

// NewChromiumHistory returns the History interface as a deliberate injection
// seam so callers depend on the abstraction, not the concrete type.
//
//nolint:ireturn // injection seam: constructor must return the History interface
func NewChromiumHistory(db *sql.DB) History { return &chromiumHistory{db: db} }

type chromiumHistory struct{ db *sql.DB }

func (h *chromiumHistory) Close() error {
	if err := h.db.Close(); err != nil {
		return fmt.Errorf("close chromium history db: %w", err)
	}
	return nil
}

func (h *chromiumHistory) GetLastSearch() (string, error) {
	var lastURL string
	err := h.db.QueryRowContext(context.Background(), `
		SELECT url FROM urls
		WHERE url LIKE 'https://www.google.com/search?%'
		ORDER BY last_visit_time DESC LIMIT 1
	`).Scan(&lastURL)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrHistoryIsEmpty
		}
		return "", fmt.Errorf("query chromium history: %w", err)
	}
	u, err := url.Parse(lastURL)
	if err != nil {
		return "", fmt.Errorf("parse history url: %w", err)
	}
	return u.Query().Get("q"), nil
}

// NewFirefoxHistory returns the History interface as a deliberate injection
// seam so callers depend on the abstraction, not the concrete type.
//
//nolint:ireturn // injection seam: constructor must return the History interface
func NewFirefoxHistory(db *sql.DB) History { return &firefoxHistory{db: db} }

type firefoxHistory struct{ db *sql.DB }

func (h *firefoxHistory) Close() error {
	if err := h.db.Close(); err != nil {
		return fmt.Errorf("close firefox history db: %w", err)
	}
	return nil
}

func (h *firefoxHistory) GetLastSearch() (string, error) {
	var value string
	err := h.db.QueryRowContext(context.Background(), `
		SELECT value FROM moz_formhistory
		WHERE fieldname = 'searchbar-history'
		ORDER BY lastUsed DESC LIMIT 1
	`).Scan(&value)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrHistoryIsEmpty
		}
		return "", fmt.Errorf("query firefox history: %w", err)
	}
	return value, nil
}

type noopHistory struct{}

func (h *noopHistory) Close() error                   { return nil }
func (h *noopHistory) GetLastSearch() (string, error) { return "", nil }

func resolveHistoryPath(browserName, fullPath string) (string, error) {
	if fullPath != "" {
		return fullPath, nil
	}
	if IsChromiumBased(browserName) {
		return fmt.Sprintf("%s/.config/%s/Default/History", os.Getenv("HOME"), browserName), nil
	}
	return "", errors.New("firefox history auto-detection unsupported; pass --history-file")
}

func openReadOnlyDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?immutable=1&mode=ro", path))
	if err != nil {
		return nil, fmt.Errorf("open history db %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if pingErr := db.PingContext(context.Background()); pingErr != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open history db %s: %w", path, pingErr)
	}
	return db, nil
}

// openHistoryDB returns the History interface as a deliberate injection seam
// so callers depend on the abstraction, not the concrete type.
//
//nolint:ireturn // injection seam: factory must return the History interface
func openHistoryDB(browserName, fullPath string) (History, error) {
	path, err := resolveHistoryPath(browserName, fullPath)
	if err != nil {
		return nil, err
	}
	db, err := openReadOnlyDB(path)
	if err != nil {
		return nil, err
	}
	if IsChromiumBased(browserName) {
		return NewChromiumHistory(db), nil
	}
	return NewFirefoxHistory(db), nil
}
