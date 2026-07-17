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

package services

import (
	"context"

	"github.com/gravitational/trace"

	discoveryconfigclient "github.com/gravitational/teleport/api/client/discoveryconfig"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/lib/utils"
)

var _ DiscoveryConfigs = (*discoveryconfigclient.Client)(nil)

// DiscoveryConfigs defines an interface for managing DiscoveryConfigs.
type DiscoveryConfigs interface {
	DiscoveryConfigsGetter
	// CreateDiscoveryConfig creates a new DiscoveryConfig resource.
	CreateDiscoveryConfig(context.Context, *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error)
	// UpdateDiscoveryConfig updates an existing DiscoveryConfig resource.
	UpdateDiscoveryConfig(context.Context, *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error)
	// UpsertDiscoveryConfig upserts a DiscoveryConfig resource.
	UpsertDiscoveryConfig(context.Context, *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error)
	// DeleteDiscoveryConfig removes the specified DiscoveryConfig resource.
	DeleteDiscoveryConfig(ctx context.Context, name string) error
	// DeleteAllDiscoveryConfigs removes all DiscoveryConfigs.
	DeleteAllDiscoveryConfigs(context.Context) error
}

// DiscoveryConfigsInternal extends DiscoveryConfigs with operations available
// only on the Auth-local store. ConditionalUpdateDiscoveryConfig deliberately
// stays out of DiscoveryConfigs: the public UpdateDiscoveryConfig API is
// unconditional, so remote clients cannot conform to a revision-checked
// contract.
type DiscoveryConfigsInternal interface {
	DiscoveryConfigs
	// ConditionalUpdateDiscoveryConfig updates a DiscoveryConfig resource if
	// the revision matches, returning CompareFailed otherwise.
	ConditionalUpdateDiscoveryConfig(context.Context, *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error)
}

// StaticSnapshotDiscoveryConfigs is the isolated persistence API for
// owner-managed static snapshot DiscoveryConfigs. The isolated range keeps
// snapshots out of DiscoveryConfig watch events and generic listings; the only
// read path is a named lookup, so the interface deliberately has no List.
// It also has no Delete: snapshots expire when their owner stops renewing
// them, and deleting one while its owner is active would only cause it to be
// recreated on the next publication.
type StaticSnapshotDiscoveryConfigs interface {
	GetStaticSnapshotDiscoveryConfig(context.Context, string) (*discoveryconfig.DiscoveryConfig, error)
	CreateStaticSnapshotDiscoveryConfig(context.Context, *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error)
	ConditionalUpdateStaticSnapshotDiscoveryConfig(context.Context, *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error)
}

// DiscoveryConfigWithStatusUpdater defines an interface for managing DiscoveryConfig resources including updating their status.
type DiscoveryConfigWithStatusUpdater interface {
	DiscoveryConfigs
	// UpdateDiscoveryConfigStatus updates the status of the specified DiscoveryConfig resource.
	UpdateDiscoveryConfigStatus(context.Context, string, discoveryconfig.Status) (*discoveryconfig.DiscoveryConfig, error)
}

// DiscoveryConfigsGetter defines methods for List/Read operations on DiscoveryConfig Resources.
type DiscoveryConfigsGetter interface {
	// ListDiscoveryConfigs returns a paginated list of all DiscoveryConfig resources.
	// An optional DiscoveryGroup can be provided to filter.
	ListDiscoveryConfigs(ctx context.Context, pageSize int, nextToken string) ([]*discoveryconfig.DiscoveryConfig, string, error)
	// GetDiscoveryConfig returns the specified DiscoveryConfig resources.
	GetDiscoveryConfig(ctx context.Context, name string) (*discoveryconfig.DiscoveryConfig, error)
}

// MarshalDiscoveryConfig marshals the DiscoveryConfig resource to JSON.
func MarshalDiscoveryConfig(discoveryConfig *discoveryconfig.DiscoveryConfig, opts ...MarshalOption) ([]byte, error) {
	if err := discoveryConfig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveRevision {
		copy := *discoveryConfig
		copy.SetRevision("")
		discoveryConfig = &copy
	}
	return utils.FastMarshal(discoveryConfig)
}

// MarshalStaticSnapshotDiscoveryConfig marshals a static snapshot
// DiscoveryConfig to its stored JSON representation and enforces the
// complete-resource size limit, covering the spec inventory and the reported
// status together. An accepted status can block a later, larger inventory
// renewal only until the record TTL-expires: the fresh publication carries no
// status, and the oversized status is then rejected against the merged record
// instead of being re-accepted.
//
// Like MarshalDiscoveryConfig, the resource is validated before it is
// serialized. On top of that, the storage boundary enforces the fail-closed
// invariants that protect stored bytes: no installer params and the
// stored-size cap. The sanitized check is separate because
// CheckAndSetDefaults deliberately does not enforce it (a claimed
// static-snapshot subkind in user-supplied YAML must not be treated as
// sanitized; see NewDiscoveryConfigWithSubKind).
func MarshalStaticSnapshotDiscoveryConfig(discoveryConfig *discoveryconfig.DiscoveryConfig, opts ...MarshalOption) ([]byte, error) {
	if err := discoveryConfig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := discoveryconfig.CheckStaticSnapshotSpecSanitized(&discoveryConfig.Spec); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// A shallow copy isolates the revision reset from the caller's resource.
	record := *discoveryConfig
	if !cfg.PreserveRevision {
		record.SetRevision("")
	}
	data, err := utils.FastMarshal(&record)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(data) > discoveryconfig.MaxStaticSnapshotSize {
		return nil, trace.LimitExceeded("static snapshot discovery config exceeds maximum stored size of %d bytes", discoveryconfig.MaxStaticSnapshotSize)
	}
	return data, nil
}

// UnmarshalStaticSnapshotDiscoveryConfig unmarshals a static snapshot from
// its stored JSON representation. Records from the isolated snapshot range
// are sanitized after unmarshal so the no-installer-params invariant holds
// on every read, even for stored bytes that predate the invariant. The
// generic UnmarshalDiscoveryConfig deliberately does not do this: it also
// handles user-supplied YAML (tctl create), where a claimed static-snapshot
// subkind must not silently delete a regular config's installer params; the
// isolated backend range, not the subkind field, is the trust boundary.
func UnmarshalStaticSnapshotDiscoveryConfig(data []byte, opts ...MarshalOption) (*discoveryconfig.DiscoveryConfig, error) {
	dc, err := UnmarshalDiscoveryConfig(data, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	discoveryconfig.SanitizeStaticSnapshotSpec(&dc.Spec)
	return dc, nil
}

// UnmarshalDiscoveryConfig unmarshals the DiscoveryConfig resource from JSON.
func UnmarshalDiscoveryConfig(data []byte, opts ...MarshalOption) (*discoveryconfig.DiscoveryConfig, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing discovery config data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var discoveryConfig *discoveryconfig.DiscoveryConfig
	if err := utils.FastUnmarshal(data, &discoveryConfig); err != nil {
		return nil, trace.BadParameter("%s", err)
	}
	if err := discoveryConfig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.Revision != "" {
		discoveryConfig.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		discoveryConfig.SetExpiry(cfg.Expires)
	}
	return discoveryConfig, nil
}
