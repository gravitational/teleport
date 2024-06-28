package feature

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/e/api/cloud"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
)

type testClient struct {
	cloud.MockedClient
	mu              sync.Mutex
	mockGetFeatures func(ctx context.Context, r *cloudapi.EmptyRequest) (*cloudapi.GetFeaturesResponse, error)
}

func (t *testClient) GetFeatures(ctx context.Context, in *cloudapi.EmptyRequest, opts ...grpc.CallOption) (*cloudapi.GetFeaturesResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.mockGetFeatures != nil {
		return t.mockGetFeatures(ctx, in)
	}

	return nil, trace.NotImplemented("MockGetFeatures is not implemented")
}

func (t *testClient) setMockGetFeatures(f func(ctx context.Context, r *cloudapi.EmptyRequest) (*cloudapi.GetFeaturesResponse, error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.mockGetFeatures = f
}

// Implement cloud.Client interface for mocked client
func (t *testClient) Close() error { return nil }

func TestNewService(t *testing.T) {
	client := &testClient{}
	tt := []struct {
		name   string
		cfg    Config
		assert func(*testing.T, *Service, error)
	}{
		{
			name: "valid configuration",
			cfg: Config{
				Backend:        newMemoryBackend(t),
				CloudClient:    client,
				Interval:       1 * time.Second,
				RequestTimeout: 1 * time.Second,
			},
			assert: func(t *testing.T, s *Service, err error) {
				require.NoError(t, err)
				require.NotNil(t, s)
			},
		},
		{
			name: "missing Cloud Client",
			cfg: Config{
				Backend:        newMemoryBackend(t),
				Interval:       1 * time.Second,
				RequestTimeout: 1 * time.Second,
			},
			assert: func(t *testing.T, s *Service, err error) {
				require.Error(t, err)
				require.Nil(t, s)
			},
		},
		{
			name: "invalid Interval",
			cfg: Config{
				Backend:        newMemoryBackend(t),
				CloudClient:    client,
				Interval:       0,
				RequestTimeout: 1 * time.Second,
			},
			assert: func(t *testing.T, s *Service, err error) {
				require.Error(t, err)
				require.Nil(t, s)
			},
		},
		{
			name: "invalid RequestTimeout",
			cfg: Config{
				CloudClient:    client,
				Backend:        newMemoryBackend(t),
				Interval:       1 * time.Second,
				RequestTimeout: 0,
			},
			assert: func(t *testing.T, s *Service, err error) {
				require.Error(t, err)
				require.Nil(t, s)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			s, err := NewService(tc.cfg)
			tc.assert(t, s, err)
		})
	}
}

func requireFeatures(t *testing.T, fakeClock clockwork.FakeClock, backend backend.Backend, ctx context.Context, want modules.Features) {
	t.Helper()

	// Advance the clock so the service fetch and stores features
	fakeClock.Advance(1 * time.Second)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		item, err := backend.Get(ctx, featuresBackendKey)
		if !assert.NoError(c, err) {
			return
		}

		stored := &modules.Features{}
		err = json.Unmarshal(item.Value, stored)
		if !assert.NoError(c, err) {
			return
		}

		diff := cmp.Diff(want, *stored)
		if !assert.Empty(c, diff) {
			t.Logf("Feature diff (-want +got):\n%s", diff)
		}
	}, 1*time.Second, time.Millisecond*100)
}

