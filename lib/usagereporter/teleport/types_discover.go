/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package usagereporter

import (
	"github.com/gravitational/trace"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	prehogv1 "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/utils"
)

func discoverMetadataToPrehog(u *usageeventsv1.DiscoverMetadata, identityUsername string) *prehogv1.DiscoverMetadata {
	return &prehogv1.DiscoverMetadata{
		Id:       u.Id,
		UserName: identityUsername,
	}
}

func validateDiscoverMetadata(u *prehogv1.DiscoverMetadata) error {
	if u == nil {
		return trace.BadParameter("metadata is required")
	}

	if len(u.Id) == 0 {
		return trace.BadParameter("metadata.id is required")
	}

	return nil
}

func discoverResourceToPrehog(u *usageeventsv1.DiscoverResourceMetadata) *prehogv1.DiscoverResourceMetadata {
	return &prehogv1.DiscoverResourceMetadata{
		Resource: prehogv1.DiscoverResource(u.Resource),
	}
}

func validateDiscoverResourceMetadata(u *prehogv1.DiscoverResourceMetadata) error {
	if u == nil {
		return trace.BadParameter("resource is required")
	}

	if u.Resource == prehogv1.DiscoverResource_DISCOVER_RESOURCE_UNSPECIFIED {
		return trace.BadParameter("invalid resource")
	}

	return nil
}

func discoverStatusToPrehog(u *usageeventsv1.DiscoverStepStatus) *prehogv1.DiscoverStepStatus {
	return &prehogv1.DiscoverStepStatus{
		Status: prehogv1.DiscoverStatus(u.Status),
		Error:  u.Error,
	}
}

func validateDiscoverStatus(u *prehogv1.DiscoverStepStatus) error {
	if u == nil {
		return trace.BadParameter("status is required")
	}

	if u.Status == prehogv1.DiscoverStatus_DISCOVER_STATUS_UNSPECIFIED {
		return trace.BadParameter("invalid status.status")
	}

	if u.Status == prehogv1.DiscoverStatus_DISCOVER_STATUS_ERROR && len(u.Error) == 0 {
		return trace.BadParameter("status.error is required when status.status is ERROR")
	}

	return nil
}

func validateDiscoverBaseEventFields(md *prehogv1.DiscoverMetadata, res *prehogv1.DiscoverResourceMetadata, st *prehogv1.DiscoverStepStatus) error {
	if err := validateDiscoverMetadata(md); err != nil {
		return trace.Wrap(err)
	}

	if err := validateDiscoverResourceMetadata(res); err != nil {
		return trace.Wrap(err)
	}

	if err := validateDiscoverStatus(st); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// UIDiscoverStartedEvent is a UI event sent when a user starts the Discover Wizard.
type UIDiscoverStartedEvent prehogv1.UIDiscoverStartedEvent

func (u *UIDiscoverStartedEvent) CheckAndSetDefaults() error {
	if err := validateDiscoverMetadata(u.Metadata); err != nil {
		return trace.Wrap(err)
	}
	if err := validateDiscoverStatus(u.Status); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (u *UIDiscoverStartedEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiDiscoverStartedEvent{
			UiDiscoverStartedEvent: &prehogv1.UIDiscoverStartedEvent{
				Metadata: &prehogv1.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Status: u.Status,
			},
		},
	}
}

// UIDiscoverResourceSelectionEvent is a UI event sent when a user starts the Discover Wizard.
type UIDiscoverResourceSelectionEvent prehogv1.UIDiscoverResourceSelectionEvent

func (u *UIDiscoverResourceSelectionEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverResourceSelectionEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiDiscoverResourceSelectionEvent{
			UiDiscoverResourceSelectionEvent: &prehogv1.UIDiscoverResourceSelectionEvent{
				Metadata: &prehogv1.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource: u.Resource,
				Status:   u.Status,
			},
		},
	}
}

// UIDiscoverDeployServiceEvent is emitted after the user installs a Teleport Agent.
// For SSH this is the Teleport 'install-node' script.
//
// For Kubernetes this is the teleport-agent helm chart installation.
//
// For Database Access this step is the installation of the teleport 'install-db' script.
// It can be skipped if the cluster already has a Database Service capable of proxying the database.
type UIDiscoverDeployServiceEvent prehogv1.UIDiscoverDeployServiceEvent

func (u *UIDiscoverDeployServiceEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverDeployServiceEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiDiscoverDeployServiceEvent{
			UiDiscoverDeployServiceEvent: &prehogv1.UIDiscoverDeployServiceEvent{
				Metadata: &prehogv1.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource: u.Resource,
				Status:   u.Status,
			},
		},
	}
}

// UIDiscoverDatabaseRegisterEvent is emitted when a user registers a database resource
// and goes to the next step.
type UIDiscoverDatabaseRegisterEvent prehogv1.UIDiscoverDatabaseRegisterEvent

func (u *UIDiscoverDatabaseRegisterEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverDatabaseRegisterEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiDiscoverDatabaseRegisterEvent{
			UiDiscoverDatabaseRegisterEvent: &prehogv1.UIDiscoverDatabaseRegisterEvent{
				Metadata: &prehogv1.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource: u.Resource,
				Status:   u.Status,
			},
		},
	}
}

// UIDiscoverDatabaseConfigureMTLSEvent is emitted when a user configures mutual TLS for self-hosted database
// and goes to the next step.
type UIDiscoverDatabaseConfigureMTLSEvent prehogv1.UIDiscoverDatabaseConfigureMTLSEvent

