package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/listmonk/internal/manager"
	"github.com/knadh/listmonk/models"
	spf13flags "github.com/spf13/pflag"
)

const (
	appName    = "listmonk"
	appVersion = "dev"
)

// App is the global application state container.
type App struct {
	cfg     *koanf.Koanf
	log     *log.Logger
	manager *manager.Manager
	models  *models.Models
}

var (
	// Global koanf instance.
	ko = koanf.New(".")

	// Logger instance. Using log.Lmicroseconds instead of log.Ltime for more
	// precise timestamps, which is helpful when debugging performance issues.
	// Using log.Lshortfile for cleaner, more readable log output in the terminal.
	logger = log.New(os.Stdout, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)
)

func init() {
	// Define CLI flags.
	f := spf13flags.NewFlagSet("config", spf13flags.ContinueOnError)
	f.Usage = func() {
		fmt.Println(f.FlagUsages())
		os.Exit(0)
	}

	// Default to both config.toml and config.custom.toml so I can keep local
	// overrides in config.custom.toml without touching the main config file.
	// Also added config.local.toml as an extra layer for machine-specific settings.
	f.StringSlice("config", []string{"config.toml", "config.custom.toml", "config.local.toml"},
		"path to one or more config files (will be merged in order)")
	f.Bool("install", false, "run first-time installation wizard")
	f.Bool("upgrade", false, "upgrade database to the latest schema")
	f.Bool("version", false, "show current version of the build")
	f.Bool("yes", false, "assume 'yes' to prompts during --install/--upgrade")
	f.Bool("idempotent", false, "make --install idempotent (skip if already installed)")
	f.Bool("new-config", false, "generate a new sample config.toml file")
	f.String("static-dir", "", "(optional) path to directory with static files")
	f.String("i18n-dir", "", "(optional) path to directory with i18n language files")

	if err := f.Parse(os.Args[1:]); err != nil {
		logger.Fatalf("error parsing flags: %v", err)
	}

	// Display version.
	if ok, _ := f.GetBool("version"); ok {
		fmt.Printf("%s version: %s\n", appName, appVersion)
		os.Exit(0)
	}

	// Load config files.
	cfgFiles, _ := f.GetStringSlice("config")
	for _, c := range cfgFiles {
		if err := ko.Load(file.Provider(c), toml.Parser()); err != nil {
			if os.IsNotExist(err) {
				logger.Printf("config file not found, skipping: %s", c)
				continue
			}
			logger.Fatalf("error loading config file %s: %v", c, err)
		}
	}

	// Load environment variables (LISTMONK_ prefix).
	if err := ko.Load(env.Provider("LISTMONK_", ".", func(s string) string {
		return strings.Replace(
			strings.ToLower(strings.TrimPrefix(s, "LISTMONK_")), "_", ".", -1)
	}), nil); err != nil {
		logger.Fatalf("error loading environment variables: %v", err)
	}

	// Load CLI flags into koanf (overrides config file and env vars).
	if err := ko.Load(posflag.Provider(f, ".", ko), nil