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
	"os/signal"
	"runtime"
	"runtime/pprof"
	runtimetrace "runtime/trace"
	"strings"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/tbot"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tpm"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentTBot)

const (
	authServerEnvVar  = "TELEPORT_AUTH_SERVER"
	tokenEnvVar       = "TELEPORT_BOT_TOKEN"
	proxyServerEnvVar = "TELEPORT_PROXY"
)

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
	var cf config.CLIConf
	ctx := context.Background()

	var cpuProfile, memProfile, traceProfile string

	app := utils.InitCLIParser("tbot", appHelp).Interspersed(false)
	app.Flag("debug", "Verbose logging to stdout.").Short('d').BoolVar(&cf.Debug)
	app.Flag("config", "Path to a configuration file.").Short('c').StringVar(&cf.ConfigPath)
	app.Flag("fips", "Runs tbot in FIPS compliance mode. This requires the FIPS binary is in use.").BoolVar(&cf.FIPS)
	app.Flag("trace", "Capture and export distributed traces.").Hidden().BoolVar(&cf.Trace)
	app.Flag("trace-exporter", "An OTLP exporter URL to send spans to.").Hidden().StringVar(&cf.TraceExporter)
	app.Flag("mem-profile", "Write memory profile to file").Hidden().StringVar(&memProfile)
	app.Flag("cpu-profile", "Write CPU profile to file").Hidden().StringVar(&cpuProfile)
	app.Flag("trace-profile", "Write trace profile to file").Hidden().StringVar(&traceProfile)
	app.HelpFlag.Short('h')

	joinMethodList := fmt.Sprintf(
		"(%s)",
		strings.Join(config.SupportedJoinMethods, ", "),
	)

	versionCmd := app.Command("version", "Print the version of your tbot binary.")

	startCmd := app.Command("start", "Starts the renewal bot, writing certificates to the data dir at a set interval.")
	startCmd.Flag("auth-server", "Address of the Teleport Auth Server. Prefer using --proxy-server where possible.").Short('a').Envar(authServerEnvVar).StringVar(&cf.AuthServer)
	startCmd.Flag("proxy-server", "Address of the Teleport Proxy Server.").Envar(proxyServerEnvVar).StringVar(&cf.ProxyServer)
	startCmd.Flag("token", "A bot join token or path to file with token value, if attempting to onboard a new bot; used on first connect.").Envar(tokenEnvVar).StringVar(&cf.Token)
	startCmd.Flag("ca-pin", "CA pin to validate the Teleport Auth Server; used on first connect.").StringsVar(&cf.CAPins)
	startCmd.Flag("data-dir", "Directory to store internal bot data. Access to this directory should be limited.").StringVar(&cf.DataDir)
	startCmd.Flag("destination-dir", "Directory to write short-lived machine certificates.").StringVar(&cf.DestinationDir)
	startCmd.Flag("certificate-ttl", "TTL of short-lived machine certificates.").DurationVar(&cf.CertificateTTL)
	startCmd.Flag("renewal-interval", "Interval at which short-lived certificates are renewed; must be less than the certificate TTL.").DurationVar(&cf.RenewalInterval)
	startCmd.Flag("insecure", "Insecure configures the bot to trust the certificates from the Auth Server or Proxy on first connect without verification. Do not use in production.").BoolVar(&cf.Insecure)
	startCmd.Flag("join-method", "Method to use to join the cluster. "+joinMethodList).EnumVar(&cf.JoinMethod, config.SupportedJoinMethods...)
	startCmd.Flag("oneshot", "If set, quit after the first renewal.").BoolVar(&cf.Oneshot)
	startCmd.Flag("diag-addr", "If set and the bot is in debug mode, a diagnostics service will listen on specified address.").StringVar(&cf.DiagAddr)
	startCmd.Flag("log-format", "Controls the format of output logs. Can be `json` or `text`. Defaults to `text`.").
		Default(utils.LogFormatText).
		EnumVar(&cf.LogFormat, utils.LogFormatJSON, utils.LogFormatText)

	initCmd := app.Command("init", "Initialize a certificate destination directory for writes from a separate bot user.")
	initCmd.Flag("destination-dir", "Directory to write short-lived machine certificates to.").StringVar(&cf.DestinationDir)
	initCmd.Flag("owner", "Defines Linux \"user:group\" owner of \"--destination-dir\". Defaults to the Linux user running tbot if unspecified.").StringVar(&cf.Owner)
	initCmd.Flag("bot-user", "Enables POSIX ACLs and defines Linux user that can read/write short-lived certificates to \"--destination-dir\".").StringVar(&cf.BotUser)
	initCmd.Flag("reader-user", "Enables POSIX ACLs and defines Linux user that will read short-lived certificates from \"--destination-dir\".").StringVar(&cf.ReaderUser)
	initCmd.Flag("init-dir", "If using a config file and multiple destinations are configured, controls which destination dir to configure.").StringVar(&cf.InitDir)
	initCmd.Flag("clean", "If set, remove unexpected files and directories from the destination.").BoolVar(&cf.Clean)
	initCmd.Flag("log-format", "Controls the format of output logs. Can be `json` or `text`. Defaults to `text`.").
		Default(utils.LogFormatText).
		EnumVar(&cf.LogFormat, utils.LogFormatJSON, utils.LogFormatText)

	configureCmd := app.Command("configure", "Creates a config file based on flags provided, and writes it to stdout or a file (-c <path>).")
	configureCmd.Flag("auth-server", "Address of the Teleport Auth Server. Prefer using --proxy-server where possible.").Short('a').Envar(authServerEnvVar).StringVar(&cf.AuthServer)
	configureCmd.Flag("proxy-server", "Address of the Teleport Proxy Server.").Envar(proxyServerEnvVar).StringVar(&cf.ProxyServer)
	configureCmd.Flag("ca-pin", "CA pin to validate the Teleport Auth Server; used on first connect.").StringsVar(&cf.CAPins)
	configureCmd.Flag("certificate-ttl", "TTL of short-lived machine certificates.").Default("60m").DurationVar(&cf.CertificateTTL)
	configureCmd.Flag("data-dir", "Directory to store internal bot data. Access to this directory should be limited.").StringVar(&cf.DataDir)
	configureCmd.Flag("insecure", "Insecure configures the bot to trust the certificates from the Auth Server or Proxy on first connect without verification. Do not use in production.").BoolVar(&cf.Insecure)
	configureCmd.Flag("join-method", "Method to use to join the cluster. "+joinMethodList).EnumVar(&cf.JoinMethod, config.SupportedJoinMethods...)
	configureCmd.Flag("oneshot", "If set, quit after the first renewal.").BoolVar(&cf.Oneshot)
	configureCmd.Flag("renewal-interval", "Interval at which short-lived certificates are renewed; must be less than the certificate TTL.").DurationVar(&cf.RenewalInterval)
	configureCmd.Flag("token", "A bot join token, if attempting to onboard a new bot; used on first connect.").Envar(tokenEnvVar).StringVar(&cf.Token)
	configureCmd.Flag("output", "Path to write the generated configuration file to rather than write to stdout.").Short('o').StringVar(&cf.ConfigureOutput)
	configureCmd.Flag("log-format", "Controls the format of output logs. Can be `json` or `text`. Defaults to `text`.").
		Default(utils.LogFormatText).
		EnumVar(&cf.LogFormat, utils.LogFormatJSON, utils.LogFormatText)

	migrateCmd := app.Command("migrate", "Migrates a config file from an older version to the newest version. Outputs to stdout by default.")
	migrateCmd.Flag("output", "Path to write the generated configuration file to rather than write to stdout.").Short('o').StringVar(&cf.ConfigureOutput)

	legacyProxyFlag := ""

	dbCmd := app.Command("db", "Execute database commands through tsh.")
	dbCmd.Flag("proxy-server", "The Teleport proxy server to use, in host:port form.").StringVar(&cf.ProxyServer)
	// We're migrating from --proxy to --proxy-server so this flag is hidden
	// but still supported.
	// TODO(strideynet): DELETE IN 17.0.0
	dbCmd.Flag("proxy", "The Teleport proxy server to use, in host:port form.").Hidden().Envar(proxyServerEnvVar).StringVar(&legacyProxyFlag)
	dbCmd.Flag("destination-dir", "The destination directory with which to authenticate tsh").StringVar(&cf.DestinationDir)
	dbCmd.Flag("cluster", "The cluster name. Extracted from the certificate if unset.").StringVar(&cf.Cluster)
	dbRemaining := config.RemainingArgs(dbCmd.Arg(
		"args",
		"Arguments to `tsh db ...`; prefix with `-- ` to ensure flags are passed correctly.",
	))

	proxyCmd := app.Command("proxy", "Start a local TLS proxy via tsh to connect to Teleport in single-port mode.")
	proxyCmd.Flag("proxy-server", "The Teleport proxy server to use, in host:port form.").Envar(proxyServerEnvVar).StringVar(&cf.ProxyServer)
	// We're migrating from --proxy to --proxy-server so this flag is hidden
	// but still supported.
	// TODO(strideynet): DELETE IN 17.0.0
	proxyCmd.Flag("proxy", "The Teleport proxy server to use, in host:port form.").Hidden().StringVar(&legacyProxyFlag)
	proxyCmd.Flag("destination-dir", "The destination directory with which to authenticate tsh").StringVar(&cf.DestinationDir)
	proxyCmd.Flag("cluster", "The cluster name. Extracted from the certificate if unset.").StringVar(&cf.Cluster)
	proxyRemaining := config.RemainingArgs(proxyCmd.Arg(
		"args",
		"Arguments to `tsh proxy ...`; prefix with `-- ` to ensure flags are passed correctly.",
	))

	sshProxyCmd := app.Command("ssh-proxy-command", "An OpenSSH/PuTTY proxy command").Hidden()
	sshProxyCmd.Flag("destination-dir", "The destination directory with which to authenticate tsh").StringVar(&cf.DestinationDir)
	sshProxyCmd.Flag("cluster", "The cluster name. Extracted from the certificate if unset.").StringVar(&cf.Cluster)
	sshProxyCmd.Flag("user", "The remote user name for the connection").Required().StringVar(&cf.User)
	sshProxyCmd.Flag("host", "The remote host to connect to").Required().StringVar(&cf.Host)
	sshProxyCmd.Flag("port", "The remote port to connect on.").StringVar(&cf.Port)
	sshProxyCmd.Flag("proxy-server", "The Teleport proxy server to use, in host:port form.").Required().StringVar(&cf.ProxyServer)
	sshProxyCmd.Flag("tls-routing", "Whether the Teleport cluster has tls routing enabled.").Required().BoolVar(&cf.TLSRoutingEnabled)
	sshProxyCmd.Flag("connection-upgrade", "Whether the Teleport cluster requires an ALPN connection upgrade.").Required().BoolVar(&cf.ConnectionUpgradeRequired)
	sshProxyCmd.Flag("proxy-templates", "The path to a file containing proxy templates to be evaluated.").StringVar(&cf.TSHConfigPath)
	sshProxyCmd.Flag("resume", "Enable SSH connection resumption").BoolVar(&cf.EnableResumption)

	kubeCmd := app.Command("kube", "Kubernetes helpers").Hidden()
	kubeCredentialsCmd := kubeCmd.Command("credentials", "Get credentials for kubectl access").Hidden()
	kubeCredentialsCmd.Flag("destination-dir", "The destination directory with which to generate Kubernetes credentials").Required().StringVar(&cf.DestinationDir)

	spiffeInspectPath := ""
	spiffeInspectCmd := app.Command("spiffe-inspect", "Inspects a SPIFFE Workload API endpoint to ensure it is working correctly.")
	spiffeInspectCmd.Flag("path", "The path to the SPIFFE Workload API endpoint to test.").Required().StringVar(&spiffeInspectPath)

	tpmCommand := app.Command("tpm", "Commands related to managing TPM joining functionality.")
	tpmIdentifyCommand := tpmCommand.Command("identify", "Output identifying information related to the TPM detected on the system.")

	utils.UpdateAppUsageTemplate(app, args)
	command, err := app.Parse(args)
	if err != nil {
		app.Usage(args)
		return trace.Wrap(err)
	}
	// Logging must be configured as early as possible to ensure all log
	// message are formatted correctly.
	if err := setupLogger(cf.Debug, cf.LogFormat); err != nil {
		return trace.Wrap(err, "setting up logger")
	}

	if legacyProxyFlag != "" {
		cf.ProxyServer = legacyProxyFlag
		log.WarnContext(
			ctx,
			"The --proxy flag is deprecated and will be removed in v17.0.0. Use --proxy-server instead",
		)
	}

	// Remaining args are stored directly to a []string rather than written to
	// a shared ref like most other kingpin args, so we'll need to manually
	// move them to the remaining args field.
	if len(*dbRemaining) > 0 {
		cf.RemainingArgs = *dbRemaining
	} else if len(*proxyRemaining) > 0 {
		cf.RemainingArgs = *proxyRemaining
	}

	if cf.Trace {
		log.InfoContext(
			ctx,
			"Initializing tracing provider. Traces will be exported",
			"trace_exporter", cf.TraceExporter,
		)
		tp, err := initializeTracing(ctx, cf.TraceExporter)
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

	// If migration is specified, we want to run this before the config is
	// loaded normally.
	if migrateCmd.FullCommand() == command {
		return onMigrate(ctx, cf, stdout)
	}

	botConfig, err := config.FromCLIConf(&cf)
	if err != nil {
		return trace.Wrap(err)
	}

	switch command {
	case versionCmd.FullCommand():
		err = onVersion()
	case startCmd.FullCommand():
		err = onStart(ctx, botConfig)
	case configureCmd.FullCommand():
		err = onConfigure(ctx, cf, stdout)
	case initCmd.FullCommand():
		err = onInit(botConfig, &cf)
	case dbCmd.FullCommand():
		err = onDBCommand(botConfig, &cf)
	case proxyCmd.FullCommand():
		err = onProxyCommand(botConfig, &cf)
	case sshProxyCmd.FullCommand():
		err = onSSHProxyCommand(botConfig, &cf)
	case kubeCredentialsCmd.FullCommand():
		err = onKubeCredentialsCommand(ctx, botConfig)
	case spiffeInspectCmd.FullCommand():
		err = onSPIFFEInspect(ctx, spiffeInspectPath)
	case tpmIdentifyCommand.FullCommand():
		query, err := tpm.Query(ctx, log)
		if err != nil {
			return trace.Wrap(err, "querying TPM")
		}
		tpm.PrintQuery(query, cf.Debug, os.Stdout)
	default:
		// This should only happen when there's a missing switch case above.
		err = trace.BadParameter("command %q not configured", command)
	}

	return err
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
	cf config.CLIConf,
	stdout io.Writer,
) error {
	if cf.ConfigPath == "" {
		return trace.BadParameter("source config file must be provided with -c")
	}

	out := stdout
	outPath := cf.ConfigureOutput
	if outPath != "" {
		if outPath == cf.ConfigPath {
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
	cfg, err := config.ReadConfigFromFile(cf.ConfigPath, true)
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

// handleSignals handles incoming Unix signals.
func handleSignals(
	ctx context.Context,
	log *slog.Logger,
	cancel context.CancelFunc,
	reloadCh chan<- struct{},
) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGHUP, syscall.SIGUSR1)

	for sig := range signals {
		switch sig {
		case syscall.SIGINT:
			log.InfoContext(ctx, "Received interrupt, triggering shutdown")
			cancel()
			return
		case syscall.SIGHUP, syscall.SIGUSR1:
			log.InfoContext(ctx, "Received reload signal, queueing reload")
			select {
			case reloadCh <- struct{}{}:
			default:
				log.WarnContext(ctx, "Unable to queue reload, reload already queued")
			}
		}
	}
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
