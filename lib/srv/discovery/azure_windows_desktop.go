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
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/semaphore"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/srv/server"
)

// errNoPrimaryPrivateIP indicates a Windows VM has no usable private IP
// address, so it cannot be registered as a dynamic desktop.
var errNoPrimaryPrivateIP = &trace.BadParameterError{Message: "no primary private IP address resolved for VM"}

// errNoVMID indicates an Azure VM has no VM ID, so it cannot be registered as a
// dynamic desktop.
var errNoVMID = &trace.BadParameterError{Message: "no VM ID resolved for Azure VM"}

// windowsDesktopEnrollment holds one group's state for one discovery cycle.
type windowsDesktopEnrollment struct {
	s        *Server
	log      *slog.Logger
	group    *server.AzureInstances
	vmTasks  *azureVMTasks
	backoff  *azureInstallerBackoff
	sem      *semaphore.Weighted
	status   discoveryGroupStatus
	enrolled []server.MatchedInstance[types.DynamicWindowsDesktop]
}

func newWindowsDesktopEnrollment(s *Server, log *slog.Logger, group *server.AzureInstances, vmTasks *azureVMTasks, backoff *azureInstallerBackoff, sem *semaphore.Weighted) *windowsDesktopEnrollment {
	return &windowsDesktopEnrollment{
		s:       s,
		log:     log,
		group:   group,
		vmTasks: vmTasks,
		backoff: backoff,
		sem:     sem,
	}
}

// refreshDesktops bumps the expiry of already-enrolled desktops. It does not
// update e.status, callers decide whether the counts should be recorded.
func (e *windowsDesktopEnrollment) refreshDesktops(syncTime time.Time) (enrolled, failed int) {
	for _, m := range e.enrolled {
		if err := checkDesktopIsDiscoveryManaged(m.Resource, e.s.DiscoveryGroup); err != nil {
			// The desktop exists but is maintained by something else (another
			// discovery group, manually created, etc). Count it as enrolled and
			// leave it for the owner to refresh.
			e.log.DebugContext(e.s.ctx, "Not refreshing dynamic Windows desktop maintained outside of this discovery group",
				"vm_id", m.VM.ID,
				"error", err,
			)
			enrolled++
			continue
		}
		if err := e.s.createAzureWindowsDesktop(m.VM, syncTime, m.Resource); err != nil {
			e.log.WarnContext(e.s.ctx, "Failed to refresh dynamic Windows desktop", "vm_id", m.VM.ID, "error", err)
			failed++
			continue
		}
		enrolled++
	}
	return enrolled, failed
}

// refreshEnrolled removes VMs that already have a dynamic desktop from the
// group and refreshes those desktops. A dynamic desktop has no agent of its
// own to heartbeat it, so if we don't touch it again here it will eventually
// expire even though the VM is still around.
func (e *windowsDesktopEnrollment) refreshEnrolled() error {
	desktops, err := e.s.dynamicWindowsDesktopWatcher.CurrentResources(e.s.ctx)
	if err != nil {
		return trace.Wrap(err, "failed to get existing desktops")
	}
	e.enrolled = e.group.FilterExistingDesktops(desktops)
	if len(e.enrolled) == 0 {
		return nil
	}
	e.log.DebugContext(e.s.ctx, "Refreshing Azure instances that have already been enrolled",
		"enrolled", len(e.enrolled),
	)
	enrolled, failed := e.refreshDesktops(e.s.clock.Now())
	e.status.enrolled += enrolled
	e.status.failed += failed
	return nil
}

// postInstallRefreshEnrolled re-refreshes enrolled desktops if the install
// took longer than the poll interval, so they don't expire mid-cycle. Counts
// were already recorded by refreshEnrolled, so they are discarded.
func (e *windowsDesktopEnrollment) postInstallRefreshEnrolled(iterationStart time.Time) {
	duration := e.s.clock.Since(iterationStart)
	if duration <= e.s.PollInterval || len(e.enrolled) == 0 {
		return
	}
	e.log.DebugContext(e.s.ctx, "Refreshing Azure instances that have already been enrolled, due to a long running install process",
		"enrolled", len(e.enrolled),
		"poll_interval", e.s.PollInterval,
		"install_duration", duration,
	)
	e.refreshDesktops(e.s.clock.Now())
}

