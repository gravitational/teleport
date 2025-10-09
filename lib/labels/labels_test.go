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
