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
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
)

func TestAccessListEventDataToUsageEvent(t *testing.T) {
	for _, tt := range []struct {
		name     string
		event    string
		req      AccessListEventData
		errCheck require.ErrorAssertionFunc
		expected *usageeventsv1.UsageEventOneOf
	}{
		{
			name:     uiAccessListStartEvent + "/success preset unspecified",
			event:    uiAccessListStartEvent,
			errCheck: require.NoError,
			req: AccessListEventData{
				ID:         "someid",
				StepStatus: "ACCESS_LIST_STATUS_SUCCESS",
			},
			expected: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListStartEvent{
				UiAccessListStartEvent: &usageeventsv1.UIAccessListStartEvent{
					Metadata: &usageeventsv1.AccessListMetadata{Id: "someid", Preset: usageeventsv1.AccessListPreset_ACCESS_LIST_PRESET_UNSPECIFIED},
					Status:   &usageeventsv1.AccessListStepStatus{Status: usageeventsv1.AccessListStatus_ACCESS_LIST_STATUS_SUCCESS},
				},
			}},
		},
		{
			name:     uiAccessListStartEvent + "/success short-term",
			event:    uiAccessListStartEvent,
			errCheck: require.NoError,
			req: AccessListEventData{
				ID:         "someid",
				StepStatus: "ACCESS_LIST_STATUS_SUCCESS",
				Preset:     "ACCESS_LIST_PRESET_SHORT_TERM",
			},
			expected: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListStartEvent{
				UiAccessListStartEvent: &usageeventsv1.UIAccessListStartEvent{
					Metadata: &usageeventsv1.AccessListMetadata{Id: "someid", Preset: usageeventsv1.AccessListPreset_ACCESS_LIST_PRESET_SHORT_TERM},
					Status:   &usageeventsv1.AccessListStepStatus{Status: usageeventsv1.AccessListStatus_ACCESS_LIST_STATUS_SUCCESS},
				},
			}},
		},
		{
			name:  uiAccessListStartEvent + "/invalid status",
			event: uiAccessListStartEvent,
			errCheck: func(tt require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got %v", err)
			},
			req: AccessListEventData{
				ID:         "someid",
				StepStatus: "SUCCESS",
			},
		},
		{
			name:     uiAccessListCompleteEvent + "/success with terraform flag",
			event:    uiAccessListCompleteEvent,
			errCheck: require.NoError,
			req: AccessListEventData{
				ID:                 "someid",
				Preset:             "ACCESS_LIST_PRESET_LONG_TERM",
				StepStatus:         "ACCESS_LIST_STATUS_SUCCESS",
				PreferredTerraform: true,
			},
			expected: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListCompleteEvent{
				UiAccessListCompleteEvent: &usageeventsv1.UIAccessListCompleteEvent{
					Metadata:           &usageeventsv1.AccessListMetadata{Id: "someid", Preset: usageeventsv1.AccessListPreset_ACCESS_LIST_PRESET_LONG_TERM},
					Status:             &usageeventsv1.AccessListStepStatus{Status: usageeventsv1.AccessListStatus_ACCESS_LIST_STATUS_SUCCESS},
					PreferredTerraform: true,
				},
			}},
		},
		{
			name:     uiAccessListCompleteEvent + "/without terraform flag",
			event:    uiAccessListCompleteEvent,
			errCheck: require.NoError,
			req: AccessListEventData{
				ID:         "someid",
				Preset:     "ACCESS_LIST_PRESET_LONG_TERM",
				StepStatus: "ACCESS_LIST_STATUS_SUCCESS",
			},
			expected: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListCompleteEvent{
				UiAccessListCompleteEvent: &usageeventsv1.UIAccessListCompleteEvent{
					Metadata: &usageeventsv1.AccessListMetadata{
						Id:     "someid",
						Preset: usageeventsv1.AccessListPreset_ACCESS_LIST_PRESET_LONG_TERM,
					},
					Status:             &usageeventsv1.AccessListStepStatus{Status: usageeventsv1.AccessListStatus_ACCESS_LIST_STATUS_SUCCESS},
					PreferredTerraform: false,
				},
			}},
		},
		{
			name:  uiAccessListCompleteEvent + "/invalid preset",
			event: uiAccessListCompleteEvent,
			errCheck: func(tt require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got %v", err)
			},
			req: AccessListEventData{
				ID:         "someid",
				Preset:     "LONG_TERM",
				StepStatus: "ACCESS_LIST_STATUS_SUCCESS",
			},
		},
		{
			name:     uiAccessListCompleteEvent + "/with error status",
			event:    uiAccessListCompleteEvent,
			errCheck: require.NoError,
			req: AccessListEventData{
				ID:              "someid",
				StepStatus:      "ACCESS_LIST_STATUS_ERROR",
				StepStatusError: "failed to create access list",
			},
			expected: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListCompleteEvent{
				UiAccessListCompleteEvent: &usageeventsv1.UIAccessListCompleteEvent{
					Metadata: &usageeventsv1.AccessListMetadata{Id: "someid"},
					Status: &usageeventsv1.AccessListStepStatus{
						Status: usageeventsv1.AccessListStatus_ACCESS_LIST_STATUS_ERROR,
						Error:  "failed to create access list",
					},
				},
			}},
		},
		{
			name:     uiAccessListDefineAccessEvent + "/success",
			event:    uiAccessListDefineAccessEvent,
			errCheck: require.NoError,
			req: AccessListEventData{
				ID:         "someid",
				StepStatus: "ACCESS_LIST_STATUS_SUCCESS",
				Preset:     "ACCESS_LIST_PRESET_LONG_TERM",
			},
			expected: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListDefineAccessEvent{
				UiAccessListDefineAccessEvent: &usageeventsv1.UIAccessListDefineAccessEvent{
					Metadata: &usageeventsv1.AccessListMetadata{Id: "someid", Preset: usageeventsv1.AccessListPreset_ACCESS_LIST_PRESET_LONG_TERM},
					Status:   &usageeventsv1.AccessListStepStatus{Status: usageeventsv1.AccessListStatus_ACCESS_LIST_STATUS_SUCCESS},
				},
			}},
		},
		{
			name:     uiAccessListDefineBasicInfoEvent + "/success",
			event:    uiAccessListDefineBasicInfoEvent,
			errCheck: require.NoError,
			req: AccessListEventData{
				ID:         "someid",
				StepStatus: "ACCESS_LIST_STATUS_SUCCESS",
				Preset:     "ACCESS_LIST_PRESET_LONG_TERM",
			},
			expected: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListDefineBasicInfoEvent{
				UiAccessListDefineBasicInfoEvent: &usageeventsv1.UIAccessListDefineBasicInfoEvent{
					Metadata: &usageeventsv1.AccessListMetadata{Id: "someid", Preset: usageeventsv1.AccessListPreset_ACCESS_LIST_PRESET_LONG_TERM},
					Status:   &usageeventsv1.AccessListStepStatus{Status: usageeventsv1.AccessListStatus_ACCESS_LIST_STATUS_SUCCESS},
				},
			}},
		},
		{
			name:     uiAccessListDefineIdentitiesEvent + "/success",
			event:    uiAccessListDefineIdentitiesEvent,
			errCheck: require.NoError,
			req: AccessListEventData{
				ID:         "someid",
				StepStatus: "ACCESS_LIST_STATUS_SUCCESS",
				Preset:     "ACCESS_LIST_PRESET_LONG_TERM",
			},
			expected: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListDefineIdentitiesEvent{
				UiAccessListDefineIdentitiesEvent: &usageeventsv1.UIAccessListDefineIdentitiesEvent{
					Metadata: &usageeventsv1.AccessListMetadata{Id: "someid", Preset: usageeventsv1.AccessListPreset_ACCESS_LIST_PRESET_LONG_TERM},
					Status:   &usageeventsv1.AccessListStepStatus{Status: usageeventsv1.AccessListStatus_ACCESS_LIST_STATUS_SUCCESS},
				},
			}},
		},
		{
			name:     uiAccessListDefineMembersEvent + "/success",
			event:    uiAccessListDefineMembersEvent,
			errCheck: require.NoError,
			req: AccessListEventData{
				ID:         "someid",
				StepStatus: "ACCESS_LIST_STATUS_SUCCESS",
				Preset:     "ACCESS_LIST_PRESET_LONG_TERM",
			},
			expected: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListDefineMembersEvent{
				UiAccessListDefineMembersEvent: &usageeventsv1.UIAccessListDefineMembersEvent{
					Metadata: &usageeventsv1.AccessListMetadata{Id: "someid", Preset: usageeventsv1.AccessListPreset_ACCESS_LIST_PRESET_LONG_TERM},
					Status:   &usageeventsv1.AccessListStepStatus{Status: usageeventsv1.AccessListStatus_ACCESS_LIST_STATUS_SUCCESS},
				},
			}},
		},
		{
			name:     uiAccessListDefineOwnersEvent + "/success",
			event:    uiAccessListDefineOwnersEvent,
			errCheck: require.NoError,
			req: AccessListEventData{
				ID:         "someid",
				StepStatus: "ACCESS_LIST_STATUS_SUCCESS",
				Preset:     "ACCESS_LIST_PRESET_LONG_TERM",
			},
			expected: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListDefineOwnersEvent{
				UiAccessListDefineOwnersEvent: &usageeventsv1.UIAccessListDefineOwnersEvent{
					Metadata: &usageeventsv1.AccessListMetadata{Id: "someid", Preset: usageeventsv1.AccessListPreset_ACCESS_LIST_PRESET_LONG_TERM},
					Status:   &usageeventsv1.AccessListStepStatus{Status: usageeventsv1.AccessListStatus_ACCESS_LIST_STATUS_SUCCESS},
				},
			}},
		},
		{
			name:     uiAccessListIntegrateEvent + "/success",
			event:    uiAccessListIntegrateEvent,
			errCheck: require.NoError,
			req: AccessListEventData{
				ID:         "someid",
				StepStatus: "ACCESS_LIST_STATUS_SUCCESS",
				Integrate:  "ACCESS_LIST_INTEGRATE_OKTA",
				Preset:     "ACCESS_LIST_PRESET_LONG_TERM",
			},
			expected: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListIntegrateEvent{
				UiAccessListIntegrateEvent: &usageeventsv1.UIAccessListIntegrateEvent{
					Metadata:  &usageeventsv1.AccessListMetadata{Id: "someid", Preset: usageeventsv1.AccessListPreset_ACCESS_LIST_PRESET_LONG_TERM},
					Integrate: usageeventsv1.AccessListIntegrate_ACCESS_LIST_INTEGRATE_OKTA,
				},
			}},
		},
		{
			name:  uiAccessListIntegrateEvent + "/invalid integrate",
			event: uiAccessListIntegrateEvent,
			errCheck: func(tt require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got %v", err)
			},
			req: AccessListEventData{
				ID:         "someid",
				StepStatus: "ACCESS_LIST_STATUS_SUCCESS",
				Integrate:  "OKTA",
				Preset:     "ACCESS_LIST_PRESET_LONG_TERM",
			},
		},
		{
			name:     uiAccessListCustomEvent + "/success",
			event:    uiAccessListCustomEvent,
			errCheck: require.NoError,
			req: AccessListEventData{
				ID:         "someid",
				StepStatus: "ACCESS_LIST_STATUS_SUCCESS",
			},
			expected: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessListCustomEvent{
				UiAccessListCustomEvent: &usageeventsv1.UIAccessListCustomEvent{
					Metadata: &usageeventsv1.AccessListMetadata{Id: "someid", Preset: usageeventsv1.AccessListPreset_ACCESS_LIST_PRESET_UNSPECIFIED},
					Status:   &usageeventsv1.AccessListStepStatus{Status: usageeventsv1.AccessListStatus_ACCESS_LIST_STATUS_SUCCESS},
				},
			}},
		},
		{
			name:  "invalid event name",
			event: "tp.ui.access_list.unknown",
			errCheck: func(tt require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got %v", err)
			},
			req: AccessListEventData{
				ID:         "someid",
				StepStatus: "ACCESS_LIST_STATUS_SUCCESS",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			got, err := tt.req.ToUsageEvent(tt.event)
			tt.errCheck(t, err)
			if err != nil {
				return
			}

			require.Equal(t, tt.expected, got)
		})
	}
}
