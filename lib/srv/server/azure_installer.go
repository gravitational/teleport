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
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
)

// AzureInstallRequest combines parameters for running commands on a set of Azure
// virtual machines.
type AzureInstallRequest struct {
	Instances       []*armcompute.VirtualMachine
	InstallerParams *types.InstallerParams
	ProxyAddrGetter func(context.Context) (string, error)
	Region          string
	ResourceGroup   string
}

// AzureInstallFailure records installation error associated with particular VM instance.
type AzureInstallFailure struct {
	// Instance is the VM instance for which the installation failed.
	Instance *armcompute.VirtualMachine
	// Error is the encountered error.
	Error error
}

// Run initiates Teleport installation on a set of virtual machines and then blocks until the
// commands have completed.
func (req *AzureInstallRequest) Run(ctx context.Context, client azure.RunCommandClient) ([]AzureInstallFailure, error) {
	// Azure treats scripts with the same content as the same invocation and
	// won't run them more than once. This is fine when the installer script
	// succeeds, but it makes troubleshooting much harder when it fails. To
	// work around this, we generate a random string and append it as a comment
	// to the script, forcing Azure to see each invocation as unique.
	script, err := installerScript(ctx, req.InstallerParams, withNonceComment(), withProxyAddrGetter(req.ProxyAddrGetter))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	g, ctx := errgroup.WithContext(ctx)

	// Somewhat arbitrary limit to make sure Teleport doesn't have to install
	// hundreds of nodes at once.
	// TODO (Tener): increase limit/make it configurable.
	const azureParallelInstallLimit = 10
	g.SetLimit(azureParallelInstallLimit)

	var failures []AzureInstallFailure
	var mu sync.Mutex

	for _, inst := range req.Instances {
		g.Go(func() error {
			// If the caller cancels, stop trying to run more commands.
			if err := ctx.Err(); err != nil {
				return err
			}

			runRequest := azure.RunCommandRequest{
				Region:        req.Region,
				ResourceGroup: req.ResourceGroup,
				VMName:        azure.StringVal(inst.Name),
				Script:        script,
			}

			runError := client.Run(ctx, runRequest)
			if runError != nil {
				failure := AzureInstallFailure{
					Instance: inst,
					Error:    runError,
				}
				mu.Lock()
				failures = append(failures, failure)
				mu.Unlock()
			}

			// return nil: local failure should not affect other runs.
			return nil
		})
	}

	groupErr := g.Wait()
	if groupErr != nil {
		return nil, trace.Wrap(groupErr)
	}

	return failures, nil
}
