/*
Copyright 2020 Gravitational, Inc.

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

package labels

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/google/uuid"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestSync(t *testing.T) {
	// Create dynamic labels and sync right away.
	l, err := NewDynamic(context.Background(), &DynamicConfig{
		Labels: map[string]types.CommandLabel{
			"foo": &types.CommandLabelV2{
				Period:  types.NewDuration(1 * time.Second),
				Command: []string{"expr", "1", "+", "3"},
			},
		},
	})
	require.NoError(t, err)
	l.Sync()

	// Check that the result contains the output of the command.
	require.Equal(t, "4", l.Get()["foo"].GetResult())
}

func TestStart(t *testing.T) {
	// Create dynamic labels and setup async update.
	l, err := NewDynamic(context.Background(), &DynamicConfig{
		Labels: map[string]types.CommandLabel{
			"foo": &types.CommandLabelV2{
				Period:  types.NewDuration(1 * time.Second),
				Command: []string{"expr", "1", "+", "3"},
			},
		},
	})
	require.NoError(t, err)
	l.Start()

	require.Eventually(t, func() bool {
		val, ok := l.Get()["foo"]
		require.True(t, ok)
		return val.GetResult() == "4"
	}, 5*time.Second, 50*time.Millisecond)
}

// TestInvalidCommand makes sure that invalid commands return a error message.
func TestInvalidCommand(t *testing.T) {
	// Create invalid labels and sync right away.
	l, err := NewDynamic(context.Background(), &DynamicConfig{
		Labels: map[string]types.CommandLabel{
			"foo": &types.CommandLabelV2{
				Period:  types.NewDuration(1 * time.Second),
				Command: []string{uuid.New().String()}},
		},
	})
	require.NoError(t, err)
	l.Sync()

	// Check that the output contains that the command was not found.
	val, ok := l.Get()["foo"]
	require.True(t, ok)
	require.Contains(t, val.GetResult(), "output:")
}

type mockIMDSClient struct {
	tags map[string]string
}

func (m *mockIMDSClient) IsAvailable() bool {
	return true
}

func (m *mockIMDSClient) GetTagKeys() ([]string, error) {
	keys := make([]string, 0, len(m.tags))
	for k := range m.tags {
		keys = append(keys, k)
	}
	return keys, nil
}

func (m *mockIMDSClient) GetTagValue(key string) (string, error) {
	if value, ok := m.tags[key]; ok {
		return value, nil
	}
	return "", trace.NotFound("Tag %q not found", key)
}

func TestEC2LabelsSync(t *testing.T) {
	tags := map[string]string{"a": "1", "b": "2"}
	imdsClient := &mockIMDSClient{
		tags: tags,
	}
	ec2Labels, err := NewEC2Labels(context.Background(), &EC2LabelConfig{
		Client: imdsClient,
	})
	require.NoError(t, err)
	ec2Labels.Sync()
	require.Equal(t, toAWSLabels(tags), ec2Labels.Get())
}

func TestEC2LabelsAsync(t *testing.T) {
	imdsClient := &mockIMDSClient{}
	clock := clockwork.NewFakeClock()
	ctx, cancel := context.WithCancel(context.Background())
	ec2Labels, err := NewEC2Labels(ctx, &EC2LabelConfig{
		Client: imdsClient,
	})
	require.NoError(t, err)
	ec2Labels.clock = clock

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
	ec2Labels.Start()
	require.Eventually(t, compareLabels(toAWSLabels(initialTags)), time.Second, 100*time.Microsecond)

	// Check that tags are updated over time.
	updatedTags := map[string]string{"a": "3", "c": "4"}
	imdsClient.tags = updatedTags
	clock.Advance(types.EC2LabelUpdatePeriod)
	require.Eventually(t, compareLabels(toAWSLabels(updatedTags)), time.Second, 100*time.Millisecond)

	// Check that service stops updating when cancelled.
	cancel()
	imdsClient.tags = map[string]string{"x": "8", "y": "9", "z": "10"}
	clock.Advance(types.EC2LabelUpdatePeriod)
	require.Eventually(t, compareLabels(toAWSLabels(updatedTags)), time.Second, 100*time.Millisecond)
}
