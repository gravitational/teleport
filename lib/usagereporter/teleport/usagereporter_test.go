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
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	prehogv1 "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/utils"
)

type mockUserMetadataClient struct {
	username string
	isSSO    bool
}

func (m mockUserMetadataClient) GetUsername(ctx context.Context) (string, error) {
	return m.username, nil
}
func (m mockUserMetadataClient) IsSSOUser(ctx context.Context) (bool, error) {
	return m.isSSO, nil
}

func TestConvertUsageEvent(t *testing.T) {
	anonymizer, err := utils.NewHMACAnonymizer("cluster-id")
	require.NoError(t, err)

	ctx := context.Background()

	expectedAnonymizedUserString := anonymizer.AnonymizeString("myuser")

	for _, tt := range []struct {
		name             string
		event            *usageeventsv1.UsageEventOneOf
		identityUsername string
		isSSOUser        bool
		errCheck         require.ErrorAssertionFunc
		expected         *prehogv1.SubmitEventRequest
	}{
		{
			name: "discover started event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverStartedEvent{
				UiDiscoverStartedEvent: &usageeventsv1.UIDiscoverStartedEvent{
					Metadata: &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Status:   &usageeventsv1.DiscoverStepStatus{Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1.SubmitEventRequest{Event: &prehogv1.SubmitEventRequest_UiDiscoverStartedEvent{
				UiDiscoverStartedEvent: &prehogv1.UIDiscoverStartedEvent{
					Metadata: &prehogv1.DiscoverMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
						Sso:      false,
					},
					Status: &prehogv1.DiscoverStepStatus{Status: prehogv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
		},
		{
			name: "discover resource selection event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverResourceSelectionEvent{
				UiDiscoverResourceSelectionEvent: &usageeventsv1.UIDiscoverResourceSelectionEvent{
					Metadata: &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Resource: &usageeventsv1.DiscoverResourceMetadata{Resource: usageeventsv1.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status:   &usageeventsv1.DiscoverStepStatus{Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1.SubmitEventRequest{Event: &prehogv1.SubmitEventRequest_UiDiscoverResourceSelectionEvent{
				UiDiscoverResourceSelectionEvent: &prehogv1.UIDiscoverResourceSelectionEvent{
					Metadata: &prehogv1.DiscoverMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
						Sso:      false,
					},
					Resource: &prehogv1.DiscoverResourceMetadata{Resource: prehogv1.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status:   &prehogv1.DiscoverStepStatus{Status: prehogv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
		},
		{
			name: "error when discover metadata dones't have id",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverResourceSelectionEvent{
				UiDiscoverResourceSelectionEvent: &usageeventsv1.UIDiscoverResourceSelectionEvent{
					Metadata: &usageeventsv1.DiscoverMetadata{Id: ""},
					Resource: &usageeventsv1.DiscoverResourceMetadata{Resource: usageeventsv1.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status:   &usageeventsv1.DiscoverStepStatus{Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
			identityUsername: "myuser",
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(tt, trace.IsBadParameter(err), "exepcted trace.IsBadParameter error, got: %v", err)
			},
		},
		{
			name: "error when discover metadata resource",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverResourceSelectionEvent{
				UiDiscoverResourceSelectionEvent: &usageeventsv1.UIDiscoverResourceSelectionEvent{
					Metadata: &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Resource: &usageeventsv1.DiscoverResourceMetadata{Resource: 0},
					Status:   &usageeventsv1.DiscoverStepStatus{Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
			identityUsername: "myuser",
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(tt, trace.IsBadParameter(err), "exepcted trace.IsBadParameter error, got: %v", err)
			},
		},
		{
			name: "error when discover has stepStatus=ERROR but no error message",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverResourceSelectionEvent{
				UiDiscoverResourceSelectionEvent: &usageeventsv1.UIDiscoverResourceSelectionEvent{
					Metadata: &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Resource: &usageeventsv1.DiscoverResourceMetadata{Resource: usageeventsv1.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status:   &usageeventsv1.DiscoverStepStatus{Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_ERROR},
				},
			}},
			identityUsername: "myuser",
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(tt, trace.IsBadParameter(err), "exepcted trace.IsBadParameter error, got: %v", err)
			},
		},
		{
			name: "when discover has resources count and its values is zero: no error",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverAutoDiscoveredResourcesEvent{
				UiDiscoverAutoDiscoveredResourcesEvent: &usageeventsv1.UIDiscoverAutoDiscoveredResourcesEvent{
					Metadata:       &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Resource:       &usageeventsv1.DiscoverResourceMetadata{Resource: usageeventsv1.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status:         &usageeventsv1.DiscoverStepStatus{Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
					ResourcesCount: 0,
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1.SubmitEventRequest{Event: &prehogv1.SubmitEventRequest_UiDiscoverAutoDiscoveredResourcesEvent{
				UiDiscoverAutoDiscoveredResourcesEvent: &prehogv1.UIDiscoverAutoDiscoveredResourcesEvent{
					Metadata: &prehogv1.DiscoverMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
						Sso:      false,
					},
					Resource:       &prehogv1.DiscoverResourceMetadata{Resource: prehogv1.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status:         &prehogv1.DiscoverStepStatus{Status: prehogv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
					ResourcesCount: 0,
				},
			}},
		},
		{
			name: "when discover has resources count and its values is positive: no error",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverAutoDiscoveredResourcesEvent{
				UiDiscoverAutoDiscoveredResourcesEvent: &usageeventsv1.UIDiscoverAutoDiscoveredResourcesEvent{
					Metadata:       &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Resource:       &usageeventsv1.DiscoverResourceMetadata{Resource: usageeventsv1.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status:         &usageeventsv1.DiscoverStepStatus{Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
					ResourcesCount: 2,
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1.SubmitEventRequest{Event: &prehogv1.SubmitEventRequest_UiDiscoverAutoDiscoveredResourcesEvent{
				UiDiscoverAutoDiscoveredResourcesEvent: &prehogv1.UIDiscoverAutoDiscoveredResourcesEvent{
					Metadata: &prehogv1.DiscoverMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
						Sso:      false,
					},
					Resource:       &prehogv1.DiscoverResourceMetadata{Resource: prehogv1.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status:         &prehogv1.DiscoverStepStatus{Status: prehogv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
					ResourcesCount: 2,
				},
			}},
		},
		{
			name: "when discover has resources count and its values is negative: bad parameter error",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverAutoDiscoveredResourcesEvent{
				UiDiscoverAutoDiscoveredResourcesEvent: &usageeventsv1.UIDiscoverAutoDiscoveredResourcesEvent{
					Metadata:       &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Resource:       &usageeventsv1.DiscoverResourceMetadata{Resource: usageeventsv1.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status:         &usageeventsv1.DiscoverStepStatus{Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
					ResourcesCount: -2,
				},
			}},
			identityUsername: "myuser",
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(tt, trace.IsBadParameter(err), "exepcted trace.IsBadParameter error, got: %v", err)
			},
		}, {
			name: "discover started event with sso user",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverStartedEvent{
				UiDiscoverStartedEvent: &usageeventsv1.UIDiscoverStartedEvent{
					Metadata: &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Status:   &usageeventsv1.DiscoverStepStatus{Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
			identityUsername: "myuser",
			isSSOUser:        true,
			errCheck:         require.NoError,
			expected: &prehogv1.SubmitEventRequest{Event: &prehogv1.SubmitEventRequest_UiDiscoverStartedEvent{
				UiDiscoverStartedEvent: &prehogv1.UIDiscoverStartedEvent{
					Metadata: &prehogv1.DiscoverMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
						Sso:      true,
					},
					Status: &prehogv1.DiscoverStepStatus{Status: prehogv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			m := mockUserMetadataClient{
				username: tt.identityUsername,
				isSSO:    tt.isSSOUser,
			}

			usageEvent, err := ConvertUsageEvent(ctx, tt.event, m)
			tt.errCheck(t, err)
			if err != nil {
				return
			}

			got := usageEvent.Anonymize(anonymizer)

			require.Equal(t, tt.expected, &got)
		})
	}
}
