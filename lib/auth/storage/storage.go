// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// Package storage provides a mechanism for interacting with
// the persisted state of a Teleport process.
//
// The state is either persisted locally on disk of the Teleport
// process via sqlite, or if running in Kubernetes, to a Kubernetes
// secret. Callers should take care when importing this package as
// it can cause dependency trees to expand rapidly and also requires
// that cgo is enbaled in order to leverage sqlite.
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"k8s.io/client-go/util/retry"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/hostid"
)

const (
	// stateName is an internal resource object name
	stateName = "state"
	// statesPrefix is a key prefix for object states
	statesPrefix = "states"
	// idsPrefix is a key prefix for identities
	idsPrefix = "ids"
	// teleportPrefix is a key prefix to store internal data
	teleportPrefix = "teleport"
	// lastKnownVersion is a key for storing version of teleport
	lastKnownVersion = "last-known-version"
)

// stateBackend implements abstraction over local or remote storage backend methods
// required for Identity/State storage.
// As in backend.Backend, Item keys are assumed to be valid UTF8, which may be enforced by the
// various Backend implementations.
type stateBackend interface {
	// Create creates item if it does not exist
	Create(ctx context.Context, i backend.Item) (*backend.Lease, error)
	// Put puts value into backend (creates if it does not
	// exists, updates it otherwise)
	Put(ctx context.Context, i backend.Item) (*backend.Lease, error)
	// Get returns a single item or not found error
	Get(ctx context.Context, key backend.Key) (*backend.Item, error)
}

// ProcessStorage is a backend for local process state,
// it helps to manage rotation for certificate authorities
// and keeps local process credentials - x509 and SSH certs and keys.
type ProcessStorage struct {
	// BackendStorage is the SQLite backend used for operations unrelated to storing/reading identities and states.
	BackendStorage backend.Backend

	// stateStorage is the backend to store agents' identities and states.
	// it is not required to close stateBackend storage because it's either the same as BackendStorage or it is Kubernetes
	// which does not require any close method
	stateStorage stateBackend
}

// Close closes all resources used by process storage backend.
func (p *ProcessStorage) Close() error {
	// we do not need to close stateBackend storage because it's either the same as backend or it's kubernetes
	// which does not require any close method
	return p.BackendStorage.Close()
}

