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

package services

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	usageevents "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	prehogv1 "github.com/gravitational/teleport/lib/prehog/gen/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/utils"
)

func TestConvertUsageEvent(t *testing.T) {
	anonymizer, err := utils.NewHMACAnonymizer("cluster-id")
	require.NoError(t, err)

	expectedAnonymizedUserString := anonymizer.AnonymizeString("myuser")

	for _, tt := range []struct {
		name             string
		event            *usageevents.UsageEventOneOf
		identityUsername string
		errCheck         require.ErrorAssertionFunc
		expected         *prehogv1.SubmitEventRequest
	}{
		{
			name: "discover started event",
			event: &usageevents.UsageEventOneOf{Event: &usageevents.UsageEventOneOf_UiDiscoverStartedEvent{
				UiDiscoverStartedEvent: &usageevents.UIDiscoverStartedEvent{
					Metadata: &usageevents.DiscoverMetadata{Id: "someid"},
					Status:   &usageevents.DiscoverStepStatus{Status: usageevents.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1.SubmitEventRequest{Event: &prehogv1.SubmitEventRequest_UiDiscoverStartedEvent{
				UiDiscoverStartedEvent: &prehogv1.UIDiscoverStartedEvent{
					Metadata: &prehogv1.DiscoverMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
					},
					Status: &prehogv1.DiscoverStepStatus{Status: prehogv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
		},
		{
			name: "discover resource selection event",
			event: &usageevents.UsageEventOneOf{Event: &usageevents.UsageEventOneOf_UiDiscoverResourceSelectionEvent{
				UiDiscoverResourceSelectionEvent: &usageevents.UIDiscoverResourceSelectionEvent{
					Metadata: &usageevents.DiscoverMetadata{Id: "someid"},
					Resource: &usageevents.DiscoverResourceMetadata{Resource: usageevents.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status:   &usageevents.DiscoverStepStatus{Status: usageevents.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1.SubmitEventRequest{Event: &prehogv1.SubmitEventRequest_UiDiscoverResourceSelectionEvent{
				UiDiscoverResourceSelectionEvent: &prehogv1.UIDiscoverResourceSelectionEvent{
					Metadata: &prehogv1.DiscoverMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
					},
					Resource: &prehogv1.DiscoverResourceMetadata{Resource: prehogv1.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status:   &prehogv1.DiscoverStepStatus{Status: prehogv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
		},
		{
			name: "error when discover metadata dones't have id",
			event: &usageevents.UsageEventOneOf{Event: &usageevents.UsageEventOneOf_UiDiscoverResourceSelectionEvent{
				UiDiscoverResourceSelectionEvent: &usageevents.UIDiscoverResourceSelectionEvent{
					Metadata: &usageevents.DiscoverMetadata{Id: ""},
					Resource: &usageevents.DiscoverResourceMetadata{Resource: usageevents.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status:   &usageevents.DiscoverStepStatus{Status: usageevents.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
			identityUsername: "myuser",
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(tt, trace.IsBadParameter(err), "exepcted trace.IsBadParameter error, got: %v", err)
			},
		},
		{
			name: "error when discover metadata resource",
			event: &usageevents.UsageEventOneOf{Event: &usageevents.UsageEventOneOf_UiDiscoverResourceSelectionEvent{
				UiDiscoverResourceSelectionEvent: &usageevents.UIDiscoverResourceSelectionEvent{
					Metadata: &usageevents.DiscoverMetadata{Id: "someid"},
					Resource: &usageevents.DiscoverResourceMetadata{Resource: 0},
					Status:   &usageevents.DiscoverStepStatus{Status: usageevents.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
			identityUsername: "myuser",
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(tt, trace.IsBadParameter(err), "exepcted trace.IsBadParameter error, got: %v", err)
			},
		},
		{
			name: "error when discover has stepStatus=ERROR but no error message",
			event: &usageevents.UsageEventOneOf{Event: &usageevents.UsageEventOneOf_UiDiscoverResourceSelectionEvent{
				UiDiscoverResourceSelectionEvent: &usageevents.UIDiscoverResourceSelectionEvent{
					Metadata: &usageevents.DiscoverMetadata{Id: "someid"},
					Resource: &usageevents.DiscoverResourceMetadata{Resource: usageevents.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status:   &usageevents.DiscoverStepStatus{Status: usageevents.DiscoverStatus_DISCOVER_STATUS_ERROR},
				},
			}},
			identityUsername: "myuser",
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(tt, trace.IsBadParameter(err), "exepcted trace.IsBadParameter error, got: %v", err)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			usageEvent, err := ConvertUsageEvent(tt.event, tt.identityUsername)
			tt.errCheck(t, err)
			if err != nil {
				return
			}

			got := usageEvent.Anonymize(anonymizer)

			require.Equal(t, tt.expected, &got)
		})
	}
}
