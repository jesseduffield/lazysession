package main

import (
	"flag"
	"fmt"
	"log"
	"runtime"

	"gopkg.in/yaml.v2"

	"github.com/go-errors/errors"

	"github.com/jesseduffield/lazysession/pkg/app"
	"github.com/jesseduffield/lazysession/pkg/config"
)

var (
	commit      string
	version     = "unversioned"
	date        string
	buildSource = "unknown"

	configFlag    = flag.Bool("config", false, "Print the current default config")
	debuggingFlag = flag.Bool("debug", false, "a boolean")
	versionFlag   = flag.Bool("v", false, "Print the current version")
)

func main() {
	flag.Parse()
	if *versionFlag {
		log.Fatalf("commit=%s, build date=%s, build source=%s, version=%s, os=%s, arch=%s\n", commit, date, buildSource, version, runtime.GOOS, runtime.GOARCH)
	}

	appConfig, err := config.NewAppConfig("lazysession", version, commit, date, buildSource, *debuggingFlag)
	if err != nil {
		log.Fatal(err.Error())
	}

	if *configFlag {
		configContent, err := yaml.Marshal(appConfig)
		if err != nil {
			log.Fatalf("%v\n", configContent)
		}
	}

	app, err := app.NewApp(appConfig)

	if err == nil {
		err = app.Run()
	}

	if err != nil {
		newErr := errors.Wrap(err, 0)
		stackTrace := newErr.ErrorStack()
		app.Log.Error(stackTrace)

		log.Fatal(fmt.Sprintf("%s\n\n%s", app.Tr.ErrorMessage, stackTrace))
	}
}
