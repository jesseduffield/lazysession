package app

import (
	logNative "log"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/jesseduffield/gocui"
	"github.com/jesseduffield/lazysession/pkg/config"
	"github.com/jesseduffield/lazysession/pkg/i18n"
	"github.com/jesseduffield/lazysession/pkg/log"
	"github.com/sirupsen/logrus"
)

const stateFilename = "state.json"
const chosenDirFilename = "chosen_dir"

// App holds everything we need to function
type App struct {
	g      *gocui.Gui
	state  State
	views  Views
	config *config.AppConfig
	Log    *logrus.Entry
	Tr     i18n.TranslationSet
	cmd    *exec.Cmd
}

// State holds the app's state
type State struct {
	FavDirs []string `json:"favDirs"`
}

// Views stores our views
type Views struct {
	main   *gocui.View
	buffer *gocui.View
}

// NewApp returns a new App
func NewApp(config *config.AppConfig) (*App, error) {
	// no idea why I need to set this: potentially because I invokve the color package before the gui is instantiated
	color.NoColor = false

	logger := log.NewLogger(config)

	tr := i18n.NewTranslationSet(logger)

	app := &App{
		config: config,
		Log:    logger,
		Tr:     tr,
	}

	return app, nil
}

func createCmd() (*exec.Cmd, error) {
	if len(os.Args) == 1 {
		return nil, errors.New("must supply command as an argument")
	}

	if len(os.Args) == 2 {
		return exec.Command(os.Args[1]), nil
	}

	return exec.Command(os.Args[1], os.Args[2:]...), nil
}

// Run runs the app
func (app *App) Run() error {
	if _, err := os.Stat(filepath.Join(app.config.ConfigDir, stateFilename)); os.IsNotExist(err) {
		if err := app.openForFirstTime(); err != nil {
			return err
		}
	}

	// remove the previously chosen dir in case we crash and end up cd'ing for no reason
	if err := app.writeString(chosenDirFilename, ""); err != nil {
		return err
	}

	if err := app.loadState(); err != nil {
		return err
	}

	cmd, err := createCmd()
	if err != nil {
		logNative.Fatalln(err)
		// this is bad, I know
		return err
	}

	app.cmd = cmd

	g, err := gocui.NewGui(gocui.OutputNormal, false, app.Log)
	if err != nil {
		return err
	}
	defer g.Close()

	app.g = g
	app.g.SetManagerFunc(app.layout)
	app.setKeybindings()

	if err := app.g.MainLoop(); err != nil && err != gocui.ErrQuit {
		return err
	}

	return nil
}

func (app *App) openForFirstTime() error {
	state := State{}
	app.state = state

	content, err := json.Marshal(state)
	if err != nil {
		return err
	}

	if err := app.writeBytes(stateFilename, content); err != nil {
		return err
	}

	return nil
}

func (app *App) saveState() error {
	content, err := json.Marshal(app.state)
	if err != nil {
		return err
	}

	return app.writeBytes(stateFilename, content)
}

func (app *App) loadState() error {
	content, err := app.readBytes(stateFilename)
	if err != nil {
		return err
	}

	return json.Unmarshal(content, &app.state)
}

func (app *App) writeString(fileName string, content string) error {
	return app.writeBytes(fileName, []byte(content))
}

func (app *App) writeBytes(fileName string, content []byte) error {
	return ioutil.WriteFile(app.config.ConfigDir+"/"+fileName, content, 0644)
}

func (app *App) readBytes(fileName string) ([]byte, error) {
	return ioutil.ReadFile(app.config.ConfigDir + "/" + fileName)
}
