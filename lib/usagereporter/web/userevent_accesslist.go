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
)

// AccessListEventData contains the required properties to create a AccessList UsageEvent.
type AccessListEventData struct {
	// ID is a unique ID per wizard session
	ID string `json:"id,omitempty"`

	// Resource is the resource type that the user selected.
	// Its possible values are the usageevents.DiscoverResource proto enum values.
	// Example: "PRESET_LONG_TERM"
	Preset string `json:"preset,omitempty"`

	// PreferredTerraform is true if user preferred using terraforms script
	// over creating an access list via the web UI.
	PreferredTerraform bool `json:"preferredTerraform,omitempty"`

	// Resource is the resource type that the user selected.
	// Its possible values are the usageevents.DiscoverResource proto enum values.
	// Example: "INTEGRATE_OKTA"
	Integrate string `json:"integrate,omitempty"`

	// StepStatus is the Wizard step status result.
	// Its possible values are the usageevents.DiscoverStepStatus proto enum values.
	// Example: "DISCOVER_STATUS_SUCCESS"
	StepStatus string `json:"stepStatus,omitempty"`
	// StepStatusError is the error of the Step, when the StepStatus is `DISCOVER_STATUS_ERROR`.
	StepStatusError string `json:"stepStatusError,omitempty"`
}

// ToUsageEvent converts a access list ui events into a v1.UsageEventOneOf.
// This is mostly copying data around, except for Enum properties.
// Enum props are converted from its string representation to its int32 values.
func (d *AccessListEventData) ToUsageEvent(eventName string) (*usageeventsv1.UsageEventOneOf, error) {
	statusEnum, ok := usageeventsv1.AccessListStatus_value[d.StepStatus]
	if !ok {
		return nil, trace.BadParameter("invalid access list stepStatus %s", d.StepStatus)
	}
	status := &usageeventsv1.AccessListStepStatus{
		Status: usageeventsv1.AccessListStatus(statusEnum),
		Error:  d.StepStatusError,
	}

	// Convert preset enum string to int
	presetEnum := int32(0)
	if len(d.Preset) > 0 {
		presetEnum, ok = usageeventsv1.AccessListPreset_value[d.Preset]
		if !ok {
			return nil, trace.BadParameter("invalid access list preset %s", d.Preset)
		}
	}
	preset := usageeventsv1.AccessListPreset(presetEnum)

	// Convert integrate enum string to int
	integrateEnum := int32(0)
	if len(d.Integrate) > 0 {
		integrateEnum, ok = usageeventsv1.AccessListIntegrate_value[d.Integrate]
		if !ok {
			return nil, trace.BadParameter("invalid access list integrate %s", d.Integrate)
		}
	}
	integrate := usageeventsv1.AccessListIntegrate(integrateEnum)

	metadata := &usageeventsv1.AccessListMetadata{
		Id:     d.ID,
		Preset: preset,
	}

	switch eventName {
	case uiAccessListCompleteEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListCompleteEvent{
			UiAccessListCompleteEvent: &usageeventsv1.UIAccessListCompleteEvent{
				Metadata:           metadata,
				Status:             status,
				PreferredTerraform: d.PreferredTerraform,
			},
		}}, nil

	case uiAccessListDefineAccessEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListDefineAccessEvent{
			UiAccessListDefineAccessEvent: &usageeventsv1.UIAccessListDefineAccessEvent{
				Metadata: metadata,
				Status:   status,
			},
		}}, nil

	case uiAccessListDefineBasicInfoEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListDefineBasicInfoEvent{
			UiAccessListDefineBasicInfoEvent: &usageeventsv1.UIAccessListDefineBasicInfoEvent{
				Metadata: metadata,
				Status:   status,
			},
		}}, nil

	case uiAccessListDefineIdentitiesEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListDefineIdentitiesEvent{
			UiAccessListDefineIdentitiesEvent: &usageeventsv1.UIAccessListDefineIdentitiesEvent{
				Metadata: metadata,
				Status:   status,
			},
		}}, nil

	case uiAccessListDefineMembersEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListDefineMembersEvent{
			UiAccessListDefineMembersEvent: &usageeventsv1.UIAccessListDefineMembersEvent{
				Metadata: metadata,
				Status:   status,
			},
		}}, nil

	case uiAccessListDefineOwnersEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListDefineOwnersEvent{
			UiAccessListDefineOwnersEvent: &usageeventsv1.UIAccessListDefineOwnersEvent{
				Metadata: metadata,
				Status:   status,
			},
		}}, nil

	case uiAccessListIntegrateEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListIntegrateEvent{
			UiAccessListIntegrateEvent: &usageeventsv1.UIAccessListIntegrateEvent{
				Metadata:  metadata,
				Integrate: integrate,
			},
		}}, nil

	case uiAccessListStartEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListStartEvent{
			UiAccessListStartEvent: &usageeventsv1.UIAccessListStartEvent{
				Metadata: metadata,
				Status:   status,
			},
		}}, nil

	case uiAccessListCustomEvent:
		return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListCustomEvent{
			UiAccessListCustomEvent: &usageeventsv1.UIAccessListCustomEvent{
				Metadata: metadata,
				Status:   status,
			},
		}}, nil
	}

	return nil, trace.BadParameter("invalid event name %q", eventName)
}
