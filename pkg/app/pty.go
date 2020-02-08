package app

import (
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"unicode/utf8"

	"github.com/davecgh/go-spew/spew"
	"github.com/jesseduffield/gocui"
	"github.com/jesseduffield/pty"
	"github.com/sirupsen/logrus"
)

type inputReader struct {
	innerReader io.Reader
	log         *logrus.Entry
}

func (r *inputReader) Read(buf []byte) (int, error) {
	n, err := r.innerReader.Read(buf)
	if err != nil {
		// we're trying to emulate stdin so we're going to swallow an EOF error
		// when the view's input buffer is empty.
		if err == io.EOF {
			return n, nil
		}
	}

	r.log.Warn(n)
	r.log.Warn(spew.Sdump(buf[0:n]))
	for i := 0; (i+1)*4 <= n; i++ {
		run, _ := utf8.DecodeRune(buf[i*4 : (i+1)*4])
		r.log.Warn(strconv.QuoteRune(run))
	}

	return n, err
}

func (app *App) runCommandInPty(view *gocui.View) error {
	view.Pty = true

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

	// I need pipe from stdin through a buffer that picks up on certain keypresses (notably scroll up, scroll down, and for now, tab, for switching between the two views. I need that to then end up as a reader. I can just compose a reader and if a scroll. I could start off by having a reader that just logs out the runes one by one.

	// // Set stdin in raw mode.
	// oldState, err := terminal.MakeRaw(int(os.Stdin.Fd()))
	// if err != nil {
	// 	panic(err)
	// }
	// defer func() { _ = terminal.Restore(int(os.Stdin.Fd()), oldState) }() // Best effort.

	r := &inputReader{innerReader: &view.InputBuf, log: app.Log}

	go func() { _, _ = io.Copy(ptmx, r) }()
	_, _ = io.Copy(view, ptmx)

	view.Pty = false

	return nil
}
