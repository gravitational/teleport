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
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/tbot/config"
)

// AuthProxyArgs is an embeddable struct that can add --auth-server and
// --proxy-server to arbitrary commands for reuse.
type AuthProxyArgs struct {
	// AuthServer is a Teleport auth server address. It may either point
	// directly to an auth server, or to a Teleport proxy server in which case
	// a tunneled auth connection will be established.
	// Prefer using Address() to pick an address.
	AuthServer string

	// ProxyServer is the teleport proxy address. Unlike `AuthServer` this must
	// explicitly point to a Teleport proxy.
	// Example: "example.teleport.sh:443"
	ProxyServer string
}

// NewStaticAuthServer returns an AuthProxyArgs with the given AuthServer field
// configured. Used in tests.
func NewStaticAuthServer(authServer string) *AuthProxyArgs {
	return &AuthProxyArgs{
		AuthServer: authServer,
	}
}

// newAuthProxyArgs initializes --auth-server and --proxy-server args on the
// given command. This can be embedded in any parent command that needs to
// accept an auth or proxy address. Note that `ApplyConfig` will need to be
// called in the parent's own `ApplyConfig`.
func newAuthProxyArgs(cmd *kingpin.CmdClause) *AuthProxyArgs {
	args := &AuthProxyArgs{}

	cmd.Flag("auth-server", "Address of the Teleport Auth Server. Prefer using --proxy-server where possible.").Short('a').Envar(AuthServerEnvVar).StringVar(&args.AuthServer)
	cmd.Flag("proxy-server", "Address of the Teleport Proxy Server.").Envar(ProxyServerEnvVar).StringVar(&args.ProxyServer)

	return args
}

func (a *AuthProxyArgs) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	if a.AuthServer != "" {
		if cfg.AuthServer != "" {
			log.WarnContext(
				context.TODO(),
				"CLI parameters are overriding configuration",
				"flag", "auth-server",
				"config_value", cfg.AuthServer,
				"cli_value", a.AuthServer,
			)
		}
		cfg.AuthServer = a.AuthServer
	}

	if a.ProxyServer != "" {
		if cfg.ProxyServer != "" {
			l.WarnContext(
				context.TODO(),
				"CLI parameters are overriding configuration",
				"flag", "proxy-server",
				"config_value", cfg.ProxyServer,
				"cli_value", a.ProxyServer,
			)
		}
		cfg.ProxyServer = a.ProxyServer
	}

	return nil
}

// sharedStartArgs are arguments that are shared between all modern `start` and
// `configure` subcommands.
type sharedStartArgs struct {
	*AuthProxyArgs

	JoinMethod      string
	Token           string
	CAPins          []string
	CertificateTTL  time.Duration
	RenewalInterval time.Duration
	Storage         string

	Oneshot  bool
	DiagAddr string
}

// newSharedStartArgs initializes shared arguments on the given parent command.
func newSharedStartArgs(cmd *kingpin.CmdClause) *sharedStartArgs {
	args := &sharedStartArgs{}
	args.AuthProxyArgs = newAuthProxyArgs(cmd)

	joinMethodList := fmt.Sprintf(
		"(%s)",
		strings.Join(config.SupportedJoinMethods, ", "),
	)

	cmd.Flag("token", "A bot join token or path to file with token value, if attempting to onboard a new bot; used on first connect.").Envar(TokenEnvVar).StringVar(&args.Token)
	cmd.Flag("ca-pin", "CA pin to validate the Teleport Auth Server; used on first connect.").StringsVar(&args.CAPins)
	cmd.Flag("certificate-ttl", "TTL of short-lived machine certificates.").DurationVar(&args.CertificateTTL)
	cmd.Flag("renewal-interval", "Interval at which short-lived certificates are renewed; must be less than the certificate TTL.").DurationVar(&args.RenewalInterval)
	cmd.Flag("join-method", "Method to use to join the cluster. "+joinMethodList).EnumVar(&args.JoinMethod, config.SupportedJoinMethods...)
	cmd.Flag("oneshot", "If set, quit after the first renewal.").BoolVar(&args.Oneshot)
	cmd.Flag("diag-addr", "If set and the bot is in debug mode, a diagnostics service will listen on specified address.").StringVar(&args.DiagAddr)
	cmd.Flag("storage", "A destination URI for tbot's internal storage, e.g. file:///foo/bar").StringVar(&args.Storage)

	return args
}

