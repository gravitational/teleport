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

// CommandStartLegacy starts with legacy behavior. This handles flags somewhat
// differently and maintains support for certain deprecated flags, so does not
// use `sharedStartArgs`.
type CommandStartLegacy struct {
	// cmd is the concrete command for this instance
	cmd *kingpin.CmdClause

	// action is the action that will be performed if this command is selected
	action MutatorAction

	// LogFormat controls the format of logging. Can be either `json` or `text`.
	// By default, this is `text`.
	LogFormat string

	// AuthServer is a Teleport auth server address. It may either point
	// directly to an auth server, or to a Teleport proxy server in which case
	// a tunneled auth connection will be established.
	// Prefer using Address() to pick an address.
	AuthServer string

	// DataDir stores the bot's internal data.
	DataDir string

	// DestinationDir stores the generated end-user certificates.
	DestinationDir string

	// Destination is a destination URI
	Destination string

	// CAPins is a list of pinned SKPI hashes of trusted auth server CAs, used
	// only on first connect.
	CAPins []string

	// Token is a bot join token.
	Token string

	// RenewalInterval is the interval at which certificates are renewed, as a
	// time.ParseDuration() string. It must be less than the certificate TTL.
	RenewalInterval time.Duration

	// CertificateTTL is the requested TTL of certificates. It should be some
	// multiple of the renewal interval to allow for failed renewals.
	CertificateTTL time.Duration

	// JoinMethod is the method the bot should use to exchange a token for the
	// initial certificate
	JoinMethod string

	// Oneshot controls whether the bot quits after a single renewal.
	Oneshot bool

	// ProxyServer is the teleport proxy address. Unlike `AuthServer` this must
	// explicitly point to a Teleport proxy.
	// Example: "example.teleport.sh:443"
	ProxyServer string

	// DiagAddr is the address the diagnostics http service should listen on.
	// If not set, no diagnostics listener is created.
	DiagAddr string

	// Insecure instructs `tbot` to trust the Auth Server without verifying the CA.
	Insecure bool
}

// NewLegacyCommand initializes and returns a command supporting
// `tbot start legacy` and `tbot configure legacy`.
func NewLegacyCommand(parentCmd *kingpin.CmdClause, action MutatorAction) *CommandStartLegacy {
	joinMethodList := fmt.Sprintf(
		"(%s)",
		strings.Join(config.SupportedJoinMethods, ", "),
	)

	c := &CommandStartLegacy{
		action: action,
		cmd:    parentCmd.Command("legacy", "Start with either a config file or a legacy output").Default(),
	}
	c.cmd.Flag("auth-server", "Address of the Teleport Auth Server. Prefer using --proxy-server where possible.").Short('a').Envar(AuthServerEnvVar).StringVar(&c.AuthServer)
	c.cmd.Flag("data-dir", "Directory to store internal bot data. Access to this directory should be limited.").StringVar(&c.DataDir)
	c.cmd.Flag("destination-dir", "Directory to write short-lived machine certificates.").StringVar(&c.DestinationDir)
	c.cmd.Flag("proxy-server", "Address of the Teleport Proxy Server.").Envar(ProxyServerEnvVar).StringVar(&c.ProxyServer)
	c.cmd.Flag("token", "A bot join token or path to file with token value, if attempting to onboard a new bot; used on first connect.").Envar(TokenEnvVar).StringVar(&c.Token)
	c.cmd.Flag("ca-pin", "CA pin to validate the Teleport Auth Server; used on first connect.").StringsVar(&c.CAPins)
	c.cmd.Flag("certificate-ttl", "TTL of short-lived machine certificates.").DurationVar(&c.CertificateTTL)
	c.cmd.Flag("renewal-interval", "Interval at which short-lived certificates are renewed; must be less than the certificate TTL.").DurationVar(&c.RenewalInterval)
	c.cmd.Flag("insecure", "Insecure configures the bot to trust the certificates from the Auth Server or Proxy on first connect without verification. Do not use in production.").BoolVar(&c.Insecure)
	c.cmd.Flag("join-method", "Method to use to join the cluster. "+joinMethodList).EnumVar(&c.JoinMethod, config.SupportedJoinMethods...)
	c.cmd.Flag("oneshot", "If set, quit after the first renewal.").BoolVar(&c.Oneshot)
	c.cmd.Flag("diag-addr", "If set and the bot is in debug mode, a diagnostics service will listen on specified address.").StringVar(&c.DiagAddr)
	c.cmd.Flag("log-format", "Controls the format of output logs. Can be `json` or `text`. Defaults to `text`.").
		Default(utils.LogFormatText).
		EnumVar(&c.LogFormat, utils.LogFormatJSON, utils.LogFormatText)

	return c
}

