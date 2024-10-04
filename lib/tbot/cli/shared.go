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
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// sharedStartArgs are arguments that are shared between all modern `start` and
// `configure` subcommands.
type sharedStartArgs struct {
	ProxyServer     string
	JoinMethod      string
	Insecure        bool
	Token           string
	CAPins          []string
	CertificateTTL  time.Duration
	RenewalInterval time.Duration
	Storage         string

	LogFormat string
	Oneshot   bool
	DiagAddr  string
}

// newSharedStartArgs initializes shared arguments on the given parent command.
func newSharedStartArgs(cmd *kingpin.CmdClause) *sharedStartArgs {
	args := &sharedStartArgs{}

	joinMethodList := fmt.Sprintf(
		"(%s)",
		strings.Join(config.SupportedJoinMethods, ", "),
	)

	cmd.Flag("proxy-server", "Address of the Teleport Proxy Server.").Envar(proxyServerEnvVar).StringVar(&args.ProxyServer)
	cmd.Flag("token", "A bot join token or path to file with token value, if attempting to onboard a new bot; used on first connect.").Envar(tokenEnvVar).StringVar(&args.Token)
	cmd.Flag("ca-pin", "CA pin to validate the Teleport Auth Server; used on first connect.").StringsVar(&args.CAPins)
	cmd.Flag("certificate-ttl", "TTL of short-lived machine certificates.").DurationVar(&args.CertificateTTL)
	cmd.Flag("renewal-interval", "Interval at which short-lived certificates are renewed; must be less than the certificate TTL.").DurationVar(&args.RenewalInterval)
	cmd.Flag("insecure", "Insecure configures the bot to trust the certificates from the Auth Server or Proxy on first connect without verification. Do not use in production.").BoolVar(&args.Insecure)
	cmd.Flag("join-method", "Method to use to join the cluster. "+joinMethodList).EnumVar(&args.JoinMethod, config.SupportedJoinMethods...)
	cmd.Flag("oneshot", "If set, quit after the first renewal.").BoolVar(&args.Oneshot)
	cmd.Flag("diag-addr", "If set and the bot is in debug mode, a diagnostics service will listen on specified address.").StringVar(&args.DiagAddr)
	cmd.Flag("log-format", "Controls the format of output logs. Can be `json` or `text`. Defaults to `text`.").
		Default(utils.LogFormatText).
		EnumVar(&args.LogFormat, utils.LogFormatJSON, utils.LogFormatText)
	cmd.Flag("storage", "A destination URI for tbot's internal storage, e.g. file:///foo/bar").StringVar(&args.Storage)

	return args
}

func (s *sharedStartArgs) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	// TODO: Weird flags that need to be addressed:
	// - Debug
	// - FIPS
	// - Insecure

	if s.Oneshot {
		cfg.Oneshot = true
	}

	// TODO: in previous versions, `insecure` is handled _after_
	// BotConfig.CheckAndSetDefaults(). This flag is checked and setting it here
	// *will* cause a behavioral change, so make sure the new behavior is sane.
	// (It is unclear why this was done.)
	if s.Insecure {
		cfg.Insecure = true
	}

	if s.ProxyServer != "" {
		if cfg.ProxyServer != "" {
			l.WarnContext(
				context.TODO(),
				"CLI parameters are overriding configuration",
				"flag", "proxy-server",
				"config_value", cfg.ProxyServer,
				"cli_value", s.ProxyServer,
			)
		}
		cfg.ProxyServer = s.ProxyServer
	}

	if s.CertificateTTL != 0 {
		if cfg.CertificateTTL != 0 {
			l.WarnContext(
				context.TODO(),
				"CLI parameters are overriding configuration",
				"flag", "certificate-ttl",
				"config_value", cfg.CertificateTTL,
				"cli_value", s.CertificateTTL,
			)
		}
		cfg.CertificateTTL = s.CertificateTTL
	}

	if s.RenewalInterval != 0 {
		if cfg.RenewalInterval != 0 {
			l.WarnContext(
				context.TODO(),
				"CLI parameters are overriding configuration",
				"flag", "renewal-interval",
				"config_value", cfg.RenewalInterval,
				"cli_value", s.RenewalInterval,
			)
		}
		cfg.RenewalInterval = s.RenewalInterval
	}

	// Storage overrides any previously-configured storage config
	if s.Storage != "" {
		if cfg.Storage != nil && cfg.Storage.Destination != nil {
			l.WarnContext(
				context.TODO(),
				"CLI parameters are overriding configuration",
				"flag", "storage",
				"config_value", cfg.Storage.Destination.String(),
				"cli_value", s.Storage,
			)
		}

		dest, err := config.DestinationFromURI(s.Storage)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Storage = &config.StorageConfig{Destination: dest}
	}

	// If any onboarding flags are set, override the whole section.
	// (CAPath, CAPins, etc follow different codepaths so we don't want a
	// situation where different fields become set weirdly due to struct
	// merging)
	if s.Token != "" || s.JoinMethod != "" || len(s.CAPins) > 0 {
		if !reflect.DeepEqual(cfg.Onboarding, config.OnboardingConfig{}) {
			// To be safe, warn about possible confusion.
			l.WarnContext(
				context.TODO(),
				"CLI parameters are overriding join configuration",
				"cli_token", s.Token,
				"cli_join_method", s.JoinMethod,
				"cli_ca_pins_count", len(s.CAPins),
			)
		}

		cfg.Onboarding = config.OnboardingConfig{
			CAPins:     s.CAPins,
			JoinMethod: types.JoinMethod(s.JoinMethod),
		}
		cfg.Onboarding.SetToken(s.Token)
	}

	return nil
}
