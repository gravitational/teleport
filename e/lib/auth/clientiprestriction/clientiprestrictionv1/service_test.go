package clientiprestrictionv1

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	clientiprestrictionv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clientiprestriction/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
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

func newTestService(t *testing.T, ma *mockAuthorizer, mccg *mockCloudClientGetter, mRE *eventstest.MockRecorderEmitter, entitlementEnabled bool) *Service {
	t.Helper()
	testModules := modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.ClientIPRestrictions: {Enabled: entitlementEnabled},
			},
		},
	}
	svc, err := NewService(ServiceConfig{
		Authorizer:        ma,
		Emitter:           mRE,
		CloudClientGetter: mccg,
		Modules:           &testModules,
		Logger:            logtest.NewLogger(),
	})
	if err != nil {
		t.Fatalf("error creating service: %v", err)
	}
	return svc
}

// testExpiry is a fixed future expiry timestamp used across tests.
var testExpiry = timestamppb.New(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))

func newCIR(cidrs []string) *clientiprestrictionv1pb.ClientIPRestriction {
	return clientiprestrictionv1pb.ClientIPRestriction_builder{
		Kind:    types.KindClientIPRestriction,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: types.MetaNameClientIPRestriction,
		}.Build(),
		Spec: clientiprestrictionv1pb.ClientIPRestrictionSpec_builder{
			AllowedCidrs: cidrs,
		}.Build(),
	}.Build()
}

func newCloudCIR(cidrs []string, revision string, status cloudv1.ClientIPRestrictionStatus) *cloudv1.ClientIPRestriction {
	return &cloudv1.ClientIPRestriction{
		Cidrs:    cidrs,
		Revision: revision,
		Status:   status,
	}
}

// newCloudCIRFull is like newCloudCIR but also sets the mode and expires fields.
func newCloudCIRFull(cidrs []string, revision string, status cloudv1.ClientIPRestrictionStatus, mode cloudv1.ClientIPRestrictionMode, expires *timestamppb.Timestamp) *cloudv1.ClientIPRestriction {
	return &cloudv1.ClientIPRestriction{
		Cidrs:    cidrs,
		Revision: revision,
		Status:   status,
		Mode:     mode,
		Expires:  expires,
	}
}

// newCIRWithMode builds a Teleport CIR request resource with the given cidrs,
// mode, and expiry set on the spec.
func newCIRWithMode(cidrs []string, mode string, expires *timestamppb.Timestamp) *clientiprestrictionv1pb.ClientIPRestriction {
	return clientiprestrictionv1pb.ClientIPRestriction_builder{
		Kind:    types.KindClientIPRestriction,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: types.MetaNameClientIPRestriction,
		}.Build(),
		Spec: clientiprestrictionv1pb.ClientIPRestrictionSpec_builder{
			AllowedCidrs: cidrs,
			Mode:         mode,
			Expires:      expires,
		}.Build(),
	}.Build()
}

// expectedFromCloud builds the expected Teleport CIR from cloud response fields,
// mirroring what fromCloud produces.
func expectedFromCloud(cidrs []string, revision string, statusStr string) *clientiprestrictionv1pb.ClientIPRestriction {
	return expectedFromCloudFull(cidrs, revision, statusStr, "", nil)
}

// expectedFromCloudFull is like expectedFromCloud but also sets the spec mode
// and expires fields.
func expectedFromCloudFull(cidrs []string, revision, statusStr, modeStr string, expires *timestamppb.Timestamp) *clientiprestrictionv1pb.ClientIPRestriction {
	return clientiprestrictionv1pb.ClientIPRestriction_builder{
		Kind:    types.KindClientIPRestriction,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name:     types.MetaNameClientIPRestriction,
			Revision: revision,
		}.Build(),
		Spec: clientiprestrictionv1pb.ClientIPRestrictionSpec_builder{
			AllowedCidrs: cidrs,
			Mode:         modeStr,
			Expires:      expires,
		}.Build(),
		Status: clientiprestrictionv1pb.ClientIPRestrictionStatus_builder{
			State: statusStr,
		}.Build(),
	}.Build()
}

func TestGetClientIPRestriction(t *testing.T) {
	t.Parallel()

	mccg := mockCloudClientGetter{}
	ma := mockAuthorizer{}
	mRE := eventstest.MockRecorderEmitter{}
	svc := newTestService(t, &ma, &mccg, &mRE, true)

	errCloudUnavailable := errors.New("cloud unavailable")

	testCases := []struct {
		description       string
		mockResponse      *cloudv1.GetClientIPRestrictionResponse
		mockResponseError error
		expectedError     error
		expectedCIR       *clientiprestrictionv1pb.ClientIPRestriction
	}{
		{
			description:       "cloud returns error",
			mockResponseError: errCloudUnavailable,
			expectedError:     errCloudUnavailable,
		},
		{
			description: "cloud returns restriction with active status",
			mockResponse: &cloudv1.GetClientIPRestrictionResponse{
				ClientIpRestriction: newCloudCIR(
					[]string{"10.0.0.0/8", "192.168.0.0/16"},
					"rev-1",
					cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_ACTIVE,
				),
			},
			expectedCIR: expectedFromCloud([]string{"10.0.0.0/8", "192.168.0.0/16"}, "rev-1", "active"),
		},
		{
			description: "cloud returns restriction with pending status",
			mockResponse: &cloudv1.GetClientIPRestrictionResponse{
				ClientIpRestriction: newCloudCIR(
					[]string{"10.0.0.0/8"},
					"rev-2",
					cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_PENDING,
				),
			},
			expectedCIR: expectedFromCloud([]string{"10.0.0.0/8"}, "rev-2", "pending"),
		},
		{
			description: "cloud returns nil restriction",
			mockResponse: &cloudv1.GetClientIPRestrictionResponse{
				ClientIpRestriction: nil,
			},
			expectedCIR: expectedFromCloud(nil, "", ""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Cleanup(func() { mRE.Reset() })

			mccg.getCIRResponse = tc.mockResponse
			mccg.getCIRResponseError = tc.mockResponseError

			got, err := svc.GetClientIPRestriction(t.Context(), &clientiprestrictionv1pb.GetClientIPRestrictionRequest{})

			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
			} else {
				require.NoError(t, err)
				if diff := cmp.Diff(tc.expectedCIR, got.GetClientIpRestriction(), protocmp.Transform()); diff != "" {
					t.Errorf("unexpected CIR returned (-want, +got):\n%s", diff)
				}
			}

			if len(mRE.Events()) != 0 {
				t.Errorf("Get must not emit audit events, but got %d", len(mRE.Events()))
			}
		})
	}
}

