/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package discovery

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/defaults"
	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/usertasks"
	"github.com/gravitational/teleport/api/utils/retryutils"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/server"
)

// FetcherStatus defines an interface for fetchers to report status
type FetcherStatus interface {
	// Status reports the last known status of the fetcher.
	Status() (uint64, error)
	// DiscoveryConfigName returns the name of the Discovery Config.
	DiscoveryConfigName() string
	// IsFromDiscoveryConfig returns true if the fetcher is associated with a Discovery Config.
	IsFromDiscoveryConfig() bool
}

// updateDiscoveryConfigStatus updates the DiscoveryConfig Status field with the current in-memory status.
// The status will be updated with the following matchers:
// - AWS Sync (TAG) status
// - AWS EC2 Auto Discover status
// - AWS RDS Auto Discover status
// - AWS EKS Auto Discover status
func (s *Server) updateDiscoveryConfigStatus(discoveryConfigNames ...string) {
	for _, discoveryConfigName := range discoveryConfigNames {
		// Static configurations (ie those in `teleport.yaml/discovery_config.<cloud>.matchers`) do not have a DiscoveryConfig resource.
		// Those are discarded because there's no Status to update.
		if discoveryConfigName == "" {
			continue
		}

		discoveryConfigStatus := discoveryconfig.Status{
			State:                          discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_SYNCING.String(),
			LastSyncTime:                   s.clock.Now(),
			IntegrationDiscoveredResources: make(map[string]*discoveryconfigv1.IntegrationDiscoveredSummary),
		}

		// Merge AWS or Azure Sync (TAG) status
		discoveryConfigStatus = s.tagSyncStatus.mergeIntoGlobalStatus(discoveryConfigName, discoveryConfigStatus)

		// Merge AWS EC2 Instances (auto discovery) status
		discoveryConfigStatus = s.awsEC2ResourcesStatus.mergeIntoGlobalStatus(discoveryConfigName, discoveryConfigStatus)

		// Merge AWS RDS databases (auto discovery) status
		discoveryConfigStatus = s.awsRDSResourcesStatus.mergeIntoGlobalStatus(discoveryConfigName, discoveryConfigStatus)

		// Merge AWS EKS clusters (auto discovery) status
		discoveryConfigStatus = s.awsEKSResourcesStatus.mergeIntoGlobalStatus(discoveryConfigName, discoveryConfigStatus)

		// Merge Azure VMs discovery status.
		discoveryConfigStatus = s.azureVMStatus.Load().mergeIntoGlobalStatus(discoveryConfigName, discoveryConfigStatus)

		// Ensure the error message is truncated to the maximum allowed size.
		// Too large error messages will cause failures when clients (which use the default MaxCallRecvMsgSize of 4MB) try to read DiscoveryConfigs.
		discoveryConfigStatus.ErrorMessage = truncateErrorMessage(discoveryConfigStatus)

		func() {
			ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
			defer cancel()

			_, err := s.AccessPoint.UpdateDiscoveryConfigStatus(ctx, discoveryConfigName, discoveryConfigStatus)
			switch {
			case trace.IsNotImplemented(err):
				s.Log.WarnContext(ctx, "UpdateDiscoveryConfigStatus method is not implemented in Auth Server. Please upgrade it to a recent version.")
			case err != nil:
				s.Log.WarnContext(ctx, "Error updating discovery config status", "discovery_config_name", discoveryConfigName, "error", err)
			}
		}()
	}
}

func truncateErrorMessage(discoveryConfigStatus discoveryconfig.Status) *string {
	if discoveryConfigStatus.ErrorMessage == nil {
		return nil
	}

	if len(*discoveryConfigStatus.ErrorMessage) <= defaults.DefaultMaxErrorMessageSize {
		return discoveryConfigStatus.ErrorMessage
	}

	newErrorMessage := (*discoveryConfigStatus.ErrorMessage)[:defaults.DefaultMaxErrorMessageSize]

	return &newErrorMessage
}

// tagSyncStatus contains all the status for both AWS and Azure fetchers grouped by DiscoveryConfig.
type tagSyncStatus struct {
	mu sync.RWMutex
	// syncResults maps the DiscoveryConfig name to a AWS or Azure fetcher result.
	// Each DiscoveryConfig might have multiple AWS or Azure matchers.
	syncResults map[string][]tagSyncResult
}

// newTagSyncStatus creates a new sync status object for storing results from the last fetch
func newTagSyncStatus() *tagSyncStatus {
	return &tagSyncStatus{
		syncResults: make(map[string][]tagSyncResult),
	}
}

// tagSyncResult stores the result of the aws_sync Matchers for a given DiscoveryConfig.
type tagSyncResult struct {
	// state is the State for the DiscoveryConfigStatus.
	// Allowed values are:
	// - DISCOVERY_CONFIG_STATE_SYNCING
	// - DISCOVERY_CONFIG_STATE_ERROR
	// - DISCOVERY_CONFIG_STATE_RUNNING
	state               string
	errorMessage        *string
	lastSyncTime        time.Time
	discoveredResources uint64
}

