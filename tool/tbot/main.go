/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"runtime/pprof"
	runtimetrace "runtime/trace"
	"time"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	autoupdate "github.com/gravitational/teleport/lib/autoupdate/agent"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/tbot"
	"github.com/gravitational/teleport/lib/tbot/cli"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tpm"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentTBot)

func main() {
	if err := Run(os.Args[1:], os.Stdout); err != nil {
		utils.FatalError(err)
	}
}

const appHelp = `Teleport Machine ID

Machine ID issues and renews short-lived certificates so your machines can
access Teleport protected resources in the same way your engineers do!

Find out more at https://goteleport.com/docs/machine-id/introduction/`

func Run(args []string, stdout io.Writer) error {
	ctx := context.Background()

	var cpuProfile, memProfile, traceProfile, configureOutPath string

	app := utils.InitCLIParser("tbot", appHelp).Interspersed(false)
	globalCfg := cli.NewGlobalArgs(app)

	// Miscellaneous args exposed globally but handled here.
	app.Flag("mem-profile", "Write memory profile to file").Hidden().StringVar(&memProfile)
	app.Flag("cpu-profile", "Write CPU profile to file").Hidden().StringVar(&cpuProfile)
	app.Flag("trace-profile", "Write trace profile to file").Hidden().StringVar(&traceProfile)
	app.HelpFlag.Short('h')

	// Construct the top-level subcommands.
	versionCmd := app.Command("version", "Print the version of your tbot binary.")

	kubeCmd := app.Command("kube", "Kubernetes helpers").Hidden()

	startCmd := app.Command("start", "Starts the renewal bot, writing certificates to the data dir at a set interval.")

	configureCmd := app.Command("configure", "Creates a config file based on flags provided, and writes it to stdout or a file (-c <path>).")
	configureCmd.Flag("output", "Path to write the generated configuration file to rather than write to stdout.").Short('o').StringVar(&configureOutPath)

	keypairCmd := app.Command("keypair", "Manage keypairs for bound-keypair joining")

	// TODO: consider discarding config flag for non-legacy. These should always be self contained.

	// Initialize all new-style commands.
	var commands []cli.CommandRunner
	commands = append(commands,
		cli.NewInitCommand(app, func(init *cli.InitCommand) error {
			return onInit(globalCfg, init)
		}),

		cli.NewMigrateCommand(app, func(migrateCfg *cli.MigrateCommand) error {
			return onMigrate(ctx, globalCfg, migrateCfg, stdout)
		}),

		cli.NewSSHProxyCommand(app, func(sshProxyCommand *cli.SSHProxyCommand) error {
			return onSSHProxyCommand(ctx, globalCfg, sshProxyCommand)
		}),

		cli.NewProxyCommand(app, func(proxyCmd *cli.ProxyCommand) error {
			return onProxyCommand(ctx, globalCfg, proxyCmd)
		}),

		cli.NewDBCommand(app, func(dbCmd *cli.DBCommand) error {
			return onDBCommand(globalCfg, dbCmd)
		}),

		cli.NewSSHMultiplexerProxyCommand(app, func(c *cli.SSHMultiplexerProxyCommand) error {
			return onSSHMultiplexProxyCommand(ctx, c.Socket, c.Data)
		}),

		cli.NewKubeCredentialsCommand(kubeCmd, func(kubeCredentialsCmd *cli.KubeCredentialsCommand) error {
			return onKubeCredentialsCommand(ctx, kubeCredentialsCmd)
		}),

		cli.NewKeypairCreateCommand(keypairCmd, func(keypairCreateCmd *cli.KeypairCreateCommand) error {
			return onKeypairCreateCommand(ctx, globalCfg, keypairCreateCmd)
		}),

		// `start` and `configure` commands
		cli.NewLegacyCommand(startCmd, buildConfigAndStart(ctx, globalCfg), cli.CommandModeStart),
		cli.NewLegacyCommand(configureCmd, buildConfigAndConfigure(ctx, globalCfg, &configureOutPath, stdout), cli.CommandModeConfigure),

		cli.NewIdentityCommand(startCmd, buildConfigAndStart(ctx, globalCfg), cli.CommandModeStart),
		cli.NewIdentityCommand(configureCmd, buildConfigAndConfigure(ctx, globalCfg, &configureOutPath, stdout), cli.CommandModeConfigure),

		cli.NewDatabaseCommand(startCmd, buildConfigAndStart(ctx, globalCfg), cli.CommandModeStart),
		cli.NewDatabaseCommand(configureCmd, buildConfigAndConfigure(ctx, globalCfg, &configureOutPath, stdout), cli.CommandModeConfigure),

		cli.NewKubernetesCommand(startCmd, buildConfigAndStart(ctx, globalCfg), cli.CommandModeStart),
		cli.NewKubernetesCommand(configureCmd, buildConfigAndConfigure(ctx, globalCfg, &configureOutPath, stdout), cli.CommandModeConfigure),

		cli.NewKubernetesV2Command(startCmd, buildConfigAndStart(ctx, globalCfg), cli.CommandModeStart),
		cli.NewKubernetesV2Command(configureCmd, buildConfigAndConfigure(ctx, globalCfg, &configureOutPath, stdout), cli.CommandModeConfigure),

		cli.NewApplicationCommand(startCmd, buildConfigAndStart(ctx, globalCfg), cli.CommandModeStart),
		cli.NewApplicationCommand(configureCmd, buildConfigAndConfigure(ctx, globalCfg, &configureOutPath, stdout), cli.CommandModeConfigure),

		cli.NewApplicationTunnelCommand(startCmd, buildConfigAndStart(ctx, globalCfg), cli.CommandModeStart),
		cli.NewApplicationTunnelCommand(configureCmd, buildConfigAndConfigure(ctx, globalCfg, &configureOutPath, stdout), cli.CommandModeConfigure),

		cli.NewDatabaseTunnelCommand(startCmd, buildConfigAndStart(ctx, globalCfg), cli.CommandModeStart),
		cli.NewDatabaseTunnelCommand(configureCmd, buildConfigAndConfigure(ctx, globalCfg, &configureOutPath, stdout), cli.CommandModeConfigure),

		cli.NewSPIFFESVIDCommand(startCmd, buildConfigAndStart(ctx, globalCfg), cli.CommandModeStart),
		cli.NewSPIFFESVIDCommand(configureCmd, buildConfigAndConfigure(ctx, globalCfg, &configureOutPath, stdout), cli.CommandModeConfigure),

		cli.NewWorkloadIdentityX509Command(startCmd, buildConfigAndStart(ctx, globalCfg), cli.CommandModeStart),
		cli.NewWorkloadIdentityX509Command(configureCmd, buildConfigAndConfigure(ctx, globalCfg, &configureOutPath, stdout), cli.CommandModeConfigure),

		cli.NewWorkloadIdentityAPICommand(startCmd, buildConfigAndStart(ctx, globalCfg), cli.CommandModeStart),
		cli.NewWorkloadIdentityAPICommand(configureCmd, buildConfigAndConfigure(ctx, globalCfg, &configureOutPath, stdout), cli.CommandModeConfigure),

		cli.NewWorkloadIdentityJWTCommand(startCmd, buildConfigAndStart(ctx, globalCfg), cli.CommandModeStart),
		cli.NewWorkloadIdentityJWTCommand(configureCmd, buildConfigAndConfigure(ctx, globalCfg, &configureOutPath, stdout), cli.CommandModeConfigure),

		cli.NewWorkloadIdentityAWSRACommand(startCmd, buildConfigAndStart(ctx, globalCfg), cli.CommandModeStart),
		cli.NewWorkloadIdentityAWSRACommand(configureCmd, buildConfigAndConfigure(ctx, globalCfg, &configureOutPath, stdout), cli.CommandModeConfigure),
	)

	// Initialize legacy-style commands. These are simple enough to not really
	// benefit from conversion to a new-style command.
	spiffeInspectPath := ""
	spiffeInspectCmd := app.Command("spiffe-inspect", "Inspects a SPIFFE Workload API endpoint to ensure it is working correctly.")
	spiffeInspectCmd.Flag("path", "The path to the SPIFFE Workload API endpoint to test.").Required().StringVar(&spiffeInspectPath)

	tpmCommand := app.Command("tpm", "Commands related to managing TPM joining functionality.")
	tpmIdentifyCommand := tpmCommand.Command("identify", "Output identifying information related to the TPM detected on the system.")

	installSystemdCmdStr, installSystemdCmdFn := setupInstallSystemdCmd(app)

	utils.UpdateAppUsageTemplate(app, args)
	command, err := app.Parse(args)
	if err != nil {
		app.Usage(args)
		return trace.Wrap(err)
	}
	// Logging must be configured as early as possible to ensure all log
	// message are formatted correctly.
	if err := setupLogger(globalCfg.Debug, globalCfg.LogFormat); err != nil {
		return trace.Wrap(err, "setting up logger")
	}

	if globalCfg.Trace {
		log.InfoContext(
			ctx,
			"Initializing tracing provider. Traces will be exported",
			"trace_exporter", globalCfg.TraceExporter,
		)
		tp, err := initializeTracing(ctx, globalCfg.TraceExporter)
		if err != nil {
			return trace.Wrap(err, "initializing tracing")
		}
		defer func() {
			ctx, cancel := context.WithTimeout(
				ctx, 5*time.Second,
			)
			defer cancel()
			log.InfoContext(ctx, "Shutting down tracing provider")
			if err := tp.Shutdown(ctx); err != nil {
				log.ErrorContext(
					ctx,
					"Failed to shut down tracing provider",
					"error", err,
				)
			}
			log.InfoContext(ctx, "Shut down tracing provider")
		}()
	}

	if cpuProfile != "" {
		log.DebugContext(ctx, "capturing CPU profile", "profile_path", cpuProfile)
		f, err := os.Create(cpuProfile)
		if err != nil {
			return trace.Wrap(err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			return trace.Wrap(err)
		}
		defer pprof.StopCPUProfile()
	}

	if memProfile != "" {
		log.DebugContext(ctx, "capturing memory profile", "profile_path", memProfile)
		defer func() {
			f, err := os.Create(memProfile)
			if err != nil {
				return
			}
			defer f.Close()
			runtime.GC()
			if err := pprof.WriteHeapProfile(f); err != nil {
				return
			}
		}()
	}

	if traceProfile != "" {
		log.DebugContext(ctx, "capturing trace profile", "profile_path", traceProfile)
		f, err := os.Create(traceProfile)
		if err != nil {
			return trace.Wrap(err)
		}
		defer f.Close()

		if err := runtimetrace.Start(f); err != nil {
			return trace.Wrap(err)
		}
		defer runtimetrace.Stop()
	}

	// Manually attempt to run all old-style commands.
	switch command {
	case versionCmd.FullCommand():
		return onVersion()
	case spiffeInspectCmd.FullCommand():
		return onSPIFFEInspect(ctx, spiffeInspectPath)
	case tpmIdentifyCommand.FullCommand():
		query, err := tpm.Query(ctx, log)
		if err != nil {
			return trace.Wrap(err, "querying TPM")
		}
		tpm.PrintQuery(query, globalCfg.Debug, os.Stdout)
		return nil
	case installSystemdCmdStr:
		return installSystemdCmdFn(ctx, log, globalCfg.ConfigPath, autoupdate.StableExecutable, os.Stdout)
	}

	// Attempt to run each new-style command.
	for _, cmd := range commands {
		match, err := cmd.TryRun(command)
		if !match {
			continue
		}

		return trace.Wrap(err)
	}

	return trace.BadParameter("command %q not configured", command)
}

// buildConfigAndStart returns a MutatorAction that will generate a config and
// run `onStart` with the result.
func buildConfigAndStart(ctx context.Context, globals *cli.GlobalArgs) cli.MutatorAction {
	return func(mutator cli.ConfigMutator) error {
		cfg, err := cli.LoadConfigWithMutators(globals, mutator)
		if err != nil {
			return trace.Wrap(err)
		}

		return trace.Wrap(onStart(ctx, cfg))
	}
}

// buildConfigAndConfigure returns a MutatorAction that will generate a config
// and run `onConfigure` with the result.
func buildConfigAndConfigure(ctx context.Context, globals *cli.GlobalArgs, outPath *string, stdout io.Writer) cli.MutatorAction {
	return func(mutator cli.ConfigMutator) error {
		cfg, err := cli.BaseConfigWithMutators(globals, mutator)
		if err != nil {
			return trace.Wrap(err)
		}

		return trace.Wrap(onConfigure(ctx, cfg, *outPath, stdout))
	}
}

func initializeTracing(
	ctx context.Context, endpoint string,
) (*tracing.Provider, error) {
	if endpoint == "" {
		return nil, trace.BadParameter("trace exporter URL must be provided")
	}

	provider, err := tracing.NewTraceProvider(ctx, tracing.Config{
		Service:     teleport.ComponentTBot,
		ExporterURL: endpoint,
		// We are using 1 here to record all spans as a result of this tbot command. Teleport
		// will respect the recording flag of remote spans even if the spans it generates
		// wouldn't otherwise be recorded due to its configured sampling rate.
		SamplingRate: 1.0,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return provider, nil
}

func onVersion() error {
	modules.GetModules().PrintVersion()
	return nil
}

func onConfigure(
	ctx context.Context,
	cfg *config.BotConfig,
	outPath string,
	stdout io.Writer,
) error {
	out := stdout
	if outPath != "" {
		f, err := os.Create(outPath)
		if err != nil {
			return trace.Wrap(err)
		}
		defer f.Close()
		out = f
	}

	// Ensure they have provided a join method to use in the configuration.
	if cfg.Onboarding.JoinMethod == types.JoinMethodUnspecified {
		return trace.BadParameter("join method must be provided")
	}

	fmt.Fprintln(out, "# tbot config file generated by `configure` command")

	enc := yaml.NewEncoder(out)
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		return trace.Wrap(err)
	}

	if err := enc.Close(); err != nil {
		return trace.Wrap(err)
	}

	if outPath != "" {
		log.InfoContext(
			ctx, "Generated config file written", "path", outPath,
		)
	}

	return nil
}

func onMigrate(
	ctx context.Context,
	globalCfg *cli.GlobalArgs,
	migrateCmd *cli.MigrateCommand,
	stdout io.Writer,
) error {
	if globalCfg.ConfigPath == "" {
		return trace.BadParameter("source config file must be provided with -c")
	}

	out := stdout
	outPath := migrateCmd.ConfigureOutput
	if outPath != "" {
		if outPath == globalCfg.ConfigPath {
			return trace.BadParameter("migrated config output path should not be the same as the source config path")
		}

		f, err := os.Create(outPath)
		if err != nil {
			return trace.Wrap(err)
		}
		defer f.Close()
		out = f
	}

	// We do not want to load an existing configuration file as this will cause
	// it to be merged with the provided flags and defaults.
	cfg, err := config.ReadConfigFromFile(globalCfg.ConfigPath, true)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating new config")
	}

	fmt.Fprintln(out, "# tbot config file generated by `migrate` command")

	enc := yaml.NewEncoder(out)
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		return trace.Wrap(err)
	}

	if err := enc.Close(); err != nil {
		return trace.Wrap(err)
	}

	if outPath != "" {
		log.InfoContext(
			ctx, "Generated config file written", "path", outPath,
		)
	}

	return nil
}