func TestCreateClientIPRestriction(t *testing.T) {
	t.Parallel()

	mccg := mockCloudClientGetter{}
	ma := mockAuthorizer{}
	mRE := eventstest.MockRecorderEmitter{}
	svc := newTestService(t, &ma, &mccg, &mRE, true)

	errAlreadyExists := errors.New("already exists")

	testCases := []struct {
		description          string
		request              *clientiprestrictionv1pb.CreateClientIPRestrictionRequest
		mockResponse         *cloudv1.CreateClientIPRestrictionResponse
		mockResponseError    error
		expectedCloudRequest *cloudv1.CreateClientIPRestrictionRequest
		expectedError        error
		expectedCIR          *clientiprestrictionv1pb.ClientIPRestriction
		expectedEventCIDRs   []string
		expectedEventMode    string
		expectedEventExpires *timestamppb.Timestamp
	}{
		{
			description:   "nil resource is rejected",
			request:       clientiprestrictionv1pb.CreateClientIPRestrictionRequest_builder{}.Build(),
			expectedError: trace.BadParameter("client_ip_restriction is required"),
		},
		{
			description: "invalid name is rejected",
			request: clientiprestrictionv1pb.CreateClientIPRestrictionRequest_builder{
				ClientIpRestriction: func() *clientiprestrictionv1pb.ClientIPRestriction {
					cir := newCIR(nil)
					cir.GetMetadata().SetName("wrong-name")
					return cir
				}(),
			}.Build(),
			expectedError: trace.BadParameter(`client_ip_restriction name must be "client-ip-restriction" or empty, got "wrong-name"`),
		},
		{
			description: "invalid metadata is rejected",
			request: clientiprestrictionv1pb.CreateClientIPRestrictionRequest_builder{
				ClientIpRestriction: func() *clientiprestrictionv1pb.ClientIPRestriction {
					cir := newCIR(nil)
					cir.GetMetadata().SetDescription("oops")
					return cir
				}(),
			}.Build(),
			expectedError: trace.BadParameter("only name and revision fields are supported on metadata for client_ip_restriction resources"),
		},
		{
			description: "cloud returns error",
			request: clientiprestrictionv1pb.CreateClientIPRestrictionRequest_builder{
				ClientIpRestriction: newCIR([]string{"10.0.0.0/8"}),
			}.Build(),
			mockResponseError:    errAlreadyExists,
			expectedCloudRequest: &cloudv1.CreateClientIPRestrictionRequest{Cidrs: []string{"10.0.0.0/8"}},
			expectedError:        errAlreadyExists,
		},
		{
			description: "successful create",
			request: clientiprestrictionv1pb.CreateClientIPRestrictionRequest_builder{
				ClientIpRestriction: newCIR([]string{"10.0.0.0/8", "172.16.0.0/12"}),
			}.Build(),
			mockResponse: &cloudv1.CreateClientIPRestrictionResponse{
				ClientIpRestriction: newCloudCIR(
					[]string{"10.0.0.0/8", "172.16.0.0/12"},
					"rev-1",
					cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_PENDING,
				),
			},
			expectedCloudRequest: &cloudv1.CreateClientIPRestrictionRequest{
				Cidrs: []string{"10.0.0.0/8", "172.16.0.0/12"},
			},
			expectedCIR:        expectedFromCloud([]string{"10.0.0.0/8", "172.16.0.0/12"}, "rev-1", "pending"),
			expectedEventCIDRs: []string{"10.0.0.0/8", "172.16.0.0/12"},
		},
		{
			description: "draft mode and expiry are forwarded and returned",
			request: clientiprestrictionv1pb.CreateClientIPRestrictionRequest_builder{
				ClientIpRestriction: newCIRWithMode([]string{"10.0.0.0/8"}, "draft", testExpiry),
			}.Build(),
			mockResponse: &cloudv1.CreateClientIPRestrictionResponse{
				ClientIpRestriction: newCloudCIRFull(
					[]string{"10.0.0.0/8"},
					"rev-1",
					cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_DRAFT,
					cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_DRAFT,
					testExpiry,
				),
			},
			expectedCloudRequest: &cloudv1.CreateClientIPRestrictionRequest{
				Cidrs:   []string{"10.0.0.0/8"},
				Mode:    cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_DRAFT,
				Expires: testExpiry,
			},
			expectedCIR:          expectedFromCloudFull([]string{"10.0.0.0/8"}, "rev-1", "draft", "draft", testExpiry),
			expectedEventCIDRs:   []string{"10.0.0.0/8"},
			expectedEventMode:    clientIPRestrictionModeDraft,
			expectedEventExpires: testExpiry,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Cleanup(func() { mRE.Reset() })

			mccg.createCIRResponse = tc.mockResponse
			mccg.createCIRResponseError = tc.mockResponseError

			got, err := svc.CreateClientIPRestriction(t.Context(), tc.request)

			if diff := cmp.Diff(tc.expectedCloudRequest, mccg.actualCreateCIRRequest, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected cloud request (-want, +got):\n%s", diff)
			}

			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				assertCIREventMatches(t, mRE.Events(), events.ClientIPRestrictionsUpdateCode, nil, "", nil)
				return
			}

			require.NoError(t, err)
			if diff := cmp.Diff(tc.expectedCIR, got.GetClientIpRestriction(), protocmp.Transform()); diff != "" {
				t.Errorf("unexpected CIR returned (-want, +got):\n%s", diff)
			}
			assertCIREventMatches(t, mRE.Events(), events.ClientIPRestrictionsUpdateCode, tc.expectedEventCIDRs, tc.expectedEventMode, tc.expectedEventExpires)
		})
	}
}

