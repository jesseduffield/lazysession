package app

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"unicode/utf8"

	"github.com/davecgh/go-spew/spew"
	"github.com/fatih/color"
	"github.com/jesseduffield/gocui"
	"github.com/jesseduffield/lazysession/pkg/utils"
	"github.com/jesseduffield/pty"
	"github.com/sirupsen/logrus"
)

type inputReader struct {
	innerReader io.Reader
	log         *logrus.Entry
}

func (r *inputReader) Read(buf []byte) (int, error) {
	// r.log.Warn("reading")
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
			// if we give the view's height the program kind of tries to wrap for us
			// but doesn't do a very good job.
			pty.Setsize(ptmx, &pty.Winsize{Cols: uint16(width), Rows: uint16(height)})
		}
	}()
	ch <- syscall.SIGWINCH // Initial resize.

	view.StdinWriter = ptmx

	_, _ = io.Copy(view, ptmx)

	fmt.Fprintf(view, "\n\n"+utils.ColoredString("command has exited, press 'q' to quit", color.FgGreen))

	view.Pty = false

	return nil
}
