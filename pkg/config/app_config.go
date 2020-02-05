package config

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"

	"github.com/shibukawa/configdir"
)

// AppConfig contains the base configuration fields required for lazygit.
type AppConfig struct {
	Debug       bool   `long:"debug" env:"DEBUG" default:"false"`
	Version     string `long:"version" env:"VERSION" default:"unversioned"`
	Commit      string `long:"commit" env:"COMMIT"`
	BuildDate   string `long:"build-date" env:"BUILD_DATE"`
	Name        string `long:"name" env:"NAME" default:"lazygit"`
	BuildSource string `long:"build-source" env:"BUILD_SOURCE" default:""`
	UserConfig  *UserConfig
	ConfigDir   string
}

// NewAppConfig makes a new app config
func NewAppConfig(name, version, commit, date string, buildSource string, debuggingFlag bool) (*AppConfig, error) {
	configDir, err := findOrCreateConfigDir(name)
	if err != nil {
		return nil, err
	}

	userConfig, err := loadUserConfig(configDir)
	if err != nil {
		return nil, err
	}

	appConfig := &AppConfig{
		Name:        name,
		Version:     version,
		Commit:      commit,
		BuildDate:   date,
		Debug:       os.Getenv("DEBUG") == "TRUE",
		BuildSource: buildSource,
		UserConfig:  userConfig,
		ConfigDir:   configDir,
	}

	return appConfig, nil
}

func findOrCreateConfigDir(projectName string) (string, error) {
	configDirs := configdir.New("jesseduffield", projectName)
	folders := configDirs.QueryFolders(configdir.Global)

	if err := folders[0].CreateParentDir("foo"); err != nil {
		return "", err
	}

	return folders[0].Path, nil
}

func loadUserConfig(configDir string) (*UserConfig, error) {
	config := getDefaultConfig()

	content, err := ioutil.ReadFile(filepath.Join(configDir, "config.yml"))
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	if err := yaml.Unmarshal(content, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// UserConfig is the user's config
type UserConfig struct {
	Gui       GuiConfig
	Reporting string
}

// GuiConfig is the user's gui config
type GuiConfig struct {
	Theme ThemeConfig
}

// ThemeConfig is the user's config
type ThemeConfig struct {
	ActiveBorderColor   []string
	InactiveBorderColor []string
	OptionsTextColor    []string
}

// getDefaultConfig returns the application default configuration
func getDefaultConfig() UserConfig {
	return UserConfig{
		Gui: GuiConfig{
			Theme: ThemeConfig{
				ActiveBorderColor:   []string{"white", "bold"},
				InactiveBorderColor: []string{"white", "blue"},
				OptionsTextColor:    []string{"blue"},
			},
		},
		Reporting: "undetermined",
	}
}