func TestUpdateClientIPRestriction(t *testing.T) {
	t.Parallel()

	mccg := mockCloudClientGetter{}
	ma := mockAuthorizer{}
	mRE := eventstest.MockRecorderEmitter{}
	svc := newTestService(t, &ma, &mccg, &mRE, true)

	errRevisionMismatch := errors.New("revision mismatch")

	testCases := []struct {
		description          string
		request              *clientiprestrictionv1pb.UpdateClientIPRestrictionRequest
		mockResponse         *cloudv1.UpdateClientIPRestrictionResponse
		mockResponseError    error
		expectedCloudRequest *cloudv1.UpdateClientIPRestrictionRequest
		expectedError        error
		expectedCIR          *clientiprestrictionv1pb.ClientIPRestriction
		expectedEventCIDRs   []string
		expectedEventMode    string
		expectedEventExpires *timestamppb.Timestamp
	}{
		{
			description:   "nil resource is rejected",
			request:       clientiprestrictionv1pb.UpdateClientIPRestrictionRequest_builder{}.Build(),
			expectedError: trace.BadParameter("client_ip_restriction is required"),
		},
		{
			description: "invalid name is rejected",
			request: clientiprestrictionv1pb.UpdateClientIPRestrictionRequest_builder{
				ClientIpRestriction: func() *clientiprestrictionv1pb.ClientIPRestriction {
					cir := newCIR(nil)
					cir.GetMetadata().SetName("wrong-name")
					return cir
				}(),
			}.Build(),
			expectedError: trace.BadParameter(`client_ip_restriction name must be "client-ip-restriction" or empty, got "wrong-name"`),
		},
		{
			description: "invalid metadata is rejected",
			request: clientiprestrictionv1pb.UpdateClientIPRestrictionRequest_builder{
				ClientIpRestriction: func() *clientiprestrictionv1pb.ClientIPRestriction {
					cir := newCIR(nil)
					cir.GetMetadata().SetLabels(map[string]string{"x": "y"})
					return cir
				}(),
			}.Build(),
			expectedError: trace.BadParameter("only name and revision fields are supported on metadata for client_ip_restriction resources"),
		},
		{
			description: "cloud returns error",
			request: clientiprestrictionv1pb.UpdateClientIPRestrictionRequest_builder{
				ClientIpRestriction: func() *clientiprestrictionv1pb.ClientIPRestriction {
					cir := newCIR([]string{"10.0.0.0/8"})
					cir.GetMetadata().SetRevision("old-rev")
					return cir
				}(),
			}.Build(),
			mockResponseError: errRevisionMismatch,
			expectedCloudRequest: &cloudv1.UpdateClientIPRestrictionRequest{
				Cidrs:    []string{"10.0.0.0/8"},
				Revision: "old-rev",
			},
			expectedError: errRevisionMismatch,
		},
		{
			description: "successful update",
			request: clientiprestrictionv1pb.UpdateClientIPRestrictionRequest_builder{
				ClientIpRestriction: func() *clientiprestrictionv1pb.ClientIPRestriction {
					cir := newCIR([]string{"10.0.0.0/8"})
					cir.GetMetadata().SetRevision("old-rev")
					return cir
				}(),
			}.Build(),
			mockResponse: &cloudv1.UpdateClientIPRestrictionResponse{
				ClientIpRestriction: newCloudCIR(
					[]string{"10.0.0.0/8"},
					"new-rev",
					cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_ACTIVE,
				),
			},
			expectedCloudRequest: &cloudv1.UpdateClientIPRestrictionRequest{
				Cidrs:    []string{"10.0.0.0/8"},
				Revision: "old-rev",
			},
			expectedCIR:        expectedFromCloud([]string{"10.0.0.0/8"}, "new-rev", "active"),
			expectedEventCIDRs: []string{"10.0.0.0/8"},
		},
		{
			description: "mode and expiry are forwarded with revision",
			request: func() *clientiprestrictionv1pb.UpdateClientIPRestrictionRequest {
				cir := newCIRWithMode([]string{"10.0.0.0/8"}, "enforced", testExpiry)
				cir.GetMetadata().SetRevision("old-rev")
				return clientiprestrictionv1pb.UpdateClientIPRestrictionRequest_builder{
					ClientIpRestriction: cir,
				}.Build()
			}(),
			mockResponse: &cloudv1.UpdateClientIPRestrictionResponse{
				ClientIpRestriction: newCloudCIRFull(
					[]string{"10.0.0.0/8"},
					"new-rev",
					cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_PENDING,
					cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_ENFORCED,
					testExpiry,
				),
			},
			expectedCloudRequest: &cloudv1.UpdateClientIPRestrictionRequest{
				Cidrs:    []string{"10.0.0.0/8"},
				Revision: "old-rev",
				Mode:     cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_ENFORCED,
				Expires:  testExpiry,
			},
			expectedCIR:          expectedFromCloudFull([]string{"10.0.0.0/8"}, "new-rev", "pending", "enforced", testExpiry),
			expectedEventCIDRs:   []string{"10.0.0.0/8"},
			expectedEventMode:    clientIPRestrictionModeEnforced,
			expectedEventExpires: testExpiry,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Cleanup(func() { mRE.Reset() })

			mccg.updateCIRResponse = tc.mockResponse
			mccg.updateCIRResponseError = tc.mockResponseError

			got, err := svc.UpdateClientIPRestriction(t.Context(), tc.request)

			if diff := cmp.Diff(tc.expectedCloudRequest, mccg.actualUpdateCIRRequest, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected cloud request (-want, +got):\n%s", diff)
			}

			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				assertCIREventMatches(t, mRE.Events(), events.ClientIPRestrictionsUpdateCode, nil, "", nil)
				return
			}

			require.NoError(t, err)
			if diff := cmp.Diff(tc.expectedCIR, got.GetClientIpRestriction(), protocmp.Transform()); diff != "" {
				t.Errorf("unexpected CIR returned (-want, +got):\n%s", diff)
			}
			assertCIREventMatches(t, mRE.Events(), events.ClientIPRestrictionsUpdateCode, tc.expectedEventCIDRs, tc.expectedEventMode, tc.expectedEventExpires)
		})
	}
}

