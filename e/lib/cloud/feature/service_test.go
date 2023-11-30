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

func TestRun_UsageBased(t *testing.T) {
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
				Kubernetes:   true,
				IsUsageBased: true,
				ProductType:  cloudapi.PRODUCT_TYPE_TEAM,
			}, nil
		},
	)

	// Run the service.
	go service.Run(ctx)
	fakeClock.BlockUntil(1)

	// Check if the features are stored in the backend.
	requireFeatures(t, fakeClock, backend, ctx, modules.Features{
		Kubernetes:          true,
		DeviceTrust:         GetUsageBasedDeviceTrustFeatureLimits(),
		AccessRequests:      GetUsageBasedAccessRequestFeatureLimits(),
		AccessList:          GetUsageBasedAccessListFeatureLimits(),
		IsUsageBasedBilling: true,
		ProductType:         modules.ProductTypeTeam,
	})

	// update features again and see if they are stored in the backend
	mockCloudClient.setMockGetFeatures(
		func(ctx context.Context, r *cloudapi.EmptyRequest) (*cloudapi.GetFeaturesResponse, error) {
			return &cloudapi.GetFeaturesResponse{
				Kubernetes:   false,
				App:          true,
				IsUsageBased: true,
				ProductType:  cloudapi.PRODUCT_TYPE_TEAM,
			}, nil
		},
	)
	// check backend again
	wantFeatures := modules.Features{
		Kubernetes:          false,
		App:                 true,
		DeviceTrust:         GetUsageBasedDeviceTrustFeatureLimits(),
		AccessRequests:      GetUsageBasedAccessRequestFeatureLimits(),
		AccessList:          GetUsageBasedAccessListFeatureLimits(),
		IsUsageBasedBilling: true,
		ProductType:         modules.ProductTypeTeam,
	}
	requireFeatures(t, fakeClock, backend, ctx, wantFeatures)

	// Test that the service wont crash if it receives an error
	mockCloudClient.setMockGetFeatures(
		func(ctx context.Context, r *cloudapi.EmptyRequest) (*cloudapi.GetFeaturesResponse, error) {
			return nil, errors.New("err fetching features")
		},
	)
	requireFeatures(t, fakeClock, backend, ctx, wantFeatures)

	// Make sure it can recover after a failed request
	mockCloudClient.setMockGetFeatures(
		func(ctx context.Context, r *cloudapi.EmptyRequest) (*cloudapi.GetFeaturesResponse, error) {
			return &cloudapi.GetFeaturesResponse{
				Db:           true,
				IsUsageBased: true,
				ProductType:  cloudapi.PRODUCT_TYPE_EUB,
			}, nil
		},
	)
	requireFeatures(t, fakeClock, backend, ctx, modules.Features{
		DB:                      true,
		DeviceTrust:             GetUsageBasedDeviceTrustFeatureLimits(),
		AccessRequests:          GetUsageBasedAccessRequestFeatureLimits(),
		AccessList:              GetUsageBasedAccessListFeatureLimits(),
		IsUsageBasedBilling:     true,
		ProductType:             modules.ProductTypeEUB,
		AdvancedAccessWorkflows: true,
		HSM:                     true,
		OIDC:                    true,
		AccessControls:          true,
		SAML:                    true,
	})

	// Test removing limit "after upgrade", which in this case
	// we go from product "eub" to "eub with igs".
	mockCloudClient.setMockGetFeatures(
		func(ctx context.Context, r *cloudapi.EmptyRequest) (*cloudapi.GetFeaturesResponse, error) {
			return &cloudapi.GetFeaturesResponse{
				IsUsageBased:               true,
				ProductType:                cloudapi.PRODUCT_TYPE_EUB,
				IdentityGovernanceSecurity: true,
			}, nil
		},
	)
	requireFeatures(t, fakeClock, backend, ctx, modules.Features{
		DeviceTrust: modules.DeviceTrustFeature{
			Enabled:           true, // always enabled
			DevicesUsageLimit: 0,
		},
		AccessRequests: modules.AccessRequestsFeature{
			MonthlyRequestLimit: 0,
		},
		AccessList: modules.AccessListFeature{
			CreateLimit: 0,
		},
		IsUsageBasedBilling:        true,
		IdentityGovernanceSecurity: true,
		AdvancedAccessWorkflows:    true,
		HSM:                        true,
		OIDC:                       true,
		AccessControls:             true,
		SAML:                       true,
		ProductType:                modules.ProductTypeEUB,
	})
}

func TestRun_Legacy_NonUsageBased(t *testing.T) {
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Despite getting feature response, teleport should still hard code
	// features.
	mockCloudClient.setMockGetFeatures(
		func(ctx context.Context, r *cloudapi.EmptyRequest) (*cloudapi.GetFeaturesResponse, error) {
			return &cloudapi.GetFeaturesResponse{
				Kubernetes:     false, // should be ignored
				AccessRequests: false, // should be ignored
				App:            false, // should be ignored
				// The two fields below are the only ones
				// modifiable.
				FeatureHiding: true,
				CustomTheme:   "llama-theme",
			}, nil
		},
	)

	// Run the service.
	go service.Run(ctx)
	fakeClock.BlockUntil(1)

	requireFeatures(t, fakeClock, backend, ctx, modules.Features{
		Kubernetes:              true,
		App:                     true,
		DB:                      true,
		Desktop:                 true,
		Cloud:                   true,
		OIDC:                    true,
		SAML:                    true,
		AccessControls:          true,
		AdvancedAccessWorkflows: true,
		HSM:                     true,
		RecoveryCodes:           true,
		FeatureHiding:           true,
		CustomTheme:             "llama-theme",
		Assist:                  false,
		DeviceTrust: modules.DeviceTrustFeature{
			Enabled: true,
		},
	})
}

func newMemoryBackend(t *testing.T) backend.Backend {
	b, err := memory.New(memory.Config{})
	require.NoError(t, err)
	return b
}
