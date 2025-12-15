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

package labels

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

type mockIMDSClient struct {
	tagsDisabled bool
	tags         map[string]string
}

func (m *mockIMDSClient) IsAvailable(ctx context.Context) bool {
	return true
}

func (m *mockIMDSClient) GetType() types.InstanceMetadataType {
	return types.InstanceMetadataTypeEC2
}

func (m *mockIMDSClient) GetTags(ctx context.Context) (map[string]string, error) {
	if m.tagsDisabled {
		return nil, trace.NotFound("Tags not available")
	}
	return m.tags, nil
}

func (m *mockIMDSClient) GetHostname(ctx context.Context) (string, error) {
	value, ok := m.tags[types.CloudHostnameTag]
	if !ok {
		return "", trace.NotFound("Tag TeleportHostname not found")
	}
	return value, nil
}

func (m *mockIMDSClient) GetID(ctx context.Context) (string, error) {
	return "", nil
}

func TestCloudLabelsSync(t *testing.T) {
	ctx := context.Background()
	tags := map[string]string{"a": "1", "b": "2"}
	expectedTags := map[string]string{"aws/a": "1", "aws/b": "2"}
	imdsClient := &mockIMDSClient{
		tags: tags,
	}
	ec2Labels, err := NewCloudImporter(ctx, &CloudConfig{
		Client:    imdsClient,
		namespace: "aws",
	})
	require.NoError(t, err)
	require.NoError(t, ec2Labels.Sync(ctx))
	require.Equal(t, expectedTags, ec2Labels.Get())
}

func TestCloudLabelsAsync(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	imdsClient := &mockIMDSClient{}
	clock := clockwork.NewFakeClock()
	ec2Labels, err := NewCloudImporter(ctx, &CloudConfig{
		Client:    imdsClient,
		namespace: "aws",
		Clock:     clock,
	})
	require.NoError(t, err)

	compareLabels := func(m map[string]string) func() bool {
		return func() bool {
			labels := ec2Labels.Get()
			if len(labels) != len(m) {
				return false
			}
			for k, v := range labels {
				if m[k] != v {
					return false
				}
			}
			return true
		}
	}

	// Check that initial tags are read.
	initialTags := map[string]string{"a": "1", "b": "2"}
	imdsClient.tags = initialTags
	ec2Labels.Start(ctx)
	require.Eventually(t, compareLabels(map[string]string{"aws/a": "1", "aws/b": "2"}), time.Second, 100*time.Microsecond)

	// Check that tags are updated over time.
	updatedTags := map[string]string{"a": "3", "c": "4"}
	imdsClient.tags = updatedTags
	clock.Advance(labelUpdatePeriod)
	require.Eventually(t, compareLabels(map[string]string{"aws/a": "3", "aws/c": "4"}), time.Second, 100*time.Millisecond)

	// Check that service stops updating when closed.
	cancel()
	imdsClient.tags = map[string]string{"x": "8", "y": "9", "z": "10"}
	clock.Advance(labelUpdatePeriod)
	require.Eventually(t, compareLabels(map[string]string{"aws/a": "3", "aws/c": "4"}), time.Second, 100*time.Millisecond)
}

func TestCloudLabelsValidKey(t *testing.T) {
	ctx := context.Background()
	tags := map[string]string{"good-label": "1", "bad-l@bel": "2"}
	expectedTags := map[string]string{"aws/good-label": "1"}
	imdsClient := &mockIMDSClient{
		tags: tags,
	}
	ec2Labels, err := NewCloudImporter(ctx, &CloudConfig{
		Client:    imdsClient,
		namespace: "aws",
	})
	require.NoError(t, err)
	require.NoError(t, ec2Labels.Sync(ctx))
	require.Equal(t, expectedTags, ec2Labels.Get())
}

func TestCloudLabelsDisabled(t *testing.T) {
	ctx := context.Background()
	imdsClient := &mockIMDSClient{
		tagsDisabled: true,
	}
	ec2Labels, err := NewCloudImporter(ctx, &CloudConfig{
		Client:    imdsClient,
		namespace: "aws",
	})
	require.NoError(t, err)
	require.NoError(t, ec2Labels.Sync(ctx))
	require.Equal(t, map[string]string{}, ec2Labels.Get())
}