// GetState reads rotation state from disk.
func (p *ProcessStorage) GetState(ctx context.Context, role types.SystemRole) (*state.StateV2, error) {
	item, err := p.stateStorage.Get(ctx, backend.NewKey(statesPrefix, strings.ToLower(role.String()), stateName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var res state.StateV2
	if err := utils.FastUnmarshal(item.Value, &res); err != nil {
		return nil, trace.BadParameter("%s", err)
	}

	// an empty InitialLocalVersion is treated as an error by CheckAndSetDefaults, but if the field
	// is missing in the underlying storage, that indicates the state was written by an older version of
	// teleport that didn't record InitialLocalVersion. In that case, we set a sentinel value to indicate
	// that the version is unknown rather than being erroneously omitted.
	if res.Spec.InitialLocalVersion == "" {
		res.Spec.InitialLocalVersion = state.UnknownLocalVersion
	}

	if err := res.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &res, nil
}

// CreateState creates process state if it does not exist yet.
func (p *ProcessStorage) CreateState(role types.SystemRole, state state.StateV2) error {
	if err := state.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := json.Marshal(state)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:   backend.NewKey(statesPrefix, strings.ToLower(role.String()), stateName),
		Value: value,
	}
	_, err = p.stateStorage.Create(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// WriteState writes local cluster state to the backend.
func (p *ProcessStorage) WriteState(role types.SystemRole, state state.StateV2) error {
	if err := state.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := json.Marshal(state)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:   backend.NewKey(statesPrefix, strings.ToLower(role.String()), stateName),
		Value: value,
	}
	_, err = p.stateStorage.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ReadIdentity reads identity using identity name and role.
func (p *ProcessStorage) ReadIdentity(name string, role types.SystemRole) (*state.Identity, error) {
	if name == "" {
		return nil, trace.BadParameter("missing parameter name")
	}
	item, err := p.stateStorage.Get(context.TODO(), backend.NewKey(idsPrefix, strings.ToLower(role.String()), name))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var res state.IdentityV2
	if err := utils.FastUnmarshal(item.Value, &res); err != nil {
		return nil, trace.BadParameter("%s", err)
	}
	if err := res.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return state.ReadIdentityFromKeyPair(res.Spec.Key, &proto.Certs{
		SSH:        res.Spec.SSHCert,
		TLS:        res.Spec.TLSCert,
		TLSCACerts: res.Spec.TLSCACerts,
		SSHCACerts: res.Spec.SSHCACerts,
	})
}

// WriteIdentity writes identity to the backend.
func (p *ProcessStorage) WriteIdentity(name string, id state.Identity) error {
	res := state.IdentityV2{
		ResourceHeader: types.ResourceHeader{
			Kind:    types.KindIdentity,
			Version: types.V2,
			Metadata: types.Metadata{
				Name: name,
			},
		},
		Spec: state.IdentitySpecV2{
			Key:        id.KeyBytes,
			SSHCert:    id.CertBytes,
			TLSCert:    id.TLSCertBytes,
			TLSCACerts: id.TLSCACertsBytes,
			SSHCACerts: id.SSHCACertBytes,
		},
	}
	if err := res.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := json.Marshal(res)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:   backend.NewKey(idsPrefix, strings.ToLower(id.ID.Role.String()), name),
		Value: value,
	}
	_, err = p.stateStorage.Put(context.TODO(), item)
	return trace.Wrap(err)
}

// GetTeleportVersion reads the last known Teleport version from storage.
func (p *ProcessStorage) GetTeleportVersion(ctx context.Context) (semver.Version, error) {
	item, err := p.BackendStorage.Get(ctx, backend.NewKey(teleportPrefix, lastKnownVersion))
	if err != nil {
		return semver.Version{}, trace.Wrap(err)
	}
	version, err := semver.NewVersion(string(item.Value))
	if err != nil {
		return semver.Version{}, trace.Wrap(err)
	}
	return *version, nil
}

// WriteTeleportVersion writes the last known Teleport version to the storage.
func (p *ProcessStorage) WriteTeleportVersion(ctx context.Context, version semver.Version) error {
	item := backend.Item{
		Key:   backend.NewKey(teleportPrefix, lastKnownVersion),
		Value: []byte(version.String()),
	}
	_, err := p.BackendStorage.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteTeleportVersion removes last known Teleport version from the process storage.
func (p *ProcessStorage) DeleteTeleportVersion(ctx context.Context) error {
	err := p.BackendStorage.Delete(ctx, backend.NewKey(teleportPrefix, lastKnownVersion))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func rdpLicenseKey(key *types.RDPLicenseKey) backend.Key {
	return backend.NewKey("rdplicense", key.Issuer, strconv.Itoa(int(key.Version)), key.Company, key.ProductID)
}

type rdpLicense struct {
	Data []byte `json:"data"`
}

// WriteRDPLicense writes an RDP license to local storage.
func (p *ProcessStorage) WriteRDPLicense(ctx context.Context, key *types.RDPLicenseKey, license []byte) error {
	value, err := json.Marshal(rdpLicense{Data: license})
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     rdpLicenseKey(key),
		Value:   value,
		Expires: p.BackendStorage.Clock().Now().Add(28 * 24 * time.Hour),
	}
	_, err = p.stateStorage.Put(ctx, item)
	return trace.Wrap(err)
}

// ReadRDPLicense reads a previously obtained license from storage.
func (p *ProcessStorage) ReadRDPLicense(ctx context.Context, key *types.RDPLicenseKey) ([]byte, error) {
	item, err := p.stateStorage.Get(ctx, rdpLicenseKey(key))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	license := rdpLicense{}
	if err := json.Unmarshal(item.Value, &license); err != nil {
		return nil, trace.Wrap(err)
	}
	return license.Data, nil
}

// ReadLocalIdentity reads, parses and returns the given pub/pri key + cert from the
// key storage (dataDir).
//
// TODO(nklaassen): delete this after ref has been removed from teleport.e
func ReadLocalIdentity(dataDir string, id state.IdentityID) (*state.Identity, error) {
	return ReadLocalIdentityForRole(dataDir, id.Role)
}

// ReadLocalIdentityForRole reads, parses and returns the given pub/pri key +
// cert from the key storage (dataDir).
func ReadLocalIdentityForRole(dataDir string, role types.SystemRole) (*state.Identity, error) {
	storage, err := NewProcessStorage(context.TODO(), dataDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer storage.Close()
	return storage.ReadIdentity(state.IdentityCurrent, role)
}

// ReadOrGenerateHostID tries to read the `host_uuid` from Kubernetes storage (if available) or local storage.
// If the read operation returns no `host_uuid`, this function tries to pick it from the first static identity provided.
// If no static identities were defined for the process, a new id is generated depending on the joining process:
// - types.JoinMethodEC2: we will use the EC2 NodeID: {accountID}-{nodeID}
// - Any other valid Joining method: a new UUID is generated.
// Finally, if a new id is generated, this function writes it into local storage and Kubernetes storage (if available).
// If kubeBackend is nil, the agent is not running in a Kubernetes Cluster.
// When facing IsAlreadyExists error, this function will retry reading the host ID from the storage.
func (p *ProcessStorage) ReadOrGenerateHostID(ctx context.Context, cfg *servicecfg.Config) (string, error) {
	var hostID string

	// Read or generate the host ID, which is used to identify the Teleport agent.
	// If running in Kubernetes, it will read from the Kubernetes secret if available,
	// otherwise it will read from the local storage.
	if err := retry.OnError(retry.DefaultRetry, trace.IsAlreadyExists, func() error {
		var err error
		hostID, err = readOrGenerateHostID(ctx, cfg, p.stateStorage)
		return err
	}); err != nil {
		return "", trace.Wrap(err)
	}

	return hostID, nil
}

func readOrGenerateHostID(ctx context.Context, cfg *servicecfg.Config, kubeBackend stateBackend) (string, error) {
	// Load `host_uuid` from different storages. If this process is running in a Kubernetes Cluster,
	// readHostUUIDFromStorages will try to read the `host_uuid` from the Kubernetes Secret. If the
	// key is empty or if not running in a Kubernetes Cluster, it will read the
	// `host_uuid` from local data directory.
	hostID, err := readHostIDFromStorages(ctx, cfg.DataDir, kubeBackend)
	if err != nil {
		if !trace.IsNotFound(err) {
			if errors.Is(err, fs.ErrPermission) {
				cfg.Logger.ErrorContext(ctx, "Teleport does not have permission to write to the data directory. Ensure that you are running as a user with appropriate permissions.", "data_dir", cfg.DataDir)
			}
			return "", trace.Wrap(err)
		}
		// if there's no host uuid initialized yet, try to read one from the
		// one of the identities
		if len(cfg.Identities) != 0 {
			hostID = cfg.Identities[0].ID.HostUUID
			cfg.Logger.InfoContext(ctx, "Taking host UUID from first identity.", "host_uuid", hostID)
		} else {
			hostID, err = hostid.Generate(ctx, cfg.JoinMethod)
			if err != nil {
				return "", trace.Wrap(err)
			}
			cfg.Logger.InfoContext(ctx, "Generating new host UUID", "host_uuid", hostID)
		}
		// persistHostUUIDToStorages will persist the host_uuid to the local storage
		// and to Kubernetes Secret if this process is running on a Kubernetes Cluster.
		if err := persistHostIDToStorages(ctx, cfg, hostID, kubeBackend); err != nil {
			return "", trace.Wrap(err)
		}
	}
	return hostID, nil
}

// readHostIDFromStorages tries to read the `host_uuid` value from different storages,
// depending on where the process is running.
// If the process is running in a Kubernetes Cluster, this function will attempt
// to read the `host_uuid` from the Kubernetes Secret. If it does not exist or
// if it is not running on a Kubernetes cluster the read is done from the local
// storage: `dataDir/host_uuid`.
func readHostIDFromStorages(ctx context.Context, dataDir string, kubeBackend stateBackend) (string, error) {
	if kubeBackend != nil {
		if hostID, err := loadHostIDFromKubeSecret(ctx, kubeBackend); err == nil && len(hostID) > 0 {
			return hostID, nil
		}
	}
	// Even if running in Kubernetes fallback to local storage if `host_uuid` was
	// not found in secret.
	hostID, err := hostid.ReadFile(dataDir)
	return hostID, trace.Wrap(err)
}

// PersistAssignedHostID writes an assigned host ID to state storage and the
// host_uuid file. This should not be called in the same process as
// ReadOrGenerateHostID, it is intended to persist a host UUID assigned by the
// Auth service that was not generated locally. With the new auth-assigned host
// persisted to storage to maintain compatibility with any other processes that
// UUID flow the agent doesn't even need to read the host ID, it is only
// may read it.
func (p *ProcessStorage) PersistAssignedHostID(ctx context.Context, cfg *servicecfg.Config, hostID string) error {
	if p.stateStorage != nil {
		if _, err := p.stateStorage.Put(
			ctx,
			backend.Item{
				Key:   backend.NewKey(hostid.FileName),
				Value: []byte(hostID),
			},
		); err != nil {
			return trace.Wrap(err, "persisting host ID to state storage")
		}
	}
	if err := hostid.WriteFile(cfg.DataDir, hostID); err != nil {
		return trace.Wrap(err, "persisting host ID to file")
	}
	return nil
}

// persistHostIDToStorages writes the host ID to local data and to
// Kubernetes Secret if this process is running on a Kubernetes Cluster.
func persistHostIDToStorages(ctx context.Context, cfg *servicecfg.Config, hostID string, kubeBackend stateBackend) error {
	// Persists the `host_uuid` into Kubernetes Secret for later reusage.
	// This is required because `host_uuid` is part of the client secret
	// and Auth connection will fail if we present a different `host_uuid`.
	if kubeBackend != nil {
		if err := writeHostIDToKubeSecret(ctx, kubeBackend, hostID); err != nil {
			// If the storage of the secret fails, don't attempt to write the host id to disk.
			return trace.Wrap(err)
		}
		// Success, write the hostid to disk.
	}

	if err := hostid.WriteFile(cfg.DataDir, hostID); err != nil {
		if errors.Is(err, fs.ErrPermission) {
			cfg.Logger.ErrorContext(ctx, "Teleport does not have permission to write to the data directory. Ensure that you are running as a user with appropriate permissions.", "data_dir", cfg.DataDir)
		}
		return trace.Wrap(err)
	}

	return nil
}

// loadHostIDFromKubeSecret reads the host_uuid from the Kubernetes secret with
// the expected key: `/host_uuid`.
func loadHostIDFromKubeSecret(ctx context.Context, kubeBackend stateBackend) (string, error) {
	item, err := kubeBackend.Get(ctx, backend.NewKey(hostid.FileName))
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(item.Value), nil
}

// writeHostIDToKubeSecret writes the `host_uuid` into the Kubernetes secret under
// the key `/host_uuid`.
func writeHostIDToKubeSecret(ctx context.Context, kubeBackend stateBackend, id string) error {
	// NOTE: The host uuid should never be updated. If we fail on conflict, we need to backtrack.
	_, err := kubeBackend.Create(
		ctx,
		backend.Item{
			Key:   backend.NewKey(hostid.FileName),
			Value: []byte(id),
		},
	)
	return trace.Wrap(err)
}