func TestUpsertClientIPRestriction(t *testing.T) {
	t.Parallel()

	mccg := mockCloudClientGetter{}
	ma := mockAuthorizer{}
	mRE := eventstest.MockRecorderEmitter{}
	svc := newTestService(t, &ma, &mccg, &mRE, true)

	errCloudError := errors.New("cloud error")

	testCases := []struct {
		description          string
		request              *clientiprestrictionv1pb.UpsertClientIPRestrictionRequest
		mockResponse         *cloudv1.UpsertClientIPRestrictionResponse
		mockResponseError    error
		expectedCloudRequest *cloudv1.UpsertClientIPRestrictionRequest
		expectedError        error
		expectedCIR          *clientiprestrictionv1pb.ClientIPRestriction
		expectedEventCIDRs   []string
		expectedEventMode    string
		expectedEventExpires *timestamppb.Timestamp
	}{
		{
			description:   "nil resource is rejected",
			request:       clientiprestrictionv1pb.UpsertClientIPRestrictionRequest_builder{}.Build(),
			expectedError: trace.BadParameter("client_ip_restriction is required"),
		},
		{
			description: "invalid name is rejected",
			request: clientiprestrictionv1pb.UpsertClientIPRestrictionRequest_builder{
				ClientIpRestriction: func() *clientiprestrictionv1pb.ClientIPRestriction {
					cir := newCIR(nil)
					cir.GetMetadata().SetName("wrong-name")
					return cir
				}(),
			}.Build(),
			expectedError: trace.BadParameter(`client_ip_restriction name must be "client-ip-restriction" or empty, got "wrong-name"`),
		},
		{
			description: "invalid metadata is rejected",
			request: clientiprestrictionv1pb.UpsertClientIPRestrictionRequest_builder{
				ClientIpRestriction: func() *clientiprestrictionv1pb.ClientIPRestriction {
					cir := newCIR(nil)
					cir.GetMetadata().SetNamespace("ns")
					return cir
				}(),
			}.Build(),
			expectedError: trace.BadParameter("only name and revision fields are supported on metadata for client_ip_restriction resources"),
		},
		{
			description: "cloud returns error",
			request: clientiprestrictionv1pb.UpsertClientIPRestrictionRequest_builder{
				ClientIpRestriction: newCIR([]string{"10.0.0.0/8"}),
			}.Build(),
			mockResponseError:    errCloudError,
			expectedCloudRequest: &cloudv1.UpsertClientIPRestrictionRequest{Cidrs: []string{"10.0.0.0/8"}},
			expectedError:        errCloudError,
		},
		{
			description: "successful upsert",
			request: clientiprestrictionv1pb.UpsertClientIPRestrictionRequest_builder{
				ClientIpRestriction: newCIR([]string{"10.0.0.0/8", "192.168.0.0/16"}),
			}.Build(),
			mockResponse: &cloudv1.UpsertClientIPRestrictionResponse{
				ClientIpRestriction: newCloudCIR(
					[]string{"10.0.0.0/8", "192.168.0.0/16"},
					"rev-1",
					cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_PENDING,
				),
			},
			expectedCloudRequest: &cloudv1.UpsertClientIPRestrictionRequest{
				Cidrs: []string{"10.0.0.0/8", "192.168.0.0/16"},
			},
			expectedCIR:        expectedFromCloud([]string{"10.0.0.0/8", "192.168.0.0/16"}, "rev-1", "pending"),
			expectedEventCIDRs: []string{"10.0.0.0/8", "192.168.0.0/16"},
		},
		{
			description: "draft mode and expiry are forwarded",
			request: clientiprestrictionv1pb.UpsertClientIPRestrictionRequest_builder{
				ClientIpRestriction: newCIRWithMode([]string{"10.0.0.0/8"}, "draft", testExpiry),
			}.Build(),
			mockResponse: &cloudv1.UpsertClientIPRestrictionResponse{
				ClientIpRestriction: newCloudCIRFull(
					[]string{"10.0.0.0/8"},
					"rev-1",
					cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_DRAFT,
					cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_DRAFT,
					testExpiry,
				),
			},
			expectedCloudRequest: &cloudv1.UpsertClientIPRestrictionRequest{
				Cidrs:   []string{"10.0.0.0/8"},
				Mode:    cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_DRAFT,
				Expires: testExpiry,
			},
			expectedCIR:          expectedFromCloudFull([]string{"10.0.0.0/8"}, "rev-1", "draft", "draft", testExpiry),
			expectedEventCIDRs:   []string{"10.0.0.0/8"},
			expectedEventMode:    clientIPRestrictionModeDraft,
			expectedEventExpires: testExpiry,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Cleanup(func() { mRE.Reset() })

			mccg.upsertCIRResponse = tc.mockResponse
			mccg.upsertCIRResponseError = tc.mockResponseError

			got, err := svc.UpsertClientIPRestriction(t.Context(), tc.request)

			if diff := cmp.Diff(tc.expectedCloudRequest, mccg.actualUpsertCIRRequest, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected cloud request (-want, +got):\n%s", diff)
			}

			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				assertCIREventMatches(t, mRE.Events(), events.ClientIPRestrictionsUpdateCode, nil, "", nil)
				return
			}

			require.NoError(t, err)
			if diff := cmp.Diff(tc.expectedCIR, got.GetClientIpRestriction(), protocmp.Transform()); diff != "" {
				t.Errorf("unexpected CIR returned (-want, +got):\n%s", diff)
			}
			assertCIREventMatches(t, mRE.Events(), events.ClientIPRestrictionsUpdateCode, tc.expectedEventCIDRs, tc.expectedEventMode, tc.expectedEventExpires)
		})
	}
}