func (d *tagSyncStatus) syncFinished(fetcher FetcherStatus, pushErr error, lastUpdate time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Only update the status for fetchers that are from the discovery config.
	if !fetcher.IsFromDiscoveryConfig() {
		return
	}

	count, statusErr := fetcher.Status()
	statusAndPushErr := trace.NewAggregate(statusErr, pushErr)

	fetcherResult := tagSyncResult{
		state:               discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING.String(),
		lastSyncTime:        lastUpdate,
		discoveredResources: count,
	}

	if statusAndPushErr != nil {
		errorMessage := statusAndPushErr.Error()
		fetcherResult.errorMessage = &errorMessage
		fetcherResult.state = discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_ERROR.String()
	}

	d.syncResults[fetcher.DiscoveryConfigName()] = append(d.syncResults[fetcher.DiscoveryConfigName()], fetcherResult)
}

func (d *tagSyncStatus) discoveryConfigs() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	ret := make([]string, 0, len(d.syncResults))
	for k := range d.syncResults {
		ret = append(ret, k)
	}
	return ret
}

func (d *tagSyncStatus) syncStarted(fetcher FetcherStatus, lastUpdate time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	// Only update the status for fetchers that are from the discovery config.
	if !fetcher.IsFromDiscoveryConfig() {
		return
	}

	fetcherResult := tagSyncResult{
		state:        discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_SYNCING.String(),
		lastSyncTime: lastUpdate,
	}

	d.syncResults[fetcher.DiscoveryConfigName()] = append(d.syncResults[fetcher.DiscoveryConfigName()], fetcherResult)
}

func (d *tagSyncStatus) mergeIntoGlobalStatus(discoveryConfigName string, existingStatus discoveryconfig.Status) discoveryconfig.Status {
	d.mu.RLock()
	defer d.mu.RUnlock()

	awsStatusFetchers, found := d.syncResults[discoveryConfigName]
	if !found {
		return existingStatus
	}

	var statusErrorMessages []string
	if existingStatus.ErrorMessage != nil {
		statusErrorMessages = append(statusErrorMessages, *existingStatus.ErrorMessage)
	}
	for _, fetcher := range awsStatusFetchers {
		existingStatus.DiscoveredResources = existingStatus.DiscoveredResources + fetcher.discoveredResources

		// Each DiscoveryConfigStatus has a global State and Error Message, but those are produced per Fetcher.
		// We choose to keep the most informative states by favoring error states/messages.
		if fetcher.errorMessage != nil {
			statusErrorMessages = append(statusErrorMessages, *fetcher.errorMessage)
		}

		if existingStatus.State != discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_ERROR.String() {
			existingStatus.State = fetcher.state
		}

		// Keep the earliest sync time.
		if existingStatus.LastSyncTime.After(fetcher.lastSyncTime) {
			existingStatus.LastSyncTime = fetcher.lastSyncTime
		}
	}

	if len(statusErrorMessages) > 0 {
		newErrorMessage := strings.Join(statusErrorMessages, "\n")
		existingStatus.ErrorMessage = &newErrorMessage
	}

	return existingStatus
}

func newAWSResourceStatusCollector(resourceType string) awsResourcesStatus {
	return awsResourcesStatus{
		resourceType: resourceType,
	}
}

// awsResourcesStatus contains all the status for AWS Matchers grouped by DiscoveryConfig for a specific matcher type.
type awsResourcesStatus struct {
	mu sync.RWMutex
	// awsResourcesResults maps the DiscoveryConfig name and integration to a summary of discovered/enrolled resources.
	awsResourcesResults map[awsResourceGroup]awsResourceGroupResult
	resourceType        string
}

// awsResourceGroup is the key for the summary
type awsResourceGroup struct {
	discoveryConfigName string
	integration         string
}

func awsResourceGroupFromLabels(labels map[string]string) awsResourceGroup {
	return awsResourceGroup{
		discoveryConfigName: labels[types.TeleportInternalDiscoveryConfigName],
		integration:         labels[types.TeleportInternalDiscoveryIntegrationName],
	}
}

// awsResourceGroupResult stores the result of the aws_sync Matchers for a given DiscoveryConfig.
type awsResourceGroupResult struct {
	found    int
	enrolled int
	failed   int
}

func (d *awsResourcesStatus) reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.awsResourcesResults = make(map[awsResourceGroup]awsResourceGroupResult)
}

func (ars *awsResourcesStatus) mergeIntoGlobalStatus(discoveryConfigName string, existingStatus discoveryconfig.Status) discoveryconfig.Status {
	ars.mu.RLock()
	defer ars.mu.RUnlock()

	for group, groupResult := range ars.awsResourcesResults {
		if group.discoveryConfigName != discoveryConfigName {
			continue
		}

		// Update global discovered resources count.
		existingStatus.DiscoveredResources = existingStatus.DiscoveredResources + uint64(groupResult.found)

		// Update counters specific to AWS resources discovered.
		existingIntegrationResources, ok := existingStatus.IntegrationDiscoveredResources[group.integration]
		if !ok {
			existingIntegrationResources = &discoveryconfigv1.IntegrationDiscoveredSummary{}
		}

		resourcesSummary := &discoveryconfigv1.ResourcesDiscoveredSummary{
			Found:    uint64(groupResult.found),
			Enrolled: uint64(groupResult.enrolled),
			Failed:   uint64(groupResult.failed),
		}

		integrationDiscoveredSummaryUpdate(existingIntegrationResources, ars.resourceType, resourcesSummary)

		existingStatus.IntegrationDiscoveredResources[group.integration] = existingIntegrationResources
	}

	return existingStatus
}