func onStart(ctx context.Context, botConfig *config.BotConfig) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	reloadCh := make(chan struct{})
	botConfig.ReloadCh = reloadCh
	go handleSignals(ctx, log, cancel, reloadCh)

	telemetrySentCh := make(chan struct{})
	go func() {
		defer close(telemetrySentCh)

		if err := sendTelemetry(
			ctx, telemetryClient(os.Getenv), os.Getenv, log, botConfig,
		); err != nil {
			log.ErrorContext(
				ctx, "Failed to send anonymous telemetry.", "error", err,
			)
		}
	}()
	// Ensures telemetry finishes sending before function exits.
	defer func() {
		select {
		case <-telemetrySentCh:
			return
		case <-ctx.Done():
		default:
		}

		waitTime := 10 * time.Second
		log.InfoContext(
			ctx,
			"Waiting for anonymous telemetry to finish sending before exiting. Press CTRL-C to cancel",
			"wait_time",
			waitTime,
		)
		ctx, cancel := context.WithTimeout(ctx, waitTime)
		defer cancel()
		select {
		case <-ctx.Done():
			log.WarnContext(
				ctx,
				"Anonymous telemetry transmission canceled due to signal or timeout",
			)
		case <-telemetrySentCh:
		}
	}()

	b := tbot.New(botConfig, log)
	return trace.Wrap(b.Run(ctx))
}

func setupLogger(debug bool, format string) error {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	switch format {
	case utils.LogFormatJSON:
	case utils.LogFormatText, "":
	default:
		return trace.BadParameter("unsupported log format %q", format)
	}

	utils.InitLogger(utils.LoggingForDaemon, level, utils.WithLogFormat(format))

	return nil
}
