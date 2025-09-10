/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package cli

import (
	"log/slog"

	"github.com/alecthomas/kingpin/v2"

	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils"
)

// GlobalArgs is a set of arguments additionally passed to all command handlers.
// Some subset of these may be applied to configuration, and a ConfigMutator is
// implemented to handle them; other fields may be referenced ad hoc.
type GlobalArgs struct {
	// FIPS instructs `tbot` to run in a mode designed to comply with FIPS
	// regulations. This means the bot should:
	// - Refuse to run if not compiled with boringcrypto
	// - Use FIPS relevant endpoints for cloud providers (e.g AWS)
	// - Restrict TLS / SSH cipher suites and TLS version
	// - RSA2048 or ECDSA with NIST-P256 curve should be used for private key generation
	FIPS bool

	// ConfigPath is a path to a YAML configuration file to load, if any.
	ConfigPath string

	// ConfigString is a base64 encoded string of a YAML configuration file to load, if any.
	ConfigString string

	// Debug enables debug-level logging, when set
	Debug bool

	// LogFormat configures the output format of the logger
	LogFormat string

	// Trace indicates whether tracing should be enabled.
	Trace bool

	// TraceExporter is a manually provided URI to send traces to instead of
	// forwarding them to the Auth service.
	TraceExporter string

	// Insecure instructs `tbot` to trust the Auth Server without verifying the CA.
	Insecure bool

	// staticConfigYAML allows tests to specify a configuration file statically
	staticConfigYAML string

	fipsSetByUser     bool
	debugSetByUser    bool
	insecureSetByUser bool
}

// NewGlobalArgs appends global flags to the application and returns a struct
// that will be populated at parse time.
func NewGlobalArgs(app *kingpin.Application) *GlobalArgs {
	g := &GlobalArgs{}

	app.Flag("debug", "Verbose logging to stdout.").Short('d').Envar(TBotDebugEnvVar).IsSetByUser(&g.debugSetByUser).BoolVar(&g.Debug)
	app.Flag("config", "Path to a configuration file.").Short('c').Envar(TBotConfigPathEnvVar).StringVar(&g.ConfigPath)
	app.Flag("config-string", "Base64 encoded configuration string.").Hidden().Envar(TBotConfigEnvVar).StringVar(&g.ConfigString)
	app.Flag("fips", "Runs tbot in FIPS compliance mode. This requires the FIPS binary is in use.").IsSetByUser(&g.fipsSetByUser).BoolVar(&g.FIPS)
	app.Flag("trace", "Capture and export distributed traces.").Hidden().BoolVar(&g.Trace)
	app.Flag("trace-exporter", "An OTLP exporter URL to send spans to.").Hidden().StringVar(&g.TraceExporter)
	app.Flag(
		"insecure",
		"Insecure configures the bot to trust the certificates from the Auth "+
			"Server or Proxy on first connect without verification. Do not use in "+
			"production.",
	).IsSetByUser(&g.insecureSetByUser).BoolVar(&g.Insecure)
	app.Flag("log-format", "Controls the format of output logs. Can be `json` or `text`. Defaults to `text`.").
		Default(utils.LogFormatText).
		EnumVar(&g.LogFormat, utils.LogFormatJSON, utils.LogFormatText)

	return g
}

// NewGlobalArgsWithStaticConfig creates a new GlobalArgs instance with a static
// YAML config. This can be used in tests to preload a config file without
// writing to the filesystem. Note that this only works for codepaths that
// make use of `LoadConfigWithMutators`.
func NewGlobalArgsWithStaticConfig(staticYAML string) *GlobalArgs {
	return &GlobalArgs{
		staticConfigYAML: staticYAML,
	}
}

func (g *GlobalArgs) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	// Note: g.ConfigPath is not checked here; the config must have already been
	// loaded.

	if g.fipsSetByUser {
		cfg.FIPS = g.FIPS
	}

	if g.debugSetByUser {
		cfg.Debug = g.Debug
	}

	if g.insecureSetByUser {
		cfg.Insecure = g.Insecure
	}

	return nil
}
