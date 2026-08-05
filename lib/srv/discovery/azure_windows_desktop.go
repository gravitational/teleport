// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package discovery

import (
	"context"
	"fmt"
	"maps"
	"net"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/srv/server"
	"github.com/gravitational/trace"
	"golang.org/x/sync/semaphore"
)

// errNoPrimaryPrivateIP indicates a Windows VM has no usable private IP
// address, so it cannot be registered as a dynamic desktop.
var errNoPrimaryPrivateIP = trace.BadParameter("no primary private IP address resolved for VM")

func (s *Server) handleAzureWindowsDesktops(group *server.AzureInstances, vmTasks *azureVMTasks, backoff *installerBackoff, sem *semaphore.Weighted) (discoveryGroupStatus, error) {
	var status discoveryGroupStatus
	log := s.Log.With("group", group)
	found := len(group.Instances)
	if found == 0 {
		log.DebugContext(s.ctx, "No Azure instances found")
		return status, nil
	}
	status.found += found

	// Filter out existing desktops, but keep refreshing their resources. A
	// dynamic desktop has no agent of its own to heartbeat it, so if we don't
	// touch it again here it will eventually expire even though the VM is
	// still around.
	desktops, err := s.dynamicWindowsDesktopWatcher.CurrentResources(s.ctx)
	if err != nil {
		return status, trace.Wrap(err, "failed to get existing desktops")
	}
	// FilterExistingDesktops filters [desktops] in place and returns the removed
	// desktops, which will be refreshed to bump their expiry.
	alreadyEnrolled := group.FilterExistingDesktops(desktops)
	if len(alreadyEnrolled) > 0 {
		log.DebugContext(s.ctx, "Refreshing Azure instances that have already been enrolled",
			"enrolled", len(alreadyEnrolled),
		)
		refreshTime := s.clock.Now()
		for _, vm := range alreadyEnrolled {
			if err := s.createAzureWindowsDesktop(vm, refreshTime); err != nil {
				log.WarnContext(s.ctx, "Failed to refresh dynamic Windows desktop", "vm_id", vm.VMID, "error", err)
				status.failed++
				continue
			}
			status.enrolled++
		}
	}

	// Exit if there are no new desktops to enroll.
	needInstall := len(group.Instances)
	if needInstall == 0 {
		log.DebugContext(s.ctx, "No new Azure instances to enroll")
		return status, nil
	}

	// Golden-image VMs already have the Windows auth package installed, so
	// skip the RunCommand step entirely and just register the desktop
	// resource.
	if group.Metadata.InstallerParams != nil && group.Metadata.InstallerParams.SkipInstallation {
		log.DebugContext(s.ctx, "Skipping installation for Azure VMs per skip_installation setting",
			"vms", genAzureInstancesLogStr(group.Instances),
		)
		syncTime := s.clock.Now()
		for _, vm := range group.Instances {
			if err := s.createAzureWindowsDesktop(vm, syncTime); err != nil {
				log.WarnContext(s.ctx, "Failed to register dynamic Windows desktop for Azure VM",
					"vm", vm.Name,
					"error", err,
				)
				status.failed++
				continue
			}
			status.enrolled++
		}
		return status, nil
	}

	// Skip desktops that have an active installation backoff.
	skipped := backoff.filter(group, s.clock.Now())
	if len(skipped) > 0 {
		log.DebugContext(s.ctx, "Skipping Azure VMs with an active installation backoff",
			"skipped", len(skipped),
		)
		for _, entry := range skipped {
			if entry.isFailedAttempt() {
				status.failed++
				continue
			}

			// If this was a successful enrollment, but the desktop wasn't filtered
			// out by FilterExistingDesktops, the desktop insert must have failed.
			// Retry the insert so a successfully enrolled desktop is left out.
			if err := s.createAzureWindowsDesktop(entry.vm, s.clock.Now()); err != nil {
				log.WarnContext(s.ctx, "Failed to re-register dynamic Windows desktop for previously enrolled Azure VM",
					"vm_id", entry.vm.VMID,
					"error", err,
				)
				status.failed++
				continue
			}
			status.enrolled++
		}
	}

	// Install the Teleport Windows auth package on the remaining Azure VMs.
	var results []server.AzureInstallResult
	var installErr error
	if len(group.Instances) > 0 {
		log.DebugContext(s.ctx, "Installing Teleport Windows auth package on Azure VMs",
			"vms", genAzureInstancesLogStr(group.Instances),
		)
		results, installErr = s.installWindowsDesktops(group, vmTasks, sem)
	}
	syncTime := s.clock.Now()
	// Record skipped desktops now that we have a post-install sync time
	for _, entry := range skipped {
		addFailedEnrollment(group, entry)
	}
	if installErr != nil {
		log.WarnContext(
			s.ctx, "Failed to install Teleport Windows auth package on Azure VMs",
			"error", installErr,
		)
		status.failed += len(group.Instances)

		issueType := classifyAzureVMEnrollmentError(installErr)
		for _, vm := range group.Instances {
			entry := backoff.recordFailedAttempt(vm, issueType, syncTime)
			addFailedEnrollment(group, entry)
		}
		return status, installErr
	}

	successful, failures := splitInstallResults(results)
	log.InfoContext(s.ctx, "Finished installation batch",
		"total_instances", len(results),
		"failures", len(failures),
	)

	// count individual failed enrollments.
	status.failed += len(failures)
	if len(failures) > 0 {
		log.WarnContext(
			s.ctx,
			"Failed to install Teleport Windows auth package on some discovered Azure VMs",
			"failures", len(failures),
		)
	}
	// Record failures as user tasks.
	for _, result := range failures {
		issueType := classifyAzureWindowsAuthPackageInstallResultIssue(result)
		entry := backoff.recordFailedAttempt(result.Instance, issueType, syncTime)
		addFailedEnrollment(group, entry)
	}

	// Register desktops which successfully installed the Teleport Auth Package.
	for _, result := range successful {
		err := s.registerDynamicWindowsDesktops(result, syncTime)
		if err != nil {
			log.WarnContext(s.ctx, "Failed to register dynamic Windows desktop for Azure VM",
				"vm", result.Instance.Name,
				"error", err,
			)
			status.failed++
			issueType := classifyAzureWindowsDesktopRegistrationError(err)
			entry := backoff.recordFailedAttempt(result.Instance, issueType, syncTime)
			addFailedEnrollment(group, entry)
			continue
		}
		status.enrolled++
		backoff.recordSuccessfulAttempt(result.Instance, syncTime)
	}

	return status, nil
}

