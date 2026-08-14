package workloadclusterv1

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadclusterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadcluster/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	cloudv1 "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestCreateWorkloadCluster(t *testing.T) {
	t.Parallel()

	testModules := modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.WorkloadClusters: {Enabled: true},
			},
			Cloud: true,
		},
	}

	mccg := mockCloudClientGetter{}

	ma := mockAuthorizer{}

	mRE := eventstest.MockRecorderEmitter{}

	svc, err := NewService(ServiceConfig{
		Authorizer:        &ma,
		Emitter:           &mRE,
		CloudClientGetter: &mccg,
		Modules:           &testModules,
		Logger:            logtest.NewLogger(),
	})
	if err != nil {
		t.Fatalf("error creating service: %v", err)
	}

	testCases := []struct {
		description string
		// request to provide to CreateWorkloadCluster
		request *workloadclusterv1.CreateWorkloadClusterRequest

		// mock response from Teleport Cloud's CreateChildCluster RPC
		mockResponse      *cloudv1.CreateChildClusterResponse
		mockResponseError error

		// expected request to be made to Teleport Cloud's CreateChildCluster RPC
		expectedCloudRequest *cloudv1.CreateChildClusterRequest

		// if set then expect an error
		expectedError string
		// expected result from CreateWorkloadCluster
		expectedWorkloadCluster *workloadclusterv1.WorkloadCluster
	}{
		{
			description:   "missing cluster configuration",
			expectedError: "name is required",
		},
		{
			description: "missing regions",
			request: workloadclusterv1.CreateWorkloadClusterRequest_builder{
				Cluster: workloadclusterv1.WorkloadCluster_builder{
					Metadata: headerv1.Metadata_builder{
						Name: "test",
					}.Build(),
				}.Build(),
			}.Build(),
			expectedError: "regions is required",
		},
		{
			description: "metadata.expires provided",
			request: workloadclusterv1.CreateWorkloadClusterRequest_builder{
				Cluster: workloadclusterv1.WorkloadCluster_builder{
					Metadata: headerv1.Metadata_builder{
						Name:    "test",
						Expires: timestamppb.Now(),
					}.Build(),
				}.Build(),
			}.Build(),
			expectedError: "only name and revision fields are supported on metadata for workload_cluster resources",
		},
		{
			description: "metadata.description provided",
			request: workloadclusterv1.CreateWorkloadClusterRequest_builder{
				Cluster: workloadclusterv1.WorkloadCluster_builder{
					Metadata: headerv1.Metadata_builder{
						Name:        "test",
						Description: "description",
					}.Build(),
				}.Build(),
			}.Build(),
			expectedError: "only name and revision fields are supported on metadata for workload_cluster resources",
		},
		{
			description: "metadata.namespace provided",
			request: workloadclusterv1.CreateWorkloadClusterRequest_builder{
				Cluster: workloadclusterv1.WorkloadCluster_builder{
					Metadata: headerv1.Metadata_builder{
						Name:      "test",
						Namespace: "namespace",
					}.Build(),
				}.Build(),
			}.Build(),
			expectedError: "only name and revision fields are supported on metadata for workload_cluster resources",
		},
		{
			description: "metadata.labels provided",
			request: workloadclusterv1.CreateWorkloadClusterRequest_builder{
				Cluster: workloadclusterv1.WorkloadCluster_builder{
					Metadata: headerv1.Metadata_builder{
						Name: "test",
						Labels: map[string]string{
							"test": "test",
						},
					}.Build(),
				}.Build(),
			}.Build(),
			expectedError: "only name and revision fields are supported on metadata for workload_cluster resources",
		},
		{
			description: "error response from ChildCluster RPC",
			request: workloadclusterv1.CreateWorkloadClusterRequest_builder{
				Cluster: workloadclusterv1.WorkloadCluster_builder{
					Metadata: headerv1.Metadata_builder{
						Name: "test",
					}.Build(),
					Spec: workloadclusterv1.WorkloadClusterSpec_builder{
						Regions: []*workloadclusterv1.Region{
							workloadclusterv1.Region_builder{
								Name: "auth_region",
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
			mockResponseError: errors.New("unable to create"),
			expectedCloudRequest: &cloudv1.CreateChildClusterRequest{
				Name: "test",
				Regions: []*cloudv1.Region{
					{
						Name: "auth_region",
					},
				},
			},
			expectedError: "unable to create",
		},
		{
			description: "successful response from CreateChildCluster RPC",
			request: workloadclusterv1.CreateWorkloadClusterRequest_builder{
				Cluster: workloadclusterv1.WorkloadCluster_builder{
					Metadata: headerv1.Metadata_builder{
						Name: "test",
					}.Build(),
					Spec: workloadclusterv1.WorkloadClusterSpec_builder{
						Regions: []*workloadclusterv1.Region{
							workloadclusterv1.Region_builder{
								Name: "auth_region",
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
			mockResponse: &cloudv1.CreateChildClusterResponse{
				Cluster: &cloudv1.ChildCluster{
					Name:     "test",
					Revision: "revision",
					Spec: &cloudv1.ChildClusterSpec{
						Regions: []*cloudv1.Region{
							{
								Name: "auth_region",
							},
						},
					},
					Status: &cloudv1.ChildClusterStatus{
						State:  "creating",
						Domain: "test.teleport.sh",
					},
				},
			},
			expectedCloudRequest: &cloudv1.CreateChildClusterRequest{
				Name: "test",
				Regions: []*cloudv1.Region{
					{
						Name: "auth_region",
					},
				},
			},
			expectedWorkloadCluster: workloadclusterv1.WorkloadCluster_builder{
				Version: types.V1,
				Kind:    types.KindWorkloadCluster,
				Metadata: headerv1.Metadata_builder{
					Name:     "test",
					Revision: "revision",
				}.Build(),
				Spec: workloadclusterv1.WorkloadClusterSpec_builder{
					Regions: []*workloadclusterv1.Region{
						workloadclusterv1.Region_builder{
							Name: "auth_region",
						}.Build(),
					},
				}.Build(),
				Status: workloadclusterv1.WorkloadClusterStatus_builder{
					State:  "creating",
					Domain: "test.teleport.sh",
				}.Build(),
			}.Build(),
		},
		{
			description: "successful response from ChildCluster RPC with all fields set",
			request: workloadclusterv1.CreateWorkloadClusterRequest_builder{
				Cluster: workloadclusterv1.WorkloadCluster_builder{
					Metadata: headerv1.Metadata_builder{
						Name: "test",
					}.Build(),
					Spec: workloadclusterv1.WorkloadClusterSpec_builder{
						Regions: []*workloadclusterv1.Region{
							workloadclusterv1.Region_builder{
								Name: "auth_region",
							}.Build(),
						},
						Bot: workloadclusterv1.Bot_builder{
							Name: "bot-name",
						}.Build(),
						Token: workloadclusterv1.Token_builder{
							JoinMethod: "iam",
							Allow: []*workloadclusterv1.Allow{
								workloadclusterv1.Allow_builder{
									AwsAccount: "aws-account",
									AwsArn:     "aws-arn",
								}.Build(),
							},
						}.Build(),
					}.Build(),
				}.Build(),
			}.Build(),
			mockResponse: &cloudv1.CreateChildClusterResponse{
				Cluster: &cloudv1.ChildCluster{
					Name:     "test",
					Revision: "revision",
					Spec: &cloudv1.ChildClusterSpec{
						Allow: []*cloudv1.Allow{
							{
								AwsAccount: "aws-account",
								AwsArn:     "aws-arn",
							},
						},
						BotName:    "bot-name",
						JoinMethod: "iam",
						Regions: []*cloudv1.Region{
							{
								Name: "auth_region",
							},
						},
					},
					Status: &cloudv1.ChildClusterStatus{
						State:  "active",
						Domain: "test.teleport.sh",
					},
				},
			},
			expectedCloudRequest: &cloudv1.CreateChildClusterRequest{
				Name: "test",
				Regions: []*cloudv1.Region{
					{
						Name: "auth_region",
					},
				},
				BotName:    "bot-name",
				JoinMethod: "iam",
				Allow: []*cloudv1.Allow{
					{
						AwsAccount: "aws-account",
						AwsArn:     "aws-arn",
					},
				},
			},
			expectedWorkloadCluster: workloadclusterv1.WorkloadCluster_builder{
				Version: types.V1,
				Kind:    types.KindWorkloadCluster,
				Metadata: headerv1.Metadata_builder{
					Name:     "test",
					Revision: "revision",
				}.Build(),
				Spec: workloadclusterv1.WorkloadClusterSpec_builder{
					Regions: []*workloadclusterv1.Region{
						workloadclusterv1.Region_builder{
							Name: "auth_region",
						}.Build(),
					},
					Bot: workloadclusterv1.Bot_builder{
						Name: "bot-name",
					}.Build(),
					Token: workloadclusterv1.Token_builder{
						JoinMethod: "iam",
						Allow: []*workloadclusterv1.Allow{
							workloadclusterv1.Allow_builder{
								AwsAccount: "aws-account",
								AwsArn:     "aws-arn",
							}.Build(),
						},
					}.Build(),
				}.Build(),
				Status: workloadclusterv1.WorkloadClusterStatus_builder{
					State:  "active",
					Domain: "test.teleport.sh",
				}.Build(),
			}.Build(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Cleanup(func() {
				mRE.Reset()
			})

			mccg.createChildClustersResponse = tc.mockResponse
			mccg.createChildClustersResponseError = tc.mockResponseError

			cc, err := svc.CreateWorkloadCluster(t.Context(), tc.request)

			if diff := cmp.Diff(tc.expectedCloudRequest, mccg.actualCreateChildClusterRequest, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected request made (-want, +got):%s\n", diff)
			}

			if tc.expectedError == "" && err != nil {
				t.Fatalf("unexpected error creating workload cluster: %v", err)
			}

			if tc.expectedError != "" {
				if err == nil {
					t.Fatal("expected an error, but got nil")
				}

				if tc.expectedError != err.Error() {
					t.Errorf("expected error to be %q, but got %q", tc.expectedError, err.Error())
				}

				if tc.request == nil || !tc.request.HasCluster() || !tc.request.GetCluster().HasMetadata() {
					return
				}

				assertEventMatches(t, mRE.Events(), events.WorkloadClusterCreateEvent, events.WorkloadClusterCreateFailureCode)

				return
			}

			if diff := cmp.Diff(tc.expectedWorkloadCluster, cc, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected workload cluster configuration returned (-want, +got):%s\n", diff)
			}

			assertEventMatches(t, mRE.Events(), events.WorkloadClusterCreateEvent, events.WorkloadClusterCreateCode)
		})
	}
}

func TestGetWorkloadCluster(t *testing.T) {
	t.Parallel()

	testModules := modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.WorkloadClusters: {Enabled: true},
			},
			Cloud: true,
		},
	}

	mccg := mockCloudClientGetter{}

	ma := mockAuthorizer{}

	mRE := eventstest.MockRecorderEmitter{}

	svc, err := NewService(ServiceConfig{
		Authorizer:        &ma,
		Emitter:           &mRE,
		CloudClientGetter: &mccg,
		Modules:           &testModules,
		Logger:            logtest.NewLogger(),
	})
	if err != nil {
		t.Fatalf("error creating service: %v", err)
	}

	testCases := []struct {
		description string
		// request to provide to GetWorkloadCluster
		request *workloadclusterv1.GetWorkloadClusterRequest

		// mock response from Teleport Cloud's GetChildCluster RPC
		mockResponse      *cloudv1.GetChildClusterResponse
		mockResponseError error

		// expected request to be made to Teleport Cloud's GetChildCluster RPC
		expectedCloudRequest *cloudv1.GetChildClusterRequest

		// if set then expect an error
		expectedError string
		// expected result from GetWorkloadCluster
		expectedWorkloadCluster *workloadclusterv1.WorkloadCluster
	}{
		{
			description: "workload cluster doesn't exist",
			request: workloadclusterv1.GetWorkloadClusterRequest_builder{
				Name: "example",
			}.Build(),
			mockResponseError: errors.New(`workload_cluster "example" doesn't exist`),
			expectedCloudRequest: &cloudv1.GetChildClusterRequest{
				Name: "example",
			},
			expectedError: `workload_cluster "example" doesn't exist`,
		},
		{
			description: "workload cluster exists",
			request: workloadclusterv1.GetWorkloadClusterRequest_builder{
				Name: "test",
			}.Build(),
			mockResponse: &cloudv1.GetChildClusterResponse{
				Cluster: &cloudv1.ChildCluster{
					Name:     "test",
					Revision: "revision",
					Spec: &cloudv1.ChildClusterSpec{
						Regions: []*cloudv1.Region{
							{
								Name: "auth_region",
							},
						},
					},
					Status: &cloudv1.ChildClusterStatus{
						State:  "creating",
						Domain: "test.teleport.sh",
					},
				},
			},
			expectedCloudRequest: &cloudv1.GetChildClusterRequest{
				Name: "test",
			},
			expectedWorkloadCluster: workloadclusterv1.WorkloadCluster_builder{
				Version: types.V1,
				Kind:    types.KindWorkloadCluster,
				Metadata: headerv1.Metadata_builder{
					Name:     "test",
					Revision: "revision",
				}.Build(),
				Spec: workloadclusterv1.WorkloadClusterSpec_builder{
					Regions: []*workloadclusterv1.Region{
						workloadclusterv1.Region_builder{
							Name: "auth_region",
						}.Build(),
					},
				}.Build(),
				Status: workloadclusterv1.WorkloadClusterStatus_builder{
					State:  "creating",
					Domain: "test.teleport.sh",
				}.Build(),
			}.Build(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Cleanup(func() {
				mRE.Reset()
			})

			mccg.getChildClustersResponse = tc.mockResponse
			mccg.getChildClustersResponseError = tc.mockResponseError

			cc, err := svc.GetWorkloadCluster(t.Context(), tc.request)

			if diff := cmp.Diff(tc.expectedCloudRequest, mccg.actualGetChildClusterRequest, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected request made (-want, +got):%s\n", diff)
			}

			if len(mRE.Events()) != 0 {
				t.Errorf("expected zero audit events to be emitted, but %d events were emitted", len(mRE.Events()))
			}

			if tc.expectedError == "" && err != nil {
				t.Fatalf("unexpected error getting workload cluster: %v", err)
			}

			if tc.expectedError != "" {
				if err == nil {
					t.Fatal("expected an error, but got nil")
				}

				if tc.expectedError != err.Error() {
					t.Errorf("expected error to be %q, but got %q", tc.expectedError, err.Error())
				}

				return
			}

			if diff := cmp.Diff(tc.expectedWorkloadCluster, cc, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected workload cluster configuration returned (-want, +got):%s\n", diff)
			}
		})
	}
}

func TestUpdateWorkloadCluster(t *testing.T) {
	t.Parallel()

	testModules := modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.WorkloadClusters: {Enabled: true},
			},
			Cloud: true,
		},
	}

	mccg := mockCloudClientGetter{}

	ma := mockAuthorizer{}

	mRE := eventstest.MockRecorderEmitter{}

	svc, err := NewService(ServiceConfig{
		Authorizer:        &ma,
		Emitter:           &mRE,
		CloudClientGetter: &mccg,
		Modules:           &testModules,
		Logger:            logtest.NewLogger(),
	})
	if err != nil {
		t.Fatalf("error creating service: %v", err)
	}

	testCases := []struct {
		description string
		// request to provide to UpdateWorkloadCluster
		request *workloadclusterv1.UpdateWorkloadClusterRequest

		// mock response from Teleport Cloud's UpdateChildCluster RPC
		mockResponse      *cloudv1.UpdateChildClusterResponse
		mockResponseError error

		// expected request to be made to Teleport Cloud's UpdateChildCluster RPC
		expectedCloudRequest *cloudv1.UpdateChildClusterRequest

		// if set then expect an error
		expectedError string

		// expected result from UpsertWorkloadClusterRequest
		expectedWorkloadCluster *workloadclusterv1.WorkloadCluster
	}{
		{
			description:   "empty request",
			expectedError: "name is required",
		},
		{
			description: "handles error from Cloud API",
			request: workloadclusterv1.UpdateWorkloadClusterRequest_builder{
				Cluster: workloadclusterv1.WorkloadCluster_builder{
					Metadata: headerv1.Metadata_builder{
						Name: "example",
					}.Build(),
					Spec: workloadclusterv1.WorkloadClusterSpec_builder{
						Regions: []*workloadclusterv1.Region{
							workloadclusterv1.Region_builder{
								Name: "us-east-1",
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
			mockResponseError: errors.New("invalid configuration"),
			expectedCloudRequest: &cloudv1.UpdateChildClusterRequest{
				Name: "example",
				Regions: []*cloudv1.Region{
					{
						Name: "us-east-1",
					},
				},
			},
			expectedError: `invalid configuration`,
		},
		{
			description: "handles success from Cloud API",
			request: workloadclusterv1.UpdateWorkloadClusterRequest_builder{
				Cluster: workloadclusterv1.WorkloadCluster_builder{
					Metadata: headerv1.Metadata_builder{
						Name:     "test",
						Revision: "old-revision",
					}.Build(),
					Spec: workloadclusterv1.WorkloadClusterSpec_builder{
						Regions: []*workloadclusterv1.Region{
							workloadclusterv1.Region_builder{
								Name: "auth_region",
							}.Build(),
						},
					}.Build(),
					Status: workloadclusterv1.WorkloadClusterStatus_builder{
						State:  "creating",
						Domain: "test.teleport.sh",
					}.Build(),
				}.Build(),
			}.Build(),
			mockResponse: &cloudv1.UpdateChildClusterResponse{
				Cluster: &cloudv1.ChildCluster{
					Name:     "test",
					Revision: "new-revision",
					Spec: &cloudv1.ChildClusterSpec{
						Regions: []*cloudv1.Region{
							{
								Name: "auth_region",
							},
						},
					},
					Status: &cloudv1.ChildClusterStatus{
						State:  "creating",
						Domain: "test.teleport.sh",
					},
				},
			},
			expectedCloudRequest: &cloudv1.UpdateChildClusterRequest{
				Name:     "test",
				Revision: "old-revision",
				Regions: []*cloudv1.Region{
					{
						Name: "auth_region",
					},
				},
			},
			expectedWorkloadCluster: workloadclusterv1.WorkloadCluster_builder{
				Version: types.V1,
				Kind:    types.KindWorkloadCluster,
				Metadata: headerv1.Metadata_builder{
					Name:     "test",
					Revision: "new-revision",
				}.Build(),
				Spec: workloadclusterv1.WorkloadClusterSpec_builder{
					Regions: []*workloadclusterv1.Region{
						workloadclusterv1.Region_builder{
							Name: "auth_region",
						}.Build(),
					},
				}.Build(),
				Status: workloadclusterv1.WorkloadClusterStatus_builder{
					State:  "creating",
					Domain: "test.teleport.sh",
				}.Build(),
			}.Build(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Cleanup(func() {
				mRE.Reset()
			})

			mccg.updateChildClustersResponse = tc.mockResponse
			mccg.updateChildClustersResponseError = tc.mockResponseError

			updatedWC, err := svc.UpdateWorkloadCluster(t.Context(), tc.request)

			if diff := cmp.Diff(tc.expectedCloudRequest, mccg.actualUpdateChildClusterRequest, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected request made (-want, +got):%s\n", diff)
			}

			if tc.expectedError == "" && err != nil {
				t.Fatalf("unexpected error upserting workload cluster: %v", err)
			}

			if tc.expectedError != "" {
				if err == nil {
					t.Fatal("expected an error, but got nil")
				}

				if tc.expectedError != err.Error() {
					t.Errorf("expected error to be %q, but got %q", tc.expectedError, err.Error())
				}

				assertEventMatches(t, mRE.Events(), events.WorkloadClusterUpdateEvent, events.WorkloadClusterUpdateFailureCode)

				return
			}

			if diff := cmp.Diff(tc.expectedWorkloadCluster, updatedWC, protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")); diff != "" {
				t.Errorf("unexpected workload cluster configuration returned (-want, +got):%s\n", diff)
			}

			assertEventMatches(t, mRE.Events(), events.WorkloadClusterUpdateEvent, events.WorkloadClusterUpdateCode)
		})
	}
}

func TestUpsertWorkloadCluster(t *testing.T) {
	t.Parallel()

	testModules := modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.WorkloadClusters: {Enabled: true},
			},
			Cloud: true,
		},
	}

	mccg := mockCloudClientGetter{}

	ma := mockAuthorizer{}

	mRE := eventstest.MockRecorderEmitter{}

	svc, err := NewService(ServiceConfig{
		Authorizer:        &ma,
		Emitter:           &mRE,
		CloudClientGetter: &mccg,
		Modules:           &testModules,
		Logger:            logtest.NewLogger(),
	})
	if err != nil {
		t.Fatalf("error creating service: %v", err)
	}

	testCases := []struct {
		description string
		// request to provide to UpsertWorkloadCluster
		request *workloadclusterv1.UpsertWorkloadClusterRequest

		// mock response from Teleport Cloud's UpsertChildCluster RPC
		mockResponse      *cloudv1.UpsertChildClusterResponse
		mockResponseError error

		// expected request to be made to Teleport Cloud's UpsertChildCluster RPC
		expectedCloudRequest *cloudv1.UpsertChildClusterRequest

		// if set then expect an error
		expectedError string

		// expected result from UpsertWorkloadClusterRequest
		expectedWorkloadCluster *workloadclusterv1.WorkloadCluster
	}{
		{
			description:   "empty request",
			expectedError: "name is required",
		},
		{
			description: "handles error from Cloud API",
			request: workloadclusterv1.UpsertWorkloadClusterRequest_builder{
				Cluster: workloadclusterv1.WorkloadCluster_builder{
					Metadata: headerv1.Metadata_builder{
						Name:     "example",
						Revision: "revision",
					}.Build(),
					Spec: workloadclusterv1.WorkloadClusterSpec_builder{
						Regions: []*workloadclusterv1.Region{
							workloadclusterv1.Region_builder{
								Name: "us-west-2",
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
			mockResponseError: errors.New("invalid configuration"),
			expectedCloudRequest: &cloudv1.UpsertChildClusterRequest{
				Name: "example",
				Regions: []*cloudv1.Region{
					{
						Name: "us-west-2",
					},
				},
			},
			expectedError: `invalid configuration`,
		},
		{
			description: "handles success from Cloud API",
			request: workloadclusterv1.UpsertWorkloadClusterRequest_builder{
				Cluster: workloadclusterv1.WorkloadCluster_builder{
					Metadata: headerv1.Metadata_builder{
						Name: "test",
					}.Build(),
					Spec: workloadclusterv1.WorkloadClusterSpec_builder{
						Regions: []*workloadclusterv1.Region{
							workloadclusterv1.Region_builder{
								Name: "auth_region",
							}.Build(),
						},
					}.Build(),
					Status: workloadclusterv1.WorkloadClusterStatus_builder{
						State:  "creating",
						Domain: "test.teleport.sh",
					}.Build(),
				}.Build(),
			}.Build(),
			mockResponse: &cloudv1.UpsertChildClusterResponse{
				Cluster: &cloudv1.ChildCluster{
					Name:     "test",
					Revision: "revision",
					Spec: &cloudv1.ChildClusterSpec{
						Regions: []*cloudv1.Region{
							{
								Name: "auth_region",
							},
						},
					},
					Status: &cloudv1.ChildClusterStatus{
						State:  "creating",
						Domain: "test.teleport.sh",
					},
				},
			},
			expectedCloudRequest: &cloudv1.UpsertChildClusterRequest{
				Name: "test",
				Regions: []*cloudv1.Region{
					{
						Name: "auth_region",
					},
				},
			},
			expectedWorkloadCluster: workloadclusterv1.WorkloadCluster_builder{
				Version: types.V1,
				Kind:    types.KindWorkloadCluster,
				Metadata: headerv1.Metadata_builder{
					Name:     "test",
					Revision: "revision",
				}.Build(),
				Spec: workloadclusterv1.WorkloadClusterSpec_builder{
					Regions: []*workloadclusterv1.Region{
						workloadclusterv1.Region_builder{
							Name: "auth_region",
						}.Build(),
					},
				}.Build(),
				Status: workloadclusterv1.WorkloadClusterStatus_builder{
					State:  "creating",
					Domain: "test.teleport.sh",
				}.Build(),
			}.Build(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Cleanup(func() {
				mRE.Reset()
			})

			mccg.upsertChildClustersResponse = tc.mockResponse
			mccg.upsertChildClustersResponseError = tc.mockResponseError

			upsertedWC, err := svc.UpsertWorkloadCluster(t.Context(), tc.request)

			if diff := cmp.Diff(tc.expectedCloudRequest, mccg.actualUpsertChildClusterRequest, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected request made (-want, +got):%s\n", diff)
			}

			if tc.expectedError == "" && err != nil {
				t.Fatalf("unexpected error upserting workload cluster: %v", err)
			}

			if tc.expectedError != "" {
				if err == nil {
					t.Fatal("expected an error, but got nil")
				}

				if tc.expectedError != err.Error() {
					t.Errorf("expected error to be %q, but got %q", tc.expectedError, err.Error())
				}

				assertEventMatches(t, mRE.Events(), events.WorkloadClusterUpdateEvent, events.WorkloadClusterUpdateFailureCode)

				return
			}

			if diff := cmp.Diff(tc.expectedWorkloadCluster, upsertedWC, protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")); diff != "" {
				t.Errorf("unexpected workload cluster configuration returned (-want, +got):%s\n", diff)
			}

			assertEventMatches(t, mRE.Events(), events.WorkloadClusterUpdateEvent, events.WorkloadClusterUpdateCode)
		})
	}
}

func TestListWorkloadClusters(t *testing.T) {
	t.Parallel()

	testModules := modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.WorkloadClusters: {Enabled: true},
			},
			Cloud: true,
		},
	}

	mccg := mockCloudClientGetter{}

	ma := mockAuthorizer{}

	mRE := eventstest.MockRecorderEmitter{}

	svc, err := NewService(ServiceConfig{
		Authorizer:        &ma,
		Emitter:           &mRE,
		CloudClientGetter: &mccg,
		Modules:           &testModules,
		Logger:            logtest.NewLogger(),
	})
	if err != nil {
		t.Fatalf("error creating service: %v", err)
	}

	testCases := []struct {
		description string
		// request to provide to ListWorkloadClusters
		request *workloadclusterv1.ListWorkloadClustersRequest

		// mock response from Teleport Cloud's ListChildClusters RPC
		mockResponse      *cloudv1.ListChildClustersResponse
		mockResponseError error

		// expected request to be made to Teleport Cloud's ListChildClusters RPC
		expectedCloudRequest *cloudv1.ListChildClustersRequest

		// if set then expect an error
		expectedError string

		// expected result from ListWorkloadClusters
		expectedWorkloadClusters *workloadclusterv1.ListWorkloadClustersResponse
	}{
		{
			description: "handles error from Cloud API",
			request: workloadclusterv1.ListWorkloadClustersRequest_builder{
				PageSize: 10,
			}.Build(),
			mockResponseError: errors.New("error"),
			expectedCloudRequest: &cloudv1.ListChildClustersRequest{
				PageSize: 10,
			},
			expectedError: "error",
		},
		{
			description: "list workload clusters",
			request: workloadclusterv1.ListWorkloadClustersRequest_builder{
				PageSize:  3,
				PageToken: "test-token",
			}.Build(),
			expectedCloudRequest: &cloudv1.ListChildClustersRequest{
				PageSize:  3,
				PageToken: "test-token",
			},
			mockResponse: &cloudv1.ListChildClustersResponse{
				Clusters: []*cloudv1.ChildCluster{
					{
						Name:     "example",
						Revision: "revision",
						Spec: &cloudv1.ChildClusterSpec{
							Regions: []*cloudv1.Region{
								{
									Name: "auth_region",
								},
							},
						},
						Status: &cloudv1.ChildClusterStatus{
							State:  "active",
							Domain: "example.teleport.sh",
						},
					},
					{
						Name:     "test",
						Revision: "revision",
						Spec: &cloudv1.ChildClusterSpec{
							Regions: []*cloudv1.Region{
								{
									Name: "auth_region",
								},
							},
						},
						Status: &cloudv1.ChildClusterStatus{
							State:  "creating",
							Domain: "test.teleport.sh",
						},
					},
				},
				NextPageToken: "next-token",
			},
			expectedWorkloadClusters: workloadclusterv1.ListWorkloadClustersResponse_builder{
				NextPageToken: "next-token",
				Clusters: []*workloadclusterv1.WorkloadCluster{
					workloadclusterv1.WorkloadCluster_builder{
						Version: types.V1,
						Kind:    types.KindWorkloadCluster,
						Metadata: headerv1.Metadata_builder{
							Name:     "example",
							Revision: "revision",
						}.Build(),
						Spec: workloadclusterv1.WorkloadClusterSpec_builder{
							Regions: []*workloadclusterv1.Region{
								workloadclusterv1.Region_builder{
									Name: "auth_region",
								}.Build(),
							},
						}.Build(),
						Status: workloadclusterv1.WorkloadClusterStatus_builder{
							Domain: "example.teleport.sh",
							State:  "active",
						}.Build(),
					}.Build(),
					workloadclusterv1.WorkloadCluster_builder{
						Version: types.V1,
						Kind:    types.KindWorkloadCluster,
						Metadata: headerv1.Metadata_builder{
							Name:     "test",
							Revision: "revision",
						}.Build(),
						Spec: workloadclusterv1.WorkloadClusterSpec_builder{
							Regions: []*workloadclusterv1.Region{
								workloadclusterv1.Region_builder{
									Name: "auth_region",
								}.Build(),
							},
						}.Build(),
						Status: workloadclusterv1.WorkloadClusterStatus_builder{
							Domain: "test.teleport.sh",
							State:  "creating",
						}.Build(),
					}.Build(),
				},
			}.Build(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Cleanup(func() {
				mRE.Reset()
			})

			mccg.listChildClustersResponse = tc.mockResponse
			mccg.listChildClustersResponseError = tc.mockResponseError

			cc, err := svc.ListWorkloadClusters(t.Context(), tc.request)
			if diff := cmp.Diff(tc.expectedCloudRequest, mccg.actualListChildClustersRequest, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected request made (-want, +got):%s\n", diff)
			}

			if len(mRE.Events()) != 0 {
				t.Errorf("expected zero audit events to be emitted, but %d events were emitted", len(mRE.Events()))
			}

			if tc.expectedError == "" && err != nil {
				t.Fatalf("unexpected error listing workload cluster: %v", err)
			}
			if tc.expectedError != "" {
				if err == nil {
					t.Fatalf("expected an error, but got nil")
				}

				if tc.expectedError != err.Error() {
					t.Errorf("expected error to be %q, but got %q", tc.expectedError, err.Error())
				}

				return
			}

			if diff := cmp.Diff(tc.expectedWorkloadClusters, cc, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected workload cluster configuration returned (-want, +got):%s\n", diff)
			}
		})
	}
}

func TestDeleteWorkloadCluster(t *testing.T) {
	t.Parallel()

	testModules := modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.WorkloadClusters: {Enabled: true},
			},
			Cloud: true,
		},
	}

	mccg := mockCloudClientGetter{}

	ma := mockAuthorizer{}

	mRE := eventstest.MockRecorderEmitter{}

	svc, err := NewService(ServiceConfig{
		Authorizer:        &ma,
		Emitter:           &mRE,
		CloudClientGetter: &mccg,
		Modules:           &testModules,
		Logger:            logtest.NewLogger(),
	})
	if err != nil {
		t.Fatalf("error creating service: %v", err)
	}

	testCases := []struct {
		description string
		// request to provide to DeleteWorkloadCluster
		request *workloadclusterv1.DeleteWorkloadClusterRequest

		// mock response from Teleport Cloud's ChildCluster RPC
		mockResponse      *cloudv1.SuspendChildClusterResponse
		mockResponseError error

		// expected request to be made to Teleport Cloud's ChildCluster RPC
		expectedChildClusterRequest *cloudv1.SuspendChildClusterRequest

		// if set then expect an error
		expectedError string
	}{
		{
			description: "handles error from Cloud API",
			request: workloadclusterv1.DeleteWorkloadClusterRequest_builder{
				Name: "test",
			}.Build(),
			mockResponseError: errors.New("error suspending in cloud"),
			expectedChildClusterRequest: &cloudv1.SuspendChildClusterRequest{
				Name: "test",
			},
			expectedError: "error suspending in cloud",
		},
		{
			description: "handles success from Cloud API",
			request: workloadclusterv1.DeleteWorkloadClusterRequest_builder{
				Name: "test",
			}.Build(),
			mockResponse: &cloudv1.SuspendChildClusterResponse{
				Cluster: &cloudv1.ChildCluster{
					Revision: "revision",
					Status: &cloudv1.ChildClusterStatus{
						State:  "suspended",
						Domain: "test.teleport.sh",
					},
				},
			},
			expectedChildClusterRequest: &cloudv1.SuspendChildClusterRequest{
				Name: "test",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Cleanup(func() {
				mRE.Reset()
			})

			mccg.suspendChildClustersResponse = tc.mockResponse
			mccg.suspendChildClustersResponseError = tc.mockResponseError

			_, err = svc.DeleteWorkloadCluster(t.Context(), tc.request)

			if diff := cmp.Diff(tc.expectedChildClusterRequest, mccg.actualSuspendChildClusterRequest, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected child cluster request made (-want, +got):%s\n", diff)
			}

			if tc.expectedError == "" && err != nil {
				t.Fatalf("unexpected error deleting workload cluster: %v", err)
			}

			if tc.expectedError != "" {
				if err == nil {
					t.Fatal("expected an error, but got nil")
				}

				if tc.expectedError != err.Error() {
					t.Errorf("expected error to be %q, but got %q", tc.expectedError, err.Error())
				}

				assertEventMatches(t, mRE.Events(), events.WorkloadClusterDeleteEvent, events.WorkloadClusterDeleteFailureCode)

				return
			}

			assertEventMatches(t, mRE.Events(), events.WorkloadClusterDeleteEvent, events.WorkloadClusterDeleteCode)
		})
	}
}

func TestEntitlementRequired(t *testing.T) {
	t.Parallel()

	testModules := modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			// workload cluster feature is not enabled
			Cloud: true,
		},
	}

	svc, err := NewService(ServiceConfig{
		Authorizer:        &mockAuthorizer{},
		Emitter:           &eventstest.MockRecorderEmitter{},
		CloudClientGetter: &mockCloudClientGetter{},
		Modules:           &testModules,
		Logger:            logtest.NewLogger(),
	})
	if err != nil {
		t.Fatalf("error creating service: %v", err)
	}

	testCases := []struct {
		description string

		makeRequest func(context.Context) error
	}{
		{
			description: "CreateWorkloadCluster",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.CreateWorkloadCluster(ctx, nil)

				return err
			},
		},
		{
			description: "GetWorkloadCluster",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.GetWorkloadCluster(ctx, nil)

				return err
			},
		},
		{
			description: "UpdateWorkloadCluster",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.UpdateWorkloadCluster(ctx, nil)

				return err
			},
		},
		{
			description: "UpsertWorkloadCluster",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.UpsertWorkloadCluster(ctx, nil)

				return err
			},
		},
		{
			description: "DeleteWorkloadCluster",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.DeleteWorkloadCluster(ctx, nil)

				return err
			},
		},
		{
			description: "ListWorkloadClusters",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.ListWorkloadClusters(ctx, nil)

				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			err := tc.makeRequest(t.Context())
			if err == nil {
				t.Fatal("expected an error that entitlement is disabled, but got nil")
			}

			if !trace.IsAccessDenied(err) {
				t.Errorf("expected error to be access denied, but got %v", err)
			}
		})
	}
}

