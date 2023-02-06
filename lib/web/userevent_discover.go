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

package web

import (
	"github.com/gravitational/trace"

	v1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
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
func (d *DiscoverEventData) ToUsageEvent(eventName string) (*v1.UsageEventOneOf, error) {
	metadata := &v1.DiscoverMetadata{
		Id: d.ID,
	}

	statusEnum, ok := v1.DiscoverStatus_value[d.StepStatus]
	if !ok {
		return nil, trace.BadParameter("invalid stepStatus %s", d.StepStatus)
	}
	status := &v1.DiscoverStepStatus{
		Status: v1.DiscoverStatus(statusEnum),
		Error:  d.StepStatusError,
	}

	var resource *v1.DiscoverResourceMetadata
	// The uiDiscoverStartedEvent does not have a resource selected yet.
	// This event is emitted when the user lands on the first screen of the Discover Wizard.
	if eventName != uiDiscoverStartedEvent {
		resourceEnum, ok := v1.DiscoverResource_value[d.Resource]
		if !ok {
			return nil, trace.BadParameter("invalid resource %s", d.Resource)
		}

		resource = &v1.DiscoverResourceMetadata{
			Resource: v1.DiscoverResource(resourceEnum),
		}
	}

	switch eventName {
	case uiDiscoverStartedEvent:
		return &v1.UsageEventOneOf{Event: &v1.UsageEventOneOf_UiDiscoverStartedEvent{
			UiDiscoverStartedEvent: &v1.UIDiscoverStartedEvent{
				Metadata: metadata,
				Status:   status,
			},
		}}, nil

	case uiDiscoverResourceSelectionEvent:
		return &v1.UsageEventOneOf{Event: &v1.UsageEventOneOf_UiDiscoverResourceSelectionEvent{
			UiDiscoverResourceSelectionEvent: &v1.UIDiscoverResourceSelectionEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	case uiDiscoverDeployServiceEvent:
		return &v1.UsageEventOneOf{Event: &v1.UsageEventOneOf_UiDiscoverDeployServiceEvent{
			UiDiscoverDeployServiceEvent: &v1.UIDiscoverDeployServiceEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	case uiDiscoverDatabaseRegisterEvent:
		return &v1.UsageEventOneOf{Event: &v1.UsageEventOneOf_UiDiscoverDatabaseRegisterEvent{
			UiDiscoverDatabaseRegisterEvent: &v1.UIDiscoverDatabaseRegisterEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	case uiDiscoverDatabaseConfigureMTLSEvent:
		return &v1.UsageEventOneOf{Event: &v1.UsageEventOneOf_UiDiscoverDatabaseConfigureMtlsEvent{
			UiDiscoverDatabaseConfigureMtlsEvent: &v1.UIDiscoverDatabaseConfigureMTLSEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	case uiDiscoverDesktopActiveDirectoryToolsInstallEvent:
		return &v1.UsageEventOneOf{Event: &v1.UsageEventOneOf_UiDiscoverDesktopActiveDirectoryToolsInstallEvent{
			UiDiscoverDesktopActiveDirectoryToolsInstallEvent: &v1.UIDiscoverDesktopActiveDirectoryToolsInstallEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	case uiDiscoverDesktopActiveDirectoryConfigureEvent:
		return &v1.UsageEventOneOf{Event: &v1.UsageEventOneOf_UiDiscoverDesktopActiveDirectoryConfigureEvent{
			UiDiscoverDesktopActiveDirectoryConfigureEvent: &v1.UIDiscoverDesktopActiveDirectoryConfigureEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	case uiDiscoverAutoDiscoveredResourcesEvent:
		return &v1.UsageEventOneOf{Event: &v1.UsageEventOneOf_UiDiscoverAutoDiscoveredResourcesEvent{
			UiDiscoverAutoDiscoveredResourcesEvent: &v1.UIDiscoverAutoDiscoveredResourcesEvent{
				Metadata:       metadata,
				Resource:       resource,
				Status:         status,
				ResourcesCount: int64(d.AutoDiscoverResourcesCount),
			},
		}}, nil

	case uiDiscoverDatabaseConfigureIAMPolicyEvent:
		return &v1.UsageEventOneOf{Event: &v1.UsageEventOneOf_UiDiscoverDatabaseConfigureIamPolicyEvent{
			UiDiscoverDatabaseConfigureIamPolicyEvent: &v1.UIDiscoverDatabaseConfigureIAMPolicyEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	case uiDiscoverPrincipalsConfigureEvent:
		return &v1.UsageEventOneOf{Event: &v1.UsageEventOneOf_UiDiscoverPrincipalsConfigureEvent{
			UiDiscoverPrincipalsConfigureEvent: &v1.UIDiscoverPrincipalsConfigureEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	case uiDiscoverTestConnectionEvent:
		return &v1.UsageEventOneOf{Event: &v1.UsageEventOneOf_UiDiscoverTestConnectionEvent{
			UiDiscoverTestConnectionEvent: &v1.UIDiscoverTestConnectionEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	case uiDiscoverCompletedEvent:
		return &v1.UsageEventOneOf{Event: &v1.UsageEventOneOf_UiDiscoverCompletedEvent{
			UiDiscoverCompletedEvent: &v1.UIDiscoverCompletedEvent{
				Metadata: metadata,
				Resource: resource,
				Status:   status,
			},
		}}, nil

	}

	return nil, trace.BadParameter("invalid event name %q", eventName)
}
