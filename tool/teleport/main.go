/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"os"

	"github.com/gravitational/teleport/lib/service"

	"github.com/gravitational/log"
	"github.com/gravitational/trace"
	"gopkg.in/alecthomas/kingpin.v2"
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