func TestDeleteClientIPRestriction(t *testing.T) {
	t.Parallel()

	mccg := mockCloudClientGetter{}
	ma := mockAuthorizer{}
	mRE := eventstest.MockRecorderEmitter{}
	svc := newTestService(t, &ma, &mccg, &mRE, true)

	errNotFound := errors.New("not found")

	testCases := []struct {
		description          string
		mockResponseError    error
		expectedCloudRequest *cloudv1.DeleteClientIPRestrictionRequest
		expectedError        error
	}{
		{
			description:          "cloud returns error",
			mockResponseError:    errNotFound,
			expectedCloudRequest: &cloudv1.DeleteClientIPRestrictionRequest{},
			expectedError:        errNotFound,
		},
		{
			description:          "successful delete",
			expectedCloudRequest: &cloudv1.DeleteClientIPRestrictionRequest{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Cleanup(func() { mRE.Reset() })

			mccg.deleteCIRResponse = &cloudv1.DeleteClientIPRestrictionResponse{}
			mccg.deleteCIRResponseError = tc.mockResponseError

			_, err := svc.DeleteClientIPRestriction(t.Context(), &clientiprestrictionv1pb.DeleteClientIPRestrictionRequest{})

			if diff := cmp.Diff(tc.expectedCloudRequest, mccg.actualDeleteCIRRequest, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected cloud request (-want, +got):\n%s", diff)
			}

			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
				assertCIREventMatches(t, mRE.Events(), events.ClientIPRestrictionsUpdateCode, nil, "", nil)
				return
			}

			require.NoError(t, err)
			// Delete passes nil cir to emitMutationEvent, so CIDRs, mode, and expiry are always absent.
			assertCIREventMatches(t, mRE.Events(), events.ClientIPRestrictionsUpdateCode, nil, "", nil)
		})
	}
}

func TestEntitlementRequired(t *testing.T) {
	t.Parallel()

	mccg := mockCloudClientGetter{}
	mRE := eventstest.MockRecorderEmitter{}
	svc := newTestService(t, &mockAuthorizer{}, &mccg, &mRE, false)

	testCases := []struct {
		description string
		makeRequest func(context.Context) error
	}{
		{
			description: "GetClientIPRestriction",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.GetClientIPRestriction(ctx, nil)
				return err
			},
		},
		{
			description: "CreateClientIPRestriction",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.CreateClientIPRestriction(ctx, nil)
				return err
			},
		},
		{
			description: "UpdateClientIPRestriction",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.UpdateClientIPRestriction(ctx, nil)
				return err
			},
		},
		{
			description: "UpsertClientIPRestriction",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.UpsertClientIPRestriction(ctx, nil)
				return err
			},
		},
		{
			description: "DeleteClientIPRestriction",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.DeleteClientIPRestriction(ctx, nil)
				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			err := tc.makeRequest(t.Context())
			if err == nil {
				t.Fatal("expected access denied error, got nil")
			}
			if !trace.IsAccessDenied(err) {
				t.Errorf("expected access denied, got %v", err)
			}
		})
	}
}

