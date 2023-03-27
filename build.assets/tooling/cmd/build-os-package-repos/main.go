/*
Copyright 2022 Gravitational, Inc.

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
	"fmt"
	"os"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

func main() {
	err := run()
	if err != nil {
		logrus.Fatal(err.Error())
	}
}

func buildSubcommandRunners() ([]Runner, error) {
	ar, err := NewAptRunner()
	if err != nil {
		return nil, trace.Wrap(err, "failed to instantiate new APT runner")
	}

	yr, err := NewYumRunner()
	if err != nil {
		return nil, trace.Wrap(err, "failed to instantiate new YUM runner")
	}

	// These should be sorted alphabetically by `Name()`
	return []Runner{
		*ar,
		*yr,
	}, nil
}

func run() error {
	subcommands, err := buildSubcommandRunners()
	if err != nil {
		return trace.Wrap(err, "failed to build subcommand runners")
	}

	// 2 = program name + subcommand
	if len(os.Args) < 2 {
		logHelp(subcommands)
		return trace.Errorf("subcommand not provided")
	}

	subcommandName := strings.ToLower(os.Args[1])
	for _, subcommand := range subcommands {
		if strings.ToLower(subcommandName) != subcommand.Name() {
			continue
		}

		// 2 = program name + subcommand, skip them and get subcommand arguments
		args := os.Args[2:]
		err := subcommand.Init(args)
		if err != nil {
			return trace.Wrap(err, "failed to initialize runner for subcommand %q", subcommandName)
		}

		setupLogger(subcommand.GetLoggerConfig())
		err = subcommand.Run()
		if err != nil {
			return trace.Wrap(err, "failed to run subcommand %q", subcommandName)
		}

		return nil
	}

	if subcommandName == "-h" {
		logHelp(subcommands)
		return nil
	}

	logHelp(subcommands)
	return trace.Errorf("no subcommands found matching %q", subcommandName)
}

func logHelp(subcommands []Runner) {
	executableName := os.Args[0]
	fmt.Printf("%s - OS package repo builder/updater\n", executableName)
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println()
	for _, subcommand := range subcommands {
		fmt.Printf("\t%s\t%s\n", subcommand.Name(), subcommand.Info())
	}
	fmt.Println()
	fmt.Printf("Use \"%s <command> -h\" for more information about a command.\n", executableName)
	fmt.Println()
}

func setupLogger(config *LoggerConfig) {
	if config.logJSON {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{})
	}
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.Level(config.logLevel))
	logrus.Debugf("Setup logger with config: %+v", config)
}
