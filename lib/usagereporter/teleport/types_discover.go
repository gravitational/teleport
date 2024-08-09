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

package usagereporter

import (
	"github.com/gravitational/trace"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/utils"
)

func discoverMetadataToPrehog(u *usageeventsv1.DiscoverMetadata, userMD UserMetadata) *prehogv1a.DiscoverMetadata {
	return &prehogv1a.DiscoverMetadata{
		Id:       u.Id,
		UserName: userMD.Username,
		Sso:      userMD.IsSSO,
	}
}

func validateDiscoverMetadata(u *prehogv1a.DiscoverMetadata) error {
	if u == nil {
		return trace.BadParameter("metadata is required")
	}

	if len(u.Id) == 0 {
		return trace.BadParameter("metadata.id is required")
	}

	return nil
}

func discoverResourceToPrehog(u *usageeventsv1.DiscoverResourceMetadata) *prehogv1a.DiscoverResourceMetadata {
	return &prehogv1a.DiscoverResourceMetadata{
		Resource: prehogv1a.DiscoverResource(u.Resource),
	}
}

func validateDiscoverResourceMetadata(u *prehogv1a.DiscoverResourceMetadata) error {
	if u == nil {
		return trace.BadParameter("resource is required")
	}

	if u.Resource == prehogv1a.DiscoverResource_DISCOVER_RESOURCE_UNSPECIFIED {
		return trace.BadParameter("invalid resource")
	}

	return nil
}

func discoverStatusToPrehog(u *usageeventsv1.DiscoverStepStatus) *prehogv1a.DiscoverStepStatus {
	return &prehogv1a.DiscoverStepStatus{
		Status: prehogv1a.DiscoverStatus(u.Status),
		Error:  u.Error,
	}
}

func validateDiscoverStatus(u *prehogv1a.DiscoverStepStatus) error {
	if u == nil {
		return trace.BadParameter("status is required")
	}

	if u.Status == prehogv1a.DiscoverStatus_DISCOVER_STATUS_UNSPECIFIED {
		return trace.BadParameter("invalid status.status")
	}

	if u.Status == prehogv1a.DiscoverStatus_DISCOVER_STATUS_ERROR && len(u.Error) == 0 {
		return trace.BadParameter("status.error is required when status.status is ERROR")
	}

	return nil
}

func validateDiscoverBaseEventFields(md *prehogv1a.DiscoverMetadata, res *prehogv1a.DiscoverResourceMetadata, st *prehogv1a.DiscoverStepStatus) error {
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
type UIDiscoverStartedEvent prehogv1a.UIDiscoverStartedEvent

func (u *UIDiscoverStartedEvent) CheckAndSetDefaults() error {
	if err := validateDiscoverMetadata(u.Metadata); err != nil {
		return trace.Wrap(err)
	}
	if err := validateDiscoverStatus(u.Status); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (u *UIDiscoverStartedEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiDiscoverStartedEvent{
			UiDiscoverStartedEvent: &prehogv1a.UIDiscoverStartedEvent{
				Metadata: &prehogv1a.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
					Sso:      u.Metadata.Sso,
				},
				Status: u.Status,
			},
		},
	}
}

// UIDiscoverResourceSelectionEvent is a UI event sent when a user starts the Discover Wizard.
type UIDiscoverResourceSelectionEvent prehogv1a.UIDiscoverResourceSelectionEvent

func (u *UIDiscoverResourceSelectionEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverResourceSelectionEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiDiscoverResourceSelectionEvent{
			UiDiscoverResourceSelectionEvent: &prehogv1a.UIDiscoverResourceSelectionEvent{
				Metadata: &prehogv1a.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource: u.Resource,
				Status:   u.Status,
			},
		},
	}
}

// UIDiscoverIntegrationAWSOIDCConnectEvent is emitted when a user is finished with the step
// that asks user to setup aws integration or select from a list of existing
// aws integrations.
type UIDiscoverIntegrationAWSOIDCConnectEvent prehogv1a.UIDiscoverIntegrationAWSOIDCConnectEvent

func (u *UIDiscoverIntegrationAWSOIDCConnectEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverIntegrationAWSOIDCConnectEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiDiscoverIntegrationAwsOidcConnectEvent{
			UiDiscoverIntegrationAwsOidcConnectEvent: &prehogv1a.UIDiscoverIntegrationAWSOIDCConnectEvent{
				Metadata: &prehogv1a.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource: u.Resource,
				Status:   u.Status,
			},
		},
	}
}

// UIDiscoverDatabaseRDSEnrollEvent is emitted when a user is finished with
// the step that asks user to select from a list of RDS databases.
type UIDiscoverDatabaseRDSEnrollEvent prehogv1a.UIDiscoverDatabaseRDSEnrollEvent

