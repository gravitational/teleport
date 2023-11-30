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
	v1 "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
)

type testClient struct {
	cloud.MockedClient
	mu              sync.Mutex
	mockGetFeatures func(ctx context.Context, r *v1.EmptyRequest) (*v1.GetFeaturesResponse, error)
}

func (t *testClient) GetFeatures(ctx context.Context, in *v1.EmptyRequest, opts ...grpc.CallOption) (*v1.GetFeaturesResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.mockGetFeatures != nil {
		return t.mockGetFeatures(ctx, in)
	}

	return nil, trace.NotImplemented("MockGetFeatures is not implemented")
}

func (t *testClient) setMockGetFeatures(f func(ctx context.Context, r *v1.EmptyRequest) (*v1.GetFeaturesResponse, error)) {
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
		func(ctx context.Context, r *v1.EmptyRequest) (*v1.GetFeaturesResponse, error) {
			return &v1.GetFeaturesResponse{
				Kubernetes:   true,
				IsUsageBased: true,
			}, nil
		},
	)

	// Run the service.
	go service.Run(ctx)
	fakeClock.BlockUntil(1)

	// Check if the features are stored in the backend.
	requireFeatures(t, fakeClock, backend, ctx, modules.Features{
		Kubernetes: true,
		DeviceTrust: modules.DeviceTrustFeature{
			Enabled:           true, // always enabled
			DevicesUsageLimit: 5,
		},
		AccessRequests: modules.AccessRequestsFeature{
			MonthlyRequestLimit: 5,
		},
		IsUsageBasedBilling: true,
	})

	// update features again and see if they are stored in the backend
	mockCloudClient.setMockGetFeatures(
		func(ctx context.Context, r *v1.EmptyRequest) (*v1.GetFeaturesResponse, error) {
			return &v1.GetFeaturesResponse{
				Kubernetes:   false,
				App:          true,
				IsUsageBased: true,
			}, nil
		},
	)
	// check backend again
	wantFeatures := modules.Features{
		Kubernetes: false,
		App:        true,
		DeviceTrust: modules.DeviceTrustFeature{
			Enabled:           true, // always enabled
			DevicesUsageLimit: 5,
		},
		AccessRequests: modules.AccessRequestsFeature{
			MonthlyRequestLimit: 5,
		},
		IsUsageBasedBilling: true,
	}
	requireFeatures(t, fakeClock, backend, ctx, wantFeatures)

	// Test that the service wont crash if it receives an error
	mockCloudClient.setMockGetFeatures(
		func(ctx context.Context, r *v1.EmptyRequest) (*v1.GetFeaturesResponse, error) {
			return nil, errors.New("err fetching features")
		},
	)
	requireFeatures(t, fakeClock, backend, ctx, wantFeatures)

	// Make sure it can recover after a failed request
	mockCloudClient.setMockGetFeatures(
		func(ctx context.Context, r *v1.EmptyRequest) (*v1.GetFeaturesResponse, error) {
			return &v1.GetFeaturesResponse{
				Db:           true,
				IsUsageBased: true,
			}, nil
		},
	)
	requireFeatures(t, fakeClock, backend, ctx, modules.Features{
		DB: true,
		DeviceTrust: modules.DeviceTrustFeature{
			Enabled:           true, // always enabled
			DevicesUsageLimit: 5,
		},
		AccessRequests: modules.AccessRequestsFeature{
			MonthlyRequestLimit: 5,
		},
		IsUsageBasedBilling: true,
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
		func(ctx context.Context, r *v1.EmptyRequest) (*v1.GetFeaturesResponse, error) {
			return &v1.GetFeaturesResponse{
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
