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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/config"
)

// LegacyDestinationDirArgs is an embeddable struct that provides legacy-style
// --destination-dir handling, largely for reuse by `start legacy` and other
// older subcommands like `init`.
type LegacyDestinationDirArgs struct {
	// DestinationDir stores the generated end-user certificates.
	DestinationDir string
}

// newLegacyDestinationDirArgs initializes the legacy --destination-dir flag on
// the given command, and returns a struct that will contain the parse result.
func newLegacyDestinationDirArgs(cmd *kingpin.CmdClause) *LegacyDestinationDirArgs {
	args := &LegacyDestinationDirArgs{}

	cmd.Flag("destination-dir", "Directory to write short-lived machine certificates.").StringVar(&args.DestinationDir)

	return args
}

func (a *LegacyDestinationDirArgs) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	if a.DestinationDir != "" {
		// WARNING:
		// See: https://github.com/gravitational/teleport/issues/27206 for
		// potential gotchas that currently exist when dealing with this
		// override behavior.

		// CLI only supports a single filesystem Destination with SSH client config
		// and all roles.
		if len(cfg.Services) > 0 {
			log.WarnContext(
				context.TODO(),
				"CLI parameters are overriding output services",
				"flag", "destination-dir",
				"cli_value", a.DestinationDir,
			)
		}

		// When using the CLI --destination-dir we configure an Identity type
		// output for that directory.
		cfg.Services = []config.ServiceConfig{
			&config.IdentityOutput{
				Destination: &config.DestinationDirectory{
					Path: a.DestinationDir,
				},
			},
		}
	}

	return nil
}

// LegacyCommand starts with legacy behavior. This handles flags somewhat
// differently and maintains support for certain deprecated flags, so does not
// use `sharedStartArgs`.
type LegacyCommand struct {
	*AuthProxyArgs
	*LegacyDestinationDirArgs

	// cmd is the concrete command for this instance
	cmd *kingpin.CmdClause

	// action is the action that will be performed if this command is selected
	action MutatorAction

	// LogFormat controls the format of logging. Can be either `json` or `text`.
	// By default, this is `text`.
	LogFormat string

	// DataDir stores the bot's internal data.
	DataDir string

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

	// DiagAddr is the address the diagnostics http service should listen on.
	// If not set, no diagnostics listener is created.
	DiagAddr string
}

// NewLegacyCommand initializes and returns a command supporting
// `tbot start legacy` and `tbot configure legacy`.
func NewLegacyCommand(parentCmd *kingpin.CmdClause, action MutatorAction, mode CommandMode) *LegacyCommand {
	joinMethodList := fmt.Sprintf(
		"(%s)",
		strings.Join(config.SupportedJoinMethods, ", "),
	)

	c := &LegacyCommand{
		action: action,
		cmd:    parentCmd.Command("legacy", fmt.Sprintf("%s tbot with either a config file or a legacy output.", mode)).Default(),
	}
	c.AuthProxyArgs = newAuthProxyArgs(c.cmd)
	c.LegacyDestinationDirArgs = newLegacyDestinationDirArgs(c.cmd)

	c.cmd.Flag("data-dir", "Directory to store internal bot data. Access to this directory should be limited.").StringVar(&c.DataDir)
	c.cmd.Flag("token", "A bot join token or path to file with token value, if attempting to onboard a new bot; used on first connect.").Envar(TokenEnvVar).StringVar(&c.Token)
	c.cmd.Flag("ca-pin", "CA pin to validate the Teleport Auth Server; used on first connect.").StringsVar(&c.CAPins)
	c.cmd.Flag("certificate-ttl", "TTL of short-lived machine certificates.").DurationVar(&c.CertificateTTL)
	c.cmd.Flag("renewal-interval", "Interval at which short-lived certificates are renewed; must be less than the certificate TTL.").DurationVar(&c.RenewalInterval)
	c.cmd.Flag("join-method", "Method to use to join the cluster. "+joinMethodList).EnumVar(&c.JoinMethod, config.SupportedJoinMethods...)
	c.cmd.Flag("oneshot", "If set, quit after the first renewal.").BoolVar(&c.Oneshot)
	c.cmd.Flag("diag-addr", "If set and the bot is in debug mode, a diagnostics service will listen on specified address.").StringVar(&c.DiagAddr)

	return c
}

func (c *LegacyCommand) TryRun(cmd string) (match bool, err error) {
	switch cmd {
	case c.cmd.FullCommand():
		err = c.action(c)
	default:
		return false, nil
	}

	return true, trace.Wrap(err)
}

func (c *LegacyCommand) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	// Note: Debug, FIPS, and Insecure are included from globals

	if c.AuthProxyArgs != nil {
		if err := c.AuthProxyArgs.ApplyConfig(cfg, l); err != nil {
			return trace.Wrap(err)
		}
	}

	if c.LegacyDestinationDirArgs != nil {
		if err := c.LegacyDestinationDirArgs.ApplyConfig(cfg, l); err != nil {
			return trace.Wrap(err)
		}
	}

	if c.Oneshot {
		cfg.Oneshot = true
	}

	if c.CertificateTTL != 0 {
		if cfg.CertificateLifetime.TTL != 0 {
			log.WarnContext(
				context.TODO(),
				"CLI parameters are overriding configuration",
				"flag", "certificate-ttl",
				"config_value", cfg.CertificateLifetime.TTL,
				"cli_value", c.CertificateTTL,
			)
		}
		cfg.CertificateLifetime.TTL = c.CertificateTTL
	}

	if c.RenewalInterval != 0 {
		if cfg.CertificateLifetime.RenewalInterval != 0 {
			log.WarnContext(
				context.TODO(),
				"CLI parameters are overriding configuration",
				"flag", "renewal-interval",
				"config_value", cfg.CertificateLifetime.RenewalInterval,
				"cli_value", c.RenewalInterval,
			)
		}
		cfg.CertificateLifetime.RenewalInterval = c.RenewalInterval
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

	return nil
}
