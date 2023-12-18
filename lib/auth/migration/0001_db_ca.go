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
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

// createDBAuthority performs a migration which creates a
// Database CA for all existing clusters that do not already
// have one. Introduced in v10.
type createDBAuthority struct {
	trustServiceFn    func(b backend.Backend) services.Trust
	configServiceFn   func(b backend.Backend) (services.ClusterConfiguration, error)
	presenceServiceFn func(b backend.Backend) services.Presence
}

func (d createDBAuthority) Version() int64 {
	return 1
}

func (d createDBAuthority) Name() string {
	return "create_db_cas"
}

// Up creates a Database CA for all known clusters. If a Database CA
// already exist for a cluster, it is skipped. If no Host or Database CA
// exist for a cluster, it is also skipped.
func (d createDBAuthority) Up(ctx context.Context, b backend.Backend) error {
	ctx, span := tracer.Start(ctx, "createDBAuthority/Up")
	defer span.End()

	if d.trustServiceFn == nil {
		d.trustServiceFn = func(b backend.Backend) services.Trust {
			return local.NewCAService(b)
		}
	}

	if d.configServiceFn == nil {
		d.configServiceFn = func(b backend.Backend) (services.ClusterConfiguration, error) {
			s, err := local.NewClusterConfigurationService(b)
			return s, trace.Wrap(err)
		}
	}

	if d.presenceServiceFn == nil {
		d.presenceServiceFn = func(b backend.Backend) services.Presence {
			return local.NewPresenceService(b)
		}
	}

	trustSvc := d.trustServiceFn(b)
	configSvc, err := d.configServiceFn(b)
	if err != nil {
		return trace.Wrap(err)
	}
	presenceSvc := d.presenceServiceFn(b)

	return trace.Wrap(d.up(ctx, configSvc, trustSvc, presenceSvc))
}

func (d createDBAuthority) up(ctx context.Context, configSvc services.ClusterConfiguration, trustSvc services.Trust, presenceSvc services.Presence) error {
	localClusterName, err := configSvc.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	trustedClusters, err := presenceSvc.GetTrustedClusters(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	allClusters := []string{localClusterName.GetClusterName()}

	for _, tc := range trustedClusters {
		allClusters = append(allClusters, tc.GetName())
	}

	for _, cluster := range allClusters {
		_, err := trustSvc.GetCertAuthority(ctx, types.CertAuthID{Type: types.DatabaseCA, DomainName: cluster}, false)
		// The migration for this cluster can be skipped since
		// a Database CA already exists.
		if err == nil {
			continue
		}

		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}

		// The Database CA does not exists, so we must check to
		// see if the Host CA exists before proceeding with the migration.
		// If both the Database and Host CA do not exist, then this cluster
		// is brand new and the migration can be avoided because they will
		// both automatically be created. If the Host CA does exist, then
		// a new Database CA should be constructed from it.
		hostCA, err := trustSvc.GetCertAuthority(ctx, types.CertAuthID{Type: types.HostCA, DomainName: cluster}, false)
		if trace.IsNotFound(err) {
			continue
		}
		if err != nil {
			return trace.Wrap(err)
		}

		logrus.Infof("Migrating Database CA cluster: %s", cluster)

		ca, ok := hostCA.(*types.CertAuthorityV2)
		if !ok {
			return trace.BadParameter("expected host CA to be *types.CertAuthorityV2, got %T", hostCA)
		}

		dbCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
			Type:        types.DatabaseCA,
			ClusterName: cluster,
			ActiveKeys: types.CAKeySet{
				TLS: ca.Spec.ActiveKeys.TLS,
			},
		})
		if err != nil {
			return trace.Wrap(err)
		}

		err = trustSvc.CreateCertAuthority(ctx, dbCA)
		if trace.IsAlreadyExists(err) {
			logrus.Warn("Database CA has already been created by a different Auth instance")
			continue
		} else if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// Down deletes existing Database CAs for all clusters.
func (d createDBAuthority) Down(ctx context.Context, b backend.Backend) error {
	tracer := tracing.NewTracer("migrations")
	_, span := tracer.Start(ctx, "migrations/CreateDBAuthorityDown")
	defer span.End()

	if d.trustServiceFn == nil {
		d.trustServiceFn = func(b backend.Backend) services.Trust {
			return local.NewCAService(b)
		}
	}

	trustSvc := d.trustServiceFn(b)
	return trace.Wrap(trustSvc.DeleteAllCertAuthorities(types.DatabaseCA))
}