func (ars *awsResourcesStatus) incrementFailed(g awsResourceGroup, count int) {
	ars.mu.Lock()
	defer ars.mu.Unlock()
	if ars.awsResourcesResults == nil {
		ars.awsResourcesResults = make(map[awsResourceGroup]awsResourceGroupResult)
	}
	groupStats := ars.awsResourcesResults[g]
	groupStats.failed = groupStats.failed + count
	ars.awsResourcesResults[g] = groupStats
}

func (ars *awsResourcesStatus) iterationStarted(g awsResourceGroup) {
	ars.mu.Lock()
	defer ars.mu.Unlock()
	if ars.awsResourcesResults == nil {
		ars.awsResourcesResults = make(map[awsResourceGroup]awsResourceGroupResult)
	}
	ars.awsResourcesResults[g] = awsResourceGroupResult{}
}

func (ars *awsResourcesStatus) incrementFound(g awsResourceGroup, count int) {
	ars.mu.Lock()
	defer ars.mu.Unlock()
	if ars.awsResourcesResults == nil {
		ars.awsResourcesResults = make(map[awsResourceGroup]awsResourceGroupResult)
	}
	groupStats := ars.awsResourcesResults[g]
	groupStats.found = groupStats.found + count
	ars.awsResourcesResults[g] = groupStats
}

func (ars *awsResourcesStatus) incrementEnrolled(g awsResourceGroup, count int) {
	ars.mu.Lock()
	defer ars.mu.Unlock()
	if ars.awsResourcesResults == nil {
		ars.awsResourcesResults = make(map[awsResourceGroup]awsResourceGroupResult)
	}
	groupStats := ars.awsResourcesResults[g]
	groupStats.enrolled = groupStats.enrolled + count
	ars.awsResourcesResults[g] = groupStats
}

// ReportEC2SSMInstallationResult is called when discovery gets the result of running the installation script in a EC2 instance.
// It will emit an audit event with the result and update the DiscoveryConfig status
func (s *Server) ReportEC2SSMInstallationResult(ctx context.Context, result *server.SSMInstallationResult) error {
	if err := s.Emitter.EmitAuditEvent(ctx, result.SSMRunEvent); err != nil {
		return trace.Wrap(err)
	}

	// Only failed runs are counted.
	// Successful ones only mean that the teleport was installed in the target host.
	// If they succeed in joining the cluster, during the next iteration, they will be countd as "enrolled"
	if result.SSMRunEvent.Metadata.Code == libevents.SSMRunSuccessCode {
		return nil
	}

	s.awsEC2ResourcesStatus.incrementFailed(awsResourceGroup{
		discoveryConfigName: result.DiscoveryConfigName,
		integration:         result.IntegrationName,
	}, 1)

	s.awsEC2Tasks.addFailedEnrollment(
		awsEC2TaskKey{
			integration:     result.IntegrationName,
			issueType:       result.IssueType,
			accountID:       result.SSMRunEvent.AccountID,
			region:          result.SSMRunEvent.Region,
			ssmDocument:     result.SSMDocumentName,
			installerScript: result.InstallerScript,
		},
		&usertasksv1.DiscoverEC2Instance{
			InvocationUrl:   result.SSMRunEvent.InvocationURL,
			DiscoveryConfig: result.DiscoveryConfigName,
			DiscoveryGroup:  s.DiscoveryGroup,
			SyncTime:        timestamppb.New(result.SSMRunEvent.Time),
			InstanceId:      result.SSMRunEvent.InstanceID,
			Name:            result.InstanceName,
		},
	)

	return nil
}

// awsEC2Tasks contains the Discover EC2 User Tasks that must be reported to the user.
type awsEC2Tasks struct {
	mu sync.RWMutex
	// instancesIssues maps the Discover EC2 User Task grouping parts to a set of instances metadata.
	instancesIssues map[awsEC2TaskKey]*usertasksv1.DiscoverEC2
	// issuesSyncQueue is used to register which groups were changed in memory but were not yet sent to the cluster.
	// When upserting User Tasks, if the group is not in issuesSyncQueue,
	// then the cluster already has the latest version of this particular group.
	issuesSyncQueue map[awsEC2TaskKey]struct{}
}

// awsEC2TaskKey identifies a UserTask group.
type awsEC2TaskKey struct {
	integration     string
	issueType       string
	accountID       string
	region          string
	ssmDocument     string
	installerScript string
}

// reset clears out any in memory issues that were recorded.
// This is used when starting a new Auto Discover EC2 watcher iteration.
func (d *awsEC2Tasks) reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.instancesIssues = make(map[awsEC2TaskKey]*usertasksv1.DiscoverEC2)
	d.issuesSyncQueue = make(map[awsEC2TaskKey]struct{})
}

