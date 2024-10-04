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
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/tbot/config"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/trace"
)

const (
	authServerEnvVar  = "TELEPORT_AUTH_SERVER"
	tokenEnvVar       = "TELEPORT_BOT_TOKEN"
	proxyServerEnvVar = "TELEPORT_PROXY"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentTBot)

type globalArgs struct {
	FIPS bool

	// These properties are not applied to the config.

	ConfigPath string
	Debug      bool
}

func (g *globalArgs) ApplyConfig(cfg *config.BotConfig) error {
	if g.FIPS {
		cfg.FIPS = g.FIPS
	}

	if g.Debug {
		cfg.Debug = g.Debug
	}

	return nil
}

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

// CommandRunner defines a contract for `TryRun` that allows commands to
// either execute (possibly returning an error), or pass execution to the next
// command candidate.
type CommandRunner interface {
	TryRun(cmd string) (match bool, err error)
}

// ConfigMutator is an interface that can apply changes to a BotConfig.
type ConfigMutator interface {
	ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error
}

// LoadConfigWithMutator builds a config from an optional config file and a CLI
// mutator. If an empty path is provided, an empty base config is used. The CLI
// mutator may override or append to the loaded configuration, if any.
// `CheckAndSetDefaults()` will be called on the end result.
func LoadConfigWithMutator(filePath string, mutator ConfigMutator) (*config.BotConfig, error) {
	var cfg *config.BotConfig
	var err error

	if filePath != "" {
		cfg, err = config.ReadConfigFromFile(filePath, false)

		if err != nil {
			return nil, trace.Wrap(err, "loading bot config from path %s", filePath)
		}
	} else {
		cfg = &config.BotConfig{}
	}

	l := log.With("config_path", filePath)
	if err := mutator.ApplyConfig(cfg, l); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return cfg, nil
}

// BaseConfigWithMutator returns a base bot config with a given CLI mutator
// applied. `CheckAndSetDefaults()` will be called on the result. This is useful
// for explicitly _not_ loading a config file, like in `tbot configure ...`
func BaseConfigWithMutator(mutator ConfigMutator) (*config.BotConfig, error) {
	cfg := &config.BotConfig{}
	if err := mutator.ApplyConfig(cfg, log); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return cfg, nil
}
