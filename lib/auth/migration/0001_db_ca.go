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
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

// createDBAuthority performs a migration which creates a
// Database CA for all existing clusters that do not already
// have one. Introduced in v10.
type createDBAuthority struct {
	trustServiceFn  func(b backend.Backend) *local.CA
	configServiceFn func(b backend.Backend) (services.ClusterConfiguration, error)
}

func (d createDBAuthority) Version() int64 {
	return 1
}

func (d createDBAuthority) Name() string {
	return "create_db_cas"
}

// Up creates a new CA for all known clusters as a copy of the old CA.
// If the new CA already exists for a cluster, it is skipped.
// If neither the old nor the new CA exist for a cluster, it is also skipped.
func (d createDBAuthority) Up(ctx context.Context, b backend.Backend) error {
	ctx, span := tracer.Start(ctx, "createDBAuthority/Up")
	defer span.End()

	if d.trustServiceFn == nil {
		d.trustServiceFn = local.NewCAService
	}

	if d.configServiceFn == nil {
		d.configServiceFn = func(b backend.Backend) (services.ClusterConfiguration, error) {
			s, err := local.NewClusterConfigurationService(b)
			return s, trace.Wrap(err)
		}
	}

	trustSvc := d.trustServiceFn(b)
	configSvc, err := d.configServiceFn(b)
	if err != nil {
		return trace.Wrap(err)
	}

	localClusterName, err := configSvc.GetClusterName(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	trustedClusters, err := trustSvc.GetTrustedClusters(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	allClusters := []string{localClusterName.GetClusterName()}

	for _, tc := range trustedClusters {
		allClusters = append(allClusters, tc.GetName())
	}

	for _, cluster := range allClusters {
		err := migrateDBAuthority(ctx, trustSvc, cluster, types.HostCA, types.DatabaseCA)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Down deletes any existing CAs of the new CA type for all clusters.
func (d createDBAuthority) Down(ctx context.Context, b backend.Backend) error {
	_, span := tracer.Start(ctx, "CreateDBAuthorityDown")
	defer span.End()

	if d.trustServiceFn == nil {
		d.trustServiceFn = func(b backend.Backend) *local.CA {
			return local.NewCAService(b)
		}
	}

	trustSvc := d.trustServiceFn(b)
	return trace.Wrap(trustSvc.DeleteAllCertAuthorities(types.DatabaseCA))
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
		slog.DebugContext(ctx, "Migrations: cert authority already exists.", "authority", toType)
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

	slog.InfoContext(ctx, "Migrating CA", "authority", toType, "cluster", cluster)

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
		slog.WarnContext(ctx, "CA has already been created by a different Auth instance", "authority", toType)
		return nil
	}
	return trace.Wrap(err)
}
