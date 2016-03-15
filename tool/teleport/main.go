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
	"fmt"
	"os"
	"strings"

	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
)

func main() {
	const testRun = false
	run(os.Args[1:], testRun)
}

// same as main() but has a testing switch
func run(cmdlineArgs []string, testRun bool) (executedCommand string, appliedConfig *service.Config) {
	var err error

	// configure logger for a typical CLI scenario until configuration file is
	// parsed
	utils.InitLoggerCLI()
	app := utils.InitCLIParser("teleport", "Clustered SSH service. Learn more at http://teleport.gravitational.com")

	// define global flags:
	var ccf CommandLineFlags
	app.Flag("debug", "Enable verbose logging to stderr").
		Short('d').
		BoolVar(&ccf.Debug)

	// define commands:
	start := app.Command("start", "Starts the Teleport service.")
	status := app.Command("status", "Print the status of the current SSH session.")
	dump := app.Command("configure", "Print the sample config file into stdout.")
	ver := app.Command("version", "Print the version.")
	app.HelpFlag.Short('h')

	// define start flags:
	start.Flag("roles",
		fmt.Sprintf("Comma-separated list of roles to start with [%s]", strings.Join(defaults.StartRoles, ","))).
		Short('r').
		StringVar(&ccf.Roles)
	start.Flag("advertise-ip",
		"IP to advertise to clients if running behind NAT").
		IPVar(&ccf.AdvertiseIP)
	start.Flag("listen-ip",
		fmt.Sprintf("IP address to bind to [%s]", defaults.BindIP)).
		Short('l').
		IPVar(&ccf.ListenIP)
	start.Flag("auth-server",
		fmt.Sprintf("Address of the auth server [%s]", defaults.AuthConnectAddr().Addr)).
		StringVar(&ccf.AuthServerAddr)
	start.Flag("token",
		"One-time token to register with an auth server [none]").
		StringVar(&ccf.AuthToken)
	start.Flag("nodename",
		"Name of this node, defaults to hostname").
		StringVar(&ccf.NodeName)
	start.Flag("config",
		fmt.Sprintf("Path to a configuration file [%v]", defaults.ConfigFilePath)).
		Short('c').ExistingFileVar(&ccf.ConfigFile)
	start.Flag("labels", "List of labels for this node").StringVar(&ccf.Labels)

	// define start's usage info (we use kingpin's "alias" field for this)
	start.Alias(usageNotes + usageExamples)

	// parse CLI commands+flags:
	command, err := app.Parse(cmdlineArgs)
	if err != nil {
		utils.FatalError(err)
	}

	// configuration merge: defaults -> file-based conf -> CLI conf
	config, err := configure(&ccf)
	if err != nil {
		utils.FatalError(err)
	}

	// execute the selected command unless we're running tests
	if !testRun {
		log.Debug(config.DebugDumpToYAML())

		switch command {
		case start.FullCommand():
			err = onStart(config)
		case status.FullCommand():
			err = onStatus(config)
		case dump.FullCommand():
			onConfigDump()
		case ver.FullCommand():
			onVersion()
		}
		if err != nil {
			utils.FatalError(err)
		}
		log.Info("teleport: clean exit")
	}
	return command, config
}

// onStart is the handler for "start" CLI command
func onStart(config *service.Config) error {
	srv, err := service.NewTeleport(config)
	if err != nil {
		return trace.Wrap(err, "initializing teleport")
	}
	if err := srv.Start(); err != nil {
		return trace.Wrap(err, "starting teleport")
	}
	srv.Wait()
	return nil
}

// onStatus is the handler for "status" CLI command
func onStatus(config *service.Config) error {
	sid := os.Getenv("SSH_SESSION_ID")
	proxyHost := os.Getenv("SSH_SESSION_WEBPROXY_ADDR")
	tuser := os.Getenv("SSH_TELEPORT_USER")
	if sid == "" || proxyHost == "" {
		fmt.Println("You are not inside of a Teleport SSH session")
		return nil
	}
	fmt.Printf("User ID    : %s, logged in as %s from %s\n", tuser, os.Getenv("USER"), os.Getenv("SSH_CLIENT"))
	fmt.Printf("Session ID : %s\nSession URL: https://%s/web/sessions/%s\n", sid, proxyHost, sid)
	return nil
}

// onConfigDump is the handler for "configure" CLI command
func onConfigDump() {
	sfc := config.MakeSampleFileConfig()
	fmt.Printf("%s\n%s\n", sampleConfComment, sfc.DebugDumpToYAML())
}

// onVersion is the handler for "version"
func onVersion() {
	utils.PrintVersion()
}
