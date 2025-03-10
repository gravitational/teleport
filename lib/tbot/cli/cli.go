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
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/tbot/config"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	// AuthServerEnvVar is the environment variable that overrides the
	// configured auth server address.
	AuthServerEnvVar = "TELEPORT_AUTH_SERVER"
	// TokenEnvVar is the environment variable that overrides the configured
	// bot token name.
	TokenEnvVar = "TELEPORT_BOT_TOKEN"
	// ProxyServerEnvVar is the environment variable that overrides the
	// configured proxy server address.
	ProxyServerEnvVar = "TELEPORT_PROXY"
	// TBotDebugEnvVar is the environment variable that enables debug logging.
	TBotDebugEnvVar = "TBOT_DEBUG"
	// TBotConfigPathEnvVar is the environment variable that overrides the
	// configured config file path.
	TBotConfigPathEnvVar = "TBOT_CONFIG_PATH"
	// TBotConfigEnvVar is the environment variable that provides tbot
	// configuration with base64 encoded string.
	TBotConfigEnvVar = "TBOT_CONFIG"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentTBot)

// CommandRunner defines a contract for `TryRun` that allows commands to
// either execute (possibly returning an error), or pass execution to the next
// command candidate.
type CommandRunner interface {
	TryRun(cmd string) (match bool, err error)
}

// MutatorAction is an action that is called by a config mutator-style command.
type MutatorAction func(mutator ConfigMutator) error

// genericMutatorHandler supplies a generic `TryRun` that works for all commands
// that - broadly - load config, mutate that config, and run an action. It's
// meant to be embedded within a command struct to provide the `TryRun`
// implementation.
type genericMutatorHandler struct {
	cmd     *kingpin.CmdClause
	mutator ConfigMutator
	action  MutatorAction
}

// newGenericMutatorHandler creates a new generic genericMutatorHandler that
// provides a generic `TryRun` implementation.
func newGenericMutatorHandler(cmd *kingpin.CmdClause, mutator ConfigMutator, action MutatorAction) *genericMutatorHandler {
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

// ConfigMutator is an interface that can apply changes to a BotConfig.
type ConfigMutator interface {
	ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error
}

// genericExecutorHandler is a helper that can be embedded to provide a simpler
// TryRun implementation that just runs a function. These functions can be
// passed in while building the CLI to more easily glue behaviors together, or
// specified directly.
type genericExecutorHandler[T any] struct {
	cmd  *kingpin.CmdClause
	args *T

	// actions is a list of functions to run when `TryRun` matches the `cmd`.
	// Generally at most one action should be exposed to the top level glue in
	// main, but commands might want to inject some handler logic for e.g.
	// flag migrations.
	actions []func(*T) error
}

// newGenericExecutorHandler creates a genericExecutorHandler with the given
// command and action to execute when that command is matched.
func newGenericExecutorHandler[T any](cmd *kingpin.CmdClause, args *T, actions ...func(*T) error) *genericExecutorHandler[T] {
	return &genericExecutorHandler[T]{
		cmd:     cmd,
		args:    args,
		actions: actions,
	}
}

func (e *genericExecutorHandler[T]) TryRun(cmd string) (match bool, err error) {
	switch cmd {
	case e.cmd.FullCommand():
		for _, action := range e.actions {
			err = action(e.args)
			if err != nil {
				break
			}
		}
	default:
		return false, nil
	}

	return true, trace.Wrap(err)
}

func applyMutators(l *slog.Logger, cfg *config.BotConfig, mutators ...ConfigMutator) error {
	for _, mutator := range mutators {
		if mutator == nil {
			continue
		}

		if err := mutator.ApplyConfig(cfg, l); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// LoadConfigWithMutators builds a config from an optional config file and a CLI
// mutator. If an empty path is provided, an empty base config is used. The CLI
// mutator may override or append to the loaded configuration, if any. The
// GlobalArgs will be applied as a mutator, and `CheckAndSetDefaults()` will be
// called on the end result.
func LoadConfigWithMutators(globals *GlobalArgs, mutators ...ConfigMutator) (*config.BotConfig, error) {
	var cfg *config.BotConfig
	var err error

	if globals.ConfigString != "" && globals.ConfigPath != "" {
		return nil, trace.BadParameter("cannot specify both config and config-string")
	} else if globals.staticConfigYAML != "" {
		cfg, err = config.ReadConfig(strings.NewReader(globals.staticConfigYAML), false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else if globals.ConfigString != "" {
		cfg, err = config.ReadConfigFromBase64String(globals.ConfigString, false)
		if err != nil {
			return nil, trace.Wrap(err, "loading bot config from base64 encoded string")
		}
	} else if globals.ConfigPath != "" {
		cfg, err = config.ReadConfigFromFile(globals.ConfigPath, false)

		if err != nil {
			return nil, trace.Wrap(err, "loading bot config from path %s", globals.ConfigPath)
		}
	} else {
		cfg = &config.BotConfig{}
	}

	mutatorsWithGlobals := append([]ConfigMutator{globals}, mutators...)

	l := log.With("config_path", globals.ConfigPath)
	if err := applyMutators(l, cfg, mutatorsWithGlobals...); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return cfg, nil
}

// BaseConfigWithMutators returns a base bot config with the given CLI mutators
// applied. `CheckAndSetDefaults()` will be called on the result. This is useful
// for explicitly _not_ loading a config file, like in `tbot configure ...`
func BaseConfigWithMutators(mutators ...ConfigMutator) (*config.BotConfig, error) {
	cfg := &config.BotConfig{}
	if err := applyMutators(log, cfg, mutators...); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return cfg, nil
}

// RemainingArgsList is a custom kingpin parser that consumes all remaining
// arguments.
type RemainingArgsList []string

func (r *RemainingArgsList) Set(value string) error {
	*r = append(*r, value)
	return nil
}

func (r *RemainingArgsList) String() string {
	return strings.Join([]string(*r), " ")
}

func (r *RemainingArgsList) IsCumulative() bool {
	return true
}

// RemainingArgs returns a list of remaining arguments for the given command.
func RemainingArgs(s kingpin.Settings) (target *[]string) {
	target = new([]string)
	s.SetValue((*RemainingArgsList)(target))
	return
}

// KingpinClause allows commands and flags to mount to either the root app
// (kingpin.Application) or a subcommand (kingpin.CmdClause)
type KingpinClause interface {
	Command(name string, help string) *kingpin.CmdClause
	Flag(name string, help string) *kingpin.FlagClause
}
