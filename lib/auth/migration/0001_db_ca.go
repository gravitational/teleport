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

package migration

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// MigrateDBClientAuthority performs a migration which creates the db_client CA
// as a copy of the existing db CA for backwards compatibility.
func MigrateDBClientAuthority(ctx context.Context, trustSvc services.Trust, cluster string) error {
	err := migrateDBAuthority(ctx, trustSvc, cluster, types.DatabaseCA, types.DatabaseClientCA)
	return trace.Wrap(err)
}

// migrateDBAuthority performs a migration which creates a new CA from an
// existing CA.
// The new CA is created as a copy of the existing CA for backwards
// compatibility.
// This func is generalized for copying db/db_client CAs, although it may appear
// to be usable for other CA types - that's why it is unexported.
func migrateDBAuthority(ctx context.Context, trustSvc services.Trust, cluster string, fromType, toType types.CertAuthType) error {
	_, err := trustSvc.GetCertAuthority(ctx, types.CertAuthID{
		Type:       toType,
		DomainName: cluster,
	}, false)
	// The migration for this cluster can be skipped since
	// the new CA already exists.
	if err == nil {
		log.Debugf("Migrations: cert authority %q already exists.", toType)
		return nil
	}
	if !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	// The new CA type does not exist, so we must check to
	// see if the existing CA exists before proceeding with the split migration.
	// If both the existing and new CA do not exist, then this cluster
	// is brand new and the migration can be avoided because they will
	// both automatically be created. If the existing CA does exist, then
	// a new CA should be constructed from it as a copy.
	existingCA, err := trustSvc.GetCertAuthority(ctx, types.CertAuthID{
		Type:       fromType,
		DomainName: cluster,
	}, true)
	if trace.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("Migrating %s CA for cluster: %s", toType, cluster)

	existingCAV2, ok := existingCA.(*types.CertAuthorityV2)
	if !ok {
		return trace.BadParameter("expected %s CA to be *types.CertAuthorityV2, got %T", fromType, existingCA)
	}

	newCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        toType,
		ClusterName: cluster,
		ActiveKeys:  existingCAV2.Spec.ActiveKeys,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = trustSvc.CreateCertAuthority(ctx, newCA)
	if trace.IsAlreadyExists(err) {
		log.Warnf("%s CA has already been created by a different Auth instance", toType)
		return nil
	}
	return trace.Wrap(err)
}