func TestUserAuthRequired(t *testing.T) {
	t.Parallel()

	mccg := mockCloudClientGetter{}
	mRE := eventstest.MockRecorderEmitter{}
	svc := newTestService(t, &mockAuthorizer{omitUnmappedIdentity: true}, &mccg, &mRE, true)

	testCases := []struct {
		description string
		makeRequest func(context.Context) error
	}{
		{
			description: "GetClientIPRestriction",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.GetClientIPRestriction(ctx, nil)
				return err
			},
		},
		{
			description: "CreateClientIPRestriction",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.CreateClientIPRestriction(ctx, nil)
				return err
			},
		},
		{
			description: "UpdateClientIPRestriction",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.UpdateClientIPRestriction(ctx, nil)
				return err
			},
		},
		{
			description: "UpsertClientIPRestriction",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.UpsertClientIPRestriction(ctx, nil)
				return err
			},
		},
		{
			description: "DeleteClientIPRestriction",
			makeRequest: func(ctx context.Context) error {
				_, err := svc.DeleteClientIPRestriction(ctx, nil)
				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			err := tc.makeRequest(t.Context())
			if err == nil {
				t.Fatal("expected connection problem, got nil")
			}
			if !trace.IsConnectionProblem(err) {
				t.Errorf("expected connection problem, got %v", err)
			}
			if len(mRE.Events()) != 0 {
				t.Errorf("expected no audit events for unauthenticated user, got %d", len(mRE.Events()))
			}
		})
	}
}

