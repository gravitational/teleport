/*
Copyright 2021-2022 Gravitational, Inc.

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
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/tbot"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentTBot,
})

const (
	authServerEnvVar = "TELEPORT_AUTH_SERVER"
	tokenEnvVar      = "TELEPORT_BOT_TOKEN"
)

func main() {
	if err := Run(os.Args[1:], os.Stdout); err != nil {
		utils.FatalError(err)
	}
}

func Run(args []string, stdout io.Writer) error {
	var cf config.CLIConf
	utils.InitLogger(utils.LoggingForDaemon, logrus.InfoLevel)

	app := utils.InitCLIParser("tbot", "tbot: Teleport Machine ID").Interspersed(false)
	app.Flag("debug", "Verbose logging to stdout").Short('d').BoolVar(&cf.Debug)
	app.Flag("config", "Path to a configuration file.").Short('c').StringVar(&cf.ConfigPath)
	app.HelpFlag.Short('h')

	versionCmd := app.Command("version", "Print the version of your tbot binary")

	startCmd := app.Command("start", "Starts the renewal bot, writing certificates to the data dir at a set interval.")
	startCmd.Flag("auth-server", "Address of the Teleport Auth Server (On-Prem installs) or Proxy Server (Cloud installs).").Short('a').Envar(authServerEnvVar).StringVar(&cf.AuthServer)
	startCmd.Flag("token", "A bot join token, if attempting to onboard a new bot; used on first connect.").Envar(tokenEnvVar).StringVar(&cf.Token)
	startCmd.Flag("ca-pin", "CA pin to validate the Teleport Auth Server; used on first connect.").StringsVar(&cf.CAPins)
	startCmd.Flag("data-dir", "Directory to store internal bot data. Access to this directory should be limited.").StringVar(&cf.DataDir)
	startCmd.Flag("destination-dir", "Directory to write short-lived machine certificates.").StringVar(&cf.DestinationDir)
	startCmd.Flag("certificate-ttl", "TTL of short-lived machine certificates.").DurationVar(&cf.CertificateTTL)
	startCmd.Flag("renewal-interval", "Interval at which short-lived certificates are renewed; must be less than the certificate TTL.").DurationVar(&cf.RenewalInterval)
	startCmd.Flag("join-method", "Method to use to join the cluster, can be \"token\" or \"iam\".").Default(config.DefaultJoinMethod).EnumVar(&cf.JoinMethod, "token", "iam")
	startCmd.Flag("oneshot", "If set, quit after the first renewal.").BoolVar(&cf.Oneshot)

	initCmd := app.Command("init", "Initialize a certificate destination directory for writes from a separate bot user.")
	initCmd.Flag("destination-dir", "Directory to write short-lived machine certificates to.").StringVar(&cf.DestinationDir)
	initCmd.Flag("owner", "Defines Linux \"user:group\" owner of \"--destination-dir\". Defaults to the Linux user running tbot if unspecified.").StringVar(&cf.Owner)
	initCmd.Flag("bot-user", "Enables POSIX ACLs and defines Linux user that can read/write short-lived certificates to \"--destination-dir\".").StringVar(&cf.BotUser)
	initCmd.Flag("reader-user", "Enables POSIX ACLs and defines Linux user that will read short-lived certificates from \"--destination-dir\".").StringVar(&cf.ReaderUser)
	initCmd.Flag("init-dir", "If using a config file and multiple destinations are configured, controls which destination dir to configure.").StringVar(&cf.InitDir)
	initCmd.Flag("clean", "If set, remove unexpected files and directories from the destination.").BoolVar(&cf.Clean)

	configureCmd := app.Command("configure", "Creates a config file based on flags provided, and writes it to stdout or a file (-c <path>).")
	configureCmd.Flag("auth-server", "Address of the Teleport Auth Server (On-Prem installs) or Proxy Server (Cloud installs).").Short('a').Envar(authServerEnvVar).StringVar(&cf.AuthServer)
	configureCmd.Flag("ca-pin", "CA pin to validate the Teleport Auth Server; used on first connect.").StringsVar(&cf.CAPins)
	configureCmd.Flag("certificate-ttl", "TTL of short-lived machine certificates.").Default("60m").DurationVar(&cf.CertificateTTL)
	configureCmd.Flag("data-dir", "Directory to store internal bot data. Access to this directory should be limited.").StringVar(&cf.DataDir)
	configureCmd.Flag("join-method", "Method to use to join the cluster, can be \"token\" or \"iam\".").Default(config.DefaultJoinMethod).EnumVar(&cf.JoinMethod, "token", "iam")
	configureCmd.Flag("oneshot", "If set, quit after the first renewal.").BoolVar(&cf.Oneshot)
	configureCmd.Flag("renewal-interval", "Interval at which short-lived certificates are renewed; must be less than the certificate TTL.").DurationVar(&cf.RenewalInterval)
	configureCmd.Flag("token", "A bot join token, if attempting to onboard a new bot; used on first connect.").Envar(tokenEnvVar).StringVar(&cf.Token)
	configureCmd.Flag("output", "Path to write the generated configuration file to rather than write to stdout.").Short('o').StringVar(&cf.ConfigureOutput)

	watchCmd := app.Command("watch", "Watch a destination directory for changes.").Hidden()

	dbCmd := app.Command("db", "Execute database commands through tsh")
	dbCmd.Flag("proxy", "The Teleport proxy server to use, in host:port form.").Required().StringVar(&cf.Proxy)
	dbCmd.Flag("destination-dir", "The destination directory with which to authenticate tsh").StringVar(&cf.DestinationDir)
	dbCmd.Flag("cluster", "The cluster name. Extracted from the certificate if unset.").StringVar(&cf.Cluster)
	dbRemaining := config.RemainingArgs(dbCmd.Arg(
		"args",
		"Arguments to `tsh db ...`; prefix with `-- ` to ensure flags are passed correctly.",
	))

	proxyCmd := app.Command("proxy", "Start a local TLS proxy via tsh to connect to Teleport in single-port mode")
	proxyCmd.Flag("proxy", "The Teleport proxy server to use, in host:port form.").Required().StringVar(&cf.Proxy)
	proxyCmd.Flag("destination-dir", "The destination directory with which to authenticate tsh").StringVar(&cf.DestinationDir)
	proxyCmd.Flag("cluster", "The cluster name. Extracted from the certificate if unset.").StringVar(&cf.Cluster)
	proxyRemaining := config.RemainingArgs(proxyCmd.Arg(
		"args",
		"Arguments to `tsh proxy ...`; prefix with `-- ` to ensure flags are passed correctly.",
	))

	utils.UpdateAppUsageTemplate(app, args)
	command, err := app.Parse(args)
	if err != nil {
		app.Usage(args)
		return trace.Wrap(err)
	}

	// Remaining args are stored directly to a []string rather than written to
	// a shared ref like most other kingpin args, so we'll need to manually
	// move them to the remaining args field.
	if len(*dbRemaining) > 0 {
		cf.RemainingArgs = *dbRemaining
	} else if len(*proxyRemaining) > 0 {
		cf.RemainingArgs = *proxyRemaining
	}

	// While in debug mode, send logs to stdout.
	if cf.Debug {
		utils.InitLogger(utils.LoggingForDaemon, logrus.DebugLevel)
	}

	botConfig, err := config.FromCLIConf(&cf)
	if err != nil {
		return trace.Wrap(err)
	}

	switch command {
	case versionCmd.FullCommand():
		err = onVersion()
	case startCmd.FullCommand():
		err = onStart(botConfig)
	case configureCmd.FullCommand():
		err = onConfigure(cf, stdout)
	case initCmd.FullCommand():
		err = onInit(botConfig, &cf)
	case watchCmd.FullCommand():
		err = onWatch(botConfig)
	case dbCmd.FullCommand():
		err = onDBCommand(botConfig, &cf)
	case proxyCmd.FullCommand():
		err = onProxyCommand(botConfig, &cf)
	default:
		// This should only happen when there's a missing switch case above.
		err = trace.BadParameter("command %q not configured", command)
	}

	return err
}

func onVersion() error {
	utils.PrintVersion()
	return nil
}

func onConfigure(
	cf config.CLIConf,
	stdout io.Writer,
) error {
	out := stdout
	outPath := cf.ConfigureOutput
	if outPath != "" {
		f, err := os.Create(outPath)
		if err != nil {
			return trace.Wrap(err)
		}
		defer f.Close()
		out = f
	}

	// We do not want to load an existing configuration file as this will cause
	// it to be merged with the provided flags and defaults.
	cf.ConfigPath = ""
	cfg, err := config.FromCLIConf(&cf)
	if err != nil {
		return nil
	}

	fmt.Fprintln(out, "# tbot config file generated by `configure` command")

	enc := yaml.NewEncoder(out)
	if err := enc.Encode(cfg); err != nil {
		return trace.Wrap(err)
	}

	if err := enc.Close(); err != nil {
		return trace.Wrap(err)
	}

	if outPath != "" {
		log.Infof(
			"Generated config file written to file: %s", outPath,
		)
	}

	return nil
}

func onWatch(botConfig *config.BotConfig) error {
	return trace.NotImplemented("watch not yet implemented")
}

func onStart(botConfig *config.BotConfig) error {
	reloadChan := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handleSignals(log, reloadChan, cancel)

	b := tbot.New(botConfig, log, reloadChan)
	return trace.Wrap(b.Run(ctx))
}

// handleSignals handles incoming Unix signals.
func handleSignals(log logrus.FieldLogger, reload chan struct{}, cancel context.CancelFunc) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGHUP, syscall.SIGUSR1)

	for signal := range signals {
		switch signal {
		case syscall.SIGINT:
			log.Info("Received interrupt, canceling...")
			cancel()
			return
		case syscall.SIGHUP, syscall.SIGUSR1:
			log.Info("Received reload signal, reloading...")
			reload <- struct{}{}
		}
	}
}
