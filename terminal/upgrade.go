package terminal

import "github.com/creack/pty"

func (t *RichTextTerminal) upgrade() {
	t.upgraded = true
	// TODO: Should we re-send everything in t.raw over the pty before doing the upgrade?

	pty.Setsize(t.src, &pty.Winsize{})
	t.upgradeHook(t.src)
}
