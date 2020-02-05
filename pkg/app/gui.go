package app

import (
	"fmt"
	"log"

	"github.com/jesseduffield/gocui"
)

func (app *App) refreshMain() error {
	app.update(func() error {
		app.views.main.Clear()
		fmt.Fprint(app.views.main, "test\ntest\ntest\ntest\ntest\ntest\ntest\ntest\ntest\ntest\ntest\n")
		return nil
	})
	return nil
}

func (app *App) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if v, err := g.SetView("main", -1, -1, maxX, maxY, 0); err != nil {
		if err.Error() != "unknown view" {
			return err
		}
		v.Frame = false
		v.Highlight = true
		app.views.main = v

		app.refreshMain()
		app.g.SetCurrentView("main")
	}

	return nil
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
}
