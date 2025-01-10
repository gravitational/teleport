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

	"github.com/gravitational/teleport/api/types"
)

// AuthorityGetter defines interface for fetching cert authority resources.
type AuthorityGetter interface {
	// GetCertAuthority returns cert authority by id
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error)
}

// Trust is responsible for managing certificate authorities
// Each authority is managing some domain, e.g. example.com
//
// There are two type of authorities, local and remote.
// Local authorities have both private and public keys, so they can
// sign public keys of users and hosts
//
// Remote authorities have only public keys available, so they can
// be only used to validate
type Trust interface {
	// AuthorityGetter retrieves certificate authorities
	AuthorityGetter

	// Clusters provides metchods for interacting with
	// trusted clusters.
	Clusters

	// CreateCertAuthority inserts a new certificate authority
	CreateCertAuthority(ctx context.Context, ca types.CertAuthority) error

	// UpsertCertAuthority updates or inserts a new certificate authority
	UpsertCertAuthority(ctx context.Context, ca types.CertAuthority) error

	// CompareAndSwapCertAuthority updates the cert authority value
	// if existing value matches existing parameter,
	// returns nil if succeeds, trace.CompareFailed otherwise
	CompareAndSwapCertAuthority(new, existing types.CertAuthority) error

	// DeleteCertAuthority deletes particular certificate authority
	DeleteCertAuthority(ctx context.Context, id types.CertAuthID) error

	// DeleteAllCertAuthorities deletes cert authorities of a certain type
	DeleteAllCertAuthorities(caType types.CertAuthType) error

	// ActivateCertAuthority moves a CertAuthority from the deactivated list to
	// the normal list.
	ActivateCertAuthority(id types.CertAuthID) error

	// DeactivateCertAuthority moves a CertAuthority from the normal list to
	// the deactivated list.
	DeactivateCertAuthority(id types.CertAuthID) error

	// UpdateUserCARoleMap updates the role map of the userCA of the specified existing cluster.
	UpdateUserCARoleMap(ctx context.Context, name string, roleMap types.RoleMap, activated bool) error
}

// TrustInternal extends the Trust interface with local-only methods used by the
// auth server for some local operations.
type TrustInternal interface {
	Trust

	// CreateTrustedCluster atomically creates a new trusted cluster along with associated resources.
	CreateTrustedCluster(context.Context, types.TrustedCluster, []types.CertAuthority) (revision string, err error)

	// UpdateTrustedCluster atomically updates a trusted cluster along with associated resources.
	UpdateTrustedCluster(context.Context, types.TrustedCluster, []types.CertAuthority) (revision string, err error)

	// DeleteTrustedClusterInternal atomically deletes a trusted cluster along with associated resources.
	DeleteTrustedClusterInternal(context.Context, string, []types.CertAuthID) error

	// CreateRemoteCluster atomically creates a new remote cluster along with associated resources.
	CreateRemoteClusterInternal(context.Context, types.RemoteCluster, []types.CertAuthority) (revision string, err error)

	// DeleteRemotClusterInternal atomically deletes a remote cluster along with associated resources.
	DeleteRemoteClusterInternal(context.Context, string, []types.CertAuthID) error

	// GetInactiveCertAuthority returns inactive certificate authority by given id. Parameter loadSigningKeys
	// controls if signing keys are loaded.
	GetInactiveCertAuthority(ctx context.Context, id types.CertAuthID, loadSigningKeys bool) (types.CertAuthority, error)

	// CreateCertAuthorities creates multiple cert authorities atomically.
	CreateCertAuthorities(context.Context, ...types.CertAuthority) (revision string, err error)

	// UpdateCertAuthority updates an existing cert authority if the revisions match.
	UpdateCertAuthority(context.Context, types.CertAuthority) (types.CertAuthority, error)

	// DeleteCertAuthorities deletes multiple cert authorities atomically.
	DeleteCertAuthorities(context.Context, ...types.CertAuthID) error

	// ActivateCertAuthorities activates multiple cert authorities atomically.
	ActivateCertAuthorities(context.Context, ...types.CertAuthID) error

	// DeactivateCertAuthorities deactivates multiple cert authorities atomically.
	DeactivateCertAuthorities(context.Context, ...types.CertAuthID) error
}

// Clusters is responsible for managing trusted clusters.
type Clusters interface {
	// UpsertTrustedCluster creates or updates a TrustedCluster in the backend.
	UpsertTrustedCluster(ctx context.Context, tc types.TrustedCluster) (types.TrustedCluster, error)

	// GetTrustedCluster returns a single TrustedCluster by name.
	GetTrustedCluster(ctx context.Context, name string) (types.TrustedCluster, error)

	// GetTrustedClusters returns all TrustedClusters in the backend.
	GetTrustedClusters(ctx context.Context) ([]types.TrustedCluster, error)

	// DeleteTrustedCluster removes a TrustedCluster from the backend by name.
	DeleteTrustedCluster(ctx context.Context, name string) error

	// UpsertTunnelConnection upserts tunnel connection
	UpsertTunnelConnection(types.TunnelConnection) error

	// GetTunnelConnections returns tunnel connections for a given cluster
	GetTunnelConnections(clusterName string, opts ...MarshalOption) ([]types.TunnelConnection, error)

	// GetAllTunnelConnections returns all tunnel connections
	GetAllTunnelConnections(opts ...MarshalOption) ([]types.TunnelConnection, error)

	// DeleteTunnelConnection deletes tunnel connection by name
	DeleteTunnelConnection(clusterName string, connName string) error

	// DeleteTunnelConnections deletes all tunnel connections for cluster
	DeleteTunnelConnections(clusterName string) error

	// DeleteAllTunnelConnections deletes all tunnel connections for cluster
	DeleteAllTunnelConnections() error

	// CreateRemoteCluster creates a remote cluster
	CreateRemoteCluster(ctx context.Context, rc types.RemoteCluster) (types.RemoteCluster, error)

	// UpdateRemoteCluster updates a remote cluster
	UpdateRemoteCluster(ctx context.Context, rc types.RemoteCluster) (types.RemoteCluster, error)

	// PatchRemoteCluster fetches a remote cluster and then calls updateFn
	// to apply any changes, before persisting the updated remote cluster.
	PatchRemoteCluster(ctx context.Context, name string, updateFn func(rc types.RemoteCluster) (types.RemoteCluster, error)) (types.RemoteCluster, error)

	// GetRemoteClusters returns a list of remote clusters
	// Prefer ListRemoteClusters
	GetRemoteClusters(ctx context.Context) ([]types.RemoteCluster, error)

	// ListRemoteClusters returns a page of remote clusters
	ListRemoteClusters(ctx context.Context, pageSize int, pageToken string) ([]types.RemoteCluster, string, error)

	// GetRemoteCluster returns a remote cluster by name
	GetRemoteCluster(ctx context.Context, clusterName string) (types.RemoteCluster, error)

	// DeleteRemoteCluster deletes remote cluster by name
	DeleteRemoteCluster(ctx context.Context, clusterName string) error

	// DeleteAllRemoteClusters deletes all remote clusters
	DeleteAllRemoteClusters(ctx context.Context) error
}
