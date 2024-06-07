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
