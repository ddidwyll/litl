// go:build linux
package clipboard

import (
	"os/exec"
	"strings"
)

type wlClipboard struct{}

func newPlatformClipboard() Clipboard { return &wlClipboard{} }

func (c *wlClipboard) Get() (string, error) {
	out, err := exec.Command("wl-paste", "--no-newline").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (c *wlClipboard) Set(text string) error {
	cmd := exec.Command("wl-copy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
