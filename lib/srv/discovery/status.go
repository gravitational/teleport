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
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	libevents "github.com/gravitational/teleport/lib/events"
	aws_sync "github.com/gravitational/teleport/lib/srv/discovery/fetchers/aws-sync"
	"github.com/gravitational/teleport/lib/srv/server"
)

// updateDiscoveryConfigStatus updates the DiscoveryConfig Status field with the current in-memory status.
// The status will be updated with the following matchers:
// - AWS Sync (TAG) status
func (s *Server) updateDiscoveryConfigStatus(discoveryConfigName string) {
	discoveryConfigStatus := discoveryconfig.Status{
		State:        discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_SYNCING.String(),
		LastSyncTime: s.clock.Now(),
	}

	// Merge AWS Sync (TAG) status
	discoveryConfigStatus = s.awsSyncStatus.mergeIntoGlobalStatus(discoveryConfigName, discoveryConfigStatus)

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	_, err := s.AccessPoint.UpdateDiscoveryConfigStatus(ctx, discoveryConfigName, discoveryConfigStatus)
	switch {
	case trace.IsNotImplemented(err):
		s.Log.Warn("UpdateDiscoveryConfigStatus method is not implemented in Auth Server. Please upgrade it to a recent version.")
	case err != nil:
		s.Log.WithError(err).WithField("discovery_config_name", discoveryConfigName).Info("Error updating discovery config status")
	}
}

// awsSyncStatus contains all the status for aws_sync Fetchers grouped by DiscoveryConfig.
type awsSyncStatus struct {
	mu sync.RWMutex
	// awsSyncResults maps the DiscoveryConfig name to a aws_sync result.
	// Each DiscoveryConfig might have multiple `aws_sync` matchers.
	awsSyncResults map[string][]awsSyncResult
}

// awsSyncResult stores the result of the aws_sync Matchers for a given DiscoveryConfig.
type awsSyncResult struct {
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

func (d *awsSyncStatus) iterationFinished(fetchers []aws_sync.AWSSync, pushErr error, lastUpdate time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.awsSyncResults = make(map[string][]awsSyncResult)
	for _, fetcher := range fetchers {
		// Only update the status for fetchers that are from the discovery config.
		if !fetcher.IsFromDiscoveryConfig() {
			continue
		}

		count, statusErr := fetcher.Status()
		statusAndPushErr := trace.NewAggregate(statusErr, pushErr)

		fetcherResult := awsSyncResult{
			state:               discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING.String(),
			lastSyncTime:        lastUpdate,
			discoveredResources: count,
		}

		if statusAndPushErr != nil {
			errorMessage := statusAndPushErr.Error()
			fetcherResult.errorMessage = &errorMessage
			fetcherResult.state = discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_ERROR.String()
		}

		d.awsSyncResults[fetcher.DiscoveryConfigName()] = append(d.awsSyncResults[fetcher.DiscoveryConfigName()], fetcherResult)
	}
}

func (d *awsSyncStatus) discoveryConfigs() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	ret := make([]string, 0, len(d.awsSyncResults))
	for k := range d.awsSyncResults {
		ret = append(ret, k)
	}
	return ret
}

func (d *awsSyncStatus) iterationStarted(fetchers []aws_sync.AWSSync, lastUpdate time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.awsSyncResults = make(map[string][]awsSyncResult)
	for _, fetcher := range fetchers {
		// Only update the status for fetchers that are from the discovery config.
		if !fetcher.IsFromDiscoveryConfig() {
			continue
		}

		fetcherResult := awsSyncResult{
			state:        discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_SYNCING.String(),
			lastSyncTime: lastUpdate,
		}

		d.awsSyncResults[fetcher.DiscoveryConfigName()] = append(d.awsSyncResults[fetcher.DiscoveryConfigName()], fetcherResult)
	}
}

