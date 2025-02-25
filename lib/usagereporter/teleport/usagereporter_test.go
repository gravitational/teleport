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

	"github.com/gravitational/teleport"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/utils"
)

func TestConvertUsageEvent(t *testing.T) {
	anonymizer, err := utils.NewHMACAnonymizer("anon-key-or-cluster-id")
	require.NoError(t, err)

	expectedAnonymizedUserString := anonymizer.AnonymizeString("myuser")
	expectedAnonymizedAccessListIDString := anonymizer.AnonymizeString("someid")

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
			name: "integration enroll step success event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiIntegrationEnrollStepEvent{
				UiIntegrationEnrollStepEvent: &usageeventsv1.UIIntegrationEnrollStepEvent{
					Metadata: &usageeventsv1.IntegrationEnrollMetadata{Id: "someid", Kind: usageeventsv1.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_AWS_IDENTITY_CENTER},
					Step:     usageeventsv1.IntegrationEnrollStep_INTEGRATION_ENROLL_STEP_AWSIC_CONNECT_OIDC,
					Status: &usageeventsv1.IntegrationEnrollStepStatus{
						Code: usageeventsv1.IntegrationEnrollStatusCode_INTEGRATION_ENROLL_STATUS_CODE_SUCCESS,
					},
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_UiIntegrationEnrollStepEvent{
				UiIntegrationEnrollStepEvent: &prehogv1a.UIIntegrationEnrollStepEvent{
					Metadata: &prehogv1a.IntegrationEnrollMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
						Kind:     prehogv1a.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_AWS_IDENTITY_CENTER,
					},
					Step: prehogv1a.IntegrationEnrollStep_INTEGRATION_ENROLL_STEP_AWSIC_CONNECT_OIDC,
					Status: &prehogv1a.IntegrationEnrollStepStatus{
						Code:  prehogv1a.IntegrationEnrollStatusCode_INTEGRATION_ENROLL_STATUS_CODE_SUCCESS,
						Error: "",
					},
				},
			}},
		},
		{
			name: "integration enroll step error event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiIntegrationEnrollStepEvent{
				UiIntegrationEnrollStepEvent: &usageeventsv1.UIIntegrationEnrollStepEvent{
					Metadata: &usageeventsv1.IntegrationEnrollMetadata{Id: "someid", Kind: usageeventsv1.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_AWS_IDENTITY_CENTER},
					Step:     usageeventsv1.IntegrationEnrollStep_INTEGRATION_ENROLL_STEP_AWSIC_CONNECT_OIDC,
					Status: &usageeventsv1.IntegrationEnrollStepStatus{
						Code:  usageeventsv1.IntegrationEnrollStatusCode_INTEGRATION_ENROLL_STATUS_CODE_ERROR,
						Error: "error",
					},
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_UiIntegrationEnrollStepEvent{
				UiIntegrationEnrollStepEvent: &prehogv1a.UIIntegrationEnrollStepEvent{
					Metadata: &prehogv1a.IntegrationEnrollMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
						Kind:     prehogv1a.IntegrationEnrollKind_INTEGRATION_ENROLL_KIND_AWS_IDENTITY_CENTER,
					},
					Step: prehogv1a.IntegrationEnrollStep_INTEGRATION_ENROLL_STEP_AWSIC_CONNECT_OIDC,
					Status: &prehogv1a.IntegrationEnrollStepStatus{
						Code:  prehogv1a.IntegrationEnrollStatusCode_INTEGRATION_ENROLL_STATUS_CODE_ERROR,
						Error: "error",
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
		{
			name: "discover create discovery config event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverCreateDiscoveryConfig{
				UiDiscoverCreateDiscoveryConfig: &usageeventsv1.UIDiscoverCreateDiscoveryConfigEvent{
					Metadata:     &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Resource:     &usageeventsv1.DiscoverResourceMetadata{Resource: usageeventsv1.DiscoverResource_DISCOVER_RESOURCE_EC2_INSTANCE},
					Status:       &usageeventsv1.DiscoverStepStatus{Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
					ConfigMethod: usageeventsv1.UIDiscoverCreateDiscoveryConfigEvent_CONFIG_METHOD_AWS_EC2_SSM,
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_UiDiscoverCreateDiscoveryConfig{
				UiDiscoverCreateDiscoveryConfig: &prehogv1a.UIDiscoverCreateDiscoveryConfigEvent{
					Metadata: &prehogv1a.DiscoverMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
						Sso:      false,
					},
					Resource:     &prehogv1a.DiscoverResourceMetadata{Resource: prehogv1a.DiscoverResource_DISCOVER_RESOURCE_EC2_INSTANCE},
					Status:       &prehogv1a.DiscoverStepStatus{Status: prehogv1a.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
					ConfigMethod: prehogv1a.UIDiscoverCreateDiscoveryConfigEvent_CONFIG_METHOD_AWS_EC2_SSM,
				},
			}},
		},
		{
			name: "discover create node event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverCreateNode{
				UiDiscoverCreateNode: &usageeventsv1.UIDiscoverCreateNodeEvent{
					Metadata: &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Resource: &usageeventsv1.DiscoverResourceMetadata{Resource: usageeventsv1.DiscoverResource_DISCOVER_RESOURCE_EC2_INSTANCE},
					Status:   &usageeventsv1.DiscoverStepStatus{Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_UiDiscoverCreateNode{
				UiDiscoverCreateNode: &prehogv1a.UIDiscoverCreateNodeEvent{
					Metadata: &prehogv1a.DiscoverMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
						Sso:      false,
					},
					Resource: &prehogv1a.DiscoverResourceMetadata{Resource: prehogv1a.DiscoverResource_DISCOVER_RESOURCE_EC2_INSTANCE},
					Status:   &prehogv1a.DiscoverStepStatus{Status: prehogv1a.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
		},
		{
			name: "discover create app server event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverCreateAppServerEvent{
				UiDiscoverCreateAppServerEvent: &usageeventsv1.UIDiscoverCreateAppServerEvent{
					Metadata: &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Resource: &usageeventsv1.DiscoverResourceMetadata{Resource: usageeventsv1.DiscoverResource_DISCOVER_RESOURCE_APPLICATION_AWS_CONSOLE},
					Status:   &usageeventsv1.DiscoverStepStatus{Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_UiDiscoverCreateAppServerEvent{
				UiDiscoverCreateAppServerEvent: &prehogv1a.UIDiscoverCreateAppServerEvent{
					Metadata: &prehogv1a.DiscoverMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
						Sso:      false,
					},
					Resource: &prehogv1a.DiscoverResourceMetadata{Resource: prehogv1a.DiscoverResource_DISCOVER_RESOURCE_APPLICATION_AWS_CONSOLE},
					Status:   &prehogv1a.DiscoverStepStatus{Status: prehogv1a.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
		},
		{
			name: "discover deploy eice event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverDeployEice{
				UiDiscoverDeployEice: &usageeventsv1.UIDiscoverDeployEICEEvent{
					Metadata: &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Resource: &usageeventsv1.DiscoverResourceMetadata{Resource: usageeventsv1.DiscoverResource_DISCOVER_RESOURCE_EC2_INSTANCE},
					Status:   &usageeventsv1.DiscoverStepStatus{Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_UiDiscoverDeployEice{
				UiDiscoverDeployEice: &prehogv1a.UIDiscoverDeployEICEEvent{
					Metadata: &prehogv1a.DiscoverMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
						Sso:      false,
					},
					Resource: &prehogv1a.DiscoverResourceMetadata{Resource: prehogv1a.DiscoverResource_DISCOVER_RESOURCE_EC2_INSTANCE},
					Status:   &prehogv1a.DiscoverStepStatus{Status: prehogv1a.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
		},
		{
			name: "discover ec2 instance selection event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverEc2InstanceSelection{
				UiDiscoverEc2InstanceSelection: &usageeventsv1.UIDiscoverEC2InstanceSelectionEvent{
					Metadata: &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Resource: &usageeventsv1.DiscoverResourceMetadata{Resource: usageeventsv1.DiscoverResource_DISCOVER_RESOURCE_EC2_INSTANCE},
					Status:   &usageeventsv1.DiscoverStepStatus{Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_UiDiscoverEc2InstanceSelection{
				UiDiscoverEc2InstanceSelection: &prehogv1a.UIDiscoverEC2InstanceSelectionEvent{
					Metadata: &prehogv1a.DiscoverMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
						Sso:      false,
					},
					Resource: &prehogv1a.DiscoverResourceMetadata{Resource: prehogv1a.DiscoverResource_DISCOVER_RESOURCE_EC2_INSTANCE},
					Status:   &prehogv1a.DiscoverStepStatus{Status: prehogv1a.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
		},
		{
			name: "access list create event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_AccessListCreate{
				AccessListCreate: &usageeventsv1.AccessListCreate{
					Metadata: &usageeventsv1.AccessListMetadata{
						Id: "someid",
					},
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_AccessListCreate{
				AccessListCreate: &prehogv1a.AccessListCreateEvent{
					UserName: expectedAnonymizedUserString,
					Metadata: &prehogv1a.AccessListMetadata{
						Id: expectedAnonymizedAccessListIDString,
					},
				},
			}},
		},
		{
			name: "access list update event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_AccessListUpdate{
				AccessListUpdate: &usageeventsv1.AccessListUpdate{
					Metadata: &usageeventsv1.AccessListMetadata{
						Id: "someid",
					},
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_AccessListUpdate{
				AccessListUpdate: &prehogv1a.AccessListUpdateEvent{
					UserName: expectedAnonymizedUserString,
					Metadata: &prehogv1a.AccessListMetadata{
						Id: expectedAnonymizedAccessListIDString,
					},
				},
			}},
		},
		{
			name: "access list delete event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_AccessListDelete{
				AccessListDelete: &usageeventsv1.AccessListDelete{
					Metadata: &usageeventsv1.AccessListMetadata{
						Id: "someid",
					},
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_AccessListDelete{
				AccessListDelete: &prehogv1a.AccessListDeleteEvent{
					UserName: expectedAnonymizedUserString,
					Metadata: &prehogv1a.AccessListMetadata{
						Id: expectedAnonymizedAccessListIDString,
					},
				},
			}},
		},
		{
			name: "access list member create event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_AccessListMemberCreate{
				AccessListMemberCreate: &usageeventsv1.AccessListMemberCreate{
					Metadata: &usageeventsv1.AccessListMetadata{
						Id: "someid",
					},
					MemberMetadata: &usageeventsv1.AccessListMemberMetadata{
						MembershipKind: accesslistv1.MembershipKind_MEMBERSHIP_KIND_USER,
					},
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_AccessListMemberCreate{
				AccessListMemberCreate: &prehogv1a.AccessListMemberCreateEvent{
					UserName: expectedAnonymizedUserString,
					Metadata: &prehogv1a.AccessListMetadata{
						Id: expectedAnonymizedAccessListIDString,
					},
					MemberKind: accesslist.MembershipKindUser,
				},
			}},
		},
		{
			name: "access list member upate event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_AccessListMemberUpdate{
				AccessListMemberUpdate: &usageeventsv1.AccessListMemberUpdate{
					Metadata: &usageeventsv1.AccessListMetadata{
						Id: "someid",
					},
					MemberMetadata: &usageeventsv1.AccessListMemberMetadata{
						MembershipKind: accesslistv1.MembershipKind_MEMBERSHIP_KIND_USER,
					},
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_AccessListMemberUpdate{
				AccessListMemberUpdate: &prehogv1a.AccessListMemberUpdateEvent{
					UserName: expectedAnonymizedUserString,
					Metadata: &prehogv1a.AccessListMetadata{
						Id: expectedAnonymizedAccessListIDString,
					},
					MemberKind: accesslist.MembershipKindUser,
				},
			}},
		},
		{
			name: "access list member delete event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_AccessListMemberDelete{
				AccessListMemberDelete: &usageeventsv1.AccessListMemberDelete{
					Metadata: &usageeventsv1.AccessListMetadata{
						Id: "someid",
					},
					MemberMetadata: &usageeventsv1.AccessListMemberMetadata{
						MembershipKind: accesslistv1.MembershipKind_MEMBERSHIP_KIND_USER,
					},
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_AccessListMemberDelete{
				AccessListMemberDelete: &prehogv1a.AccessListMemberDeleteEvent{
					UserName: expectedAnonymizedUserString,
					Metadata: &prehogv1a.AccessListMetadata{
						Id: expectedAnonymizedAccessListIDString,
					},
					MemberKind: accesslist.MembershipKindUser,
				},
			}},
		},
		{
			name: "access list grants to user event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_AccessListGrantsToUser{
				AccessListGrantsToUser: &usageeventsv1.AccessListGrantsToUser{
					CountRolesGranted:           5,
					CountTraitsGranted:          6,
					CountInheritedRolesGranted:  0,
					CountInheritedTraitsGranted: 0,
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_AccessListGrantsToUser{
				AccessListGrantsToUser: &prehogv1a.AccessListGrantsToUserEvent{
					UserName:                    expectedAnonymizedUserString,
					CountRolesGranted:           5,
					CountTraitsGranted:          6,
					CountInheritedRolesGranted:  0,
					CountInheritedTraitsGranted: 0,
				},
			}},
		},
		{
			name: "access list review create event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_AccessListReviewCreate{
				AccessListReviewCreate: &usageeventsv1.AccessListReviewCreate{
					Metadata: &usageeventsv1.AccessListMetadata{
						Id: "someid",
					},
					DaysPastNextAuditDate:   5,
					ReviewFrequencyChanged:  true,
					ReviewDayOfMonthChanged: false,
					NumberOfRemovedMembers:  20,
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_AccessListReviewCreate{
				AccessListReviewCreate: &prehogv1a.AccessListReviewCreateEvent{
					UserName: expectedAnonymizedUserString,
					Metadata: &prehogv1a.AccessListMetadata{
						Id: expectedAnonymizedAccessListIDString,
					},
					DaysPastNextAuditDate:   5,
					ReviewFrequencyChanged:  true,
					ReviewDayOfMonthChanged: false,
					NumberOfRemovedMembers:  20,
				},
			}},
		},
		{
			name: "access list review delete event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_AccessListReviewDelete{
				AccessListReviewDelete: &usageeventsv1.AccessListReviewDelete{
					Metadata: &usageeventsv1.AccessListMetadata{
						Id: "someid",
					},
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_AccessListReviewDelete{
				AccessListReviewDelete: &prehogv1a.AccessListReviewDeleteEvent{
					UserName: expectedAnonymizedUserString,
					Metadata: &prehogv1a.AccessListMetadata{
						Id: expectedAnonymizedAccessListIDString,
					},
				},
			},
			},
		},
		{
			name: "discovery fetch event",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_DiscoveryFetchEvent{
				DiscoveryFetchEvent: &usageeventsv1.DiscoveryFetchEvent{
					CloudProvider: "AWS",
					ResourceType:  "rds",
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_DiscoveryFetchEvent{
				DiscoveryFetchEvent: &prehogv1a.DiscoveryFetchEvent{
					CloudProvider: "AWS",
					ResourceType:  "rds",
				},
			}},
		},
		{
			name: "discover kube eks enroll",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverKubeEksEnrollEvent{
				UiDiscoverKubeEksEnrollEvent: &usageeventsv1.UIDiscoverKubeEKSEnrollEvent{
					Metadata: &usageeventsv1.DiscoverMetadata{Id: "someid"},
					Resource: &usageeventsv1.DiscoverResourceMetadata{Resource: usageeventsv1.DiscoverResource_DISCOVER_RESOURCE_KUBERNETES_EKS},
					Status:   &usageeventsv1.DiscoverStepStatus{Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_UiDiscoverKubeEksEnrollEvent{
				UiDiscoverKubeEksEnrollEvent: &prehogv1a.UIDiscoverKubeEKSEnrollEvent{
					Metadata: &prehogv1a.DiscoverMetadata{
						Id:       "someid",
						UserName: expectedAnonymizedUserString,
						Sso:      false,
					},
					Resource: &prehogv1a.DiscoverResourceMetadata{Resource: prehogv1a.DiscoverResource_DISCOVER_RESOURCE_KUBERNETES_EKS},
					Status:   &prehogv1a.DiscoverStepStatus{Status: prehogv1a.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
		},
		{
			name: "discover kube eks enroll",
			event: &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessGraphCrownJewelDiffView{
				UiAccessGraphCrownJewelDiffView: &usageeventsv1.UIAccessGraphCrownJewelDiffViewEvent{
					AffectedResourceType:   "ssh",
					AffectedResourceSource: "TELEPORT",
				},
			}},
			identityUsername: "myuser",
			errCheck:         require.NoError,
			expected: &prehogv1a.SubmitEventRequest{Event: &prehogv1a.SubmitEventRequest_UiAccessGraphCrownJewelDiffView{
				UiAccessGraphCrownJewelDiffView: &prehogv1a.UIAccessGraphCrownJewelDiffViewEvent{
					AffectedResourceType:   "ssh",
					AffectedResourceSource: "TELEPORT",
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

func TestEmitEditorChangeEvent(t *testing.T) {
	tt := []struct {
		name           string
		username       string
		prevRoles      []string
		newRoles       []string
		expectedStatus prehogv1a.EditorChangeStatus
	}{
		{
			name:           "Role is granted to user",
			username:       "user1",
			prevRoles:      []string{"role1", "role2"},
			newRoles:       []string{"role1", "role2", teleport.PresetEditorRoleName},
			expectedStatus: prehogv1a.EditorChangeStatus_EDITOR_CHANGE_STATUS_ROLE_GRANTED,
		},
		{
			name:           "Role is removed from user",
			username:       "user2",
			prevRoles:      []string{"role1", "role2", teleport.PresetEditorRoleName},
			newRoles:       []string{"role1", "role2"},
			expectedStatus: prehogv1a.EditorChangeStatus_EDITOR_CHANGE_STATUS_ROLE_REMOVED,
		},
		{
			name:      "Role remains the same",
			username:  "user3",
			prevRoles: []string{"role1", "role2", teleport.PresetEditorRoleName},
			newRoles:  []string{"role1", "role2", teleport.PresetEditorRoleName},
		},
		{
			name:      "Role is not granted or removed",
			username:  "user4",
			prevRoles: []string{"role1", "role2"},
			newRoles:  []string{"role1", "role2"},
		},
		{
			name:           "User is granted the editor role but had other roles",
			username:       "user5",
			prevRoles:      []string{"role1", "role2"},
			newRoles:       []string{"role1", "role2", teleport.PresetEditorRoleName},
			expectedStatus: prehogv1a.EditorChangeStatus_EDITOR_CHANGE_STATUS_ROLE_GRANTED,
		},
		{
			name:           "User is removed from the editor role but still has other roles",
			username:       "user6",
			prevRoles:      []string{"role1", "role2", teleport.PresetEditorRoleName},
			newRoles:       []string{"role1", "role2"},
			expectedStatus: prehogv1a.EditorChangeStatus_EDITOR_CHANGE_STATUS_ROLE_REMOVED,
		},
		{
			name:           "No previous roles, editor role granted",
			username:       "user7",
			prevRoles:      nil,
			newRoles:       []string{teleport.PresetEditorRoleName},
			expectedStatus: prehogv1a.EditorChangeStatus_EDITOR_CHANGE_STATUS_ROLE_GRANTED,
		},
		{
			name:           "Only had editor role, role removed",
			username:       "user8",
			prevRoles:      []string{teleport.PresetEditorRoleName},
			newRoles:       nil,
			expectedStatus: prehogv1a.EditorChangeStatus_EDITOR_CHANGE_STATUS_ROLE_REMOVED,
		},
		{
			name:      "Nil roles",
			username:  "user9",
			prevRoles: nil,
			newRoles:  nil,
		},
		{
			name:           "Granted multiple roles, including editor",
			username:       "user10",
			prevRoles:      []string{"role1", "role2"},
			newRoles:       []string{"role1", "role2", "role3", teleport.PresetEditorRoleName},
			expectedStatus: prehogv1a.EditorChangeStatus_EDITOR_CHANGE_STATUS_ROLE_GRANTED,
		},
		{
			name:           "Removed from multiple roles, including editor",
			username:       "user11",
			prevRoles:      []string{"role1", "role2", "role3", teleport.PresetEditorRoleName},
			newRoles:       []string{"role1", "role2"},
			expectedStatus: prehogv1a.EditorChangeStatus_EDITOR_CHANGE_STATUS_ROLE_REMOVED,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var submittedEvents []Anonymizable
			mockSubmit := func(a ...Anonymizable) {
				submittedEvents = append(submittedEvents, a...)
			}

			EmitEditorChangeEvent(tc.username, tc.prevRoles, tc.newRoles, mockSubmit)

			if tc.expectedStatus == prehogv1a.EditorChangeStatus_EDITOR_CHANGE_STATUS_ROLE_GRANTED || tc.expectedStatus == prehogv1a.EditorChangeStatus_EDITOR_CHANGE_STATUS_ROLE_REMOVED {
				require.NotEmpty(t, submittedEvents)
				event, ok := submittedEvents[0].(*EditorChangeEvent)
				require.True(t, ok, "event is not of type *EditorChangeEvent")
				require.Equal(t, tc.expectedStatus, event.Status)
				require.Equal(t, tc.username, event.UserName)
			} else {
				require.Empty(t, submittedEvents, "No event should have been submitted")
			}
		})
	}
}
