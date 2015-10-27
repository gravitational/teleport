package main

import (
	"os"

	"github.com/gravitational/teleport/lib/service"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	log.Initialize("console", "INFO")
	if err := run(); err != nil {
		log.Errorf("teleport error: %v", err)
		os.Exit(1)
	}
	log.Infof("teleport completed successfully")
}

func run() error {
	app := kingpin.New("teleport", "Teleport is a clustering SSH server")
	configPath := app.Flag("config", "Path to a configuration file in YAML format").ExistingFile()
	useEnv := app.Flag("env", "Configure teleport from environment variables").Bool()

	_, err := app.Parse(os.Args[1:])
	if err != nil {
		return trace.Wrap(err)
	}

	var cfg service.Config
	if *useEnv {
		if err := service.ParseEnv(&cfg); err != nil {
			return trace.Wrap(err)
		}
	} else if *configPath != "" {
		if err := service.ParseYAMLFile(*configPath, &cfg); err != nil {
			return trace.Wrap(err)
		}
	} else {
		return trace.Errorf("Use either --config or --env flags, see --help for details")
	}

	log.Infof("starting with configuration: %#v", cfg)

	srv, err := service.NewTeleport(cfg)
	if err != nil {
		return trace.Wrap(err, "initializing teleport")
	}
	if err := srv.Start(); err != nil {
		return trace.Wrap(err, "starting teleport")
	}
	srv.Wait()
	return nil
}
