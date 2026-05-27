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

// AzureInstallResultPoller polls the result of an Azure VM installation.
type AzureInstallResultPoller interface {
	// Poll polls for a result. It returns true when polling is complete, either
	// because the result is ready or the poller reached a terminal failure.
	Poll(ctx context.Context) bool
	// Result returns an [AzureInstallResult]. The result may contain an API error
	// if the install command failed to run or the result is not yet ready.
	Result(ctx context.Context) AzureInstallResult
}

// Run initiates Teleport installation on a set of virtual machines without waiting for the commands to complete.
func (req *AzureInstallRequest) Run(ctx context.Context, client azure.RunCommandClient) ([]AzureInstallResultPoller, error) {
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

	var resultPollers []AzureInstallResultPoller
	var mu sync.Mutex
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

			runCommandPoller, err := client.Run(ctx, runRequest)
			mu.Lock()
			resultPollers = append(resultPollers, &azureInstallResultPoller{
				instance: inst,
				poller:   runCommandPoller,
				apiError: err,
			})
			mu.Unlock()

			// local failure should not affect other runs.
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}
	return resultPollers, nil
}

// azureInstallResultPoller polls a run command result.
type azureInstallResultPoller struct {
	instance *azure.VirtualMachine
	poller   azure.RunCommandResultPoller
	apiError error
}

// Poll polls for a result.
func (p *azureInstallResultPoller) Poll(ctx context.Context) bool {
	if p.apiError != nil || p.poller == nil {
		return true
	}

	err := trace.Wrap(p.poller.Poll(ctx))
	if err != nil {
		p.apiError = err
		return true
	}
	return p.poller.Done()
}

// Result returns an [AzureInstallResult]. The result may contain an API error
// if the install command failed to run or the result is not yet ready.
func (p *azureInstallResultPoller) Result(ctx context.Context) AzureInstallResult {
	if p.apiError != nil {
		return AzureInstallResult{
			Instance: p.instance,
			APIError: p.apiError,
		}
	}
	if p.poller == nil {
		return AzureInstallResult{
			Instance: p.instance,
			APIError: trace.BadParameter("missing Azure run command poller"),
		}
	}

	result, err := p.poller.Result(ctx)
	return AzureInstallResult{
		APIError:      err,
		Instance:      p.instance,
		CommandResult: result,
	}
}
