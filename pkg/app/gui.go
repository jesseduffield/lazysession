package app

import (
	"fmt"

	"github.com/jesseduffield/gocui"
)

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
	if len(app.state.History) == 0 || app.state.History[len(app.state.History)-1] != buffer {
		app.state.History = append(app.state.History, buffer)
	}

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
