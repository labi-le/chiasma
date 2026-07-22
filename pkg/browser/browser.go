package browser

import "slices"

const firefoxBrowser = "firefox"

var (
	ChromiumBasedBrowsers = []string{"google-chrome", "vivaldi", "chromium", "brave", "opera"}
	FirefoxBasedBrowsers  = []string{firefoxBrowser}
)

func AvailableBrowsers() []string {
	return append(ChromiumBasedBrowsers, FirefoxBasedBrowsers...)
}

func IsChromiumBased(browser string) bool {
	return slices.Contains(ChromiumBasedBrowsers, browser)
}
