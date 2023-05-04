package feature

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/api/cloud"
	v1 "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
)

type testClient struct {
	cloud.MockedClient
}

// Implement cloud.Client interface for mocked client
func (tc testClient) Close() error { return nil }

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

	mockCloudClient.MockGetFeatures = func(ctx context.Context, r *v1.EmptyRequest) (*v1.GetFeaturesResponse, error) {
		return &v1.GetFeaturesResponse{
			Kubernetes: true,
		}, nil
	}

	// Run the service.
	go service.Run(ctx)
	fakeClock.BlockUntil(1)

	// Advance the clock so the service fetch and stores features
	fakeClock.Advance(1 * time.Second)
	require.Eventually(t, func() bool {
		// Check if the features are stored in the backend.
		item, err := backend.Get(ctx, featuresBackendKey)
		if err != nil {
			return false
		}

		stored := &modules.Features{}
		err = json.Unmarshal(item.Value, stored)
		if err != nil {
			return false
		}

		return modules.Features{
			Kubernetes: true,
		} == *stored
	}, time.Second, time.Millisecond*100)

	// update features again and see if they are stored in the backend
	mockCloudClient.MockGetFeatures = func(ctx context.Context, r *v1.EmptyRequest) (*v1.GetFeaturesResponse, error) {
		return &v1.GetFeaturesResponse{
			Kubernetes: false,
			App:        true,
		}, nil
	}
	// Wait for the service to fetch and store the features.
	fakeClock.Advance(1 * time.Second)
	// check backend again
	require.Eventually(t, func() bool {
		// Check if the features are stored in the backend.
		item, err := backend.Get(ctx, featuresBackendKey)
		if err != nil {
			return false
		}
		stored := &modules.Features{}

		err = json.Unmarshal(item.Value, stored)
		if err != nil {
			return false
		}

		return modules.Features{
			Kubernetes: false,
			App:        true,
		} == *stored
	}, time.Second, time.Millisecond*100)

	// Test that the service wont crash if it receives an error
	mockCloudClient.MockGetFeatures = func(ctx context.Context, r *v1.EmptyRequest) (*v1.GetFeaturesResponse, error) {
		return nil, errors.New("err fetching features")
	}
	fakeClock.Advance(1 * time.Second)

	require.Eventually(t, func() bool {
		// check backend again, the same value as before is expected
		item, err := backend.Get(ctx, featuresBackendKey)
		if err != nil {
			return false
		}
		stored := &modules.Features{}

		err = json.Unmarshal(item.Value, stored)
		if err != nil {
			return false
		}

		return modules.Features{
			Kubernetes: false,
			App:        true,
		} == *stored
	}, time.Second, time.Millisecond*100)

	// Make sure it can recover after a failed request
	mockCloudClient.MockGetFeatures = func(ctx context.Context, r *v1.EmptyRequest) (*v1.GetFeaturesResponse, error) {
		return &v1.GetFeaturesResponse{
			Db: true,
		}, nil
	}
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

		return modules.Features{
			DB: true,
		} == *stored
	}, time.Second, time.Millisecond*100)
}

func newMemoryBackend(t *testing.T) backend.Backend {
	b, err := memory.New(memory.Config{})
	require.NoError(t, err)
	return b
}
