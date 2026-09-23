// Command playwright-install fetches the browsers the e2e suite drives.
package main

import (
	"log"

	"github.com/mxschmitt/playwright-go"
)

func main() {
	if err := playwright.Install(&playwright.RunOptions{Browsers: []string{"chromium"}}); err != nil {
		log.Fatal(err)
	}
}