// addFailedEnrollment adds an enrollment failure of a given instance.
func (d *awsEC2Tasks) addFailedEnrollment(g awsEC2TaskKey, instance *usertasksv1.DiscoverEC2Instance) {
	// Only failures associated with an Integration are reported.
	// There's no major blocking for showing non-integration User Tasks, but this keeps scope smaller.
	if g.integration == "" {
		return
	}
	if g.issueType == "" {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.instancesIssues == nil {
		d.instancesIssues = make(map[awsEC2TaskKey]*usertasksv1.DiscoverEC2)
	}
	if _, ok := d.instancesIssues[g]; !ok {
		d.instancesIssues[g] = &usertasksv1.DiscoverEC2{
			Instances:       make(map[string]*usertasksv1.DiscoverEC2Instance),
			AccountId:       g.accountID,
			Region:          g.region,
			SsmDocument:     g.ssmDocument,
			InstallerScript: g.installerScript,
		}
	}
	if instance != nil {
		d.instancesIssues[g].Instances[instance.InstanceId] = instance
	}

	if d.issuesSyncQueue == nil {
		d.issuesSyncQueue = make(map[awsEC2TaskKey]struct{})
	}
	d.issuesSyncQueue[g] = struct{}{}
}

// awsEKSTasks contains the Discover EKS User Tasks that must be reported to the user.
type awsEKSTasks struct {
	mu sync.RWMutex
	// clusterIssues maps the EKS Task Key to a set of clusters.
	// Each Task Key represents a single User Task that is going to be created for a set of EKS Clusters that suffer from the same issue.
	clusterIssues map[awsEKSTaskKey]*usertasksv1.DiscoverEKS
	// issuesSyncQueue is used to register which groups were changed in memory but were not yet sent to the cluster.
	// When upserting User Tasks, if the group is not in issuesSyncQueue,
	// then the cluster already has the latest version of this particular group.
	issuesSyncQueue map[awsEKSTaskKey]struct{}
}

// awsEKSTaskKey identifies a UserTask group.
type awsEKSTaskKey struct {
	integration     string
	issueType       string
	accountID       string
	region          string
	appAutoDiscover bool
}

// reset clears out any in memory issues that were recorded.
// This is used when starting a new Auto Discover EKS watcher iteration.
func (d *awsEKSTasks) reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.clusterIssues = make(map[awsEKSTaskKey]*usertasksv1.DiscoverEKS)
	d.issuesSyncQueue = make(map[awsEKSTaskKey]struct{})
}

// addFailedEnrollment adds an enrollment failure of a given cluster.
func (d *awsEKSTasks) addFailedEnrollment(g awsEKSTaskKey, cluster *usertasksv1.DiscoverEKSCluster) {
	// Only failures associated with an Integration are reported.
	// There's no major blocking for showing non-integration User Tasks, but this keeps scope smaller.
	if g.integration == "" {
		return
	}

	if g.issueType == "" {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.clusterIssues == nil {
		d.clusterIssues = make(map[awsEKSTaskKey]*usertasksv1.DiscoverEKS)
	}
	if _, ok := d.clusterIssues[g]; !ok {
		d.clusterIssues[g] = &usertasksv1.DiscoverEKS{
			Clusters:        make(map[string]*usertasksv1.DiscoverEKSCluster),
			AccountId:       g.accountID,
			Region:          g.region,
			AppAutoDiscover: g.appAutoDiscover,
		}
	}
	d.clusterIssues[g].Clusters[cluster.Name] = cluster

	if d.issuesSyncQueue == nil {
		d.issuesSyncQueue = make(map[awsEKSTaskKey]struct{})
	}
	d.issuesSyncQueue[g] = struct{}{}
}

// awsRDSTasks contains the Discover RDS User Tasks that must be reported to the user.
type awsRDSTasks struct {
	mu sync.RWMutex
	// databaseIssues maps the RDS Task Key to a set of databases.
	// Each Task Key represents a single User Task that is going to be created for a set of RDS Databases that suffer from the same issue.
	databaseIssues map[awsRDSTaskKey]*usertasksv1.DiscoverRDS
	// issuesSyncQueue is used to register which groups were changed in memory but were not yet sent to the database.
	// When upserting User Tasks, if the group is not in issuesSyncQueue,
	// then the database already has the latest version of this particular group.
	issuesSyncQueue map[awsRDSTaskKey]struct{}
}

// awsRDSTaskKey identifies a UserTask group.
type awsRDSTaskKey struct {
	integration string
	issueType   string
	accountID   string
	region      string
}

// reset clears out any in memory issues that were recorded.
// This is used when starting a new Auto Discover RDS watcher iteration.
func (d *awsRDSTasks) reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.databaseIssues = make(map[awsRDSTaskKey]*usertasksv1.DiscoverRDS)
	d.issuesSyncQueue = make(map[awsRDSTaskKey]struct{})
}