// dropInvalidVMs filters out Azure VMs that have no VM ID or primary private IP.
// The VM ID is used as the name of the dynamic Windows desktop resource that
// will be created. The private IP address is used to reach the VM.
func (e *windowsDesktopEnrollment) dropInvalidVMs() {
	e.group.Instances = slices.DeleteFunc(e.group.Instances, func(vm *azure.VirtualMachine) bool {
		err := validateWindowsVM(vm)
		if err == nil {
			return false
		}
		e.log.WarnContext(e.s.ctx, "Skipping Azure VM that failed validation; a dynamic Windows desktop cannot be registered for it",
			"vm", vm.Name,
			"error", err,
		)
		e.status.failed++

		if errors.Is(err, errNoPrimaryPrivateIP) {
			e.recordFailure(vm, classifyAzureWindowsDesktopRegistrationError(errNoPrimaryPrivateIP), e.s.clock.Now())
		}
		return true
	})
}

// registerDesktop creates or refreshes the dynamic Windows desktop for vm and
// records the outcome in the group's status. existing is the current desktop
// when known, or nil.
func (e *windowsDesktopEnrollment) registerDesktop(vm *azure.VirtualMachine, existing types.DynamicWindowsDesktop, syncTime time.Time) bool {
	if err := e.s.createAzureWindowsDesktop(vm, syncTime, existing); err != nil {
		e.log.WarnContext(e.s.ctx, "Failed to register dynamic Windows desktop for Azure VM",
			"vm", vm.Name,
			"vm_id", vm.ID,
			"error", err,
		)
		e.status.failed++
		return false
	}
	e.status.enrolled++
	return true
}

// skipInstallation handles the case where the SkipInstallation
// flag is set and only registration is needed.
// Returns if SkipInstallation is set or not.
func (e *windowsDesktopEnrollment) skipInstallation() bool {
	// Respect the SkipInstallation flag, which is used in cases where the Windows
	// auth package is pre-baked into the VM's image. For these cases, we only
	// need to register the desktop resource.
	if e.group.Metadata.InstallerParams != nil && e.group.Metadata.InstallerParams.SkipInstallation {
		e.log.DebugContext(e.s.ctx, "Skipping installation for Azure VMs per skip_installation setting",
			"vms", genAzureInstancesLogStr(e.group.Instances),
		)
		syncTime := e.s.clock.Now()
		for _, vm := range e.group.Instances {
			e.registerDesktop(vm, nil, syncTime)
		}
		return true
	}
	return false
}

// recordFailure records a failed enrollment attempt for the given VM at the
// specified time.
func (e *windowsDesktopEnrollment) recordFailure(vm *azure.VirtualMachine, issueType string, at time.Time) {
	entry := e.backoff.recordFailedAttempt(vm, issueType, at)
	addFailedEnrollment(e.group, entry)
}

// filterVMsWithBackoff filters out VMs that have an active installation backoff
// and retries any VMs that failed because of a previous insert failure.
func (e *windowsDesktopEnrollment) filterVMsWithBackoff() []installerBackoffEntry[*azure.VirtualMachine] {
	skipped := e.backoff.filter(e.group, e.s.clock.Now())
	if len(skipped) > 0 {
		e.log.DebugContext(e.s.ctx, "Skipping Azure VMs with an active installation backoff",
			"skipped", len(skipped),
		)
		for _, entry := range skipped {
			if entry.isFailedAttempt() {
				e.status.failed++
				continue
			}

			// If this was a successful enrollment, but the desktop wasn't filtered
			// out by FilterExistingDesktops, the desktop insert must have failed.
			// Retry the insert so a successfully enrolled desktop isn't left out.
			if !e.registerDesktop(entry.target, nil, e.s.clock.Now()) {
				// Re-record the successful install so retryAfter keeps moving forward.
				// This keeps the VM on the insert-only retry path instead of eventually
				// becoming retryable for a full re-install and possible reboot if
				// restart_after_enrollment is set.
				e.backoff.recordSuccessfulAttempt(entry.target, e.s.clock.Now())
			}
		}
	}
	return skipped
}

