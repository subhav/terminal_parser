package terminal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"syscall"

	"github.com/creack/pty"
)

// RichTextTerminal is a terminal emulator which is focused on making rich text (HTML) output work *well*, even if that
// comes at the expense of support for interactive terminal applications.
//
// It behaves like a terminal with a single, 4k-char-wide line of output and an infinitely-deep scrollback buffer.
//   - Any attempt to move the cursor to an absolute position will silently fail.
//   - TODO: Any attempt to move the cursor to a different relative line will instead copy that line to a new line.
//
// It also has some unique features:
//   - If it observes that an application is requesting full-screen mode, it will stop running and instead upgrade to a
//     full-featured terminal.
//   - TODO: It tries to detect a prompt and notify the client that it's waiting for input.
type RichTextTerminal struct {
	*parser
	screen

	src *os.File
	raw bytes.Buffer

	upgraded    bool
	upgradeHook func(*os.File)
}

func New(src *os.File, opts ...RichTextTerminalOption) *RichTextTerminal {
	// TODO: Might be a good idea to check here if src is a tty

	t := &RichTextTerminal{
		src: src,
	}
	rd := io.TeeReader(src, &t.raw)
	t.parser = newParser(rd, t)
	t.screen = newScreen()

	for _, opt := range opts {
		opt(t)
	}

	err := pty.Setsize(src, &pty.Winsize{
		Rows: 1,
		// This seems to make apt not display a progress bar. Interesting.
		Cols: 0,
	})
	if err != nil {
		panic(err) // TODO: handle error
	}

	return t
}

func (t *RichTextTerminal) Run(ctx context.Context) {
	// There are effectively two nested state machines: the parser, which reads bytes from the pty and calls event
	// handlers on escape sequences, and the terminal, which advances the parser and updates the screen on those calls.
	var err error
	for err == nil {
		if t.upgraded {
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
		}

		err = t.parser.Continue()
	}

	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, syscall.EIO) {
		log.Printf("parser.Run exited with: %v", err)
	}
}

type RichTextTerminalOption func(*RichTextTerminal)

func WithUpgradeHook(hook func(*os.File)) RichTextTerminalOption {
	return func(t *RichTextTerminal) {
		t.upgradeHook = hook
	}
}
