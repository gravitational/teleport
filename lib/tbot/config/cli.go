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

package config

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

type globalArgs struct {
	FIPS bool

	// These properties are not applied to the config.

	ConfigPath string
	Debug      bool
}

func (g *globalArgs) ApplyConfig(cfg *BotConfig) error {
	if g.FIPS {
		cfg.FIPS = g.FIPS
	}

	if g.Debug {
		cfg.Debug = g.Debug
	}

	return nil
}

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
		strings.Join(SupportedJoinMethods, ", "),
	)

	cmd.Flag("proxy-server", "Address of the Teleport Proxy Server.").Envar(proxyServerEnvVar).StringVar(&args.ProxyServer)
	cmd.Flag("token", "A bot join token or path to file with token value, if attempting to onboard a new bot; used on first connect.").Envar(tokenEnvVar).StringVar(&args.Token)
	cmd.Flag("ca-pin", "CA pin to validate the Teleport Auth Server; used on first connect.").StringsVar(&args.CAPins)
	cmd.Flag("certificate-ttl", "TTL of short-lived machine certificates.").DurationVar(&args.CertificateTTL)
	cmd.Flag("renewal-interval", "Interval at which short-lived certificates are renewed; must be less than the certificate TTL.").DurationVar(&args.RenewalInterval)
	cmd.Flag("insecure", "Insecure configures the bot to trust the certificates from the Auth Server or Proxy on first connect without verification. Do not use in production.").BoolVar(&args.Insecure)
	cmd.Flag("join-method", "Method to use to join the cluster. "+joinMethodList).EnumVar(&args.JoinMethod, SupportedJoinMethods...)
	cmd.Flag("oneshot", "If set, quit after the first renewal.").BoolVar(&args.Oneshot)
	cmd.Flag("diag-addr", "If set and the bot is in debug mode, a diagnostics service will listen on specified address.").StringVar(&args.DiagAddr)
	cmd.Flag("log-format", "Controls the format of output logs. Can be `json` or `text`. Defaults to `text`.").
		Default(utils.LogFormatText).
		EnumVar(&args.LogFormat, utils.LogFormatJSON, utils.LogFormatText)
	cmd.Flag("storage", "A destination URI for tbot's internal storage.").StringVar(&args.Storage)

	return args
}

func (s *sharedStartArgs) ApplyConfig(cfg *BotConfig, l *slog.Logger) error {
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

		dest, err := destinationFromURI(s.Storage)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Storage = &StorageConfig{Destination: dest}
	}

	// If any onboarding flags are set, override the whole section.
	// (CAPath, CAPins, etc follow different codepaths so we don't want a
	// situation where different fields become set weirdly due to struct
	// merging)
	if s.Token != "" || s.JoinMethod != "" || len(s.CAPins) > 0 {
		if !reflect.DeepEqual(cfg.Onboarding, OnboardingConfig{}) {
			// To be safe, warn about possible confusion.
			l.WarnContext(
				context.TODO(),
				"CLI parameters are overriding join configuration",
				"cli_token", s.Token,
				"cli_join_method", s.JoinMethod,
				"cli_ca_pins_count", len(s.CAPins),
			)
		}

		cfg.Onboarding = OnboardingConfig{
			CAPins:     s.CAPins,
			JoinMethod: types.JoinMethod(s.JoinMethod),
		}
		cfg.Onboarding.SetToken(s.Token)
	}

	return nil
}

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
		strings.Join(SupportedJoinMethods, ", "),
	)

	c := &CommandStartLegacy{
		action: action,
		cmd:    parentCmd.Command("legacy", "Start with either a config file or a legacy output").Default(),
	}
	c.cmd.Flag("auth-server", "Address of the Teleport Auth Server. Prefer using --proxy-server where possible.").Short('a').Envar(authServerEnvVar).StringVar(&c.AuthServer)
	c.cmd.Flag("data-dir", "Directory to store internal bot data. Access to this directory should be limited.").StringVar(&c.DataDir)
	c.cmd.Flag("destination-dir", "Directory to write short-lived machine certificates.").StringVar(&c.DestinationDir)
	c.cmd.Flag("proxy-server", "Address of the Teleport Proxy Server.").Envar(proxyServerEnvVar).StringVar(&c.ProxyServer)
	c.cmd.Flag("token", "A bot join token or path to file with token value, if attempting to onboard a new bot; used on first connect.").Envar(tokenEnvVar).StringVar(&c.Token)
	c.cmd.Flag("ca-pin", "CA pin to validate the Teleport Auth Server; used on first connect.").StringsVar(&c.CAPins)
	c.cmd.Flag("certificate-ttl", "TTL of short-lived machine certificates.").DurationVar(&c.CertificateTTL)
	c.cmd.Flag("renewal-interval", "Interval at which short-lived certificates are renewed; must be less than the certificate TTL.").DurationVar(&c.RenewalInterval)
	c.cmd.Flag("insecure", "Insecure configures the bot to trust the certificates from the Auth Server or Proxy on first connect without verification. Do not use in production.").BoolVar(&c.Insecure)
	c.cmd.Flag("join-method", "Method to use to join the cluster. "+joinMethodList).EnumVar(&c.JoinMethod, SupportedJoinMethods...)
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