func TestUserAuthRequired(t *testing.T) {
	t.Parallel()

	testModules := modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.WorkloadClusters: {Enabled: true},
			},
			Cloud: true,
		},
	}

	mRE := eventstest.MockRecorderEmitter{}

	svc, err := NewService(ServiceConfig{
		Authorizer: &mockAuthorizer{
			omitUnmappedIdentity: true,
		},
		Emitter:           &mRE,
		CloudClientGetter: &mockCloudClientGetter{},
		Modules:           &testModules,
		Logger:            logtest.NewLogger(),
	})
	if err != nil {
		t.Fatalf("error creating service: %v", err)
	}

	testCases := []struct {
		description string

		makeRequest func(context.Context) error
	}{
		{
			description: "CreateWorkloadCluster",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.CreateWorkloadCluster(ctx, nil)

				return err
			},
		},
		{
			description: "GetWorkloadCluster",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.GetWorkloadCluster(ctx, nil)

				return err
			},
		},
		{
			description: "UpdateWorkloadCluster",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.UpdateWorkloadCluster(ctx, nil)

				return err
			},
		},
		{
			description: "UpsertWorkloadCluster",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.UpsertWorkloadCluster(ctx, nil)

				return err
			},
		},
		{
			description: "DeleteWorkloadCluster",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.DeleteWorkloadCluster(ctx, nil)

				return err
			},
		},
		{
			description: "ListWorkloadClusters",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.ListWorkloadClusters(ctx, nil)

				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			err := tc.makeRequest(t.Context())
			if err == nil {
				t.Fatal("expected an error that API is only for users, but got nil")
			}

			if !trace.IsConnectionProblem(err) {
				t.Errorf("expected error to be connection problem, but got %v", err)
			}

			if len(mRE.Events()) != 0 {
				t.Errorf("expected zero audit events to be emitted when not an authenticated user, but got %d events", len(mRE.Events()))
			}
		})
	}
}

