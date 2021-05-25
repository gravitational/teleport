/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package workflows_test

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/api/client/workflows"
	"github.com/gravitational/teleport/api/types"
	"github.com/stretchr/testify/require"
)

// mock values used by the mockClient
var (
	mockReqs = []types.AccessRequest{
		&types.AccessRequestV3{
			Metadata: types.Metadata{
				Name: "1",
			},
		},
		&types.AccessRequestV3{
			Metadata: types.Metadata{
				Name: "2",
			},
		},
		&types.AccessRequestV3{
			Metadata: types.Metadata{
				Name: "3",
			},
		},
	}
	mockDataMap = workflows.PluginDataMap{
		"plugin": "data",
	}
	mockPluginData = []types.PluginData{&types.PluginDataV3{
		Spec: types.PluginDataSpecV3{
			Entries: map[string]*types.PluginDataEntry{
				"plugin-name": {Data: mockDataMap},
			},
		},
	}}
)

// mockClient is a mock workflows.Client.
type mockClient struct {
	// mockWatcher is used to simulate a real watcher returned by NewWatcher.
	mockWatcher types.Watcher
}

func newMockClient(watcher types.Watcher) *mockClient {
	return &mockClient{watcher}
}

func (m *mockClient) CreateAccessRequest(ctx context.Context, req types.AccessRequest) error {
	return nil
}

// GetAccessRequests returns mockReqs. If an ID filter is provided, only the request with that ID is returned.
func (m *mockClient) GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
	if filter.ID != "" {
		for _, req := range mockReqs {
			if req.GetName() == filter.ID {
				return []types.AccessRequest{req}, nil
			}
		}
		return []types.AccessRequest{}, nil
	}
	return mockReqs, nil
}
func (m *mockClient) SetAccessRequestState(ctx context.Context, params types.AccessRequestUpdate) error {
	return nil
}

// GetPluginData returns mockPluginData
func (m *mockClient) GetPluginData(ctx context.Context, filter types.PluginDataFilter) ([]types.PluginData, error) {
	return mockPluginData, nil
}

func (m *mockClient) UpdatePluginData(ctx context.Context, params types.PluginDataUpdateParams) error {
	return nil
}

// NewWatcher returns the client's mockWatcher.
func (m *mockClient) NewWatcher(ctx context.Context, filter types.Watch) (types.Watcher, error) {
	return m.mockWatcher, nil
}

func TestGetRequest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	plugin := workflows.NewPlugin(ctx, "plugin-name", newMockClient(nil))

	req, err := plugin.GetRequest(ctx, "2")
	require.NoError(t, err)
	require.Equal(t, "2", req.GetName())

	_, err = plugin.GetRequest(ctx, "4")
	require.Error(t, err)
}

func TestGetPluginData(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	plugin := workflows.NewPlugin(ctx, "plugin-name", newMockClient(nil))

	dataMap, err := plugin.GetPluginData(ctx, "")
	require.NoError(t, err)
	require.Equal(t, mockDataMap, dataMap)
}