func (d *awsSyncStatus) mergeIntoGlobalStatus(discoveryConfigName string, existingStatus discoveryconfig.Status) discoveryconfig.Status {
	d.mu.RLock()
	defer d.mu.RUnlock()

	awsStatusFetchers, found := d.awsSyncResults[discoveryConfigName]
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

// ReportEC2SSMInstallationResult is called when discovery gets the result of running the installation script in a EC2 instance.
// It will emit an audit event with the result and update the DiscoveryConfig status
func (s *Server) ReportEC2SSMInstallationResult(ctx context.Context, result *server.SSMInstallationResult) error {
	if err := s.Emitter.EmitAuditEvent(ctx, result.SSMRunEvent); err != nil {
		return trace.Wrap(err)
	}

	// Only notify user when something fails.
	if result.SSMRunEvent.Metadata.Code == libevents.SSMRunSuccessCode {
		return nil
	}

	region := result.SSMRunEvent.Region
	instanceID := result.SSMRunEvent.InstanceID
	resourceKey := ec2DiscoveredKey{
		region:         region,
		integration:    result.Integration,
		enrollMode:     types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
		failureCode:    failureCodeUnknownError,
		unhandledError: result.SSMRunEvent.Status,
	}
	resourceStatus := ec2DiscoveredStatus{
		instanceName:     result.InstanceName,
		ssmInvocationURL: result.SSMRunEvent.InvocationURL,
		instanceID:       instanceID,
	}

	s.enqueueFailedEnrollment(resourceKey, resourceStatus)

	return nil
}

// ec2DiscoveredKey uniquely identifies an ec2 instance and an enroll mode.
type ec2DiscoveredKey struct {
	region         string
	integration    string
	enrollMode     types.InstallParamEnrollMode
	failureCode    string
	unhandledError string
}

// ec2DiscoveredResourceStatus reports the result of auto-enrolling the ec2 instance into the cluster.
type ec2DiscoveredStatus struct {
	instanceID       string
	instanceName     string
	ssmInvocationURL string
}

const (
	failureCodeUnsupportedOS          = "unsupportedOS"
	failureCodeSSMAgentConnectionLost = "ssmConnectionLost"
	failureCodeSSMAgentMissing        = "ssmMissing"
	failureCodeUnknownError           = "unkownError"
)

var failureCodes = map[string]struct {
	errorMessage string
	nextSteps    string
}{
	failureCodeUnsupportedOS: {
		errorMessage: "You can only auto-enroll linux instances.",
		nextSteps:    "Ensure the matcher does not match this instance tags.",
	},
	failureCodeSSMAgentConnectionLost: {
		errorMessage: "The SSM Agent is not responding.",
		nextSteps:    "Restart or reinstall the SSM service. See https://docs.aws.amazon.com/systems-manager/latest/userguide/ami-preinstalled-agent.html#verify-ssm-agent-status for more details.",
	},
	failureCodeSSMAgentMissing: {
		errorMessage: "EC2 Instance is not registered in SSM.",
		nextSteps:    "Make sure that the instance has AmazonSSMManagedInstanceCore policy assigned.",
	},
	failureCodeUnknownError: {},
}

// enqueueFailedEnrollment adds a new
func (s *Server) enqueueFailedEnrollment(k ec2DiscoveredKey, status ec2DiscoveredStatus) {
	errorFields, ok := failureCodes[k.failureCode]

	fmt.Println("FAILURE CODE", k.failureCode)
	if !ok || errorFields.errorMessage == "" {
		errorFields = struct {
			errorMessage string
			nextSteps    string
		}{
			errorMessage: k.unhandledError,
			nextSteps:    "-",
		}
	}

	labels := map[string]string{
		types.NotificationTitleLabel: "Failed to auto enroll an EC2 instance",
		"error":                      errorFields.errorMessage,
		"next-steps":                 errorFields.nextSteps,
		"instance-id":                status.instanceID,
		"instance-name":              status.instanceName,
		"aws-region":                 k.region,
		"integration":                k.integration,
	}
	if status.ssmInvocationURL != "" {
		labels["invocation-url"] = status.ssmInvocationURL
	}

	fmt.Println("LABELS", labels)

	// Create notifications
	_, err := s.AccessPoint.CreateGlobalNotification(s.ctx, &notificationsv1.GlobalNotification{
		Spec: &notificationsv1.GlobalNotificationSpec{
			Matcher: &notificationsv1.GlobalNotificationSpec_ByPermissions{
				ByPermissions: &notificationsv1.ByPermissions{
					RoleConditions: []*types.RoleConditions{{Rules: []types.Rule{{
						Resources: []string{types.KindIntegration},
						Verbs:     []string{types.VerbList},
					}}}},
				},
			},
			Notification: &notificationsv1.Notification{
				Spec: &notificationsv1.NotificationSpec{
					Created: timestamppb.New(s.clock.Now()),
				},
				SubKind: types.NotificationAWSOIDCAutoDiscoverEC2FailedSubKind,
				Metadata: &headerv1.Metadata{
					Labels:  labels,
					Expires: timestamppb.New(time.Now().Add(s.PollInterval)),
				},
			},
		},
	})
	if err != nil {
		s.Log.WithError(err).WithField("instance_id", status.instanceID).Info("Error notifying user about EC2 SSM installation failure.")
	}

	// Create notifications
	_, err = s.AccessPoint.CreateGlobalNotification(s.ctx, &notificationsv1.GlobalNotification{
		Spec: &notificationsv1.GlobalNotificationSpec{
			Matcher: &notificationsv1.GlobalNotificationSpec_ByPermissions{
				ByPermissions: &notificationsv1.ByPermissions{
					RoleConditions: []*types.RoleConditions{{Rules: []types.Rule{{
						Resources: []string{types.KindIntegration},
						Verbs:     []string{types.VerbList},
					}}}},
				},
			},
			Notification: &notificationsv1.Notification{
				Spec: &notificationsv1.NotificationSpec{
					Created: timestamppb.New(s.clock.Now()),
				},
				SubKind: types.NotificationAWSOIDCIntegrationHasTasksSubKind,
				Metadata: &headerv1.Metadata{
					Labels: map[string]string{
						types.NotificationTitleLabel: "Integration requires attention",
						"integration":                k.integration,
					},
					Expires: timestamppb.New(time.Now().Add(s.PollInterval)),
				},
			},
		},
	})
	if err != nil {
		s.Log.WithError(err).WithField("instance_id", status.instanceID).Info("Error notifying user about EC2 SSM installation failure.")
	}
}
