package auth

import "github.com/yenst/tuinear/internal/browser"

func OpenBrowser(url string) error {
	return browser.Open(url)
}