func (u *UIDiscoverDatabaseConfigureMTLSEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverDatabaseConfigureMTLSEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiDiscoverDatabaseConfigureMtlsEvent{
			UiDiscoverDatabaseConfigureMtlsEvent: &prehogv1.UIDiscoverDatabaseConfigureMTLSEvent{
				Metadata: &prehogv1.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource: u.Resource,
				Status:   u.Status,
			},
		},
	}
}

// UIDiscoverDesktopActiveDirectoryToolsInstallEvent is emitted when the user is asked to run the install Active Directory tools script.
// This happens on the Desktop flow.
type UIDiscoverDesktopActiveDirectoryToolsInstallEvent prehogv1.UIDiscoverDesktopActiveDirectoryToolsInstallEvent

func (u *UIDiscoverDesktopActiveDirectoryToolsInstallEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverDesktopActiveDirectoryToolsInstallEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiDiscoverDesktopActiveDirectoryToolsInstallEvent{
			UiDiscoverDesktopActiveDirectoryToolsInstallEvent: &prehogv1.UIDiscoverDesktopActiveDirectoryToolsInstallEvent{
				Metadata: &prehogv1.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource: u.Resource,
				Status:   u.Status,
			},
		},
	}
}

// UIDiscoverDesktopActiveDirectoryConfigureEvent is emitted when the user is asked to run the Configure Active Directory script.
// This happens on the Desktop flow.
type UIDiscoverDesktopActiveDirectoryConfigureEvent prehogv1.UIDiscoverDesktopActiveDirectoryConfigureEvent

func (u *UIDiscoverDesktopActiveDirectoryConfigureEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverDesktopActiveDirectoryConfigureEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiDiscoverDesktopActiveDirectoryConfigureEvent{
			UiDiscoverDesktopActiveDirectoryConfigureEvent: &prehogv1.UIDiscoverDesktopActiveDirectoryConfigureEvent{
				Metadata: &prehogv1.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource: u.Resource,
				Status:   u.Status,
			},
		},
	}
}

// UIDiscoverAutoDiscoveredResourcesEvent is emitted when the user is presented with the list of auto discovered resources.
// resources_count field must contain the number of discovered resources when the user leaves the screen.
type UIDiscoverAutoDiscoveredResourcesEvent prehogv1.UIDiscoverAutoDiscoveredResourcesEvent

func (u *UIDiscoverAutoDiscoveredResourcesEvent) CheckAndSetDefaults() error {
	if u.ResourcesCount < 0 {
		return trace.BadParameter("resources count must be 0 or more")
	}
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverAutoDiscoveredResourcesEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiDiscoverAutoDiscoveredResourcesEvent{
			UiDiscoverAutoDiscoveredResourcesEvent: &prehogv1.UIDiscoverAutoDiscoveredResourcesEvent{
				Metadata: &prehogv1.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource:       u.Resource,
				Status:         u.Status,
				ResourcesCount: u.ResourcesCount,
			},
		},
	}
}

// UIDiscoverDatabaseConfigureIAMPolicyEvent is emitted when a user configured IAM for RDS database
// and proceeded to the next step.
type UIDiscoverDatabaseConfigureIAMPolicyEvent prehogv1.UIDiscoverDatabaseConfigureIAMPolicyEvent

func (u *UIDiscoverDatabaseConfigureIAMPolicyEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverDatabaseConfigureIAMPolicyEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiDiscoverDatabaseConfigureIamPolicyEvent{
			UiDiscoverDatabaseConfigureIamPolicyEvent: &prehogv1.UIDiscoverDatabaseConfigureIAMPolicyEvent{
				Metadata: &prehogv1.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource: u.Resource,
				Status:   u.Status,
			},
		},
	}
}

// UIDiscoverPrincipalsConfigureEvent emitted on "Setup Access" screen when user has updated their principals
// and proceeded to the next step.
type UIDiscoverPrincipalsConfigureEvent prehogv1.UIDiscoverPrincipalsConfigureEvent

func (u *UIDiscoverPrincipalsConfigureEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverPrincipalsConfigureEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiDiscoverPrincipalsConfigureEvent{
			UiDiscoverPrincipalsConfigureEvent: &prehogv1.UIDiscoverPrincipalsConfigureEvent{
				Metadata: &prehogv1.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource: u.Resource,
				Status:   u.Status,
			},
		},
	}
}

// UIDiscoverTestConnectionEvent emitted on the "Test Connection" screen
// when the user clicked tested connection to their resource.
type UIDiscoverTestConnectionEvent prehogv1.UIDiscoverTestConnectionEvent

func (u *UIDiscoverTestConnectionEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverTestConnectionEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiDiscoverTestConnectionEvent{
			UiDiscoverTestConnectionEvent: &prehogv1.UIDiscoverTestConnectionEvent{
				Metadata: &prehogv1.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource: u.Resource,
				Status:   u.Status,
			},
		},
	}
}

// UIDiscoverCompletedEvent is emitted when user completes the Discover wizard.
type UIDiscoverCompletedEvent prehogv1.UIDiscoverCompletedEvent

func (u *UIDiscoverCompletedEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverCompletedEvent) Anonymize(a utils.Anonymizer) prehogv1.SubmitEventRequest {
	return prehogv1.SubmitEventRequest{
		Event: &prehogv1.SubmitEventRequest_UiDiscoverCompletedEvent{
			UiDiscoverCompletedEvent: &prehogv1.UIDiscoverCompletedEvent{
				Metadata: &prehogv1.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource: u.Resource,
				Status:   u.Status,
			},
		},
	}
}
