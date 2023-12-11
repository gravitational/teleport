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

package automaticupgrades

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/backend"
)

type fakeKubeBackend struct {
	data map[string]string
}

func newFakeKubeBackend() *fakeKubeBackend {
	return &fakeKubeBackend{
		data: make(map[string]string),
	}
}

func (b *fakeKubeBackend) Put(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	b.data[string(item.Key)] = string(item.Value)
	return nil, nil
}

// TestKubeControllerDriverSchedule verifies the schedule export behavior of the
// kube controller driver.
func TestKubeControllerDriverSchedule(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bk := newFakeKubeBackend()

	driver, err := NewKubeControllerDriver(KubeControllerDriverConfig{
		Backend: bk,
	})
	require.NoError(t, err)

	require.Equal(t, "kube", driver.Kind())

	// verify basic schedule creation
	err = driver.SyncSchedule(ctx, proto.ExportUpgradeWindowsResponse{
		KubeControllerSchedule: "fake-schedule",
	})
	require.NoError(t, err)

	key := "agent-maintenance-schedule"

	require.Equal(t, "fake-schedule", bk.data[key])

	// verify overwrite of existing schedule
	err = driver.SyncSchedule(ctx, proto.ExportUpgradeWindowsResponse{
		KubeControllerSchedule: "fake-schedule-2",
	})
	require.NoError(t, err)

	require.Equal(t, "fake-schedule-2", bk.data[key])

	// verify reset of schedule
	err = driver.ResetSchedule(ctx)
	require.NoError(t, err)

	require.Equal(t, "", bk.data[key])

	// verify reset of empty schedule has no effect
	err = driver.ResetSchedule(ctx)
	require.NoError(t, err)

	require.Equal(t, "", bk.data[key])

	// setup another fake schedule
	err = driver.SyncSchedule(ctx, proto.ExportUpgradeWindowsResponse{
		KubeControllerSchedule: "fake-schedule-3",
	})
	require.NoError(t, err)

	require.Equal(t, "fake-schedule-3", bk.data[key])

	// verify that empty schedule is equivalent to reset
	err = driver.SyncSchedule(ctx, proto.ExportUpgradeWindowsResponse{})
	require.NoError(t, err)

	require.Equal(t, "", bk.data[key])
}

// TestSystemdUnitDriverSchedule verifies the schedule export behavior of the
// systemd unit driver.
func TestSystemdUnitDriverSchedule(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// use a sub-directory of a temp dir in order to verify that
	// driver creates dir when needed.
	dir := filepath.Join(t.TempDir(), "config")

	driver, err := NewSystemdUnitDriver(SystemdUnitDriverConfig{
		ConfigDir: dir,
	})
	require.NoError(t, err)

	require.Equal(t, "unit", driver.Kind())

	// verify basic schedule creation
	err = driver.SyncSchedule(ctx, proto.ExportUpgradeWindowsResponse{
		SystemdUnitSchedule: "fake-schedule",
	})
	require.NoError(t, err)

	schedPath := filepath.Join(dir, "schedule")

	sb, err := os.ReadFile(schedPath)
	require.NoError(t, err)

	require.Equal(t, "fake-schedule", string(sb))

	// verify overwrite of existing schedule
	err = driver.SyncSchedule(ctx, proto.ExportUpgradeWindowsResponse{
		SystemdUnitSchedule: "fake-schedule-2",
	})
	require.NoError(t, err)

	sb, err = os.ReadFile(schedPath)
	require.NoError(t, err)

	require.Equal(t, "fake-schedule-2", string(sb))

	// verify reset/deletion of schedule
	err = driver.ResetSchedule(ctx)
	require.NoError(t, err)

	sb, err = os.ReadFile(schedPath)
	require.NoError(t, err)
	require.Equal(t, "", string(sb))

	// verify that duplicate resets succeed
	err = driver.ResetSchedule(ctx)
	require.NoError(t, err)

	// set up another schedule
	err = driver.SyncSchedule(ctx, proto.ExportUpgradeWindowsResponse{
		SystemdUnitSchedule: "fake-schedule-3",
	})
	require.NoError(t, err)

	sb, err = os.ReadFile(schedPath)
	require.NoError(t, err)

	require.Equal(t, "fake-schedule-3", string(sb))

	// verify that an empty schedule value is treated equivalent to a reset
	err = driver.SyncSchedule(ctx, proto.ExportUpgradeWindowsResponse{})
	require.NoError(t, err)

	sb, err = os.ReadFile(schedPath)
	require.NoError(t, err)
	require.Equal(t, "", string(sb))
}

// TestKubeControllerDriverMetadata verifies metadata export behavior of the kube
// controller driver.
func TestKubeControllerDriverMetadata(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bk := newFakeKubeBackend()

	driver, err := NewKubeControllerDriver(KubeControllerDriverConfig{
		Backend: bk,
	})
	require.NoError(t, err)
	require.Equal(t, "kube", driver.Kind())

	var out AgentMetadata

	// verify metadata creation
	md1 := AgentMetadata{
		UUID:     "uuid-1",
		Hostname: "hostname-1",
		Version:  "14.2.1",
	}
	require.NoError(t, driver.SyncMetadata(ctx, md1))

	require.NoError(t, json.Unmarshal([]byte(bk.data[kubeMetadataKey]), &out))
	require.Equal(t, md1, out)

	// verify overwrite of existing metadata
	md2 := AgentMetadata{
		UUID:     "uuid-2",
		Hostname: "hostname-2",
		Version:  "14.2.2",
	}
	require.NoError(t, driver.SyncMetadata(ctx, md2))

	require.NoError(t, json.Unmarshal([]byte(bk.data[kubeMetadataKey]), &out))
	require.Equal(t, md2, out)
}

// TestSystemdUnitDriverMetadata verifies the metadata export behavior of the systemd
// unit driver.
func TestSystemdUnitDriverMetadata(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// use a sub-directory of a temp dir in order to verify that
	// driver creates dir when needed.
	dir := filepath.Join(t.TempDir(), "config")

	driver, err := NewSystemdUnitDriver(SystemdUnitDriverConfig{
		ConfigDir: dir,
	})
	require.NoError(t, err)
	require.Equal(t, "unit", driver.Kind())

	var out AgentMetadata
	metadataPath := filepath.Join(dir, unitMetadataFile)

	// verify metadata creation
	md1 := AgentMetadata{
		UUID:     "uuid-1",
		Hostname: "hostname-1",
		Version:  "14.2.1",
	}
	require.NoError(t, driver.SyncMetadata(ctx, md1))

	b, err := os.ReadFile(metadataPath)
	require.NoError(t, err)

	require.NoError(t, json.Unmarshal(b, &out))
	require.Equal(t, md1, out)

	// verify overwrite of existing metadata
	md2 := AgentMetadata{
		UUID:     "uuid-2",
		Hostname: "hostname-2",
		Version:  "14.2.2",
	}
	require.NoError(t, driver.SyncMetadata(ctx, md2))

	b, err = os.ReadFile(metadataPath)
	require.NoError(t, err)

	require.NoError(t, json.Unmarshal(b, &out))
	require.Equal(t, md2, out)
}