// installWindowsDesktops installs the Teleport Windows auth package on a batch
// of Azure VMs.
// TODO(danielashare): Switch vmTasks to use new Windows specific user tasks
func (s *Server) installWindowsDesktops(group *server.AzureInstances, vmTasks *azureVMTasks, sem *semaphore.Weighted) ([]server.AzureInstallResult, error) {
	azureClients, err := s.getAzureClients(s.ctx, group.Metadata.Integration)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get Azure clients", "integration", group.Metadata.Integration)
	}

	runClient, err := azureClients.GetRunCommandClient(s.ctx, group.Metadata.SubscriptionID)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get Azure Run Command client", "subscription", group.Metadata.SubscriptionID)
	}

	const maxResportedErrors = 10
	reporter := &limitedErrorReporter{
		reportLimit: maxResportedErrors,
		logger:      s.Log,
	}

	var mu sync.Mutex
	var results []server.AzureInstallResult

	req := &server.AzureInstallRequest{
		Instances:       group.Instances,
		Region:          group.Metadata.Region,
		ResourceGroup:   group.Metadata.ResourceGroup,
		InstallerParams: group.Metadata.InstallerParams,
		ProxyAddrGetter: s.publicProxyAddress,
		AcquireLease: func(ctx context.Context) (release func(), err error) {
			if err := sem.Acquire(ctx, 1); err != nil {
				return nil, trace.Wrap(err)
			}
			return func() { sem.Release(1) }, nil
		},
		OnRunCommandFinished: func(result server.AzureInstallResult) {
			s.emitAzureInstallEvents(s.Log, group.Metadata, result)
			mu.Lock()
			results = append(results, result)
			mu.Unlock()
			if result.Failure() {
				reporter.report(s.ctx, result)
			}
		},
	}

	err = req.RunWindowsAuthPackage(s.ctx, runClient)
	if err != nil {
		return results, trace.Wrap(err, "failed to run Azure Windows auth package installer")
	}

	reporter.summary(s.ctx)
	return results, nil
}

// registerDynamicWindowsDesktops registers a dynamic Windows desktop resource
// for a successfully enrolled Azure VM.
func (s *Server) registerDynamicWindowsDesktops(result server.AzureInstallResult, syncTime time.Time) error {
	if result.Failure() {
		return trace.BadParameter("cannot register dynamic Windows desktop for failed installation result: %v", result)
	}
	return s.createAzureWindowsDesktop(result.Instance, syncTime)
}

