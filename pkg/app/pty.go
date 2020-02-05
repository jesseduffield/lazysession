package app

import (
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/jesseduffield/gocui"
	"github.com/jesseduffield/pty"
	"golang.org/x/crypto/ssh/terminal"
)

func (app *App) runCommandInPty(view *gocui.View) error {
	ptmx, err := pty.Start(app.cmd)
	if err != nil {
		return err
	}
	// Make sure to close the pty at the end.
	defer func() { _ = ptmx.Close() }() // Best effort.

	// Handle pty size.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			width, height := view.Size()
			pty.Setsize(ptmx, &pty.Winsize{Cols: uint16(width), Rows: uint16(height)})
		}
	}()
	ch <- syscall.SIGWINCH // Initial resize.

	// Set stdin in raw mode.
	oldState, err := terminal.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer func() { _ = terminal.Restore(int(os.Stdin.Fd()), oldState) }() // Best effort.

	go func() { _, _ = io.Copy(ptmx, os.Stdin) }()
	_, _ = io.Copy(view, ptmx)

	return nil
}