// handleAzureWindowsDesktops handles the enrollment of Azure Windows desktops
// for a given group. It has multiple steps:
//  1. Drop invalid VMs (missing VM ID or Private IP address).
//  2. Refresh already enrolled desktops to bump their expiry.
//  3. If skip_installation is set, register desktops without installing the
//     package.
//  4. Skip desktops with active installation backoff.
//  5. Install the Teleport Windows auth package on remaining VMs.
//  6. Refresh enrolled desktops post-installation if the above installation took
//     longer than the discovery service's poll interval.
//  7. Register desktops for successful installs and record skipped/failed
//     enrollments.
func (s *Server) handleAzureWindowsDesktops(group *server.AzureInstances, vmTasks *azureVMTasks, backoff *azureInstallerBackoff, sem *semaphore.Weighted) (discoveryGroupStatus, error) {
	e := newWindowsDesktopEnrollment(s, s.Log.With("group", group), group, vmTasks, backoff, sem)

	found := len(e.group.Instances)
	if found == 0 {
		e.log.DebugContext(s.ctx, "No Azure instances found")
		return e.status, nil
	}
	e.status.found += found
	iterationStart := e.s.clock.Now()

	e.dropInvalidVMs()

	if err := e.refreshEnrolled(); err != nil {
		return e.status, err
	}

	if len(e.group.Instances) == 0 {
		e.log.DebugContext(e.s.ctx, "No new Azure instances to enroll")
		return e.status, nil
	}

	// If SkipInstallation is set, register desktops without installing the package.
	if e.skipInstallation() {
		return e.status, nil
	}

	// Skip desktops that have an active installation backoff.
	skipped := e.filterVMsWithBackoff()

	// Install the Teleport Windows auth package on the remaining Azure VMs.
	var results []server.AzureInstallResult
	var installErr error
	if len(e.group.Instances) > 0 {
		e.log.DebugContext(e.s.ctx, "Installing Teleport Windows auth package on Azure VMs",
			"vms", genAzureInstancesLogStr(e.group.Instances),
		)
		results, installErr = e.s.installWindowsDesktops(e.group, e.vmTasks, e.sem)
	}
	postInstallSyncTime := e.s.clock.Now()

	// Refresh enrolled desktops after an installation attempt.
	e.postInstallRefreshEnrolled(iterationStart)

	// Record skipped desktops after the install so their user tasks share the
	// post-install sync time
	for _, entry := range skipped {
		addFailedEnrollment(e.group, entry)
	}

	successful, failures := splitInstallResults(results)
	e.log.InfoContext(e.s.ctx, "Finished installation batch",
		"total_instances", len(results),
		"failures", len(failures),
	)

	// Register desktops which successfully installed the Teleport Auth Package.
	for _, result := range successful {
		// Record success even if the desktop insert fails, so the next cycle
		// retries just the insert instead of reinstalling.
		e.registerDesktop(result.Instance, nil, postInstallSyncTime)
		e.backoff.recordSuccessfulAttempt(result.Instance, postInstallSyncTime)
	}

	// Record VMs whose install ran and failed.
	e.status.failed += len(failures)
	if len(failures) > 0 {
		e.log.WarnContext(
			e.s.ctx,
			"Failed to install Teleport Windows auth package on some discovered Azure VMs",
			"failures", len(failures),
		)
	}
	// Record failures as user tasks.
	for _, result := range failures {
		e.recordFailure(result.Instance, classifyAzureWindowsAuthPackageInstallResultIssue(result), postInstallSyncTime)
	}

	// If the Azure call to install didn't have any errors, we can return early
	// here. VMs that failed individually have already been recorded above.
	if installErr == nil {
		return e.status, nil
	}

	// The call to Azure to install the auth package failed, so classify the error
	// and record it for each VM without a result from the above attempt.
	e.log.WarnContext(
		e.s.ctx, "Failed to install Teleport Windows auth package on Azure VMs",
		"error", installErr,
	)
	ran := make(map[*azure.VirtualMachine]struct{}, len(results))
	for _, result := range results {
		ran[result.Instance] = struct{}{}
	}

	issueType := classifyAzureVMEnrollmentError(installErr)
	for _, vm := range e.group.Instances {
		// If the VM has already been accounted for, skip it.
		if _, ok := ran[vm]; ok {
			continue
		}
		e.status.failed++
		e.recordFailure(vm, issueType, postInstallSyncTime)
	}

	return e.status, installErr
}

// validateWindowsVM checks that the given Azure VM has the necessary properties
// to be registered as a dynamic Windows desktop. It returns an error if any
// required property is missing.
func validateWindowsVM(vm *azure.VirtualMachine) error {
	switch {
	case vm.VMID == "":
		return errNoVMID
	case vm.PrimaryPrivateIP == "":
		return errNoPrimaryPrivateIP
	}
	return nil
}

// installWindowsDesktops installs the Teleport Windows auth package on a batch
// of Azure VMs.
// TODO(danielashare): Switch vmTasks to use new Windows specific user tasks
func (s *Server) installWindowsDesktops(group *server.AzureInstances, vmTasks *azureVMTasks, sem *semaphore.Weighted) ([]server.AzureInstallResult, error) {
	azureClients, err := s.getAzureClients(s.ctx, group.Metadata.Integration)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get Azure clients")
	}

	runClient, err := azureClients.GetRunCommandClient(s.ctx, group.Metadata.SubscriptionID)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get Azure Run Command client")
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

