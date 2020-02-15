package app

import "github.com/jesseduffield/gocui"

type binding struct {
	key      interface{}
	viewName string
	handler  func() error
	modifier gocui.Modifier
}

func (app *App) setKeybindings() error {
	bindings := []binding{
		{
			key:      gocui.MouseWheelDown,
			handler:  app.scrollMainDown,
			viewName: "main",
			modifier: gocui.ModNone,
		},
		{
			key:      gocui.MouseWheelUp,
			handler:  app.scrollMainUp,
			viewName: "main",
			modifier: gocui.ModNone,
		},
		{
			key:      gocui.KeyTab,
			handler:  app.switchView,
			viewName: "main",
			modifier: gocui.ModNone,
		},
		{
			key:      gocui.KeyTab,
			handler:  app.switchView,
			viewName: "",
			modifier: gocui.ModNone,
		},
		{
			key:      gocui.KeyEnter,
			handler:  app.flushBuffer,
			viewName: "buffer",
			modifier: gocui.ModNone,
		},
		{
			key:      gocui.KeyArrowUp,
			handler:  app.prevHistoryItem,
			viewName: "buffer",
			modifier: gocui.ModNone,
		},
		{
			key:      gocui.KeyArrowDown,
			handler:  app.nextHistoryItem,
			viewName: "buffer",
			modifier: gocui.ModNone,
		},
	}

	quitKeys := []interface{}{gocui.KeyEsc, 'q', gocui.KeyCtrlC}
	for _, key := range quitKeys {
		bindings = append(bindings, binding{
			key:      key,
			handler:  app.quit,
			viewName: "",
		})
	}

	for _, binding := range bindings {
		if err := app.g.SetBlindKeybinding(binding.viewName, nil, binding.key, gocui.ModNone, binding.handler); err != nil {
			return err
		}
	}

	return nil
}
