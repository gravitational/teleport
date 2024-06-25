/*
Copyright 2015-2021 Gravitational, Inc.

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
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

// cli is CLI configuration
var cli CLI

const (
	// pluginName is the plugin name
	pluginName = "Teleport event handler"

	// pluginDescription is the plugin description
	pluginDescription = "Forwards Teleport AuditLog to external sources"

	// gracefulShutdownTimeout is the graceful shutdown timeout
	gracefulShutdownTimeout = 5 * time.Second
)

func main() {
	logger.Init()

	ctx := kong.Parse(
		&cli,
		kong.UsageOnError(),
		kong.Configuration(KongTOMLResolver),
		kong.Name(pluginName),
		kong.Description(pluginDescription),
	)

	if cli.Debug {
		enableLogDebug()
	}

	switch {
	case ctx.Command() == "version":
		lib.PrintVersion(pluginName, Version, Gitref)
	case strings.HasPrefix(ctx.Command(), "configure"):
		err := RunConfigureCmd(&cli.Configure)
		if err != nil {
			fmt.Println(trace.DebugReport(err))
			os.Exit(-1)
		}
	case ctx.Command() == "start":
		err := start()

		if err != nil {
			lib.Bail(err)
		} else {
			logger.Standard().Info("Successfully shut down")
		}
	}
}

// turn on log debugging
func enableLogDebug() {
	err := logger.Setup(logger.Config{Severity: "debug", Output: "stderr"})
	if err != nil {
		fmt.Println(trace.DebugReport(err))
		os.Exit(-1)
	}
}

// start spawns the main process
func start() error {
	app, err := NewApp(&cli.Start)
	if err != nil {
		return trace.Wrap(err)
	}

	go lib.ServeSignals(app, gracefulShutdownTimeout)

	return trace.Wrap(
		app.Run(context.Background()),
	)
}
