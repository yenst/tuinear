package auth

import "github.com/jihmy/tuinear/internal/browser"

func OpenBrowser(url string) error {
	return browser.Open(url)
}