func TestFromCloud(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description string
		input       *cloudv1.ClientIPRestriction
		expected    *clientiprestrictionv1pb.ClientIPRestriction
	}{
		{
			description: "nil input returns empty singleton",
			input:       nil,
			expected: clientiprestrictionv1pb.ClientIPRestriction_builder{
				Kind:    types.KindClientIPRestriction,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: types.MetaNameClientIPRestriction,
				}.Build(),
				Spec:   &clientiprestrictionv1pb.ClientIPRestrictionSpec{},
				Status: &clientiprestrictionv1pb.ClientIPRestrictionStatus{},
			}.Build(),
		},
		{
			description: "active status maps to 'active'",
			input: newCloudCIR(
				[]string{"10.0.0.0/8"},
				"rev-1",
				cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_ACTIVE,
			),
			expected: expectedFromCloud([]string{"10.0.0.0/8"}, "rev-1", "active"),
		},
		{
			description: "pending status maps to 'pending'",
			input: newCloudCIR(
				[]string{"192.168.0.0/16"},
				"rev-2",
				cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_PENDING,
			),
			expected: expectedFromCloud([]string{"192.168.0.0/16"}, "rev-2", "pending"),
		},
		{
			description: "unspecified status maps to 'unknown'",
			input: newCloudCIR(
				[]string{},
				"rev-3",
				cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_UNSPECIFIED,
			),
			expected: expectedFromCloud([]string{}, "rev-3", "unknown"),
		},
		{
			description: "draft status maps to 'draft'",
			input: newCloudCIR(
				[]string{"10.0.0.0/8"},
				"rev-draft",
				cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_DRAFT,
			),
			expected: expectedFromCloud([]string{"10.0.0.0/8"}, "rev-draft", "draft"),
		},
		{
			description: "expired status maps to 'expired'",
			input: newCloudCIR(
				[]string{"10.0.0.0/8"},
				"rev-expired",
				cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_EXPIRED,
			),
			expected: expectedFromCloud([]string{"10.0.0.0/8"}, "rev-expired", "expired"),
		},
		{
			// A lapsed test run: Cloud leaves mode enforced and keeps the elapsed expiry, so both must
			// survive the conversion. The status is what tells the user it is no longer enforced.
			description: "lapsed test run keeps enforced mode and the elapsed expiry",
			input: newCloudCIRFull(
				[]string{"10.0.0.0/8"},
				"rev-lapsed",
				cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_EXPIRED,
				cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_ENFORCED,
				testExpiry,
			),
			expected: expectedFromCloudFull([]string{"10.0.0.0/8"}, "rev-lapsed", "expired", "enforced", testExpiry),
		},
		{
			description: "draft mode and expiry are preserved",
			input: newCloudCIRFull(
				[]string{"10.0.0.0/8"},
				"rev-5",
				cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_DRAFT,
				cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_DRAFT,
				testExpiry,
			),
			expected: expectedFromCloudFull([]string{"10.0.0.0/8"}, "rev-5", "draft", "draft", testExpiry),
		},
		{
			description: "enforced mode maps to 'enforced'",
			input: newCloudCIRFull(
				[]string{"10.0.0.0/8"},
				"rev-6",
				cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_ACTIVE,
				cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_ENFORCED,
				nil,
			),
			expected: expectedFromCloudFull([]string{"10.0.0.0/8"}, "rev-6", "active", "enforced", nil),
		},
		{
			description: "unspecified mode maps to empty string",
			input: newCloudCIRFull(
				[]string{"10.0.0.0/8"},
				"rev-7",
				cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_ACTIVE,
				cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_UNSPECIFIED,
				nil,
			),
			expected: expectedFromCloudFull([]string{"10.0.0.0/8"}, "rev-7", "active", "", nil),
		},
		{
			description: "multiple CIDRs preserved",
			input: newCloudCIR(
				[]string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
				"rev-4",
				cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_ACTIVE,
			),
			expected: expectedFromCloud(
				[]string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
				"rev-4",
				"active",
			),
		},
		{
			description: "revision is preserved",
			input: newCloudCIR(
				nil,
				"specific-revision-abc123",
				cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_ACTIVE,
			),
			expected: expectedFromCloud(nil, "specific-revision-abc123", "active"),
		},
		{
			description: "kind and version are always set correctly",
			input:       newCloudCIR(nil, "", cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_UNSPECIFIED),
			expected: clientiprestrictionv1pb.ClientIPRestriction_builder{
				Kind:    types.KindClientIPRestriction,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: types.MetaNameClientIPRestriction,
				}.Build(),
				Spec:   clientiprestrictionv1pb.ClientIPRestrictionSpec_builder{}.Build(),
				Status: clientiprestrictionv1pb.ClientIPRestrictionStatus_builder{State: "unknown"}.Build(),
			}.Build(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			got := fromCloud(tc.input)
			if diff := cmp.Diff(tc.expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("fromCloud mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestValidateClientIPRestriction(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description   string
		input         *clientiprestrictionv1pb.ClientIPRestriction
		expectedError error
	}{
		{
			description:   "nil resource is rejected",
			input:         nil,
			expectedError: trace.BadParameter("client_ip_restriction is required"),
		},
		{
			description: "empty name is accepted",
			input:       newCIR(nil),
		},
		{
			description: "canonical name is accepted",
			input: func() *clientiprestrictionv1pb.ClientIPRestriction {
				cir := newCIR(nil)
				cir.GetMetadata().SetName(types.MetaNameClientIPRestriction)
				return cir
			}(),
		},
		{
			description: "arbitrary name is rejected",
			input: func() *clientiprestrictionv1pb.ClientIPRestriction {
				cir := newCIR(nil)
				cir.GetMetadata().SetName("my-custom-name")
				return cir
			}(),
			expectedError: trace.BadParameter(`client_ip_restriction name must be "client-ip-restriction" or empty, got "my-custom-name"`),
		},
		{
			description: "description is rejected",
			input: func() *clientiprestrictionv1pb.ClientIPRestriction {
				cir := newCIR(nil)
				cir.GetMetadata().SetDescription("some description")
				return cir
			}(),
			expectedError: trace.BadParameter("only name and revision fields are supported on metadata for client_ip_restriction resources"),
		},
		{
			description: "expires is rejected",
			input: func() *clientiprestrictionv1pb.ClientIPRestriction {
				cir := newCIR(nil)
				cir.GetMetadata().SetExpires(timestamppb.Now())
				return cir
			}(),
			expectedError: trace.BadParameter("only name and revision fields are supported on metadata for client_ip_restriction resources"),
		},
		{
			description: "labels are rejected",
			input: func() *clientiprestrictionv1pb.ClientIPRestriction {
				cir := newCIR(nil)
				cir.GetMetadata().SetLabels(map[string]string{"env": "prod"})
				return cir
			}(),
			expectedError: trace.BadParameter("only name and revision fields are supported on metadata for client_ip_restriction resources"),
		},
		{
			description: "namespace is rejected",
			input: func() *clientiprestrictionv1pb.ClientIPRestriction {
				cir := newCIR(nil)
				cir.GetMetadata().SetNamespace("default")
				return cir
			}(),
			expectedError: trace.BadParameter("only name and revision fields are supported on metadata for client_ip_restriction resources"),
		},
		{
			description: "revision is accepted",
			input: func() *clientiprestrictionv1pb.ClientIPRestriction {
				cir := newCIR([]string{"10.0.0.0/8"})
				cir.GetMetadata().SetRevision("rev-abc")
				return cir
			}(),
		},
		{
			description: "draft mode is accepted",
			input:       newCIRWithMode([]string{"10.0.0.0/8"}, "draft", nil),
		},
		{
			description: "enforced mode is accepted",
			input:       newCIRWithMode([]string{"10.0.0.0/8"}, "enforced", nil),
		},
		{
			description: "spec expires is accepted",
			input:       newCIRWithMode([]string{"10.0.0.0/8"}, "draft", testExpiry),
		},
		{
			description:   "invalid mode is rejected",
			input:         newCIRWithMode([]string{"10.0.0.0/8"}, "bogus", nil),
			expectedError: trace.BadParameter(`client_ip_restriction mode must be "draft", "enforced", or empty, got "bogus"`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			err := validateClientIPRestriction(tc.input)
			if tc.expectedError != nil {
				require.ErrorIs(t, err, tc.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCloudStatusToString(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    cloudv1.ClientIPRestrictionStatus
		expected string
	}{
		{"active", cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_ACTIVE, "active"},
		{"pending", cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_PENDING, "pending"},
		{"draft", cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_DRAFT, "draft"},
		{"expired", cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_EXPIRED, "expired"},
		{"unspecified", cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_UNSPECIFIED, "unknown"},
		{"out of range", cloudv1.ClientIPRestrictionStatus(99), "unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, cloudStatusToString(tc.input))
		})
	}
}

func TestModeToCloud(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    string
		expected cloudv1.ClientIPRestrictionMode
	}{
		{"draft", "draft", cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_DRAFT},
		{"enforced", "enforced", cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_ENFORCED},
		{"empty", "", cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_UNSPECIFIED},
		{"bogus", "bogus", cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_UNSPECIFIED},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, modeToCloud(tc.input))
		})
	}
}

func TestCloudModeToString(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    cloudv1.ClientIPRestrictionMode
		expected string
	}{
		{"draft", cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_DRAFT, "draft"},
		{"enforced", cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_ENFORCED, "enforced"},
		{"unspecified", cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_UNSPECIFIED, ""},
		{"out of range", cloudv1.ClientIPRestrictionMode(99), ""},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, cloudModeToString(tc.input))
		})
	}
}

func TestModeCloudRoundTrip(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		mode string
	}{
		{"empty", ""},
		{"draft", "draft"},
		{"enforced", "enforced"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.mode, cloudModeToString(modeToCloud(tc.mode)))
		})
	}
}

// --- mocks ---

