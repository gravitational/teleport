/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mocks

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/tlsca"
)

var _ gcp.SQLAdminClient = (*GCPSQLAdminClientMock)(nil)

// GCPSQLAdminClientMock implements the gcp.GCPSQLAdminClient interface for tests.
type GCPSQLAdminClientMock struct {
	// DatabaseInstance is returned from GetDatabaseInstance.
	DatabaseInstance *sqladmin.DatabaseInstance
	// EphemeralCert is returned from GenerateEphemeralCert.
	EphemeralCert *tls.Certificate
}

func (g *GCPSQLAdminClientMock) UpdateUser(ctx context.Context, db types.Database, dbUser string, user *sqladmin.User) error {
	return nil
}

func (g *GCPSQLAdminClientMock) GetDatabaseInstance(ctx context.Context, db types.Database) (*sqladmin.DatabaseInstance, error) {
	return g.DatabaseInstance, nil
}

func (g *GCPSQLAdminClientMock) GenerateEphemeralCert(ctx context.Context, db types.Database, identity tlsca.Identity) (*tls.Certificate, error) {
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
