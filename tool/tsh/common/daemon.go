/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/lib/teleterm"
	"github.com/gravitational/teleport/lib/utils"
)

// onDaemonStart implements the "tsh daemon start" command.
func onDaemonStart(cf *CLIConf) error {
	homeDir := profile.FullProfilePath(cf.HomePath)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cf.Debug {
		utils.InitLogger(utils.LoggingForDaemon, slog.LevelDebug)
	} else {
		utils.InitLogger(utils.LoggingForDaemon, slog.LevelInfo)
	}

	err := teleterm.Serve(ctx, teleterm.Config{
		HomeDir:            homeDir,
		CertsDir:           cf.DaemonCertsDir,
		Addr:               cf.DaemonAddr,
		InsecureSkipVerify: cf.InsecureSkipVerify,
		PrehogAddr:         cf.DaemonPrehogAddr,
		KubeconfigsDir:     cf.DaemonKubeconfigsDir,
		AgentsDir:          cf.DaemonAgentsDir,
		InstallationID:     cf.DaemonInstallationID,
		AddKeysToAgent:     cf.AddKeysToAgent,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
