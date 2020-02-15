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

	if v, err := g.SetView("main", -1, -1, width, height-4, 0); err != nil {
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

	if v, err := g.SetView("buffer", 0, height-4, width-1, height-2, 0); err != nil {
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

	if width != app.prevWidth || height != app.prevHeight {
		app.prevWidth = width
		app.prevHeight = height
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

func (app *App) quit() error {
	app.saveState()
	return gocui.ErrQuit
}

func (app *App) update(f func() error) {
	app.g.Update(func(*gocui.Gui) error {
		return f()
	})
}

func (app *App) switchView() error {
	if app.g.CurrentView() == app.views.main {
		_, err := app.g.SetCurrentView("buffer")
		return err
	}
	_, err := app.g.SetCurrentView("main")
	return err
}

func (app *App) flushBuffer() error {
	buffer := app.views.buffer.Buffer()
	app.views.buffer.Clear()
	app.state.History = append(app.state.History, buffer)
	app.state.historyIndex = -1
	app.views.main.StdinWriter.Write([]byte(buffer + "\r"))
	return nil
}

func (app *App) nextHistoryItem() error {
	if app.state.historyIndex == -1 {
		return nil
	}
	app.views.buffer.Clear()
	if app.state.historyIndex < len(app.state.History)-1 {
		app.state.historyIndex++
		fmt.Fprint(app.views.buffer, app.state.History[app.state.historyIndex])
	} else {
		fmt.Fprint(app.views.buffer, app.state.currentLine)
		app.state.historyIndex = -1
	}
	return nil
}

func (app *App) prevHistoryItem() error {
	if app.state.historyIndex == -1 {
		if len(app.state.History) == 0 {
			return nil
		}
		app.state.currentLine = app.views.buffer.Buffer()
		app.state.historyIndex = len(app.state.History) - 1
	} else if app.state.historyIndex > 0 {
		app.state.historyIndex--
	}
	app.views.buffer.Clear()
	fmt.Fprint(app.views.buffer, app.state.History[app.state.historyIndex])
	return nil
}

func (app *App) scrollMainDown() error {
	return app.scrollDownView("main")
}

func (app *App) scrollMainUp() error {
	return app.scrollUpView("main")
}

func (app *App) scrollUpView(viewName string) error {
	mainView, _ := app.g.View(viewName)
	mainView.Autoscroll = false
	ox, oy := mainView.Origin()
	scrollHeight := 1
	newOy := oy - scrollHeight
	if newOy <= 0 {
		newOy = 0
	}
	return mainView.SetOrigin(ox, newOy)
}

func (app *App) scrollDownView(viewName string) error {
	mainView, _ := app.g.View(viewName)
	mainView.Autoscroll = false
	ox, oy := mainView.Origin()
	_, sy := mainView.Size()
	y := oy + sy
	scrollHeight := 1
	if y < mainView.LinesHeight()-1 {
		if err := mainView.SetOrigin(ox, oy+scrollHeight); err != nil {
			return err
		}
	} else {
		mainView.Autoscroll = true
	}

	return nil
}
