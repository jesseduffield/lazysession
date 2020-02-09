package app

import (
	"fmt"
	"time"

	"github.com/jesseduffield/gocui"
)

func (app *App) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if v, err := g.SetView("main", -1, -1, maxX, maxY-3, 0); err != nil {
		if err.Error() != "unknown view" {
			return err
		}
		v.Frame = false
		v.Wrap = false
		v.Autoscroll = true
		app.views.main = v

		app.g.SetCurrentView("main")
		go app.onFirstRender()
	}

	if v, err := g.SetView("buffer", 0, maxY-3, maxX-1, maxY-1, 0); err != nil {
		if err.Error() != "unknown view" {
			return err
		}
		v.Frame = true
		v.Wrap = true
		v.Autoscroll = true
		v.Editable = true
		app.views.buffer = v
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
	}()

	// TODO, get gocui to receive a callback on taint so that we don't need to poll
	ticker := time.NewTicker(time.Millisecond * 30)
	for range ticker.C {
		app.g.Update(func(*gocui.Gui) error {
			return nil
		})
	}
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func (app *App) update(f func() error) {
	app.g.Update(func(*gocui.Gui) error {
		return f()
	})
}

func (app *App) setKeybindings() error {
	quitKeys := []interface{}{gocui.KeyEsc, 'q', gocui.KeyCtrlC}
	for _, key := range quitKeys {
		if err := app.g.SetKeybinding("", nil, key, gocui.ModNone, quit); err != nil {
			return err
		}
	}
	if err := app.g.SetKeybinding("main", nil, gocui.MouseWheelDown, gocui.ModNone, app.scrollMainDown); err != nil {
		return err
	}
	if err := app.g.SetKeybinding("main", nil, gocui.MouseWheelUp, gocui.ModNone, app.scrollMainUp); err != nil {
		return err
	}
	if err := app.g.SetKeybinding("main", nil, gocui.KeyTab, gocui.ModNone, app.switchView); err != nil {
		return err
	}
	if err := app.g.SetKeybinding("", nil, gocui.KeyTab, gocui.ModNone, app.switchView); err != nil {
		return err
	}
	if err := app.g.SetKeybinding("buffer", nil, gocui.KeyEnter, gocui.ModNone, app.flushBuffer); err != nil {
		return err
	}
	if err := app.g.SetKeybinding("buffer", nil, gocui.KeyArrowUp, gocui.ModNone, app.prevHistoryItem); err != nil {
		return err
	}
	if err := app.g.SetKeybinding("buffer", nil, gocui.KeyArrowDown, gocui.ModNone, app.nextHistoryItem); err != nil {
		return err
	}
	return nil
}

func (app *App) switchView(g *gocui.Gui, v *gocui.View) error {
	if app.g.CurrentView() == app.views.main {
		_, err := app.g.SetCurrentView("buffer")
		return err
	}
	_, err := app.g.SetCurrentView("main")
	return err
}

func (app *App) flushBuffer(g *gocui.Gui, v *gocui.View) error {
	buffer := app.views.buffer.Buffer()
	app.views.buffer.Clear()
	app.state.History = append(app.state.History, buffer)
	app.state.historyIndex = -1
	app.views.main.StdinWriter.Write([]byte(buffer + "\r"))
	return nil
}

func (app *App) nextHistoryItem(g *gocui.Gui, v *gocui.View) error {
	if app.state.historyIndex == -1 {
		return nil
	}
	app.views.buffer.Clear()
	if app.state.historyIndex < len(app.state.History)-1 {
		app.state.historyIndex++
		fmt.Fprint(app.views.buffer, app.state.History[app.state.historyIndex])
	} else {
		fmt.Fprint(app.views.buffer, app.state.currentLine)
	}
	return nil
}

func (app *App) prevHistoryItem(g *gocui.Gui, v *gocui.View) error {
	if app.state.historyIndex == -1 {
		app.state.currentLine = app.views.buffer.Buffer()
		app.state.historyIndex = len(app.state.History) - 1
	} else if app.state.historyIndex > 0 {
		app.state.historyIndex--
	}
	app.views.buffer.Clear()
	fmt.Fprint(app.views.buffer, app.state.History[app.state.historyIndex])
	return nil
}

func (app *App) scrollMainDown(g *gocui.Gui, v *gocui.View) error {
	return app.scrollDownView("main")
}

func (app *App) scrollMainUp(g *gocui.Gui, v *gocui.View) error {
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