func (c *CommandStartLegacy) TryRun(cmd string) (match bool, err error) {
	switch cmd {
	case c.cmd.FullCommand():
		err = c.action(c)
	default:
		return false, nil
	}

	return true, trace.Wrap(err)
}

func (c *CommandStartLegacy) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	// TODO: Weird flags that need to be addressed:
	// - Debug
	// - FIPS
	// - Insecure

	if c.Oneshot {
		cfg.Oneshot = true
	}

	if c.AuthServer != "" {
		if cfg.AuthServer != "" {
			log.WarnContext(
				context.TODO(),
				"CLI parameters are overriding configuration",
				"flag", "auth-server",
				"config_value", cfg.AuthServer,
				"cli_value", c.AuthServer,
			)
		}
		cfg.AuthServer = c.AuthServer
	}

	if c.ProxyServer != "" {
		if cfg.ProxyServer != "" {
			log.WarnContext(
				context.TODO(),
				"CLI parameters are overriding configuration",
				"flag", "proxy-server",
				"config_value", cfg.ProxyServer,
				"cli_value", c.ProxyServer,
			)
		}
		cfg.ProxyServer = c.ProxyServer
	}

	if c.CertificateTTL != 0 {
		if cfg.CertificateTTL != 0 {
			log.WarnContext(
				context.TODO(),
				"CLI parameters are overriding configuration",
				"flag", "certificate-ttl",
				"config_value", cfg.CertificateTTL,
				"cli_value", c.CertificateTTL,
			)
		}
		cfg.CertificateTTL = c.CertificateTTL
	}

	if c.RenewalInterval != 0 {
		if cfg.RenewalInterval != 0 {
			log.WarnContext(
				context.TODO(),
				"CLI parameters are overriding configuration",
				"flag", "renewal-interval",
				"config_value", cfg.RenewalInterval,
				"cli_value", c.RenewalInterval,
			)
		}
		cfg.RenewalInterval = c.RenewalInterval
	}

	// DataDir overrides any previously-configured storage config
	if c.DataDir != "" {
		if cfg.Storage != nil && cfg.Storage.Destination != nil {
			log.WarnContext(
				context.TODO(),
				"CLI parameters are overriding configuration",
				"flag", "data-dir",
				"config_value", cfg.Storage.Destination.String(),
				"cli_value", c.DataDir,
			)
		}

		dest, err := config.DestinationFromURI(c.DataDir)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Storage = &config.StorageConfig{Destination: dest}
	}

	// If any onboarding flags are set, override the whole section.
	// (CAPath, CAPins, etc follow different codepaths so we don't want a
	// situation where different fields become set weirdly due to struct
	// merging)
	if c.Token != "" || c.JoinMethod != "" || len(c.CAPins) > 0 {
		if !reflect.DeepEqual(cfg.Onboarding, config.OnboardingConfig{}) {
			// To be safe, warn about possible confusion.
			log.WarnContext(
				context.TODO(),
				"CLI parameters are overriding join configuration",
				"cli_token", c.Token,
				"cli_join_method", c.JoinMethod,
				"cli_ca_pins_count", len(c.CAPins),
			)
		}

		cfg.Onboarding = config.OnboardingConfig{
			CAPins:     c.CAPins,
			JoinMethod: types.JoinMethod(c.JoinMethod),
		}
		cfg.Onboarding.SetToken(c.Token)
	}

	// TODO:
	// if c.FIPS {
	// 	cfg.FIPS = c.FIPS
	// }

	if c.DiagAddr != "" {
		if cfg.DiagAddr != "" {
			log.WarnContext(
				context.TODO(),
				"CLI parameters are overriding configuration",
				"flag", "diag-addr",
				"config_value", cfg.DiagAddr,
				"cli_value", c.DiagAddr,
			)
		}
		cfg.DiagAddr = c.DiagAddr
	}

	// TODO: This is now set _before_ CheckAndSetDefaults() which causes a mild
	// change in behavior. Verify this is tolerable.
	if c.Insecure {
		cfg.Insecure = true
	}

	return nil
}
