package app

import (
	"log"
	"time"

	"github.com/jesseduffield/gocui"
)

func (app *App) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if v, err := g.SetView("main", 0, 0, maxX-1, maxY-1, 0); err != nil {
		if err.Error() != "unknown view" {
			return err
		}
		v.Frame = true
		v.Wrap = false
		v.Autoscroll = true
		app.views.main = v

		app.g.SetCurrentView("main")
		go app.onFirstRender()
	}

	return nil
}

func (app *App) onFirstRender() {
	go func() {
		if err := app.runCommandInPty(app.views.main); err != nil {
			panic(err)
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

func (app *App) setKeybindings() {
	quitKeys := []interface{}{gocui.KeyEsc, 'q', gocui.KeyCtrlC}
	for _, key := range quitKeys {
		if err := app.g.SetKeybinding("", nil, key, gocui.ModNone, quit); err != nil {
			log.Panicln(err)
		}
	}
	if err := app.g.SetKeybinding("main", nil, gocui.MouseWheelDown, gocui.ModNone, app.scrollMainDown); err != nil {
		log.Panicln(err)
	}
	if err := app.g.SetKeybinding("main", nil, gocui.MouseWheelUp, gocui.ModNone, app.scrollMainUp); err != nil {
		log.Panicln(err)
	}
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