type mockCloudClientGetter struct {
	getCIRResponse         *cloudv1.GetClientIPRestrictionResponse
	getCIRResponseError    error
	createCIRResponse      *cloudv1.CreateClientIPRestrictionResponse
	createCIRResponseError error
	updateCIRResponse      *cloudv1.UpdateClientIPRestrictionResponse
	updateCIRResponseError error
	upsertCIRResponse      *cloudv1.UpsertClientIPRestrictionResponse
	upsertCIRResponseError error
	deleteCIRResponse      *cloudv1.DeleteClientIPRestrictionResponse
	deleteCIRResponseError error

	actualGetCIRRequest    *cloudv1.GetClientIPRestrictionRequest
	actualCreateCIRRequest *cloudv1.CreateClientIPRestrictionRequest
	actualUpdateCIRRequest *cloudv1.UpdateClientIPRestrictionRequest
	actualUpsertCIRRequest *cloudv1.UpsertClientIPRestrictionRequest
	actualDeleteCIRRequest *cloudv1.DeleteClientIPRestrictionRequest
}

func (m *mockCloudClientGetter) GetCloudClient() cloudv1.TenantsServiceClient {
	return &mockTenantsClient{mockCloudClientGetter: m}
}

type mockTenantsClient struct {
	cloudv1.TenantsServiceClient
	mockCloudClientGetter *mockCloudClientGetter
}

func (m *mockTenantsClient) GetClientIPRestriction(ctx context.Context, req *cloudv1.GetClientIPRestrictionRequest, _ ...grpc.CallOption) (*cloudv1.GetClientIPRestrictionResponse, error) {
	m.mockCloudClientGetter.actualGetCIRRequest = req
	return m.mockCloudClientGetter.getCIRResponse, m.mockCloudClientGetter.getCIRResponseError
}

func (m *mockTenantsClient) CreateClientIPRestriction(ctx context.Context, req *cloudv1.CreateClientIPRestrictionRequest, _ ...grpc.CallOption) (*cloudv1.CreateClientIPRestrictionResponse, error) {
	m.mockCloudClientGetter.actualCreateCIRRequest = req
	return m.mockCloudClientGetter.createCIRResponse, m.mockCloudClientGetter.createCIRResponseError
}

func (m *mockTenantsClient) UpdateClientIPRestriction(ctx context.Context, req *cloudv1.UpdateClientIPRestrictionRequest, _ ...grpc.CallOption) (*cloudv1.UpdateClientIPRestrictionResponse, error) {
	m.mockCloudClientGetter.actualUpdateCIRRequest = req
	return m.mockCloudClientGetter.updateCIRResponse, m.mockCloudClientGetter.updateCIRResponseError
}

func (m *mockTenantsClient) UpsertClientIPRestriction(ctx context.Context, req *cloudv1.UpsertClientIPRestrictionRequest, _ ...grpc.CallOption) (*cloudv1.UpsertClientIPRestrictionResponse, error) {
	m.mockCloudClientGetter.actualUpsertCIRRequest = req
	return m.mockCloudClientGetter.upsertCIRResponse, m.mockCloudClientGetter.upsertCIRResponseError
}

func (m *mockTenantsClient) DeleteClientIPRestriction(ctx context.Context, req *cloudv1.DeleteClientIPRestrictionRequest, _ ...grpc.CallOption) (*cloudv1.DeleteClientIPRestrictionResponse, error) {
	m.mockCloudClientGetter.actualDeleteCIRRequest = req
	return m.mockCloudClientGetter.deleteCIRResponse, m.mockCloudClientGetter.deleteCIRResponseError
}

type mockAuthorizer struct {
	omitUnmappedIdentity bool
}

func (m *mockAuthorizer) Authorize(_ context.Context) (*authz.Context, error) {
	authCtx := authz.Context{
		AdminActionAuthState: authz.AdminActionAuthMFAVerifiedWithReuse,
		Checker:              &accessChecker{},
	}
	if !m.omitUnmappedIdentity {
		authCtx.UnmappedIdentity = authz.LocalUser{}
	}
	return &authCtx, nil
}

type accessChecker struct {
	services.AccessChecker
}

func (ac *accessChecker) CheckAccessToRule(_ services.RuleContext, _, _, _ string) error {
	return nil
}

func assertCIREventMatches(t *testing.T, emittedEvents []apievents.AuditEvent, expectedCode string, expectedCIDRs []string, expectedMode string, expectedExpires *timestamppb.Timestamp) {
	t.Helper()
	if len(emittedEvents) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(emittedEvents))
	}
	e := emittedEvents[0]
	if e.GetType() != events.ClientIPRestrictionsUpdateEvent {
		t.Errorf("expected event type %s, got %s", events.ClientIPRestrictionsUpdateEvent, e.GetType())
	}
	if e.GetCode() != expectedCode {
		t.Errorf("expected event code %s, got %s", expectedCode, e.GetCode())
	}
	cirEvent, ok := e.(*apievents.ClientIPRestrictionsUpdate)
	if !ok {
		t.Fatalf("expected event to be *apievents.ClientIPRestrictionsUpdate, got %T", e)
	}
	if diff := cmp.Diff(expectedCIDRs, cirEvent.ClientIPRestrictions); diff != "" {
		t.Errorf("unexpected CIDRs in audit event (-want, +got):\n%s", diff)
	}
	if cirEvent.Mode != expectedMode {
		t.Errorf("expected mode %q in audit event, got %q", expectedMode, cirEvent.Mode)
	}
	var expectedExpiresTime time.Time
	if expectedExpires != nil {
		expectedExpiresTime = expectedExpires.AsTime()
	}
	if !cirEvent.EnforcementExpires.Equal(expectedExpiresTime) {
		t.Errorf("expected enforcement expiry %v in audit event, got %v", expectedExpiresTime, cirEvent.EnforcementExpires)
	}
}