// addFailedEnrollment adds an enrollment failure of a given database.
func (d *awsRDSTasks) addFailedEnrollment(g awsRDSTaskKey, database *usertasksv1.DiscoverRDSDatabase) {
	// Only failures associated with an Integration are reported.
	// There's no major blocking for showing non-integration User Tasks, but this keeps scope smaller.
	if g.integration == "" {
		return
	}

	if g.issueType == "" {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.databaseIssues == nil {
		d.databaseIssues = make(map[awsRDSTaskKey]*usertasksv1.DiscoverRDS)
	}
	if _, ok := d.databaseIssues[g]; !ok {
		d.databaseIssues[g] = &usertasksv1.DiscoverRDS{
			Databases: make(map[string]*usertasksv1.DiscoverRDSDatabase),
			AccountId: g.accountID,
			Region:    g.region,
		}
	}
	d.databaseIssues[g].Databases[database.Name] = database

	if d.issuesSyncQueue == nil {
		d.issuesSyncQueue = make(map[awsRDSTaskKey]struct{})
	}
	d.issuesSyncQueue[g] = struct{}{}
}

// acquireSemaphoreForUserTask tries to acquire a semaphore lock for this user task.
// It returns a func which must be called to release the lock.
// It also returns a context which is tied to the lease and will be canceled if the lease ends.
func (s *taskUpdater) acquireSemaphoreForUserTask(userTaskName string) (releaseFn func(), ctx context.Context, err error) {
	// Use the deterministic task name as semaphore name.
	semaphoreName := userTaskName
	semaphoreExpiration := 10 * time.Second

	// AcquireSemaphoreLock will retry until the semaphore is acquired.
	// This prevents multiple discovery services to write AWS resources in parallel.
	// lease must be released to cleanup the resource in auth server.
	lease, err := services.AcquireSemaphoreLockWithRetry(
		s.ctx,
		services.SemaphoreLockConfigWithRetry{
			SemaphoreLockConfig: services.SemaphoreLockConfig{
				Service: s.AccessPoint,
				Params: types.AcquireSemaphoreRequest{
					SemaphoreKind: types.KindUserTask,
					SemaphoreName: semaphoreName,
					MaxLeases:     1,
					Holder:        s.ServerID,
				},
				Expiry: semaphoreExpiration,
				Clock:  s.clock,
			},
			Retry: retryutils.LinearConfig{
				Clock:  s.clock,
				First:  time.Second,
				Step:   semaphoreExpiration / 2,
				Max:    semaphoreExpiration,
				Jitter: retryutils.DefaultJitter,
			},
		},
	)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Once the lease parent context is canceled, the lease will be released.
	ctxWithLease, cancel := context.WithCancel(lease)

	releaseFn = func() {
		cancel()
		lease.Stop()
		if err := lease.Wait(); err != nil {
			s.Log.WarnContext(ctx, "Error cleaning up UserTask semaphore", "semaphore", semaphoreName, "error", err)
		}
	}

	return releaseFn, ctxWithLease, nil
}

// mergeUpsertDiscoverEC2Task takes the current DiscoverEC2 User Task issues stored in memory and
// merges them against the ones that exist in the cluster.
//
// All of this flow is protected by a lock to ensure there's no race between this and other DiscoveryServices.
func (s *taskUpdater) mergeUpsertDiscoverEC2Task(taskGroup awsEC2TaskKey, failedInstances *usertasksv1.DiscoverEC2) error {
	// Permission-related issues occur before instances can be discovered, so we allow empty instances.
	if len(failedInstances.Instances) == 0 && !usertasks.IsPermissionIssueType(taskGroup.issueType) {
		return nil
	}

	userTaskName := usertasks.TaskNameForDiscoverEC2(usertasks.TaskNameForDiscoverEC2Parts{
		Integration:     taskGroup.integration,
		IssueType:       taskGroup.issueType,
		AccountID:       taskGroup.accountID,
		Region:          taskGroup.region,
		SSMDocument:     taskGroup.ssmDocument,
		InstallerScript: taskGroup.installerScript,
	})

	releaseFn, ctxWithLease, err := s.acquireSemaphoreForUserTask(userTaskName)
	if err != nil {
		return trace.Wrap(err)
	}
	defer releaseFn()

	// Fetch the current task because it might have instances discovered by another group of DiscoveryServices.
	currentUserTask, err := s.AccessPoint.GetUserTask(ctxWithLease, userTaskName)
	switch {
	case trace.IsNotFound(err):
	case err != nil:
		return trace.Wrap(err)
	default:
		mergeExistingInstances(s, currentUserTask.Spec.DiscoverEc2.Instances, failedInstances.Instances)
	}

	// If the DiscoveryService is stopped, or the issue does not happen again
	// the task is removed to prevent users from working on issues that are no longer happening.
	taskExpiration := s.clock.Now().Add(2 * s.PollInterval)

	task, err := usertasks.NewDiscoverEC2UserTask(
		&usertasksv1.UserTaskSpec{
			Integration: taskGroup.integration,
			TaskType:    usertasks.TaskTypeDiscoverEC2,
			IssueType:   taskGroup.issueType,
			State:       usertasks.TaskStateOpen,
			DiscoverEc2: failedInstances,
		},
		usertasks.WithExpiration(taskExpiration),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := s.AccessPoint.UpsertUserTask(ctxWithLease, task); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *Server) upsertTasksForAWSEC2FailedEnrollments() {
	s.awsEC2Tasks.mu.Lock()
	defer s.awsEC2Tasks.mu.Unlock()
	for g := range s.awsEC2Tasks.issuesSyncQueue {
		if err := s.taskUpdater().mergeUpsertDiscoverEC2Task(g, s.awsEC2Tasks.instancesIssues[g]); err != nil {
			s.Log.WarnContext(s.ctx, "Failed to create discover ec2 user task",
				"integration", g.integration,
				"issue_type", g.issueType,
				"aws_account_id", g.accountID,
				"aws_region", g.region,
				"error", err,
			)
			continue
		}

		delete(s.awsEC2Tasks.issuesSyncQueue, g)
	}
}

func (s *Server) upsertTasksForAWSEKSFailedEnrollments() {
	s.awsEKSTasks.mu.Lock()
	defer s.awsEKSTasks.mu.Unlock()
	for g := range s.awsEKSTasks.issuesSyncQueue {
		if err := s.taskUpdater().mergeUpsertDiscoverEKSTask(g, s.awsEKSTasks.clusterIssues[g]); err != nil {
			s.Log.WarnContext(s.ctx, "Failed to create discover eks user task",
				"integration", g.integration,
				"issue_type", g.issueType,
				"aws_account_id", g.accountID,
				"aws_region", g.region,
				"error", err,
			)
			continue
		}

		delete(s.awsEKSTasks.issuesSyncQueue, g)
	}
}

// mergeUpsertDiscoverEKSTask takes the current DiscoverEKS User Task issues stored in memory and
// merges them against the ones that exist in the cluster.
//
// All of this flow is protected by a lock to ensure there's no race between this and other DiscoveryServices.
func (s *taskUpdater) mergeUpsertDiscoverEKSTask(taskGroup awsEKSTaskKey, failedClusters *usertasksv1.DiscoverEKS) error {
	if len(failedClusters.Clusters) == 0 {
		return nil
	}

	userTaskName := usertasks.TaskNameForDiscoverEKS(usertasks.TaskNameForDiscoverEKSParts{
		Integration:     taskGroup.integration,
		IssueType:       taskGroup.issueType,
		AccountID:       taskGroup.accountID,
		Region:          taskGroup.region,
		AppAutoDiscover: taskGroup.appAutoDiscover,
	})

	releaseFn, ctxWithLease, err := s.acquireSemaphoreForUserTask(userTaskName)
	if err != nil {
		return trace.Wrap(err)
	}
	defer releaseFn()

	// Fetch the current task because it might have instances discovered by another group of DiscoveryServices.
	currentUserTask, err := s.AccessPoint.GetUserTask(ctxWithLease, userTaskName)
	switch {
	case trace.IsNotFound(err):
	case err != nil:
		return trace.Wrap(err)
	default:
		mergeExistingInstances(s, currentUserTask.Spec.DiscoverEks.Clusters, failedClusters.Clusters)
	}

	// If the DiscoveryService is stopped, or the issue does not happen again
	// the task is removed to prevent users from working on issues that are no longer happening.
	taskExpiration := s.clock.Now().Add(2 * s.PollInterval)

	task, err := usertasks.NewDiscoverEKSUserTask(
		&usertasksv1.UserTaskSpec{
			Integration: taskGroup.integration,
			TaskType:    usertasks.TaskTypeDiscoverEKS,
			IssueType:   taskGroup.issueType,
			State:       usertasks.TaskStateOpen,
			DiscoverEks: failedClusters,
		},
		usertasks.WithExpiration(taskExpiration),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := s.AccessPoint.UpsertUserTask(ctxWithLease, task); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *Server) upsertTasksForAWSRDSFailedEnrollments() {
	s.awsRDSTasks.mu.Lock()
	defer s.awsRDSTasks.mu.Unlock()
	for g := range s.awsRDSTasks.issuesSyncQueue {
		if err := s.taskUpdater().mergeUpsertDiscoverRDSTask(g, s.awsRDSTasks.databaseIssues[g]); err != nil {
			s.Log.WarnContext(s.ctx, "Failed to create discover rds user task",
				"integration", g.integration,
				"issue_type", g.issueType,
				"aws_account_id", g.accountID,
				"aws_region", g.region,
				"error", err,
			)
			continue
		}

		delete(s.awsRDSTasks.issuesSyncQueue, g)
	}
}

// mergeUpsertDiscoverRDSTask takes the current DiscoverRDS User Task issues stored in memory and
// merges them against the ones that exist in the cluster.
//
// All of this flow is protected by a lock to ensure there's no race between this and other DiscoveryServices.
func (s *taskUpdater) mergeUpsertDiscoverRDSTask(taskGroup awsRDSTaskKey, failedDatabases *usertasksv1.DiscoverRDS) error {
	if len(failedDatabases.Databases) == 0 {
		return nil
	}

	userTaskName := usertasks.TaskNameForDiscoverRDS(usertasks.TaskNameForDiscoverRDSParts{
		Integration: taskGroup.integration,
		IssueType:   taskGroup.issueType,
		AccountID:   taskGroup.accountID,
		Region:      taskGroup.region,
	})

	releaseFn, ctxWithLease, err := s.acquireSemaphoreForUserTask(userTaskName)
	if err != nil {
		return trace.Wrap(err)
	}
	defer releaseFn()

	// Fetch the current task because it might have instances discovered by another group of DiscoveryServices.
	currentUserTask, err := s.AccessPoint.GetUserTask(ctxWithLease, userTaskName)
	switch {
	case trace.IsNotFound(err):
	case err != nil:
		return trace.Wrap(err)
	default:
		mergeExistingInstances(s, currentUserTask.Spec.DiscoverRds.Databases, failedDatabases.Databases)
	}

	// If the DiscoveryService is stopped, or the issue does not happen again
	// the task is removed to prevent users from working on issues that are no longer happening.
	taskExpiration := s.clock.Now().Add(2 * s.PollInterval)

	task, err := usertasks.NewDiscoverRDSUserTask(
		&usertasksv1.UserTaskSpec{
			Integration: taskGroup.integration,
			TaskType:    usertasks.TaskTypeDiscoverRDS,
			IssueType:   taskGroup.issueType,
			State:       usertasks.TaskStateOpen,
			DiscoverRds: failedDatabases,
		},
		usertasks.WithExpiration(taskExpiration),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := s.AccessPoint.UpsertUserTask(ctxWithLease, task); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func mergeExistingInstances[Instance interface {
	GetSyncTime() *timestamppb.Timestamp
	GetDiscoveryGroup() string
}](s *taskUpdater, oldInstances map[string]Instance, freshInstances map[string]Instance) {
	issueExpiration := s.clock.Now().Add(-2 * s.PollInterval)

	for instanceKey, instance := range oldInstances {
		// Each DiscoveryService works on all the DiscoveryConfigs assigned to a given DiscoveryGroup.
		// So, it's safe to say that current DiscoveryService has the last state for a given DiscoveryGroup.
		// If other VMs exist for this DiscoveryGroup, they can be discarded because, as said before, the current DiscoveryService has the last state for a given DiscoveryGroup.
		if instance.GetDiscoveryGroup() == s.DiscoveryGroup {
			continue
		}

		// For existing instances whose sync time is too far in the past, just drop them.
		// This ensures that if a resource is removed from its hosting platform, it will eventually be removed from the relevant User Tasks' list.
		// This also covers the case where DiscoveryConfig change moves a particular resource out of scope (because of labels/regions or other matchers).
		if instance.GetSyncTime().AsTime().Before(issueExpiration) {
			continue
		}

		// Merge existing cluster state into in-memory object, but only if we don't have a fresh key.
		if _, found := freshInstances[instanceKey]; !found {
			freshInstances[instanceKey] = instance
		}
	}
}

// azureVMTaskKey is an Azure VM-specific part of task key.
type azureVMTaskKey struct {
	subscriptionID string
	resourceGroup  string
	region         string
}

// azureVMTasks contains the Discover Azure VM User Tasks that must be reported to the user.
type azureVMTasks struct {
	taskGroups map[usertasks.TaskGroup]map[azureVMTaskKey]*usertasksv1.DiscoverAzureVM
}

// addFailedEnrollment adds an enrollment failure of a given VM.
func (t *azureVMTasks) addFailedEnrollment(tg usertasks.TaskGroup, key azureVMTaskKey, vm *usertasksv1.DiscoverAzureVMInstance) {
	// Only failures associated with an Integration are reported.
	if tg.Integration == "" {
		return
	}
	if tg.IssueType == "" {
		return
	}

	if t.taskGroups == nil {
		t.taskGroups = make(map[usertasks.TaskGroup]map[azureVMTaskKey]*usertasksv1.DiscoverAzureVM)
	}

	tgMap := t.taskGroups[tg]
	if tgMap == nil {
		tgMap = make(map[azureVMTaskKey]*usertasksv1.DiscoverAzureVM)
		t.taskGroups[tg] = tgMap
	}

	data := tgMap[key]
	if data == nil {
		data = &usertasksv1.DiscoverAzureVM{
			Instances:      make(map[string]*usertasksv1.DiscoverAzureVMInstance),
			SubscriptionId: key.subscriptionID,
			ResourceGroup:  key.resourceGroup,
			Region:         key.region,
		}
		tgMap[key] = data
	}

	data.Instances[vm.VmId] = vm
}

// upsertAll upserts all collected Azure VM user tasks to the backend.
func (t *azureVMTasks) upsertAll(s *taskUpdater) {
	expiryTime := s.clock.Now().Add(2 * s.PollInterval)

	for taskGroup, group := range t.taskGroups {
		for azureKey, vmData := range group {
			// skip empty entries
			if len(vmData.GetInstances()) == 0 {
				continue
			}

			log := s.Log.With("issue_type", taskGroup.IssueType,
				"integration", taskGroup.Integration,
				"subscription_id", azureKey.subscriptionID,
				"resource_group", azureKey.resourceGroup,
				"region", azureKey.region)

			task, err := usertasks.NewDiscoverAzureVMUserTask(taskGroup, expiryTime, vmData)
			if err != nil {
				log.WarnContext(s.ctx, "Failed to construct Discovery User Task (this is a bug)", "error", err)
				continue
			}

			err = s.mergeUpsertUserTask(task, s.mergeAzure)
			if err != nil {
				log.WarnContext(s.ctx, "Failed to upsert Discovery User Task", "error", err)
				continue
			}
		}
	}
}

func (s *Server) taskUpdater() *taskUpdater {
	return &taskUpdater{
		ctx:            s.ctx,
		clock:          s.clock,
		DiscoveryGroup: s.Config.DiscoveryGroup,
		ServerID:       s.Config.ServerID,
		AccessPoint:    s.Config.AccessPoint,
		PollInterval:   s.Config.PollInterval,
		Log:            s.Config.Log,
	}
}

type taskUpdaterAccessPoint interface {
	types.Semaphores
	GetUserTask(ctx context.Context, name string) (*usertasksv1.UserTask, error)
	UpsertUserTask(ctx context.Context, req *usertasksv1.UserTask) (*usertasksv1.UserTask, error)
}

type taskUpdater struct {
	ctx   context.Context
	clock clockwork.Clock

	// subset of Config fields
	DiscoveryGroup string
	ServerID       string
	AccessPoint    taskUpdaterAccessPoint
	PollInterval   time.Duration
	Log            *slog.Logger
}

func (s *taskUpdater) mergeUpsertUserTask(newTask *usertasksv1.UserTask, mergeUserTasks func(oldTask *usertasksv1.UserTaskSpec, newTask *usertasksv1.UserTaskSpec)) error {
	taskName := newTask.GetMetadata().GetName()

	releaseFn, ctxWithLease, err := s.acquireSemaphoreForUserTask(taskName)
	if err != nil {
		return trace.Wrap(err)
	}
	defer releaseFn()

	// Fetch the current task because it might have VMs discovered by another group of DiscoveryServices.
	oldTask, err := s.AccessPoint.GetUserTask(ctxWithLease, taskName)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	if oldTask != nil && oldTask.Spec != nil {
		mergeUserTasks(oldTask.GetSpec(), newTask.GetSpec())
	}

	_, err = s.AccessPoint.UpsertUserTask(ctxWithLease, newTask)
	if err != nil {
		return trace.Wrap(err)
	}

	s.Log.InfoContext(s.ctx, "Upserted user task", "task", taskName, "issue_type", newTask.GetSpec().IssueType, "integration", newTask.GetSpec().Integration)

	return nil
}

func (s *taskUpdater) mergeAzure(oldSpec *usertasksv1.UserTaskSpec, newSpec *usertasksv1.UserTaskSpec) {
	if oldSpec == nil || oldSpec.DiscoverAzureVm == nil {
		return
	}
	if newSpec.DiscoverAzureVm == nil {
		newSpec.DiscoverAzureVm = &usertasksv1.DiscoverAzureVM{}
	}
	if newSpec.DiscoverAzureVm.Instances == nil {
		newSpec.DiscoverAzureVm.Instances = make(map[string]*usertasksv1.DiscoverAzureVMInstance)
	}
	mergeExistingInstances(s, oldSpec.DiscoverAzureVm.Instances, newSpec.DiscoverAzureVm.Instances)
}

type statusType int

const (
	statusFound statusType = iota
	statusEnrolled
	statusFailed
)

type fetcherGroupKey struct {
	discoveryConfigName string
	integration         string
}

// resourceStatusMap tracks discovery status (found/enrolled/failed counts)
// per fetcher group key (discovery config + integration combination).
type resourceStatusMap struct {
	resourceType string
	results      map[fetcherGroupKey]map[statusType]int
}

func newStatusMap(resourceType string) *resourceStatusMap {
	return &resourceStatusMap{
		resourceType: resourceType,
		results:      make(map[fetcherGroupKey]map[statusType]int),
	}
}

func (s *resourceStatusMap) add(key fetcherGroupKey, results map[statusType]int) {
	if s.results[key] == nil {
		s.results[key] = make(map[statusType]int)
	}
	for k, v := range results {
		s.results[key][k] += v
	}
}

func (s *resourceStatusMap) mergeIntoGlobalStatus(discoveryConfigName string, existingStatus discoveryconfig.Status) discoveryconfig.Status {
	if s == nil {
		// nil resourceStatusMap is valid, just empty.
		return existingStatus
	}

	for key, results := range s.results {
		if key.discoveryConfigName != discoveryConfigName {
			continue
		}

		if results == nil {
			continue
		}

		// Update global discovered resources count.
		existingStatus.DiscoveredResources = existingStatus.DiscoveredResources + uint64(results[statusFailed])

		// Initialize map if needed.
		if existingStatus.IntegrationDiscoveredResources == nil {
			existingStatus.IntegrationDiscoveredResources = make(map[string]*discoveryconfigv1.IntegrationDiscoveredSummary)
		}

		// Update counters specific to resources discovered.
		var summary *discoveryconfigv1.IntegrationDiscoveredSummary
		summary = existingStatus.IntegrationDiscoveredResources[key.integration]
		if summary == nil {
			summary = &discoveryconfigv1.IntegrationDiscoveredSummary{}
		}

		resourcesSummary := &discoveryconfigv1.ResourcesDiscoveredSummary{
			Found:    uint64(results[statusFound]),
			Enrolled: uint64(results[statusEnrolled]),
			Failed:   uint64(results[statusFailed]),
		}

		integrationDiscoveredSummaryUpdate(summary, s.resourceType, resourcesSummary)

		existingStatus.IntegrationDiscoveredResources[key.integration] = summary
	}

	return existingStatus
}

func (s *resourceStatusMap) discoveryConfigs() []string {
	if s == nil {
		return nil
	}

	names := map[string]struct{}{}
	for key := range s.results {
		names[key.discoveryConfigName] = struct{}{}
	}
	return slices.Collect(maps.Keys(names))
}

func integrationDiscoveredSummaryUpdate(summary *discoveryconfigv1.IntegrationDiscoveredSummary, resourceType string, resourcesSummary *discoveryconfigv1.ResourcesDiscoveredSummary) {
	switch resourceType {
	case types.AWSMatcherEC2:
		summary.AwsEc2 = resourcesSummary
	case types.AWSMatcherRDS:
		summary.AwsRds = resourcesSummary
	case types.AWSMatcherEKS:
		summary.AwsEks = resourcesSummary
	case types.AzureMatcherVM:
		summary.AzureVms = resourcesSummary
	default:
		slog.WarnContext(context.Background(), "Unknown integration discovered summary resource type (this is a bug)", "resource_type", resourceType)
	}
}
