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

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
)

type resolver interface {
	Resolve(ctx context.Context) (*azure.RunCommandResult, error)
}

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

type runCommandClient interface {
	// nopush: TODO(gavin): fix godoc
	// Run runs Teleport installation command on a virtual machine.
	RunAsync(ctx context.Context, req azure.RunCommandRequest) (resolver, error)
}

// nopush: TODO(gavin): godoc
// Run initiates Teleport installation on a set of virtual machines and then blocks until the
// commands have completed.
func (req *AzureInstallRequest) RunAsync(ctx context.Context, client azure.RunCommandClient) ([]*AzureInstallResultCollector, error) {
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

	var resultCollectors []*AzureInstallResultCollector
	var resultCollectorsMu sync.Mutex
	for _, inst := range req.Instances {
		inst := inst
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
			}

			pendingCommandResult, err := client.RunAsync(ctx, runRequest)
			resultCollectorsMu.Lock()
			resultCollectors = append(resultCollectors, &AzureInstallResultCollector{
				instance:             inst,
				pendingCommandResult: pendingCommandResult,
				apiError:             err,
			})
			resultCollectorsMu.Unlock()

			// local failure should not affect other runs.
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}
	return resultCollectors, nil
}

// nopush: TODO(gavin): godoc
type AzureInstallResultCollector struct {
	instance             *azure.VirtualMachine
	pendingCommandResult resultResolver
	apiError             error
}

type resultResolver interface {
	Poll(ctx context.Context) error
	Done() bool
	Result(ctx context.Context) (*azure.RunCommandResult, error)
}

// nopush: TODO(gavin): godoc
func (c *AzureInstallResultCollector) Collect(ctx context.Context) (AzureInstallResult, bool) {
	if c.apiError != nil {
		return AzureInstallResult{
			Instance: c.instance,
			APIError: c.apiError,
		}, true
	}

	if err := c.pendingCommandResult.Poll(ctx); err != nil {
		return AzureInstallResult{
			Instance: c.instance,
			APIError: err,
		}, c.pendingCommandResult.Done()
	}

	if !c.pendingCommandResult.Done() {
		return AzureInstallResult{
			Instance: c.instance,
		}, false
	}

	result, err := c.pendingCommandResult.Result(ctx)
	return AzureInstallResult{
		APIError:      err,
		Instance:      c.instance,
		CommandResult: result,
	}, true
}

func (c *AzureInstallResultCollector) Instance() *azure.VirtualMachine {
	return c.instance
}
