package browser

// NewHistoryProvider returns the History interface as a deliberate injection
// seam so callers depend on the abstraction, not the concrete type.
//
//nolint:ireturn // injection seam: factory must return the History interface
func NewHistoryProvider(name string, customPath string) (History, error) {
	if name == NoopBrowser {
		return &noopHistory{}, nil
	}

	return openHistoryDB(name, customPath)
}