func (s *sharedStartArgs) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	// Note: Debug, FIPS, and Insecure are included from globals.

	if s.AuthProxyArgs != nil {
		if err := s.AuthProxyArgs.ApplyConfig(cfg, l); err != nil {
			return trace.Wrap(err)
		}
	}

	if s.Oneshot {
		cfg.Oneshot = true
	}

	if s.CertificateTTL != 0 {
		if cfg.CredentialLifetime.TTL != 0 {
			l.WarnContext(
				context.TODO(),
				"CLI parameters are overriding configuration",
				"flag", "certificate-ttl",
				"config_value", cfg.CredentialLifetime.TTL,
				"cli_value", s.CertificateTTL,
			)
		}
		cfg.CredentialLifetime.TTL = s.CertificateTTL
	}

	if s.RenewalInterval != 0 {
		if cfg.CredentialLifetime.RenewalInterval != 0 {
			l.WarnContext(
				context.TODO(),
				"CLI parameters are overriding configuration",
				"flag", "renewal-interval",
				"config_value", cfg.CredentialLifetime.RenewalInterval,
				"cli_value", s.RenewalInterval,
			)
		}
		cfg.CredentialLifetime.RenewalInterval = s.RenewalInterval
	}

	if s.DiagAddr != "" {
		if cfg.DiagAddr != "" {
			log.WarnContext(
				context.TODO(),
				"CLI parameters are overriding configuration",
				"flag", "diag-addr",
				"config_value", cfg.DiagAddr,
				"cli_value", s.DiagAddr,
			)
		}
		cfg.DiagAddr = s.DiagAddr
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

// sharedDestinationArgs are arguments common to all commands that accept a
// --destination flag and any related flags. Downstream commands will need to
// call `BuildDestination()` to retrieve the value.
type sharedDestinationArgs struct {
	Destination  string
	ReaderUsers  []string
	ReaderGroups []string
}

// newSharedDestinationArgs initializes args that provide --destination and
// related flags.
func newSharedDestinationArgs(cmd *kingpin.CmdClause) *sharedDestinationArgs {
	args := &sharedDestinationArgs{}

	cmd.Flag("destination", "A destination URI, such as file:///foo/bar").Required().StringVar(&args.Destination)
	cmd.Flag("reader-user", "An additional user name or UID that should be allowed by ACLs to read this destination. Only valid for file destinations on Linux.").StringsVar(&args.ReaderUsers)
	cmd.Flag("reader-group", "An additional group name or GID that should be allowed by ACLs to read this destination. Only valid for file destinations on Linux.").StringsVar(&args.ReaderGroups)

	return args
}

func (s *sharedDestinationArgs) BuildDestination() (bot.Destination, error) {
	dest, err := config.DestinationFromURI(s.Destination)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(s.ReaderUsers) > 0 || len(s.ReaderGroups) > 0 {
		// These flags are only supported on directory destinations, so ensure
		// that's what was built.

		dd, ok := dest.(*config.DestinationDirectory)
		if !ok {
			return nil, trace.BadParameter("--reader-user and --reader-group are only compatible with file destinations")
		}

		for _, r := range s.ReaderUsers {
			dd.Readers = append(dd.Readers, &botfs.ACLSelector{
				User: r,
			})
		}

		for _, r := range s.ReaderGroups {
			dd.Readers = append(dd.Readers, &botfs.ACLSelector{
				Group: r,
			})
		}
	}

	return dest, nil
}

// CommandMode is a simple enum to help shared start/configure command
// substitute the correct verb based on whether they are being used for "start"
// or "configure" actions.
type CommandMode int

const (
	// CommandModeStart indicates a command instance will be used for
	// `tbot start ...`
	CommandModeStart CommandMode = iota

	// CommandModeConfigure indicates a command instance will be used for
	// `tbot configure ...`
	CommandModeConfigure
)

func (c CommandMode) String() string {
	switch c {
	case CommandModeConfigure:
		return "Configures"
	default:
		return "Starts"
	}
}
