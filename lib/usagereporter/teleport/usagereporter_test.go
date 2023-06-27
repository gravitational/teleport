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
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/utils"
)

func TestConvertUsageEvent(t *testing.T) {
	anonymizer, err := utils.NewHMACAnonymizer("cluster-id")
	require.NoError(t, err)

	expectedAnonymizedUserString := anonymizer.AnonymizeString("myuser")

	for _, tt := range []struct {
		name             string
		event            *usageeventsv1.UsageEventOneOf
		identityUsername string
		isSSOUser        bool
		errCheck         require.ErrorAssertionFunc
		expected         *prehogv1a.SubmitEventRequest
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
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_UiDiscoverStartedEvent{
				UiDiscoverStartedEvent: &prehogv1a.UIDiscoverStartedEvent{
					Metadata: &prehogv1a.DiscoverMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
						Sso:      false,
					},
					Status: &prehogv1a.DiscoverStepStatus{Status: prehogv1a.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
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
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_UiDiscoverResourceSelectionEvent{
				UiDiscoverResourceSelectionEvent: &prehogv1a.UIDiscoverResourceSelectionEvent{
					Metadata: &prehogv1a.DiscoverMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
						Sso:      false,
					},
					Resource: &prehogv1a.DiscoverResourceMetadata{Resource: prehogv1a.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status:   &prehogv1a.DiscoverStepStatus{Status: prehogv1a.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
		},
		{
			name: "error when discover metadata doesn't have id",
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
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_UiDiscoverAutoDiscoveredResourcesEvent{
				UiDiscoverAutoDiscoveredResourcesEvent: &prehogv1a.UIDiscoverAutoDiscoveredResourcesEvent{
					Metadata: &prehogv1a.DiscoverMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
						Sso:      false,
					},
					Resource:       &prehogv1a.DiscoverResourceMetadata{Resource: prehogv1a.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status:         &prehogv1a.DiscoverStepStatus{Status: prehogv1a.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
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
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_UiDiscoverAutoDiscoveredResourcesEvent{
				UiDiscoverAutoDiscoveredResourcesEvent: &prehogv1a.UIDiscoverAutoDiscoveredResourcesEvent{
					Metadata: &prehogv1a.DiscoverMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
						Sso:      false,
					},
					Resource:       &prehogv1a.DiscoverResourceMetadata{Resource: prehogv1a.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status:         &prehogv1a.DiscoverStepStatus{Status: prehogv1a.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
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
		},
		{
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
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_UiDiscoverStartedEvent{
				UiDiscoverStartedEvent: &prehogv1a.UIDiscoverStartedEvent{
					Metadata: &prehogv1a.DiscoverMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
						Sso:      true,
					},
					Status: &prehogv1a.DiscoverStepStatus{Status: prehogv1a.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
		},
		{
			name: "integration enroll started event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiIntegrationEnrollStartEvent{
				UiIntegrationEnrollStartEvent: &usageeventsv1.UIIntegrationEnrollStartEvent{
					Metadata: &usageeventsv1.IntegrationEnrollMetadata{Id: "someid", Kind: usageeventsv1.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_AWS_OIDC},
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_UiIntegrationEnrollStartEvent{
				UiIntegrationEnrollStartEvent: &prehogv1a.UIIntegrationEnrollStartEvent{
					Metadata: &prehogv1a.IntegrationEnrollMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
						Kind:     prehogv1a.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_AWS_OIDC,
					},
				},
			}},
		},
		{
			name: "discover deploy service event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverDeployServiceEvent{
				UiDiscoverDeployServiceEvent: &usageeventsv1.UIDiscoverDeployServiceEvent{
					Metadata:     &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Resource:     &usageeventsv1.DiscoverResourceMetadata{Resource: usageeventsv1.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status:       &usageeventsv1.DiscoverStepStatus{Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
					DeployMethod: usageeventsv1.UIDiscoverDeployServiceEvent_DEPLOY_METHOD_AUTO,
					DeployType:   usageeventsv1.UIDiscoverDeployServiceEvent_DEPLOY_TYPE_AMAZON_ECS,
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_UiDiscoverDeployServiceEvent{
				UiDiscoverDeployServiceEvent: &prehogv1a.UIDiscoverDeployServiceEvent{
					Metadata: &prehogv1a.DiscoverMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
						Sso:      false,
					},
					Resource:     &prehogv1a.DiscoverResourceMetadata{Resource: prehogv1a.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status:       &prehogv1a.DiscoverStepStatus{Status: prehogv1a.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
					DeployMethod: prehogv1a.UIDiscoverDeployServiceEvent_DEPLOY_METHOD_AUTO,
					DeployType:   prehogv1a.UIDiscoverDeployServiceEvent_DEPLOY_TYPE_AMAZON_ECS,
				},
			}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			userMD := UserMetadata{
				Username: tt.identityUsername,
				IsSSO:    tt.isSSOUser,
			}
			usageEvent, err := ConvertUsageEvent(tt.event, userMD)
			tt.errCheck(t, err)
			if err != nil {
				return
			}

			got := usageEvent.Anonymize(anonymizer)

			require.Equal(t, tt.expected, &got)
		})
	}
}
