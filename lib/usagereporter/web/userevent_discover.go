// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package usagereporter

import (
	"github.com/gravitational/trace"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
)

// DiscoverEventData contains the required properties to create a Discover UsageEvent.
type DiscoverEventData struct {
	// ID is a unique ID per wizard session
	ID string `json:"id,omitempty"`

	// Resource is the resource type that the user selected.
	// Its possible values are the usageevents.DiscoverResource proto enum values.
	// Example: "DISCOVER_RESOURCE_SERVER"
	Resource string `json:"resource,omitempty"`

	// AutoDiscoverResourcesCount is the number of auto-discovered resources in the Auto Discovering resources screen.
	// This value is only considered for the 'tp.ui.discover.autoDiscoveredResources'.
	AutoDiscoverResourcesCount int `json:"autoDiscoverResourcesCount,omitempty"`

	// SelectedResourcesCount is the number of resources that a user has selected
	// eg: number of RDS databases selected in the RDS enrollment screen for the
	// event tp.ui.discover.database.enroll.rds
	SelectedResourcesCount int `json:"selectedResourcesCount,omitempty"`

	// StepStatus is the Wizard step status result.
	// Its possible values are the usageevents.DiscoverStepStatus proto enum values.
	// Example: "DISCOVER_STATUS_SUCCESS"
	StepStatus string `json:"stepStatus,omitempty"`
	// StepStatusError is the error of the Step, when the StepStatus is `DISCOVER_STATUS_ERROR`.
	StepStatusError string `json:"stepStatusError,omitempty"`
}

// ToUsageEvent converts a discoverEventData into a v1.UsageEventOneOf.
// This is mostly copying data around, except for Enum properties.
// Enum props are converted from its string representation to its int32 values.
func (d *DiscoverEventData) ToUsageEvent(eventName string) (*usageeventsv1.UsageEventOneOf, error) {
	metadata := &usageeventsv1.DiscoverMetadata{
		Id: d.ID,
	}

	statusEnum, ok := usageeventsv1.DiscoverStatus_value[d.StepStatus]
	if !ok {
		return nil, trace.BadParameter("invalid stepStatus %s", d.StepStatus)
	}
	status := &usageeventsv1.DiscoverStepStatus{
		Status: usageeventsv1.DiscoverStatus(statusEnum),
		Error:  d.StepStatusError,
	}

	var resource *usageeventsv1.DiscoverResourceMetadata
	// The uiDiscoverStartedEvent does not have a resource selected yet.
	// This event is emitted when the user lands on the first screen of the Discover Wizard.
	if eventName != uiDiscoverStartedEvent {
		resourceEnum, ok := usageeventsv1.DiscoverResource_value[d.Resource]
		if !ok {
			return nil, trace.BadParameter("invalid resource %s", d.Resource)
		}

		resource = &usageeventsv1.DiscoverResourceMetadata{
			Resource: usageeventsv1.DiscoverResource(resourceEnum),
		}
	}

	switch eventName {
	case uiDiscoverStartedEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverStartedEvent{
			UiDiscoverStartedEvent: &usageeventsv1.UIDiscoverStartedEvent{
				Metadata: metadata,
				Status:   status,
			},
		}}, nil

	case uiDiscoverResourceSelectionEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverResourceSelectionEvent{
			UiDiscoverResourceSelectionEvent: &usageeventsv1.UIDiscoverResourceSelectionEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	case uiDiscoverIntegrationAWSOIDCConnectEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverIntegrationAwsOidcConnectEvent{
			UiDiscoverIntegrationAwsOidcConnectEvent: &usageeventsv1.UIDiscoverIntegrationAWSOIDCConnectEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	case uiDiscoverDatabaseRDSEnrollEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverDatabaseRdsEnrollEvent{
			UiDiscoverDatabaseRdsEnrollEvent: &usageeventsv1.UIDiscoverDatabaseRDSEnrollEvent{
				Metadata:               metadata,
				Resource:               resource,
				Status:                 status,
				SelectedResourcesCount: int64(d.SelectedResourcesCount),
			},
		}}, nil

	case uiDiscoverDeployServiceEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverDeployServiceEvent{
			UiDiscoverDeployServiceEvent: &usageeventsv1.UIDiscoverDeployServiceEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	case uiDiscoverDatabaseRegisterEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverDatabaseRegisterEvent{
			UiDiscoverDatabaseRegisterEvent: &usageeventsv1.UIDiscoverDatabaseRegisterEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	case uiDiscoverDatabaseConfigureMTLSEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverDatabaseConfigureMtlsEvent{
			UiDiscoverDatabaseConfigureMtlsEvent: &usageeventsv1.UIDiscoverDatabaseConfigureMTLSEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	case uiDiscoverDesktopActiveDirectoryToolsInstallEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverDesktopActiveDirectoryToolsInstallEvent{
			UiDiscoverDesktopActiveDirectoryToolsInstallEvent: &usageeventsv1.UIDiscoverDesktopActiveDirectoryToolsInstallEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	case uiDiscoverDesktopActiveDirectoryConfigureEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverDesktopActiveDirectoryConfigureEvent{
			UiDiscoverDesktopActiveDirectoryConfigureEvent: &usageeventsv1.UIDiscoverDesktopActiveDirectoryConfigureEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	case uiDiscoverAutoDiscoveredResourcesEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverAutoDiscoveredResourcesEvent{
			UiDiscoverAutoDiscoveredResourcesEvent: &usageeventsv1.UIDiscoverAutoDiscoveredResourcesEvent{
				Metadata:       metadata,
				Resource:       resource,
				Status:         status,
				ResourcesCount: int64(d.AutoDiscoverResourcesCount),
			},
		}}, nil

	case uiDiscoverDatabaseConfigureIAMPolicyEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverDatabaseConfigureIamPolicyEvent{
			UiDiscoverDatabaseConfigureIamPolicyEvent: &usageeventsv1.UIDiscoverDatabaseConfigureIAMPolicyEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	case uiDiscoverPrincipalsConfigureEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverPrincipalsConfigureEvent{
			UiDiscoverPrincipalsConfigureEvent: &usageeventsv1.UIDiscoverPrincipalsConfigureEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	case uiDiscoverTestConnectionEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverTestConnectionEvent{
			UiDiscoverTestConnectionEvent: &usageeventsv1.UIDiscoverTestConnectionEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	case uiDiscoverCompletedEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverCompletedEvent{
			UiDiscoverCompletedEvent: &usageeventsv1.UIDiscoverCompletedEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	}

	return nil, trace.BadParameter("invalid event name %q", eventName)
}
