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

package server

import (
	"context"
	"log/slog"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/cloud/azure"
)

// AzureInstaller handles running commands that install Teleport on Azure
// virtual machines.
type AzureInstaller struct {
	Emitter apievents.Emitter
	Logger  *slog.Logger
}

// AzureRunRequest combines parameters for running commands on a set of Azure
// virtual machines.
type AzureRunRequest struct {
	Client          azure.RunCommandClient
	Instances       []*armcompute.VirtualMachine
	InstallerParams *types.InstallerParams
	ProxyAddrGetter func(context.Context) (string, error)
	Region          string
	ResourceGroup   string
}

// Run runs a command on a set of virtual machines and then blocks until the
// commands have completed.
func (ai *AzureInstaller) Run(ctx context.Context, req AzureRunRequest) error {
	ai.Logger.DebugContext(ctx, "running installer for request", "instance_count", len(req.Instances), "region", req.Region, "resource_group", req.ResourceGroup)
	// Azure treats scripts with the same content as the same invocation and
	// won't run them more than once. This is fine when the installer script
	// succeeds, but it makes troubleshooting much harder when it fails. To
	// work around this, we generate a random string and append it as a comment
	// to the script, forcing Azure to see each invocation as unique.
	script, err := installerScript(ctx, req.InstallerParams, withNonceComment(), withProxyAddrGetter(req.ProxyAddrGetter))
	if err != nil {
		return trace.Wrap(err)
	}
	g, ctx := errgroup.WithContext(ctx)
	// Somewhat arbitrary limit to make sure Teleport doesn't have to install
	// hundreds of nodes at once.
	g.SetLimit(10)

	//ai.Logger.DebugContext(ctx, "installation script", "script", script, "params", req.Params)

	for _, inst := range req.Instances {
		ai.Logger.DebugContext(ctx, "running installer for instance", "instance", inst.Name)
		g.Go(func() error {
			runRequest := azure.RunCommandRequest{
				Region:        req.Region,
				ResourceGroup: req.ResourceGroup,
				VMName:        azure.StringVal(inst.Name),
				Script:        script,
			}
			// TODO: refactor to remove req.Client?
			// TODO: any error returned will break installation?
			errRun := req.Client.Run(ctx, runRequest)
			if errRun != nil {
				ai.Logger.WarnContext(ctx, "🛑 error running installer for instance", "instance", inst.Name, "err", errRun.Error())
			} else {
				ai.Logger.DebugContext(ctx, "✅ installed", "instance", inst.Name)
			}
			return nil
		})
	}
	return trace.Wrap(g.Wait())
}
