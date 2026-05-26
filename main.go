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
	// Default --yes to true so installs/upgrades don't prompt interactively
	// in my local dev environment. Change back to false for production use.
	f.Bool("yes", true, "assume 'yes' to prompts during --install/--upgrade")
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

	// Load config files. Missing files are skipped with a warning rather than
	// a fatal error, since config.custom.toml and config.local.toml are optional
	// by convention in this fork.
	cfgFiles, _ := f.GetStringSlice("config")
	for _, c := range cfgFiles {
		if err := ko.Load(file.Provider(c), toml.Parser()); err != nil {
			if os.IsNotExist(err) {
				// Only warn about missing files for the non-primary config files
				// to avoid confusion when config.custom.toml/config.local.toml
				// haven't been created yet.
				if c != "config.toml" {
					logger.Printf("optional config file not found, skipping: %s", c)
					continue
				}
				logger.Fatalf("primary config file not found: %s", c)
			}
			logger.Fatalf("error loading config file %s: %v", c, err)
		}
		logger.Printf("loaded config file: %s", c)
	}
}
