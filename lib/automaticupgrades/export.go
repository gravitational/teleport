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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/kubernetes"
	"github.com/gravitational/teleport/lib/defaults"
)

const (
	// kubeSchedKey is the key under which the kube controller schedule is exported
	kubeSchedKey = "agent-maintenance-schedule"

	// kubeMetadataKey is the key under which the kube controller metadata is exported
	kubeMetadataKey = "agent-metadata"

	// unitScheduleFile is the name of the file to which the unit schedule is exported.
	unitScheduleFile = "schedule"

	// unitMetadataFile is the name of the file to which the metadata is exported.
	unitMetadataFile = "metadata"

	// unitConfigDir is the configuration directory of the teleport-upgrade unit.
	unitConfigDir = "/etc/teleport-upgrade.d"
)

// AgentMetadata contains agent metadata to exported to an external upgrader.
type AgentMetadata struct {
	UUID     string `json:"uuid,omitempty"`
	Hostname string `json:"hostname,omitempty"`
	Version  string `json:"version,omitempty"`
}

// Driver represents a type capable of exporting the maintenance window schedule
// or agent metadata to an external upgrader, such as the teleport-upgrade systemd
// timer or the kube-updater controller.
type Driver interface {
	// Kind gets the upgrader kind associated with this export driver.
	Kind() string

	// SyncSchedule exports the appropriate maintenance window schedule if one is present, or
	// resets/clears the maintenance window if the schedule response returns no viable scheduling
	// info.
	SyncSchedule(ctx context.Context, rsp proto.ExportUpgradeWindowsResponse) error

	// SyncMetadata exports the agent metadata.
	SyncMetadata(ctx context.Context, metadata AgentMetadata) error

	// ResetSchedule forcibly clears any previously exported maintenance window values. This should be
	// called if teleport experiences prolonged loss of auth connectivity, which may be an indicator
	// that the control plane has been upgraded s.t. this agent is no longer compatible.
	ResetSchedule(ctx context.Context) error
}

// NewDriver sets up a new export driver corresponding to the specified upgrader kind.
func NewDriver(kind string) (Driver, error) {
	switch kind {
	case types.UpgraderKindKubeController:
		return NewKubeControllerDriver(KubeControllerDriverConfig{})
	case types.UpgraderKindSystemdUnit:
		return NewSystemdUnitDriver(SystemdUnitDriverConfig{})
	default:
		return nil, trace.BadParameter("unsupported upgrader kind: %q", kind)
	}
}

type KubeControllerDriverConfig struct {
	// Backend is an optional backend. Must be an instance of the kuberenets shared-state backend
	// if not nil.
	Backend KubernetesBackend
}

// KubernetesBackend interface for kube shared storage backend.
type KubernetesBackend interface {
	// Put puts value into backend (creates if it does not
	// exists, updates it otherwise)
	Put(ctx context.Context, i backend.Item) (*backend.Lease, error)
}

type kubeDriver struct {
	cfg KubeControllerDriverConfig
}

func NewKubeControllerDriver(cfg KubeControllerDriverConfig) (Driver, error) {
	if cfg.Backend == nil {
		var err error
		cfg.Backend, err = kubernetes.NewShared()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &kubeDriver{cfg: cfg}, nil
}

func (e *kubeDriver) Kind() string {
	return types.UpgraderKindKubeController
}

func (e *kubeDriver) SyncSchedule(ctx context.Context, rsp proto.ExportUpgradeWindowsResponse) error {
	if rsp.KubeControllerSchedule == "" {
		return e.ResetSchedule(ctx)
	}
	return trace.Wrap(e.sync(ctx, kubeSchedKey, []byte(rsp.GetKubeControllerSchedule())))
}

func (e *kubeDriver) SyncMetadata(ctx context.Context, metadata AgentMetadata) error {
	b, err := json.Marshal(metadata)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(e.sync(ctx, kubeMetadataKey, b))
}

func (e *kubeDriver) sync(ctx context.Context, dst string, data []byte) error {
	_, err := e.cfg.Backend.Put(ctx, backend.Item{
		Key:   []byte(dst),
		Value: data,
	})

	return trace.Wrap(err)
}

func (e *kubeDriver) ResetSchedule(ctx context.Context) error {
	// kube backend doesn't support deletes right now, so just set
	// the key to empty.
	_, err := e.cfg.Backend.Put(ctx, backend.Item{
		Key:   []byte(kubeSchedKey),
		Value: []byte{},
	})

	return trace.Wrap(err)
}

type SystemdUnitDriverConfig struct {
	// ConfigDir is the directory from which the teleport-upgrade periodic loads its
	// configuration parameters. Most notably, the 'schedule' file.
	ConfigDir string
}

type systemdDriver struct {
	cfg SystemdUnitDriverConfig
}

func NewSystemdUnitDriver(cfg SystemdUnitDriverConfig) (Driver, error) {
	if cfg.ConfigDir == "" {
		cfg.ConfigDir = unitConfigDir
	}

	return &systemdDriver{cfg: cfg}, nil
}

func (e *systemdDriver) Kind() string {
	return types.UpgraderKindSystemdUnit
}

func (e *systemdDriver) SyncSchedule(ctx context.Context, rsp proto.ExportUpgradeWindowsResponse) error {
	if len(rsp.SystemdUnitSchedule) == 0 {
		// treat an empty schedule value as equivalent to a reset
		return e.ResetSchedule(ctx)
	}
	return trace.Wrap(e.sync(ctx, e.scheduleFile(), []byte(rsp.GetSystemdUnitSchedule())))
}

func (e *systemdDriver) SyncMetadata(ctx context.Context, metadata AgentMetadata) error {
	b, err := json.Marshal(metadata)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(e.sync(ctx, e.metadataFile(), b))
}

func (e *systemdDriver) sync(ctx context.Context, dst string, data []byte) error {
	// ensure config dir exists. if created it is set to 755, which is reasonably safe and seems to
	// be the standard choice for config dirs like this in /etc/.
	if err := os.MkdirAll(e.cfg.ConfigDir, defaults.DirectoryPermissions); err != nil {
		return trace.Wrap(err)
	}

	// export file. if created it is set to 644, which is reasonable for a sensitive but non-secret config value.
	if err := os.WriteFile(dst, data, defaults.FilePermissions); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (e *systemdDriver) ResetSchedule(_ context.Context) error {
	if _, err := os.Stat(e.scheduleFile()); os.IsNotExist(err) {
		return nil
	}

	// note that we blank the file rather than deleting it, this is intended to allow us to
	// preserve custom file permissions, such as those that might be used in a scenario where
	// teleport is operating with limited privileges.
	if err := os.WriteFile(e.scheduleFile(), []byte{}, teleport.FileMaskOwnerOnly); err != nil {
		return trace.Errorf("failed to reset schedule file: %v", err)
	}

	return nil
}

func (e *systemdDriver) scheduleFile() string {
	return filepath.Join(e.cfg.ConfigDir, unitScheduleFile)
}

func (e *systemdDriver) metadataFile() string {
	return filepath.Join(e.cfg.ConfigDir, unitMetadataFile)
}
