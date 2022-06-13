/*
Copyright 2022 Gravitational, Inc.

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
package ec2

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

type mockIMDSClient struct {
	tags map[string]string
}

func (m *mockIMDSClient) IsAvailable(ctx context.Context) bool {
	return true
}

func (m *mockIMDSClient) GetTagKeys(ctx context.Context) ([]string, error) {
	keys := make([]string, 0, len(m.tags))
	for k := range m.tags {
		keys = append(keys, k)
	}
	return keys, nil
}

func (m *mockIMDSClient) GetTagValue(ctx context.Context, key string) (string, error) {
	if value, ok := m.tags[key]; ok {
		return value, nil
	}
	return "", trace.NotFound("Tag %q not found", key)
}

func TestEC2LabelsSync(t *testing.T) {
	ctx := context.Background()
	tags := map[string]string{"a": "1", "b": "2"}
	expectedTags := map[string]string{"aws/a": "1", "aws/b": "2"}
	imdsClient := &mockIMDSClient{
		tags: tags,
	}
	ec2Labels, err := New(ctx, &Config{
		Client: imdsClient,
	})
	require.NoError(t, err)
	require.NoError(t, ec2Labels.Sync(ctx))
	require.Equal(t, expectedTags, ec2Labels.Get())
}

func TestEC2LabelsAsync(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	imdsClient := &mockIMDSClient{}
	clock := clockwork.NewFakeClock()
	ec2Labels, err := New(ctx, &Config{
		Client: imdsClient,
		Clock:  clock,
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
	clock.Advance(ec2LabelUpdatePeriod)
	require.Eventually(t, compareLabels(map[string]string{"aws/a": "3", "aws/c": "4"}), time.Second, 100*time.Millisecond)

	// Check that service stops updating when closed.
	cancel()
	imdsClient.tags = map[string]string{"x": "8", "y": "9", "z": "10"}
	clock.Advance(ec2LabelUpdatePeriod)
	require.Eventually(t, compareLabels(map[string]string{"aws/a": "3", "aws/c": "4"}), time.Second, 100*time.Millisecond)
}

func TestEC2LabelsValidKey(t *testing.T) {
	ctx := context.Background()
	tags := map[string]string{"good-label": "1", "bad-l@bel": "2"}
	expectedTags := map[string]string{"aws/good-label": "1"}
	imdsClient := &mockIMDSClient{
		tags: tags,
	}
	ec2Labels, err := New(ctx, &Config{
		Client: imdsClient,
	})
	require.NoError(t, err)
	require.NoError(t, ec2Labels.Sync(ctx))
	require.Equal(t, expectedTags, ec2Labels.Get())
}
