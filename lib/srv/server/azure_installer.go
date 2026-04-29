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
	"fmt"
	"log/slog"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
)

// AzureInstallRequest combines parameters for running commands on a set of Azure
// virtual machines.
type AzureInstallRequest struct {
	Instances            []*armcompute.VirtualMachine
	InstallerParams      *types.InstallerParams
	ProxyAddrGetter      func(context.Context) (string, error)
	Region               string
	ResourceGroup        string
	OnRunCommandFinished func(result AzureInstallResult)
	// Logger is used for ancillary diagnostics (e.g. power-state lookup errors
	// that fail open). When nil, slog.Default() is used.
	Logger *slog.Logger
}

// AzureInstallResult stores installation results for particular VM instance.
type AzureInstallResult struct {
	// Instance is VM instance.
	Instance *armcompute.VirtualMachine
	// APIError is potential API error encountered.
	APIError error
	// CommandResult is the result of run command: execution status, exit code, stdout, stderr.
	CommandResult *azure.RunCommandResult
	// SkipReason is set when the install was intentionally not attempted
	// (e.g. VM not running). When non-empty, APIError and CommandResult are
	// nil and the result must not be counted as a failure or audited.
	SkipReason string
}

// Skipped reports whether the install was intentionally not attempted.
func (r AzureInstallResult) Skipped() bool {
	return r.SkipReason != ""
}

// Failure returns true if the installation result is considered a failure.
// Skipped results are not failures.
func (r AzureInstallResult) Failure() bool {
	if r.Skipped() {
		return false
	}
	return r.APIError != nil || (r.CommandResult != nil && r.CommandResult.Failure())
}

// Run initiates Teleport installation on a set of virtual machines and then blocks until the
// commands have completed. Immediately before each Run Command, it issues a per-VM Get
// with $expand=instanceView to verify the VM is running. Non-running VMs are reported as
// skipped (not failed) via OnRunCommandFinished. A power-state lookup error fails open:
// the install is attempted anyway and any genuine ARM error surfaces through Run Command.
func (req *AzureInstallRequest) Run(
	ctx context.Context,
	runClient azure.RunCommandClient,
	vmClient azure.VirtualMachinesClient,
) error {
	logger := req.Logger
	if logger == nil {
		logger = slog.Default()
	}

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
	// TODO (Tener): increase limit/make it configurable.
	const azureParallelInstallLimit = 10
	g.SetLimit(azureParallelInstallLimit)

	for _, inst := range req.Instances {
		g.Go(func() error {
			// If the caller cancels, stop trying to run more commands.
			if err := ctx.Err(); err != nil {
				return err
			}

			vmName := azure.StringVal(inst.Name)

			// Pre-flight power-state check. Skipping this and letting Run
			// Command fail on a stopped VM would generate per-cycle 409s,
			// spurious audit events, and user tasks. Fail open on lookup
			// error so a flaky ARM read never drops a legitimately running VM.
			if vmClient != nil {
				state, stateErr := vmClient.GetPowerState(ctx, req.ResourceGroup, vmName)
				switch {
				case stateErr != nil:
					logger.DebugContext(ctx, "Power-state lookup failed; proceeding with install attempt",
						"vm_name", vmName,
						"resource_id", azure.StringVal(inst.ID),
						"error", stateErr,
					)
				case state != azure.PowerStateRunning && state != azure.PowerStateUnknown:
					if req.OnRunCommandFinished != nil {
						req.OnRunCommandFinished(AzureInstallResult{
							Instance:   inst,
							SkipReason: fmt.Sprintf("VM not running (power state: %s)", state),
						})
					}
					return nil
				}
			}

			runRequest := azure.RunCommandRequest{
				Region:        req.Region,
				ResourceGroup: req.ResourceGroup,
				VMName:        vmName,
				Script:        script,
			}

			commandResult, apiError := runClient.Run(ctx, runRequest)
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
