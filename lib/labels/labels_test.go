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

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
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
