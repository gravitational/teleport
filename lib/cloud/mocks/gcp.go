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

package mocks

import (
	"context"
	"crypto"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/gcp"
)

var _ gcp.SQLAdminClient = (*GCPSQLAdminClientMock)(nil)

// GCPSQLAdminClientMock implements the gcp.GCPSQLAdminClient interface for tests.
type GCPSQLAdminClientMock struct {
	// DatabaseInstance is returned from GetDatabaseInstance.
	DatabaseInstance *sqladmin.DatabaseInstance
	// GetDatabaseInstanceError is returned from GetDatabaseInstance.
	GetDatabaseInstanceError error
	// EphemeralCert is returned from GenerateEphemeralCert.
	EphemeralCert string
	// DatabaseUser is returned from GetUser.
	DatabaseUser *sqladmin.User
}

func (g *GCPSQLAdminClientMock) GetUser(ctx context.Context, db types.Database, dbUser string) (*sqladmin.User, error) {
	if g.DatabaseUser == nil {
		return nil, trace.AccessDenied("unauthorized")
	}
	return g.DatabaseUser, nil
}

func (g *GCPSQLAdminClientMock) UpdateUser(ctx context.Context, db types.Database, dbUser string, user *sqladmin.User) error {
	return nil
}

func (g *GCPSQLAdminClientMock) GetDatabaseInstance(ctx context.Context, db types.Database) (*sqladmin.DatabaseInstance, error) {
	return g.DatabaseInstance, g.GetDatabaseInstanceError
}

func (g *GCPSQLAdminClientMock) GenerateEphemeralCert(_ context.Context, _ types.Database, _ time.Time, _ crypto.PublicKey) (string, error) {
	return g.EphemeralCert, nil
}

// GKEClusterEntry is an entry in the GKEMock.Clusters list.
type GKEClusterEntry struct {
	gcp.ClusterDetails
	Config *rest.Config
	TTL    time.Duration
}

// GKEMock implements the gcp.GKEClient interface for tests.
type GKEMock struct {
	gcp.GKEClient
	Clusters []GKEClusterEntry
	Notify   chan struct{}
	Clock    clockwork.Clock
}

func (g *GKEMock) GetClusterRestConfig(ctx context.Context, cfg gcp.ClusterDetails) (*rest.Config, time.Time, error) {
	defer func() {
		g.Notify <- struct{}{}
	}()
	for _, cluster := range g.Clusters {
		if cluster.ClusterDetails == cfg {
			return cluster.Config, g.Clock.Now().Add(cluster.TTL), nil
		}
	}
	return nil, time.Now(), trace.NotFound("cluster not found")
}