type mockCloudClientGetter struct {
	// mock response to return
	createChildClustersResponse       *cloudv1.CreateChildClusterResponse
	createChildClustersResponseError  error
	getChildClustersResponse          *cloudv1.GetChildClusterResponse
	getChildClustersResponseError     error
	updateChildClustersResponse       *cloudv1.UpdateChildClusterResponse
	updateChildClustersResponseError  error
	upsertChildClustersResponse       *cloudv1.UpsertChildClusterResponse
	upsertChildClustersResponseError  error
	suspendChildClustersResponse      *cloudv1.SuspendChildClusterResponse
	suspendChildClustersResponseError error
	listChildClustersResponse         *cloudv1.ListChildClustersResponse
	listChildClustersResponseError    error

	// the request made that can be asserted against for tests
	actualCreateChildClusterRequest  *cloudv1.CreateChildClusterRequest
	actualGetChildClusterRequest     *cloudv1.GetChildClusterRequest
	actualUpdateChildClusterRequest  *cloudv1.UpdateChildClusterRequest
	actualUpsertChildClusterRequest  *cloudv1.UpsertChildClusterRequest
	actualSuspendChildClusterRequest *cloudv1.SuspendChildClusterRequest
	actualListChildClustersRequest   *cloudv1.ListChildClustersRequest
}

