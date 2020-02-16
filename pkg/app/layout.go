package app

import (
	"fmt"
	"time"

	"github.com/jesseduffield/gocui"
	"github.com/jesseduffield/pty"
)

func (app *App) onResize() error {
	if app.ptmx == nil {
		return nil
	}
	width, height := app.views.main.Size()
	return pty.Setsize(app.ptmx, &pty.Winsize{Cols: uint16(width + 1), Rows: uint16(height)})
}

func (app *App) layout(g *gocui.Gui) error {
	width, height := g.Size()

	bufferHeight := 3
	if app.views.buffer != nil {
		linesHeight := app.views.buffer.LinesHeight()
		if linesHeight == 0 {
			linesHeight = 1
		}
		bufferHeight = linesHeight + 2
	}

	infoHeight := 1

	if v, err := g.SetView("main", -1, -1, width, height-bufferHeight-infoHeight, 0); err != nil {
		if err.Error() != "unknown view" {
			return err
		}
		v.Frame = false
		// wrap off works for rails c, not for irb
		// if we turn on wrap then to get rails c to work we need to act like we have
		// a really wide window.
		// for vim you need to be honest about the width, and set wrap to false
		// for rails c to work with wrap false, you need a carriage return to create a new line
		v.Wrap = true
		// autoscroll is best turned off when you're in a full-window application like vim or lazygit. It would be good to make this adjustable while in the app.
		// TODO: take escape codes like [?1049 to say we're turning off wrap and autoscroll.
		v.Autoscroll = true
		app.views.main = v

		app.g.SetCurrentView("main")
	}

	if v, err := g.SetView("buffer", 0, height-bufferHeight-infoHeight, width-1, height-2, 0); err != nil {
		if err.Error() != "unknown view" {
			return err
		}
		v.Frame = true
		v.Wrap = true
		v.Autoscroll = true
		v.Editable = true
		app.views.buffer = v
	}

	if v, err := g.SetView("info", -1, height-2, width-1, height, 0); err != nil {
		if err.Error() != "unknown view" {
			return err
		}
		v.Frame = false
		app.views.info = v
		fmt.Fprint(v, "use tab to switch from the program to the buffer")
	}

	if !app.started {
		app.started = true
		go app.onFirstRender()
	}

	mainViewWidth, mainViewHeight := app.views.main.Size()
	if mainViewWidth != app.prevWidth || mainViewHeight != app.prevHeight {
		app.prevWidth = mainViewWidth
		app.prevHeight = mainViewHeight
		if err := app.onResize(); err != nil {
			return err
		}
	}

	return nil
}

func (app *App) onFirstRender() {
	go func() {
		if err := app.runCommandInPty(app.views.main); err != nil {
			app.g.Update(func(*gocui.Gui) error {
				return err
			})
		}

		// doing this so that if the buffer is focused we can press 'q' to exit
		app.views.buffer.Editable = false
	}()

	// TODO, get gocui to receive a callback on taint so that we don't need to poll
	ticker := time.NewTicker(time.Millisecond * 30)
	for range ticker.C {
		app.g.Update(func(*gocui.Gui) error {
			return nil
		})
	}
}