func (u *UIDiscoverDatabaseRDSEnrollEvent) CheckAndSetDefaults() error {
	if u.SelectedResourcesCount < 0 {
		return trace.BadParameter("selected resources count must be 0 or more")
	}
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverDatabaseRDSEnrollEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiDiscoverDatabaseRdsEnrollEvent{
			UiDiscoverDatabaseRdsEnrollEvent: &prehogv1a.UIDiscoverDatabaseRDSEnrollEvent{
				Metadata: &prehogv1a.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource:               u.Resource,
				Status:                 u.Status,
				SelectedResourcesCount: u.SelectedResourcesCount,
			},
		},
	}
}

// UIDiscoverKubeEKSEnrollEvent is emitted when a user is finished with
// the step that asks user to select from a list of EKS clusters.
type UIDiscoverKubeEKSEnrollEvent prehogv1a.UIDiscoverKubeEKSEnrollEvent

func (u *UIDiscoverKubeEKSEnrollEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverKubeEKSEnrollEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiDiscoverKubeEksEnrollEvent{
			UiDiscoverKubeEksEnrollEvent: &prehogv1a.UIDiscoverKubeEKSEnrollEvent{
				Metadata: &prehogv1a.DiscoverMetadata{
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
type UIDiscoverDeployServiceEvent prehogv1a.UIDiscoverDeployServiceEvent

func (u *UIDiscoverDeployServiceEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverDeployServiceEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiDiscoverDeployServiceEvent{
			UiDiscoverDeployServiceEvent: &prehogv1a.UIDiscoverDeployServiceEvent{
				Metadata: &prehogv1a.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource:     u.Resource,
				Status:       u.Status,
				DeployMethod: u.DeployMethod,
				DeployType:   u.DeployType,
			},
		},
	}
}

// UIDiscoverCreateDiscoveryConfigEvent is emitted after a discovery config was created.
type UIDiscoverCreateDiscoveryConfigEvent prehogv1a.UIDiscoverCreateDiscoveryConfigEvent

func (u *UIDiscoverCreateDiscoveryConfigEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverCreateDiscoveryConfigEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiDiscoverCreateDiscoveryConfig{
			UiDiscoverCreateDiscoveryConfig: &prehogv1a.UIDiscoverCreateDiscoveryConfigEvent{
				Metadata: &prehogv1a.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource:     u.Resource,
				Status:       u.Status,
				ConfigMethod: u.ConfigMethod,
			},
		},
	}
}

// UIDiscoverDatabaseRegisterEvent is emitted when a user registers a database resource
// and goes to the next step.
type UIDiscoverDatabaseRegisterEvent prehogv1a.UIDiscoverDatabaseRegisterEvent

func (u *UIDiscoverDatabaseRegisterEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverDatabaseRegisterEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiDiscoverDatabaseRegisterEvent{
			UiDiscoverDatabaseRegisterEvent: &prehogv1a.UIDiscoverDatabaseRegisterEvent{
				Metadata: &prehogv1a.DiscoverMetadata{
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
type UIDiscoverDatabaseConfigureMTLSEvent prehogv1a.UIDiscoverDatabaseConfigureMTLSEvent

func (u *UIDiscoverDatabaseConfigureMTLSEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverDatabaseConfigureMTLSEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiDiscoverDatabaseConfigureMtlsEvent{
			UiDiscoverDatabaseConfigureMtlsEvent: &prehogv1a.UIDiscoverDatabaseConfigureMTLSEvent{
				Metadata: &prehogv1a.DiscoverMetadata{
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
type UIDiscoverDesktopActiveDirectoryToolsInstallEvent prehogv1a.UIDiscoverDesktopActiveDirectoryToolsInstallEvent

func (u *UIDiscoverDesktopActiveDirectoryToolsInstallEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverDesktopActiveDirectoryToolsInstallEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiDiscoverDesktopActiveDirectoryToolsInstallEvent{
			UiDiscoverDesktopActiveDirectoryToolsInstallEvent: &prehogv1a.UIDiscoverDesktopActiveDirectoryToolsInstallEvent{
				Metadata: &prehogv1a.DiscoverMetadata{
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
type UIDiscoverDesktopActiveDirectoryConfigureEvent prehogv1a.UIDiscoverDesktopActiveDirectoryConfigureEvent

func (u *UIDiscoverDesktopActiveDirectoryConfigureEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverDesktopActiveDirectoryConfigureEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiDiscoverDesktopActiveDirectoryConfigureEvent{
			UiDiscoverDesktopActiveDirectoryConfigureEvent: &prehogv1a.UIDiscoverDesktopActiveDirectoryConfigureEvent{
				Metadata: &prehogv1a.DiscoverMetadata{
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
type UIDiscoverAutoDiscoveredResourcesEvent prehogv1a.UIDiscoverAutoDiscoveredResourcesEvent

func (u *UIDiscoverAutoDiscoveredResourcesEvent) CheckAndSetDefaults() error {
	if u.ResourcesCount < 0 {
		return trace.BadParameter("resources count must be 0 or more")
	}
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverAutoDiscoveredResourcesEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiDiscoverAutoDiscoveredResourcesEvent{
			UiDiscoverAutoDiscoveredResourcesEvent: &prehogv1a.UIDiscoverAutoDiscoveredResourcesEvent{
				Metadata: &prehogv1a.DiscoverMetadata{
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

// UIDiscoverEC2InstanceSelectionEvent is emitted when the user is finished with the step that asks the user to select an EC2 Instance to enroll.
type UIDiscoverEC2InstanceSelectionEvent prehogv1a.UIDiscoverEC2InstanceSelectionEvent

func (u *UIDiscoverEC2InstanceSelectionEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverEC2InstanceSelectionEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiDiscoverEc2InstanceSelection{
			UiDiscoverEc2InstanceSelection: &prehogv1a.UIDiscoverEC2InstanceSelectionEvent{
				Metadata: &prehogv1a.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource: u.Resource,
				Status:   u.Status,
			},
		},
	}
}

// UIDiscoverDeployEICEEvent is emitted when the user deploys an EC2 Instance Connect Endpoint.
type UIDiscoverDeployEICEEvent prehogv1a.UIDiscoverDeployEICEEvent

func (u *UIDiscoverDeployEICEEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverDeployEICEEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiDiscoverDeployEice{
			UiDiscoverDeployEice: &prehogv1a.UIDiscoverDeployEICEEvent{
				Metadata: &prehogv1a.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource: u.Resource,
				Status:   u.Status,
			},
		},
	}
}

// UIDiscoverCreateNodeEvent is emitted when the node is created in Teleport.
type UIDiscoverCreateNodeEvent prehogv1a.UIDiscoverCreateNodeEvent

func (u *UIDiscoverCreateNodeEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverCreateNodeEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiDiscoverCreateNode{
			UiDiscoverCreateNode: &prehogv1a.UIDiscoverCreateNodeEvent{
				Metadata: &prehogv1a.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource: u.Resource,
				Status:   u.Status,
			},
		},
	}
}

// UIDiscoverCreateAppServerEvent is emitted when the node is created in Teleport.
type UIDiscoverCreateAppServerEvent prehogv1a.UIDiscoverCreateAppServerEvent

func (u *UIDiscoverCreateAppServerEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverCreateAppServerEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiDiscoverCreateAppServerEvent{
			UiDiscoverCreateAppServerEvent: &prehogv1a.UIDiscoverCreateAppServerEvent{
				Metadata: &prehogv1a.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource: u.Resource,
				Status:   u.Status,
			},
		},
	}
}

// UIDiscoverDatabaseConfigureIAMPolicyEvent is emitted when a user configured IAM for RDS database
// and proceeded to the next step.
type UIDiscoverDatabaseConfigureIAMPolicyEvent prehogv1a.UIDiscoverDatabaseConfigureIAMPolicyEvent

func (u *UIDiscoverDatabaseConfigureIAMPolicyEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverDatabaseConfigureIAMPolicyEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiDiscoverDatabaseConfigureIamPolicyEvent{
			UiDiscoverDatabaseConfigureIamPolicyEvent: &prehogv1a.UIDiscoverDatabaseConfigureIAMPolicyEvent{
				Metadata: &prehogv1a.DiscoverMetadata{
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
type UIDiscoverPrincipalsConfigureEvent prehogv1a.UIDiscoverPrincipalsConfigureEvent

func (u *UIDiscoverPrincipalsConfigureEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverPrincipalsConfigureEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiDiscoverPrincipalsConfigureEvent{
			UiDiscoverPrincipalsConfigureEvent: &prehogv1a.UIDiscoverPrincipalsConfigureEvent{
				Metadata: &prehogv1a.DiscoverMetadata{
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
type UIDiscoverTestConnectionEvent prehogv1a.UIDiscoverTestConnectionEvent

func (u *UIDiscoverTestConnectionEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverTestConnectionEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiDiscoverTestConnectionEvent{
			UiDiscoverTestConnectionEvent: &prehogv1a.UIDiscoverTestConnectionEvent{
				Metadata: &prehogv1a.DiscoverMetadata{
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
type UIDiscoverCompletedEvent prehogv1a.UIDiscoverCompletedEvent

func (u *UIDiscoverCompletedEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateDiscoverBaseEventFields(u.Metadata, u.Resource, u.Status))
}

func (u *UIDiscoverCompletedEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiDiscoverCompletedEvent{
			UiDiscoverCompletedEvent: &prehogv1a.UIDiscoverCompletedEvent{
				Metadata: &prehogv1a.DiscoverMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Resource: u.Resource,
				Status:   u.Status,
			},
		},
	}
}
