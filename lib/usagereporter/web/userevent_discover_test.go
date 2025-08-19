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
			errCheck: func(tt require.TestingT, err error, i ...any) {
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
			errCheck: func(tt require.TestingT, err error, i ...any) {
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
			name:     uiDiscoverDeployServiceEvent + "/success_test_deployed_method_type_setting",
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
		{
			name:     uiDiscoverCreateDiscoveryConfigEvent + "/success_test",
			event:    uiDiscoverCreateDiscoveryConfigEvent,
			errCheck: require.NoError,
			req: DiscoverEventData{
				ID:                    "someid",
				Resource:              "DISCOVER_RESOURCE_EC2_INSTANCE",
				StepStatus:            "DISCOVER_STATUS_SUCCESS",
				DiscoveryConfigMethod: "CONFIG_METHOD_AWS_EC2_SSM",
			},
			expected: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverCreateDiscoveryConfig{
				UiDiscoverCreateDiscoveryConfig: &usageeventsv1.UIDiscoverCreateDiscoveryConfigEvent{
					Metadata: &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Resource: &usageeventsv1.DiscoverResourceMetadata{Resource: usageeventsv1.DiscoverResource_DISCOVER_RESOURCE_EC2_INSTANCE},
					Status: &usageeventsv1.DiscoverStepStatus{
						Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS,
					},
					ConfigMethod: usageeventsv1.UIDiscoverCreateDiscoveryConfigEvent_CONFIG_METHOD_AWS_EC2_SSM,
				},
			}},
		},
		{
			name:     uiDiscoverCreateAppServerEvent + "/success_test",
			event:    uiDiscoverCreateAppServerEvent,
			errCheck: require.NoError,
			req: DiscoverEventData{
				ID:         "someid",
				Resource:   "DISCOVER_RESOURCE_APPLICATION_AWS_CONSOLE",
				StepStatus: "DISCOVER_STATUS_SUCCESS",
			},
			expected: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverCreateAppServerEvent{
				UiDiscoverCreateAppServerEvent: &usageeventsv1.UIDiscoverCreateAppServerEvent{
					Metadata: &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Resource: &usageeventsv1.DiscoverResourceMetadata{Resource: usageeventsv1.DiscoverResource_DISCOVER_RESOURCE_APPLICATION_AWS_CONSOLE},
					Status: &usageeventsv1.DiscoverStepStatus{
						Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS,
					},
				},
			}},
		},
		{
			name:     uiDiscoverKubeEKSEnrollEvent + "/success_test",
			event:    uiDiscoverKubeEKSEnrollEvent,
			errCheck: require.NoError,
			req: DiscoverEventData{
				ID:         "someid",
				Resource:   "DISCOVER_RESOURCE_KUBERNETES_EKS",
				StepStatus: "DISCOVER_STATUS_SUCCESS",
			},
			expected: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverKubeEksEnrollEvent{
				UiDiscoverKubeEksEnrollEvent: &usageeventsv1.UIDiscoverKubeEKSEnrollEvent{
					Metadata: &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Resource: &usageeventsv1.DiscoverResourceMetadata{Resource: usageeventsv1.DiscoverResource_DISCOVER_RESOURCE_KUBERNETES_EKS},
					Status: &usageeventsv1.DiscoverStepStatus{
						Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS,
					},
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