func (m *mockCloudClientGetter) GetCloudClient() cloudv1.TenantsServiceClient {
	mtsc := mockTenantsClient{
		mockCloudClientGetter: m,
	}

	return &mtsc
}

type mockTenantsClient struct {
	cloudv1.TenantsServiceClient

	mockCloudClientGetter *mockCloudClientGetter
}

func (m *mockTenantsClient) CreateChildCluster(ctx context.Context, req *cloudv1.CreateChildClusterRequest, opts ...grpc.CallOption) (*cloudv1.CreateChildClusterResponse, error) {
	m.mockCloudClientGetter.actualCreateChildClusterRequest = req

	return m.mockCloudClientGetter.createChildClustersResponse, m.mockCloudClientGetter.createChildClustersResponseError
}

func (m *mockTenantsClient) GetChildCluster(ctx context.Context, req *cloudv1.GetChildClusterRequest, opts ...grpc.CallOption) (*cloudv1.GetChildClusterResponse, error) {
	m.mockCloudClientGetter.actualGetChildClusterRequest = req

	return m.mockCloudClientGetter.getChildClustersResponse, m.mockCloudClientGetter.getChildClustersResponseError
}

func (m *mockTenantsClient) UpdateChildCluster(ctx context.Context, req *cloudv1.UpdateChildClusterRequest, opts ...grpc.CallOption) (*cloudv1.UpdateChildClusterResponse, error) {
	m.mockCloudClientGetter.actualUpdateChildClusterRequest = req

	return m.mockCloudClientGetter.updateChildClustersResponse, m.mockCloudClientGetter.updateChildClustersResponseError
}

