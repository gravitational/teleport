/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"path/filepath"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/tshwrap"
	"github.com/gravitational/teleport/lib/utils"
)

func onDBCommand(botConfig *config.BotConfig, cf *config.CLIConf) error {
	wrapper, err := tshwrap.New()
	if err != nil {
		return trace.Wrap(err)
	}

	if err := tshwrap.CheckTSHSupported(wrapper); err != nil {
		return trace.Wrap(err)
	}

	destination, err := tshwrap.GetDestinationDirectory(botConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	env, err := tshwrap.GetEnvForTSH(destination.Path)
	if err != nil {
		return trace.Wrap(err)
	}

	identityPath := filepath.Join(destination.Path, config.IdentityFilePath)
	identity, err := tshwrap.LoadIdentity(identityPath)
	if err != nil {
		return trace.Wrap(err)
	}

	args := []string{"-i", identityPath, "db", "--proxy=" + cf.Proxy}
	if cf.Cluster != "" {
		// If we caught --cluster in our args, pass it through.
		args = append(args, "--cluster="+cf.Cluster)
	} else if !utils.HasPrefixAny("--cluster", cf.RemainingArgs) {
		// If no `--cluster` was provided after a `--`, pass along the cluster
		// name in the identity.
		args = append(args, "--cluster="+identity.RouteToCluster)
	}
	args = append(args, cf.RemainingArgs...)

	// Pass through the debug flag, and prepend to satisfy argument ordering
	// needs (`-d` must precede `db`).
	if botConfig.Debug {
		args = append([]string{"-d"}, args...)
	}

	return trace.Wrap(wrapper.Exec(env, args...), "executing `tsh db`")
}
