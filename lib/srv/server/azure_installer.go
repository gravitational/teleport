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

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
)

// AzureInstallRequest combines parameters for running commands on a set of Azure
// virtual machines.
type AzureInstallRequest struct {
	Instances            []*azure.VirtualMachine
	InstallerParams      *types.InstallerParams
	ProxyAddrGetter      func(context.Context) (string, error)
	Region               string
	ResourceGroup        string
	OnRunCommandFinished func(result AzureInstallResult)
}

// AzureInstallResult stores installation results for particular VM instance.
type AzureInstallResult struct {
	// Instance is the Azure Virtual Machine the installation was attempted on.
	Instance *azure.VirtualMachine
	// APIError is potential API error encountered.
	APIError error
	// CommandResult is the result of run command: execution status, exit code, stdout, stderr.
	CommandResult *azure.RunCommandResult
}

// Failure returns true if the installation result is considered a failure.
func (r AzureInstallResult) Failure() bool {
	return r.APIError != nil || (r.CommandResult != nil && r.CommandResult.Failure())
}

// Run initiates Teleport installation on a set of virtual machines and then blocks until the
// commands have completed.
func (req *AzureInstallRequest) Run(ctx context.Context, client azure.RunCommandClient) error {
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

	// Bound how many installs run (and wait for results) at once.
	const azureParallelInstallLimit = 1000
	g.SetLimit(azureParallelInstallLimit)

	// Bound how many commands are dispatched (BeginCreateOrUpdate) concurrently,
	// independently of how many results we wait on. Dispatch is fast, so this
	// just avoids hammering the Azure API when many installs start together.
	const azureDispatchLimit = 50
	dispatchLimit := semaphore.NewWeighted(azureDispatchLimit)
	acquireDispatch := func(ctx context.Context) (func(), error) {
		if err := dispatchLimit.Acquire(ctx, 1); err != nil {
			return nil, trace.Wrap(err)
		}
		return func() { dispatchLimit.Release(1) }, nil
	}

	for _, inst := range req.Instances {
		g.Go(func() error {
			// If the caller cancels, stop trying to run more commands.
			if err := ctx.Err(); err != nil {
				return err
			}

			runRequest := azure.RunCommandRequest{
				Region:                      req.Region,
				ResourceGroup:               req.ResourceGroup,
				VMName:                      inst.Name,
				Script:                      script,
				UniformScaleSetName:         inst.UniformScaleSetName,
				UniformScaleSetVMInstanceID: inst.UniformScaleSetVMInstanceID,
				AcquireDispatch:             acquireDispatch,
			}

			commandResult, apiError := client.Run(ctx, runRequest)
			if req.OnRunCommandFinished != nil {
				req.OnRunCommandFinished(AzureInstallResult{
					Instance:      inst,
					APIError:      apiError,
					CommandResult: commandResult,
				})
			}

			// local failure should not affect other runs.
			return nil
		})
	}

	return trace.Wrap(g.Wait())
}