func (m *mockTenantsClient) UpsertChildCluster(ctx context.Context, req *cloudv1.UpsertChildClusterRequest, opts ...grpc.CallOption) (*cloudv1.UpsertChildClusterResponse, error) {
	m.mockCloudClientGetter.actualUpsertChildClusterRequest = req

	return m.mockCloudClientGetter.upsertChildClustersResponse, m.mockCloudClientGetter.upsertChildClustersResponseError
}

func (m *mockTenantsClient) SuspendChildCluster(ctx context.Context, req *cloudv1.SuspendChildClusterRequest, opts ...grpc.CallOption) (*cloudv1.SuspendChildClusterResponse, error) {
	m.mockCloudClientGetter.actualSuspendChildClusterRequest = req

	return m.mockCloudClientGetter.suspendChildClustersResponse, m.mockCloudClientGetter.suspendChildClustersResponseError
}

func (m *mockTenantsClient) ListChildClusters(ctx context.Context, req *cloudv1.ListChildClustersRequest, opts ...grpc.CallOption) (*cloudv1.ListChildClustersResponse, error) {
	m.mockCloudClientGetter.actualListChildClustersRequest = req

	return m.mockCloudClientGetter.listChildClustersResponse, m.mockCloudClientGetter.listChildClustersResponseError
}

type mockAuthorizer struct {
	omitUnmappedIdentity bool
}

func (m *mockAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	ac := accessChecker{}

	authCtx := authz.Context{
		AdminActionAuthState: authz.AdminActionAuthMFAVerifiedWithReuse,
		Checker:              &ac,
	}

	if !m.omitUnmappedIdentity {
		authCtx.UnmappedIdentity = authz.LocalUser{}
	}

	return &authCtx, nil
}

type accessChecker struct {
	services.AccessChecker
}

func (ac *accessChecker) CheckAccessToRule(ctx services.RuleContext, namespace, rule, verb string) error {
	return nil
}

func assertEventMatches(t *testing.T, events []apievents.AuditEvent, expectedType string, expectedCode string) {
	t.Helper()

	if len(events) != 1 {
		t.Fatalf("expected one audit event to be emitted, but %d events were emitted", len(events))
	}

	e := events[0]
	if e.GetType() != expectedType {
		t.Errorf("expected audit event type to be %s, but got %s", expectedType, e.GetType())
	}
	if e.GetCode() != expectedCode {
		t.Errorf("expected audit event code to be %s, but got %s", expectedCode, e.GetCode())
	}
}
