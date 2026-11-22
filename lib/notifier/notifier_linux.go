//go:build linux

package notifier

import "os/exec"

type notifySendNotifier struct{}

func New() Notifier { return &notifySendNotifier{} }

func (n *notifySendNotifier) Notify(title, msg string) {
	exec.Command("dunst", "-print", "TITL", title, msg).Run()
}
