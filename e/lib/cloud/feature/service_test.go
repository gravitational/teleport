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

func TestRun(t *testing.T) {
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
				Kubernetes: true,
			}, nil
		},
	)

	// Run the service.
	go service.Run(ctx)
	fakeClock.BlockUntil(1)

	requireFeatures := func(t *testing.T, want modules.Features) {
		t.Helper()

		// Advance the clock so the service fetch and stores features
		fakeClock.Advance(1 * time.Second)

		require.Eventually(t, func() bool {
			item, err := backend.Get(ctx, featuresBackendKey)
			if err != nil {
				return false
			}

			stored := &modules.Features{}
			err = json.Unmarshal(item.Value, stored)
			if err != nil {
				return false
			}

			diff := cmp.Diff(want, *stored)
			if diff == "" {
				return true
			}
			t.Logf("Feature diff (-want +got):\n%s", diff)
			return false
		}, 1*time.Second, time.Millisecond*100)
	}

	// Check if the features are stored in the backend.
	requireFeatures(t, modules.Features{
		Kubernetes: true,
		DeviceTrust: modules.DeviceTrustFeature{
			Enabled: true, // always enabled
		},
	})

	// update features again and see if they are stored in the backend
	mockCloudClient.setMockGetFeatures(
		func(ctx context.Context, r *v1.EmptyRequest) (*v1.GetFeaturesResponse, error) {
			return &v1.GetFeaturesResponse{
				Kubernetes: false,
				App:        true,
			}, nil
		},
	)
	// check backend again
	wantFeatures := modules.Features{
		Kubernetes: false,
		App:        true,
		DeviceTrust: modules.DeviceTrustFeature{
			Enabled: true, // always enabled
		},
	}
	requireFeatures(t, wantFeatures)

	// Test that the service wont crash if it receives an error
	mockCloudClient.setMockGetFeatures(
		func(ctx context.Context, r *v1.EmptyRequest) (*v1.GetFeaturesResponse, error) {
			return nil, errors.New("err fetching features")
		},
	)
	requireFeatures(t, wantFeatures)

	// Make sure it can recover after a failed request
	mockCloudClient.setMockGetFeatures(
		func(ctx context.Context, r *v1.EmptyRequest) (*v1.GetFeaturesResponse, error) {
			return &v1.GetFeaturesResponse{
				Db: true,
			}, nil
		},
	)
	requireFeatures(t, modules.Features{
		DB: true,
		DeviceTrust: modules.DeviceTrustFeature{
			Enabled: true, // always enabled
		},
	})
}

func newMemoryBackend(t *testing.T) backend.Backend {
	b, err := memory.New(memory.Config{})
	require.NoError(t, err)
	return b
}
