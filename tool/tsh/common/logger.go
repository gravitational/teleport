// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"fmt"
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
func initLogger(cf *CLIConf, purpose utils.LoggingPurpose, opts loggingOpts) (*slog.Logger, error) {
	cf.OSLog = opts.osLog
	cf.Debug = opts.debug || opts.osLog

	initLoggerOpts := getPlatformInitLoggerOpts(cf)

	level := slog.LevelWarn
	if cf.Debug {
		level = slog.LevelDebug
	}

	logger, err := utils.InitLogger(purpose, level, initLoggerOpts...)
	return logger, trace.Wrap(err)
}

type loggingOpts struct {
	debug bool
	osLog bool
}

// parseLoggingOptsFromEnv calculates logging opts taking into account only env vars.
func parseLoggingOptsFromEnv() loggingOpts {
	var opts loggingOpts
	opts.debug, _ = strconv.ParseBool(os.Getenv(debugEnvVar))
	opts.osLog, _ = strconv.ParseBool(os.Getenv(osLogEnvVar))
	return opts
}

// parseLoggingOptsFromEnvAndArgv calculates logging opts taking into account env vars and argv.
// Before calling this function, make sure that argv has been processed by kingpin (by calling
// kingpin.Application.Parse) so that cf fields set from argv are up-to-date.
//
// CLI flags take precedence over env vars.
func parseLoggingOptsFromEnvAndArgv(cf *CLIConf) loggingOpts {
	opts := parseLoggingOptsFromEnv()

	if cf.DebugSetByUser {
		opts.debug = cf.Debug
	}

	if cf.OSLogSetByUser {
		opts.osLog = cf.OSLog
	}

	return opts
}

func printInitLoggerError(err error) {
	// If initLogger, logger and slog.Default() are likely not going to output any messages anywhere.
	// That's why this functions prints directly to stderr instead.
	fmt.Fprintf(os.Stderr, "WARNING: Could not initialize the logger due to an error, no messages will be logged %s\n\n", trace.DebugReport(err))
}
