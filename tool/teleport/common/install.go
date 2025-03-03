// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/srv/server/installer"
	libutils "github.com/gravitational/teleport/lib/utils"
)

// installAutoDiscoverNodeFlags contains the required flags used to install, configure and start a teleport process.
type installAutoDiscoverNodeFlags struct {
	// ProxyPublicAddr is the proxy public address that the instance will connect to.
	// Eg, proxy.example.com
	ProxyPublicAddr string

	// TeleportPackage contains the teleport package name.
	// Allowed values: teleport, teleport-ent
	TeleportPackage string

	// RepositoryChannel is the repository channel to use.
	// Eg stable/cloud or stable/rolling
	RepositoryChannel string

	// AutoUpgradesString indicates whether the installed binaries should auto upgrade.
	// System must use systemd to enable AutoUpgrades.
	// Bool CLI Flags can only be used as `--flag1` for true or `--no-flag1` for false.
	// However, we can only inject a string value here, so we must use a String Flag and then parse its value.
	AutoUpgradesString string

	// AzureClientID is the client ID of the managed identity to use when joining
	// the cluster. Only applicable for the azure join method.
	AzureClientID string

	// TokenName is the token name to be used by the instance to join the cluster.
	TokenName string
}

// onInstallAutoDiscoverNode is the handler of the "install autodiscover-node" CLI command.
func onInstallAutoDiscoverNode(cfg installAutoDiscoverNodeFlags) error {
	ctx := context.Background()
	// Ensure we print output to the user. LogLevel at this point was set to Error.
	libutils.InitLogger(libutils.LoggingForDaemon, slog.LevelInfo)

	autoUpgrades, err := utils.ParseBool(cfg.AutoUpgradesString)
	if err != nil {
		return trace.BadParameter("--auto-upgrade must be either true or false")
	}

	teleportInstaller, err := installer.NewAutoDiscoverNodeInstaller(&installer.AutoDiscoverNodeInstallerConfig{
		ProxyPublicAddr:   cfg.ProxyPublicAddr,
		TeleportPackage:   cfg.TeleportPackage,
		RepositoryChannel: cfg.RepositoryChannel,
		AutoUpgrades:      autoUpgrades,
		AzureClientID:     cfg.AzureClientID,
		TokenName:         cfg.TokenName,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(teleportInstaller.Install(ctx))
}
