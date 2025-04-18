package common

import (
	"log/slog"
	"os"
	"strconv"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	debugEnvVar = teleport.VerboseLogsEnvVar // "TELEPORT_DEBUG"
	osLogEnvVar = "TELEPORT_OS_LOG"
)

// initLogger initializes the logger.
//
// It is called twice, first soon after launching tsh before argv is parsed and then again after
// kingpin parses argv. This makes it possible to debug early startup functionality, particularly
// command aliases.
func initLogger(cf *CLIConf, opts debugOpts) error {
	cf.OSLog = opts.osLog
	cf.Debug = opts.debug || opts.osLog

	loggerOpts := getPlatformInitLoggerOpts(cf)

	level := slog.LevelWarn
	if cf.Debug {
		level = slog.LevelDebug
	}

	return trace.Wrap(utils.InitLogger(utils.LoggingForCLI, level, loggerOpts...))
}

type debugOpts struct {
	debug bool
	osLog bool
}

// parseDebugOptsFromEnv calculates debug opts taking into account only env vars.
func parseDebugOptsFromEnv() debugOpts {
	var opts debugOpts
	opts.debug, _ = strconv.ParseBool(os.Getenv(debugEnvVar))
	opts.osLog, _ = strconv.ParseBool(os.Getenv(osLogEnvVar))
	return opts
}

// parseDebugFromEnvAndArgv calculates debug opts taking into account env vars and argv.
// It should be called only after calling kingpin.Application.Parse, so that
// kingpin.FlagCause.IsSetByUser is processed by kingpin.
//
// CLI flags take precedence over env vars.
func parseDebugOptsFromEnvAndArgv(cf *CLIConf) debugOpts {
	opts := parseDebugOptsFromEnv()

	if cf.DebugSet {
		opts.debug = cf.Debug
	}

	if cf.OSLogSet {
		opts.osLog = cf.OSLog
	}

	return opts
}
