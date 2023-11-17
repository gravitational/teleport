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
)

func onProxyCommand(botConfig *config.BotConfig, cf *config.CLIConf) error {
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
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(timothyb89):  We could consider supporting a --cluster passthrough
	//  here as in `tbot db ...`.
	args := []string{"-i", identityPath, "proxy", "--proxy=" + cf.Proxy}
	args = append(args, cf.RemainingArgs...)

	// Pass through the debug flag, and prepend to satisfy argument ordering
	// needs (`-d` must precede `proxy`).
	if botConfig.Debug {
		args = append([]string{"-d"}, args...)
	}

	return trace.Wrap(wrapper.Exec(env, args...), "executing `tsh proxy`")
}