// createAzureWindowsDesktop creates or refreshes the dynamic Windows desktop
// resource for an Azure VM. It is called both for newly enrolled VMs and for
// VMs that were already enrolled in a previous cycle. A dynamic desktop has
// no agent of its own to heartbeat it, so it must be refreshed on every
// discovery cycle or it will eventually expire even though the VM is still
// around.
func (s *Server) createAzureWindowsDesktop(vm *azure.VirtualMachine, syncTime time.Time) error {
	if vm.PrimaryPrivateIP == "" {
		return trace.Wrap(errNoPrimaryPrivateIP, "cannot register dynamic Windows desktop for Azure VM %q with no primary private IP", vm.Name)
	}

	name := fmt.Sprintf("azure-windows-%s", vm.VMID)
	labels := maps.Clone(vm.Tags)
	if labels == nil {
		labels = map[string]string{}
	}
	// Internal labels.
	labels[types.TeleportInternalDiscoveryGroupName] = s.DiscoveryGroup
	labels[types.SubscriptionIDLabelInternal] = vm.Subscription
	labels[types.ResourceGroupLabelInternal] = vm.ResourceGroup
	labels[types.RegionLabelInternal] = vm.Location
	labels[types.VMIDLabelInternal] = vm.VMID
	// Public labels.
	labels[types.SubscriptionIDLabel] = vm.Subscription
	labels[types.ResourceGroupLabel] = vm.ResourceGroup
	labels[types.OriginLabel] = types.OriginCloud
	labels[types.CloudLabel] = types.CloudAzure
	labels[types.RegionLabel] = vm.Location
	labels[types.VMIDLabel] = vm.VMID

	desktop, err := types.NewDynamicWindowsDesktopV1(name, labels, types.DynamicWindowsDesktopSpecV1{
		Addr:  net.JoinHostPort(vm.PrimaryPrivateIP, "3389"),
		NonAD: true,
	})
	if err != nil {
		return trace.Wrap(err, "failed to create dynamic Windows desktop resource for Azure VM %q", vm.Name)
	}
	// Set a generous expiry time in case we're unable to reach the Azure VM. It
	// is better to have a lingering desktop resource than to have it disappear.
	desktop.SetExpiry(syncTime.Add(s.PollInterval * 5))

	// Perform a non-cached read on the desktop resource before writing it to the
	// backend, rather than calling create immediately to prevent warn logs when
	// the resource already exists.
	old, err := s.AccessPoint.GetDynamicWindowsDesktop(s.ctx, name)
	switch {
	case trace.IsNotFound(err):
		if _, err := s.AccessPoint.CreateDynamicWindowsDesktop(s.ctx, desktop); err != nil {
			return trace.Wrap(err, "failed to register dynamic Windows desktop for Azure VM %q", vm.Name)
		}
	case err != nil:
		return trace.Wrap(err, "failed to look up dynamic Windows desktop for Azure VM %q", vm.Name)
	default:
		if err := checkDesktopIsDiscoveryManaged(old, s.DiscoveryGroup); err != nil {
			return trace.Wrap(err, "not refreshing dynamic Windows desktop for Azure VM %q", vm.Name)
		}
		// UpdateDynamicWindowsDesktop uses a conditional update, which requires
		// the revision of the resource being written to match the revision
		// currently stored in the backend.
		desktop.SetRevision(old.GetRevision())
		if _, err := s.AccessPoint.UpdateDynamicWindowsDesktop(s.ctx, desktop); err != nil {
			return trace.Wrap(err, "failed to refresh dynamic Windows desktop for Azure VM %q", vm.Name)
		}
	}
	return nil
}

// checkDesktopIsDiscoveryManaged reports whether an existing dynamic Windows
// desktop is safe for auto-discovery to overwrite.
func checkDesktopIsDiscoveryManaged(old types.DynamicWindowsDesktop, discoveryGroup string) error {
	origin, err := types.GetOrigin(old)
	if err != nil {
		return trace.Wrap(err)
	}

	// Check that the origin is cloud.
	if origin != types.OriginCloud {
		return trace.CompareFailed("the resource origin indicates that it is not managed by auto-discovery")
	}

	// Check that the discovery group matches, but also tolerate the case where
	// the discovery group label is missing. This can happen when a desktop was
	// discovered by an agent that didn't have a discovery group set. We still
	// want to allow the desktop to be refreshed.
	oldDiscoveryGroup, _ := old.GetLabel(types.TeleportInternalDiscoveryGroupName)
	if oldDiscoveryGroup != "" && oldDiscoveryGroup != discoveryGroup {
		return trace.CompareFailed("the resource is in a different discovery group")
	}
	return nil
}

// addFailedEnrollment creates a user task for a failed enrollment attempt.
func addFailedEnrollment(instances *server.AzureInstances, entry installerBackoffEntry) {
	// Static matchers don't have a discovery config resource, so skip creating
	// user tasks because validation requires a discovery config name.
	if instances.Metadata.DiscoveryConfigName == NoDiscoveryConfig {
		return
	}
	if !entry.isFailedAttempt() {
		// Don't surface user tasks for successful enrollments
		return
	}

	// TODO(danielashare): Create a user task here once the Windows VM user task
	// is implemented.
}
