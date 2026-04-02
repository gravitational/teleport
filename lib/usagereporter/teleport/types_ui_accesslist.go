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

func accessListMetadataToPrehog(u *usageeventsv1.AccessListMetadata, userMD UserMetadata) *prehogv1a.AccessListMetadata {
	if u == nil {
		return nil
	}
	return &prehogv1a.AccessListMetadata{
		Id:       u.Id,
		UserName: userMD.Username,
		Preset:   prehogv1a.AccessListPreset(u.Preset),
	}
}

func validateAccessListMetadata(u *prehogv1a.AccessListMetadata) error {
	if u == nil {
		return trace.BadParameter("metadata is required")
	}

	if len(u.Id) == 0 {
		return trace.BadParameter("metadata.id is required")
	}

	return nil
}

func accessListStatusToPrehog(u *usageeventsv1.AccessListStepStatus) *prehogv1a.AccessListStepStatus {
	if u == nil {
		return nil
	}
	return &prehogv1a.AccessListStepStatus{
		Status: prehogv1a.AccessListStatus(u.Status),
		Error:  u.Error,
	}
}

func accessListIntegrateToPrehog(integrate usageeventsv1.AccessListIntegrate) prehogv1a.AccessListIntegrate {
	return prehogv1a.AccessListIntegrate(integrate)
}

func validateAccessListBaseEventFields(md *prehogv1a.AccessListMetadata, st *prehogv1a.AccessListStepStatus) error {
	if err := validateAccessListMetadata(md); err != nil {
		return trace.Wrap(err)
	}

	if st == nil {
		return trace.BadParameter("status is required")
	}

	return nil
}

type UIAccessListDefineAccessEvent prehogv1a.UIAccessListDefineAccessEvent

func (u *UIAccessListDefineAccessEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateAccessListBaseEventFields(u.Metadata, u.Status))
}

func (u *UIAccessListDefineAccessEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiAccessListDefineAccessEvent{
			UiAccessListDefineAccessEvent: &prehogv1a.UIAccessListDefineAccessEvent{
				Metadata: &prehogv1a.AccessListMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
					Preset:   u.Metadata.Preset,
				},
				Status: u.Status,
			},
		},
	}
}

type UIAccessListDefineIdentitiesEvent prehogv1a.UIAccessListDefineIdentitiesEvent

func (u *UIAccessListDefineIdentitiesEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateAccessListBaseEventFields(u.Metadata, u.Status))
}

func (u *UIAccessListDefineIdentitiesEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiAccessListDefineIdentitiesEvent{
			UiAccessListDefineIdentitiesEvent: &prehogv1a.UIAccessListDefineIdentitiesEvent{
				Metadata: &prehogv1a.AccessListMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
					Preset:   u.Metadata.Preset,
				},
				Status: u.Status,
			},
		},
	}
}

type UIAccessListBasicInfoEvent prehogv1a.UIAccessListDefineBasicInfoEvent

func (u *UIAccessListBasicInfoEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateAccessListBaseEventFields(u.Metadata, u.Status))
}

func (u *UIAccessListBasicInfoEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiAccessListDefineBasicInfoEvent{
			UiAccessListDefineBasicInfoEvent: &prehogv1a.UIAccessListDefineBasicInfoEvent{
				Metadata: &prehogv1a.AccessListMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
					Preset:   u.Metadata.Preset,
				},
				Status: u.Status,
			},
		},
	}
}

type UIAccessListDefineMembersEvent prehogv1a.UIAccessListDefineMembersEvent

func (u *UIAccessListDefineMembersEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateAccessListBaseEventFields(u.Metadata, u.Status))
}

func (u *UIAccessListDefineMembersEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiAccessListDefineMembersEvent{
			UiAccessListDefineMembersEvent: &prehogv1a.UIAccessListDefineMembersEvent{
				Metadata: &prehogv1a.AccessListMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
					Preset:   u.Metadata.Preset,
				},
				Status: u.Status,
			},
		},
	}
}

type UIAccessListDefineOwnersEvent prehogv1a.UIAccessListDefineOwnersEvent

func (u *UIAccessListDefineOwnersEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateAccessListBaseEventFields(u.Metadata, u.Status))
}

func (u *UIAccessListDefineOwnersEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiAccessListDefineOwnersEvent{
			UiAccessListDefineOwnersEvent: &prehogv1a.UIAccessListDefineOwnersEvent{
				Metadata: &prehogv1a.AccessListMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
					Preset:   u.Metadata.Preset,
				},
				Status: u.Status,
			},
		},
	}
}

type UIAccessListStartedEvent prehogv1a.UIAccessListStartEvent

func (u *UIAccessListStartedEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateAccessListBaseEventFields(u.Metadata, u.Status))
}

func (u *UIAccessListStartedEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiAccessListStartEvent{
			UiAccessListStartEvent: &prehogv1a.UIAccessListStartEvent{
				Metadata: &prehogv1a.AccessListMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
					Preset:   u.Metadata.Preset,
				},
				Status: u.Status,
			},
		},
	}
}

type UIAccessListCompletedEvent prehogv1a.UIAccessListCompleteEvent

func (u *UIAccessListCompletedEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateAccessListBaseEventFields(u.Metadata, u.Status))
}

func (u *UIAccessListCompletedEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiAccessListCompleteEvent{
			UiAccessListCompleteEvent: &prehogv1a.UIAccessListCompleteEvent{
				Metadata: &prehogv1a.AccessListMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
					Preset:   u.Metadata.Preset,
				},
				Status:             u.Status,
				PreferredTerraform: u.PreferredTerraform,
			},
		},
	}
}

type UIAccessListCustomEvent prehogv1a.UIAccessListCustomEvent

func (u *UIAccessListCustomEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateAccessListBaseEventFields(u.Metadata, u.Status))
}

func (u *UIAccessListCustomEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiAccessListCustomEvent{
			UiAccessListCustomEvent: &prehogv1a.UIAccessListCustomEvent{
				Metadata: &prehogv1a.AccessListMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
				},
				Status: u.Status,
			},
		},
	}
}

type UIAccessListIntegrateEvent prehogv1a.UIAccessListIntegrateEvent

func (u *UIAccessListIntegrateEvent) CheckAndSetDefaults() error {
	return trace.Wrap(validateAccessListMetadata(u.Metadata))
}

func (u *UIAccessListIntegrateEvent) Anonymize(a utils.Anonymizer) prehogv1a.SubmitEventRequest {
	return prehogv1a.SubmitEventRequest{
		Event: &prehogv1a.SubmitEventRequest_UiAccessListIntegrateEvent{
			UiAccessListIntegrateEvent: &prehogv1a.UIAccessListIntegrateEvent{
				Metadata: &prehogv1a.AccessListMetadata{
					Id:       u.Metadata.Id,
					UserName: a.AnonymizeString(u.Metadata.UserName),
					Preset:   u.Metadata.Preset,
				},
				Integrate: u.Integrate,
			},
		},
	}
}