// createAzureWindowsDesktop creates or refreshes the dynamic Windows desktop
// resource for an Azure VM. It is called both for newly enrolled VMs and for
// VMs that were already enrolled in a previous cycle. A dynamic desktop has
// no agent of its own to heartbeat it, so it must be refreshed on every
// discovery cycle or it will eventually expire even though the VM is still
// around.
func (s *Server) createAzureWindowsDesktop(vm *azure.VirtualMachine, syncTime time.Time, existing types.DynamicWindowsDesktop) error {
	if vm.PrimaryPrivateIP == "" {
		return trace.Wrap(errNoPrimaryPrivateIP, "cannot register dynamic Windows desktop for Azure VM %q with no primary private IP", vm.Name)
	}
	if vm.VMID == "" {
		// Should never happen, but if it does, we can't register the desktop
		// because the VM can't be tracked by the discovery service.
		return trace.BadParameter("cannot register dynamic Windows desktop for Azure VM %q with no VM ID", vm.Name)
	}

	name := fmt.Sprintf("azure-windows-%s", vm.VMID)
	labels := make(map[string]string)
	for k, v := range vm.Tags {
		if types.IsValidLabelKey(k) {
			labels[k] = v
		} else {
			s.Log.DebugContext(s.ctx, "Skipping Azure VM tag that is not a valid label key",
				"tag", k,
				"vm", vm.Name,
			)
		}
	}
	// Internal labels.
	labels[types.TeleportInternalDiscoveryGroupName] = s.DiscoveryGroup
	labels[types.SubscriptionIDLabelInternal] = vm.Subscription
	labels[types.ResourceGroupLabelInternal] = vm.ResourceGroup
	labels[types.RegionLabelInternal] = vm.Location
	labels[types.VMIDLabelInternal] = vm.VMID
	// Public labels.
	labels[types.DiscoveryTypeLabel] = types.AzureMatcherWindowsVM
	labels[types.SubscriptionIDLabel] = vm.Subscription
	labels[types.ResourceGroupLabel] = vm.ResourceGroup
	labels[types.OriginLabel] = types.OriginCloud
	labels[types.CloudLabel] = types.CloudAzure
	labels[types.RegionLabel] = vm.Location
	labels[types.VMIDLabel] = vm.VMID
	labels[types.ADLabel] = "false"

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

	// If we haven't been supplied an existing dynamic Windows desktop resource,
	// because this call isn't part of the refresh process, optimistically attempt
	// to create the desktop to avoid a read and update on the resource. If it
	// already exists, we'll have to retrieve it and go down the update path.
	if existing == nil {
		_, err := s.AccessPoint.CreateDynamicWindowsDesktop(s.ctx, desktop)
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err, "failed to register dynamic Windows desktop for Azure VM %q", vm.Name)
		}
		if existing, err = s.AccessPoint.GetDynamicWindowsDesktop(s.ctx, name); err != nil {
			return trace.Wrap(err, "failed to get dynamic Windows desktop for Azure VM %q", vm.Name)
		}
	}

	updateDesktop := func() error {
		desktop.SetRevision(existing.GetRevision())
		_, err := s.AccessPoint.UpdateDynamicWindowsDesktop(s.ctx, desktop)
		return trace.Wrap(err)
	}

	if err := checkDesktopIsDiscoveryManaged(existing, s.DiscoveryGroup); err != nil {
		return trace.Wrap(err, "not refreshing dynamic Windows desktop owned by another discovery group")
	}

	err = updateDesktop()
	if trace.IsCompareFailed(err) {
		// The revision changed, re-read and re-check ownership in case another
		// owner replaced it and try again.
		existing, err = s.AccessPoint.GetDynamicWindowsDesktop(s.ctx, name)
		if err == nil {
			err = checkDesktopIsDiscoveryManaged(existing, s.DiscoveryGroup)
		}
		if err == nil {
			err = updateDesktop()
		}
	}
	if trace.IsNotFound(err) {
		// The desktop expired or was deleted after we last read it, so recreate it.
		_, err = s.AccessPoint.CreateDynamicWindowsDesktop(s.ctx, desktop)
	}
	return trace.Wrap(err, "failed to refresh dynamic Windows desktop for Azure VM %q", vm.Name)
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

	// Narrow to desktops created by Azure Windows VM discovery specifically,
	// rather than any cloud-imported resource.
	if discoveryType, _ := old.GetLabel(types.DiscoveryTypeLabel); discoveryType != types.AzureMatcherWindowsVM {
		return trace.CompareFailed("the resource is not managed by Azure Windows VM auto-discovery")
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
func addFailedEnrollment(instances *server.AzureInstances, entry installerBackoffEntry[*azure.VirtualMachine]) {
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