func TestRun_UpdateCloudFeatures(t *testing.T) {
	t.Parallel()

	mockCloudClient := &testClient{}
	backend := newMemoryBackend(t)

	fakeClock := clockwork.NewFakeClock()
	cfg := Config{
		Backend:        backend,
		CloudClient:    mockCloudClient,
		Interval:       500 * time.Millisecond,
		RequestTimeout: 500 * time.Millisecond,
		Clock:          fakeClock,
	}
	service, err := NewService(cfg)
	require.NoError(t, err)

	// Test case: Successfully fetch and store features from the mocked Cloud client.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockCloudClient.setMockGetFeatures(
		func(ctx context.Context, r *cloudapi.EmptyRequest) (*cloudapi.GetFeaturesResponse, error) {
			return &cloudapi.GetFeaturesResponse{
				IsUsageBased: true,
				ProductType:  cloudapi.ProductType_PRODUCT_TYPE_TEAM,
				Entitlements: map[string]*cloudapi.EntitlementInfo{
					"K8s": {Enabled: true, Limit: 0},
				},
			}, nil
		},
	)

	// Run the service.
	go service.Run(ctx)
	fakeClock.BlockUntil(1)

	// Check if the features are stored in the backend.
	requireFeatures(t, fakeClock, backend, ctx, modules.Features{
		IsUsageBasedBilling: true,
		ProductType:         modules.ProductTypeTeam,
		AccessControls:      true,
		Assist:              false,
		Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			entitlements.K8s:                    {Enabled: true, Limit: 0},
			entitlements.AccessLists:            {},
			entitlements.AccessMonitoring:       {},
			entitlements.AccessRequests:         {},
			entitlements.App:                    {},
			entitlements.CloudAuditLogRetention: {},
			entitlements.DB:                     {},
			entitlements.Desktop:                {},
			entitlements.DeviceTrust:            {},
			entitlements.ExternalAuditStorage:   {},
			entitlements.FeatureHiding:          {},
			entitlements.HSM:                    {},
			entitlements.Identity:               {},
			entitlements.JoinActiveSessions:     {},
			entitlements.MobileDeviceManagement: {},
			entitlements.OIDC:                   {},
			entitlements.OktaSCIM:               {},
			entitlements.OktaUserSync:           {},
			entitlements.Policy:                 {},
			entitlements.SAML:                   {},
			entitlements.SessionLocks:           {},
			entitlements.UpsellAlert:            {},
			entitlements.UsageReporting:         {},
		},
	})

	// update features
	mockCloudClient.setMockGetFeatures(
		func(ctx context.Context, r *cloudapi.EmptyRequest) (*cloudapi.GetFeaturesResponse, error) {
			return &cloudapi.GetFeaturesResponse{
				IsUsageBased: true,
				ProductType:  cloudapi.ProductType_PRODUCT_TYPE_TEAM,
				Entitlements: map[string]*cloudapi.EntitlementInfo{
					"K8s": {Enabled: false, Limit: 0},
					"App": {Enabled: true, Limit: 0},
				},
			}, nil
		},
	)
	// check backend for updated features
	requireFeatures(t, fakeClock, backend, ctx, modules.Features{
		IsUsageBasedBilling: true,
		ProductType:         modules.ProductTypeTeam,
		AccessControls:      true,
		Assist:              false,
		Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			entitlements.App:                    {Enabled: true, Limit: 0},
			entitlements.K8s:                    {Enabled: false, Limit: 0},
			entitlements.AccessLists:            {},
			entitlements.AccessMonitoring:       {},
			entitlements.AccessRequests:         {},
			entitlements.CloudAuditLogRetention: {},
			entitlements.DB:                     {},
			entitlements.Desktop:                {},
			entitlements.DeviceTrust:            {},
			entitlements.ExternalAuditStorage:   {},
			entitlements.FeatureHiding:          {},
			entitlements.HSM:                    {},
			entitlements.Identity:               {},
			entitlements.JoinActiveSessions:     {},
			entitlements.MobileDeviceManagement: {},
			entitlements.OIDC:                   {},
			entitlements.OktaSCIM:               {},
			entitlements.OktaUserSync:           {},
			entitlements.Policy:                 {},
			entitlements.SAML:                   {},
			entitlements.SessionLocks:           {},
			entitlements.UpsellAlert:            {},
			entitlements.UsageReporting:         {},
		},
	})

	// test that the service won't crash if it receives an error
	mockCloudClient.setMockGetFeatures(
		func(ctx context.Context, r *cloudapi.EmptyRequest) (*cloudapi.GetFeaturesResponse, error) {
			return nil, errors.New("err fetching features")
		},
	)
	// assert backend has last-known features still stored after error
	requireFeatures(t, fakeClock, backend, ctx, modules.Features{
		IsUsageBasedBilling: true,
		ProductType:         modules.ProductTypeTeam,
		AccessControls:      true,
		Assist:              false,
		Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			entitlements.App:                    {Enabled: true, Limit: 0},
			entitlements.K8s:                    {Enabled: false, Limit: 0},
			entitlements.AccessLists:            {},
			entitlements.AccessMonitoring:       {},
			entitlements.AccessRequests:         {},
			entitlements.CloudAuditLogRetention: {},
			entitlements.DB:                     {},
			entitlements.Desktop:                {},
			entitlements.DeviceTrust:            {},
			entitlements.ExternalAuditStorage:   {},
			entitlements.FeatureHiding:          {},
			entitlements.HSM:                    {},
			entitlements.Identity:               {},
			entitlements.JoinActiveSessions:     {},
			entitlements.MobileDeviceManagement: {},
			entitlements.OIDC:                   {},
			entitlements.OktaSCIM:               {},
			entitlements.OktaUserSync:           {},
			entitlements.Policy:                 {},
			entitlements.SAML:                   {},
			entitlements.SessionLocks:           {},
			entitlements.UpsellAlert:            {},
			entitlements.UsageReporting:         {},
		},
	})

	// make sure the service can recover after a failed request; return a limit
	mockCloudClient.setMockGetFeatures(
		func(ctx context.Context, r *cloudapi.EmptyRequest) (*cloudapi.GetFeaturesResponse, error) {
			return &cloudapi.GetFeaturesResponse{
				IsUsageBased: true,
				ProductType:  cloudapi.ProductType_PRODUCT_TYPE_EUB,
				Entitlements: map[string]*cloudapi.EntitlementInfo{
					"DB":          {Enabled: true, Limit: 0},
					"AccessLists": {Enabled: true, Limit: 1},
				}}, nil
		},
	)
	// check backend for updated features
	requireFeatures(t, fakeClock, backend, ctx, modules.Features{
		IsUsageBasedBilling: true,
		ProductType:         modules.ProductTypeEUB,
		AccessControls:      true,
		Assist:              false,
		Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			entitlements.DB:                     {Enabled: true, Limit: 0},
			entitlements.AccessLists:            {Enabled: true, Limit: 1},
			entitlements.AccessMonitoring:       {},
			entitlements.AccessRequests:         {},
			entitlements.App:                    {},
			entitlements.CloudAuditLogRetention: {},
			entitlements.Desktop:                {},
			entitlements.DeviceTrust:            {},
			entitlements.ExternalAuditStorage:   {},
			entitlements.FeatureHiding:          {},
			entitlements.HSM:                    {},
			entitlements.Identity:               {},
			entitlements.JoinActiveSessions:     {},
			entitlements.K8s:                    {},
			entitlements.MobileDeviceManagement: {},
			entitlements.OIDC:                   {},
			entitlements.OktaSCIM:               {},
			entitlements.OktaUserSync:           {},
			entitlements.Policy:                 {},
			entitlements.SAML:                   {},
			entitlements.SessionLocks:           {},
			entitlements.UpsellAlert:            {},
			entitlements.UsageReporting:         {},
		},
	})

	// Test removing limit; limit read from response
	mockCloudClient.setMockGetFeatures(
		func(ctx context.Context, r *cloudapi.EmptyRequest) (*cloudapi.GetFeaturesResponse, error) {
			return &cloudapi.GetFeaturesResponse{
				IsUsageBased: true,
				ProductType:  cloudapi.ProductType_PRODUCT_TYPE_EUB,
				Entitlements: map[string]*cloudapi.EntitlementInfo{
					"AccessLists": {Enabled: true, Limit: 0},
				}}, nil
		},
	)
	requireFeatures(t, fakeClock, backend, ctx, modules.Features{
		IsUsageBasedBilling: true,
		ProductType:         modules.ProductTypeEUB,
		AccessControls:      true,
		Assist:              false,
		Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			entitlements.AccessLists:            {Enabled: true, Limit: 0},
			entitlements.DB:                     {},
			entitlements.AccessMonitoring:       {},
			entitlements.AccessRequests:         {},
			entitlements.App:                    {},
			entitlements.CloudAuditLogRetention: {},
			entitlements.Desktop:                {},
			entitlements.DeviceTrust:            {},
			entitlements.ExternalAuditStorage:   {},
			entitlements.FeatureHiding:          {},
			entitlements.HSM:                    {},
			entitlements.Identity:               {},
			entitlements.JoinActiveSessions:     {},
			entitlements.K8s:                    {},
			entitlements.MobileDeviceManagement: {},
			entitlements.OIDC:                   {},
			entitlements.OktaSCIM:               {},
			entitlements.OktaUserSync:           {},
			entitlements.Policy:                 {},
			entitlements.SAML:                   {},
			entitlements.SessionLocks:           {},
			entitlements.UpsellAlert:            {},
			entitlements.UsageReporting:         {},
		},
	})

	// Unknown entitlements are dropped
	mockCloudClient.setMockGetFeatures(
		func(ctx context.Context, r *cloudapi.EmptyRequest) (*cloudapi.GetFeaturesResponse, error) {
			return &cloudapi.GetFeaturesResponse{
				IsUsageBased: false,
				Entitlements: map[string]*cloudapi.EntitlementInfo{
					"Foo":     {Enabled: true, Limit: 0},
					"Bar":     {Enabled: true, Limit: 0},
					"Desktop": {Enabled: true, Limit: 0},
					"baz":     {Enabled: true, Limit: 0},
				}}, nil
		},
	)
	requireFeatures(t, fakeClock, backend, ctx, modules.Features{
		IsUsageBasedBilling: false,
		AccessControls:      true,
		Assist:              false,
		Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			entitlements.Desktop:                {Enabled: true, Limit: 0},
			entitlements.AccessLists:            {},
			entitlements.AccessMonitoring:       {},
			entitlements.AccessRequests:         {},
			entitlements.App:                    {},
			entitlements.CloudAuditLogRetention: {},
			entitlements.DB:                     {},
			entitlements.DeviceTrust:            {},
			entitlements.ExternalAuditStorage:   {},
			entitlements.FeatureHiding:          {},
			entitlements.HSM:                    {},
			entitlements.Identity:               {},
			entitlements.JoinActiveSessions:     {},
			entitlements.K8s:                    {},
			entitlements.MobileDeviceManagement: {},
			entitlements.OIDC:                   {},
			entitlements.OktaSCIM:               {},
			entitlements.OktaUserSync:           {},
			entitlements.Policy:                 {},
			entitlements.SAML:                   {},
			entitlements.SessionLocks:           {},
			entitlements.UpsellAlert:            {},
			entitlements.UsageReporting:         {},
		},
	})
}

func newMemoryBackend(t *testing.T) backend.Backend {
	b, err := memory.New(memory.Config{})
	require.NoError(t, err)
	return b
}
