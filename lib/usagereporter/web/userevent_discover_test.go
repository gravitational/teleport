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
)

func TestDiscoverEventDataToUsageEvent(t *testing.T) {
	for _, tt := range []struct {
		name     string
		event    string
		req      DiscoverEventData
		errCheck require.ErrorAssertionFunc
		expected *usageeventsv1.UsageEventOneOf
	}{
		{
			name:     uiDiscoverStartedEvent + "/success",
			event:    uiDiscoverStartedEvent,
			errCheck: require.NoError,
			req: DiscoverEventData{
				ID:         "someid",
				StepStatus: "DISCOVER_STATUS_SUCCESS",
			},
			expected: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverStartedEvent{
				UiDiscoverStartedEvent: &usageeventsv1.UIDiscoverStartedEvent{
					Metadata: &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Status:   &usageeventsv1.DiscoverStepStatus{Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
		},
		{
			name:     uiDiscoverResourceSelectionEvent + "/success",
			event:    uiDiscoverResourceSelectionEvent,
			errCheck: require.NoError,
			req: DiscoverEventData{
				ID:              "someid",
				Resource:        "DISCOVER_RESOURCE_SERVER",
				StepStatus:      "DISCOVER_STATUS_ERROR",
				StepStatusError: "Failed to fetch available resources",
			},
			expected: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverResourceSelectionEvent{
				UiDiscoverResourceSelectionEvent: &usageeventsv1.UIDiscoverResourceSelectionEvent{
					Metadata: &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Resource: &usageeventsv1.DiscoverResourceMetadata{Resource: usageeventsv1.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status: &usageeventsv1.DiscoverStepStatus{
						Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_ERROR,
						Error:  "Failed to fetch available resources",
					},
				},
			}},
		},
		{
			name:  uiDiscoverResourceSelectionEvent + "/invalid resource",
			event: uiDiscoverResourceSelectionEvent,
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got %v", err)
			},
			req: DiscoverEventData{
				ID:              "someid",
				Resource:        "SERVER",
				StepStatus:      "DISCOVER_STATUS_ERROR",
				StepStatusError: "Failed to fetch available resources",
			},
		},
		{
			name:  uiDiscoverResourceSelectionEvent + "/invalid status",
			event: uiDiscoverResourceSelectionEvent,
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got %v", err)
			},
			req: DiscoverEventData{
				ID:              "someid",
				Resource:        "DISCOVER_RESOURCE_SERVER",
				StepStatus:      "ERROR",
				StepStatusError: "Failed to fetch available resources",
			},
		},
		{
			name:     uiDiscoverAutoDiscoveredResourcesEvent + "/success",
			event:    uiDiscoverAutoDiscoveredResourcesEvent,
			errCheck: require.NoError,
			req: DiscoverEventData{
				ID:                         "someid",
				Resource:                   "DISCOVER_RESOURCE_SERVER",
				StepStatus:                 "DISCOVER_STATUS_SUCCESS",
				AutoDiscoverResourcesCount: 3,
			},
			expected: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverAutoDiscoveredResourcesEvent{
				UiDiscoverAutoDiscoveredResourcesEvent: &usageeventsv1.UIDiscoverAutoDiscoveredResourcesEvent{
					Metadata: &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Resource: &usageeventsv1.DiscoverResourceMetadata{Resource: usageeventsv1.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status: &usageeventsv1.DiscoverStepStatus{
						Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS,
					},
					ResourcesCount: 3,
				},
			}},
		},
		{
			name:     uiDiscoverDeployServiceEvent + "/success_test_deployed_method_setting",
			event:    uiDiscoverDeployServiceEvent,
			errCheck: require.NoError,
			req: DiscoverEventData{
				ID:         "someid",
				Resource:   "DISCOVER_RESOURCE_SERVER",
				StepStatus: "DISCOVER_STATUS_SUCCESS",
				ServiceDeploy: discoverServiceDeploy{
					Method: "DEPLOY_METHOD_AUTO",
					Type:   "DEPLOY_TYPE_AMAZON_ECS",
				},
			},
			expected: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverDeployServiceEvent{
				UiDiscoverDeployServiceEvent: &usageeventsv1.UIDiscoverDeployServiceEvent{
					Metadata: &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Resource: &usageeventsv1.DiscoverResourceMetadata{Resource: usageeventsv1.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status: &usageeventsv1.DiscoverStepStatus{
						Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS,
					},
					DeployMethod: usageeventsv1.UIDiscoverDeployServiceEvent_DEPLOY_METHOD_AUTO,
					DeployType:   usageeventsv1.UIDiscoverDeployServiceEvent_DEPLOY_TYPE_AMAZON_ECS,
				},
			}},
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
