package app

import (
	"io"
	"os"

	"github.com/jesseduffield/gocui"
	"github.com/jesseduffield/pty"
	"golang.org/x/crypto/ssh/terminal"
)

func (app *App) runCommandInPty(view *gocui.View) error {
	// width, height := view.Size()
	// app.Log.Warn(width, height)
	ptmx, err := pty.StartWithSize(app.cmd, &pty.Winsize{Cols: 200, Rows: 200})
	if err != nil {
		return err
	}
	// Make sure to close the pty at the end.
	defer func() { _ = ptmx.Close() }() // Best effort.

	// // Handle pty size.
	// ch := make(chan os.Signal, 1)
	// signal.Notify(ch, syscall.SIGWINCH)
	// go func() {
	// 	for range ch {
	// 		if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
	// 			log.Printf("error resizing pty: %s", err)
	// 		}
	// 	}
	// }()
	// ch <- syscall.SIGWINCH // Initial resize.

	// Set stdin in raw mode.
	oldState, err := terminal.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer func() { _ = terminal.Restore(int(os.Stdin.Fd()), oldState) }() // Best effort.

	go func() { _, _ = io.Copy(ptmx, os.Stdin) }()
	_, _ = io.Copy(os.Stdout, ptmx)

	return nil
}