func (c *CommandStartLegacy) ApplyConfig(cfg *BotConfig, l *slog.Logger) error {
	// TODO: Weird flags that need to be addressed:
	// - Debug
	// - FIPS
	// - Insecure

	// if c.Debug {
	// 	cfg.Debug = true
	// }

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

		dest, err := destinationFromURI(c.DataDir)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Storage = &StorageConfig{Destination: dest}
	}

	// If any onboarding flags are set, override the whole section.
	// (CAPath, CAPins, etc follow different codepaths so we don't want a
	// situation where different fields become set weirdly due to struct
	// merging)
	if c.Token != "" || c.JoinMethod != "" || len(c.CAPins) > 0 {
		if !reflect.DeepEqual(cfg.Onboarding, OnboardingConfig{}) {
			// To be safe, warn about possible confusion.
			log.WarnContext(
				context.TODO(),
				"CLI parameters are overriding join configuration",
				"cli_token", c.Token,
				"cli_join_method", c.JoinMethod,
				"cli_ca_pins_count", len(c.CAPins),
			)
		}

		cfg.Onboarding = OnboardingConfig{
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

// MutatorAction is an action that is called by a config mutator-style command.
type MutatorAction func(mutator CLIConfigMutator) error

// genericMutatorHandler supplies a generic `TryRun` that works for all commands
// that - broadly - load config, mutate that config, and run an action. It's
// meant to be embedded within a command struct to provide the `TryRun`
// implementation.
type genericMutatorHandler struct {
	cmd     *kingpin.CmdClause
	mutator CLIConfigMutator
	action  MutatorAction
}

// newGenericMutatorHandler creates a new generic genericMutatorHandler that
// provides a generic `TryRun` implementation.
func newGenericMutatorHandler(cmd *kingpin.CmdClause, mutator CLIConfigMutator, action MutatorAction) *genericMutatorHandler {
	return &genericMutatorHandler{
		cmd:     cmd,
		mutator: mutator,
		action:  action,
	}
}

func (g *genericMutatorHandler) TryRun(cmd string) (match bool, err error) {
	switch cmd {
	case g.cmd.FullCommand():
		err = g.action(g.mutator)
	default:
		return false, nil
	}

	return true, trace.Wrap(err)
}

// CommandStartIdentity implements `tbot start identity` and
// `tbot configure identity`.
type CommandStartIdentity struct {
	*sharedStartArgs
	*genericMutatorHandler

	Destination string
	Cluster     string
}

func NewIdentityCommand(parentCmd *kingpin.CmdClause, action MutatorAction) *CommandStartIdentity {
	cmd := parentCmd.Command("identity", "Start with an identity output for SSH and Teleport API access").Alias("ssh").Alias("id")

	c := &CommandStartIdentity{}
	c.sharedStartArgs = newSharedStartArgs(cmd)
	c.genericMutatorHandler = newGenericMutatorHandler(cmd, c, action)

	cmd.Flag("destination", "A destination URI, such as file:///foo/bar").Required().StringVar(&c.Destination)
	cmd.Flag("cluster", "The name of a specific cluster for which to issue an identity if using a leaf cluster").StringVar(&c.Cluster)

	// TODO: roles? ssh_config mode?

	return c
}

func (c *CommandStartIdentity) ApplyConfig(cfg *BotConfig, l *slog.Logger) error {
	if err := c.sharedStartArgs.ApplyConfig(cfg, l); err != nil {
		return trace.Wrap(err)
	}

	dest, err := destinationFromURI(c.Destination)
	if err != nil {
		return trace.Wrap(err)
	}

	cfg.Services = append(cfg.Services, &IdentityOutput{
		Destination: dest,
		Cluster:     c.Cluster,
	})

	return nil
}

// CommandStartDatabase implements `tbot start database` and
// `tbot configure database`.
type CommandStartDatabase struct {
	*sharedStartArgs
	*genericMutatorHandler

	Destination string
	Format      string
	Service     string
	Username    string
	Database    string
}

func NewStartDatabaseCommand(parentCmd *kingpin.CmdClause, action MutatorAction) *CommandStartDatabase {
	cmd := parentCmd.Command("database", "Starts with a database output").Alias("db")

	c := &CommandStartDatabase{}
	c.sharedStartArgs = newSharedStartArgs(cmd)
	c.genericMutatorHandler = newGenericMutatorHandler(cmd, c, action)

	cmd.Flag("destination", "A destination URI, such as file:///foo/bar").Required().StringVar(&c.Destination)
	cmd.Flag("format", "The database output format if necessary").Default("").EnumVar(&c.Format, SupportedDatabaseFormatStrings()...)
	cmd.Flag("service", "The database service name").Required().StringVar(&c.Service)
	cmd.Flag("username", "The database user name").Required().StringVar(&c.Username)
	cmd.Flag("database", "The name of the database available in the requested database service").Required().StringVar(&c.Database)

	return c
}

func (c *CommandStartDatabase) ApplyConfig(cfg *BotConfig, l *slog.Logger) error {
	if err := c.sharedStartArgs.ApplyConfig(cfg, l); err != nil {
		return trace.Wrap(err)
	}

	dest, err := destinationFromURI(c.Destination)
	if err != nil {
		return trace.Wrap(err)
	}

	cfg.Services = append(cfg.Services, &DatabaseOutput{
		Destination: dest,
		Format:      DatabaseFormat(c.Format),
		Username:    c.Username,
		Database:    c.Database,
		Service:     c.Service,
	})

	return nil
}

// CLICommandRunner defines a contract for `TryRun` that allows commands to
// either execute (possibly returning an error), or pass execution to the next
// command candidate.
type CLICommandRunner interface {
	TryRun(cmd string) (match bool, err error)
}

// CLIConfigMutator defines
type CLIConfigMutator interface {
	ApplyConfig(cfg *BotConfig, l *slog.Logger) error
}
