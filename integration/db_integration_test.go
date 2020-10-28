/*
Copyright 2020 Gravitational, Inc.

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

package integration

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/testlog"

	"github.com/jackc/pgconn"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
)

// TestDatabaseAccessRootCluster tests a scenario where a user connects to a
// database service running in a root cluster.
func TestDatabaseAccessRootCluster(t *testing.T) {
	pack := setupDatabaseTest(t)

	// Connect to the database service in root cluster.
	client, err := postgres.MakeTestClient(context.Background(), postgres.TestClientConfig{
		AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
		AuthServer: pack.root.cluster.Process.GetAuthServer(),
		Address:    fmt.Sprintf("%v:%v", Loopback, pack.root.cluster.GetPortWeb()),
		Cluster:    pack.root.cluster.Secrets.SiteName,
		Username:   pack.root.user.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: pack.root.dbService.Name,
			Protocol:    pack.root.dbService.Protocol,
			Username:    "postgres",
			Database:    "test",
		},
	})
	require.NoError(t, err)

	// Execute a query.
	result, err := client.Exec(context.Background(), "select 1").ReadAll()
	require.NoError(t, err)
	require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, result)
	require.Equal(t, uint32(1), pack.root.postgres.QueryCount())
	require.Equal(t, uint32(0), pack.leaf.postgres.QueryCount())

	// Disconnect.
	err = client.Close(context.Background())
	require.NoError(t, err)
}

// TestDatabaseAccessLeafCluster tests a scenario where a user connects to a
// database service running in a leaf cluster via a root cluster.
func TestDatabaseAccessLeafCluster(t *testing.T) {
	pack := setupDatabaseTest(t)

	// Connect to the database service in leaf cluster via root cluster.
	client, err := postgres.MakeTestClient(context.Background(), postgres.TestClientConfig{
		AuthClient: pack.root.cluster.GetSiteAPI(pack.root.cluster.Secrets.SiteName),
		AuthServer: pack.root.cluster.Process.GetAuthServer(),
		Address:    fmt.Sprintf("%v:%v", Loopback, pack.root.cluster.GetPortWeb()), // Connecting via root cluster.
		Cluster:    pack.leaf.cluster.Secrets.SiteName,
		Username:   pack.root.user.GetName(),
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: pack.leaf.dbService.Name,
			Protocol:    pack.leaf.dbService.Protocol,
			Username:    "postgres",
			Database:    "test",
		},
	})
	require.NoError(t, err)

	// Execute a query.
	result, err := client.Exec(context.Background(), "select 1").ReadAll()
	require.NoError(t, err)
	require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, result)
	require.Equal(t, uint32(1), pack.leaf.postgres.QueryCount())
	require.Equal(t, uint32(0), pack.root.postgres.QueryCount())

	// Disconnect.
	err = client.Close(context.Background())
	require.NoError(t, err)
}

type databasePack struct {
	root databaseClusterPack
	leaf databaseClusterPack
}

type databaseClusterPack struct {
	cluster      *TeleInstance
	user         services.User
	role         services.Role
	dbService    service.Database
	dbProcess    *service.TeleportProcess
	dbAuthClient *auth.Client
	postgresAddr string
	postgres     *postgres.TestServer
}

func setupDatabaseTest(t *testing.T) *databasePack {
	// Some global setup.
	tracer := utils.NewTracer(utils.ThisFunction()).Start()
	t.Cleanup(func() { tracer.Stop() })
	utils.InitLoggerForTests(testing.Verbose())
	lib.SetInsecureDevMode(true)
	SetTestTimeouts(100 * time.Millisecond)
	log := testlog.FailureOnly(t)

	// Create ports allocator.
	startPort := utils.PortStartingNumber + (3 * AllocatePortsNum) + 1
	ports, err := utils.GetFreeTCPPorts(AllocatePortsNum, startPort)
	require.NoError(t, err)

	// Generate keypair.
	privateKey, publicKey, err := testauthority.New().GenerateKeyPair("")
	require.NoError(t, err)

	p := &databasePack{
		root: databaseClusterPack{
			postgresAddr: fmt.Sprintf("localhost:%v", ports.PopInt()),
		},
		leaf: databaseClusterPack{
			postgresAddr: fmt.Sprintf("localhost:%v", ports.PopInt()),
		},
	}

	// Create root cluster.
	p.root.cluster = NewInstance(InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New(),
		NodeName:    Host,
		Ports:       ports.PopIntSlice(5),
		Priv:        privateKey,
		Pub:         publicKey,
		log:         log,
	})

	// Create leaf cluster.
	p.leaf.cluster = NewInstance(InstanceConfig{
		ClusterName: "leaf.example.com",
		HostID:      uuid.New(),
		NodeName:    Host,
		Ports:       ports.PopIntSlice(5),
		Priv:        privateKey,
		Pub:         publicKey,
		log:         log,
	})

	// Make root cluster config.
	rcConf := service.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Auth.Preference.SetSecondFactor("off")
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebInterface = true

	// Make leaf cluster config.
	lcConf := service.MakeDefaultConfig()
	lcConf.DataDir = t.TempDir()
	lcConf.Auth.Enabled = true
	lcConf.Auth.Preference.SetSecondFactor("off")
	lcConf.Proxy.Enabled = true
	lcConf.Proxy.DisableWebInterface = true

	// Establish trust b/w root and leaf.
	err = p.root.cluster.CreateEx(p.leaf.cluster.Secrets.AsSlice(), rcConf)
	require.NoError(t, err)
	err = p.leaf.cluster.CreateEx(p.root.cluster.Secrets.AsSlice(), lcConf)
	require.NoError(t, err)

	// Start both clusters.
	err = p.leaf.cluster.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		p.leaf.cluster.StopAll()
	})
	err = p.root.cluster.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		p.root.cluster.StopAll()
	})

	// Setup users and roles on both clusters.
	p.setupUsersAndRoles(t)

	// Update root's certificate authority on leaf to configure role mapping.
	ca, err := p.leaf.cluster.Process.GetAuthServer().GetCertAuthority(services.CertAuthID{
		Type:       services.UserCA,
		DomainName: p.root.cluster.Secrets.SiteName,
	}, false)
	require.NoError(t, err)
	ca.SetRoles(nil) // Reset roles, otherwise they will take precedence.
	ca.SetRoleMap(services.RoleMap{
		{Remote: p.root.role.GetName(), Local: []string{p.leaf.role.GetName()}},
	})
	err = p.leaf.cluster.Process.GetAuthServer().UpsertCertAuthority(ca)
	require.NoError(t, err)

	// Create and start database service in the root cluster.
	p.root.dbService = service.Database{
		Name:     "root-postgres",
		Protocol: defaults.ProtocolPostgres,
		URI:      p.root.postgresAddr,
	}
	rdConf := service.MakeDefaultConfig()
	rdConf.DataDir = t.TempDir()
	rdConf.Token = "static-token-value"
	rdConf.AuthServers = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        net.JoinHostPort(Loopback, p.root.cluster.GetPortWeb()),
		},
	}
	rdConf.Databases.Enabled = true
	rdConf.Databases.Databases = []service.Database{p.root.dbService}
	p.root.dbProcess, p.root.dbAuthClient, err = p.root.cluster.StartDatabase(rdConf)
	require.NoError(t, err)
	t.Cleanup(func() {
		p.root.dbProcess.Close()
	})

	// Create and start database service in the leaf cluster.
	p.leaf.dbService = service.Database{
		Name:     "leaf-postgres",
		Protocol: defaults.ProtocolPostgres,
		URI:      p.leaf.postgresAddr,
	}
	ldConf := service.MakeDefaultConfig()
	ldConf.DataDir = t.TempDir()
	ldConf.Token = "static-token-value"
	ldConf.AuthServers = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        net.JoinHostPort(Loopback, p.leaf.cluster.GetPortWeb()),
		},
	}
	ldConf.Databases.Enabled = true
	ldConf.Databases.Databases = []service.Database{p.leaf.dbService}
	p.leaf.dbProcess, p.leaf.dbAuthClient, err = p.leaf.cluster.StartDatabase(ldConf)
	require.NoError(t, err)
	t.Cleanup(func() {
		p.leaf.dbProcess.Close()
	})

	// Create and start test Postgres in the root cluster.
	p.root.postgres, err = postgres.MakeTestServer(p.root.dbAuthClient, p.root.dbService.Name, p.root.postgresAddr)
	require.NoError(t, err)
	go p.root.postgres.Serve()
	t.Cleanup(func() {
		p.root.postgres.Close()
	})

	// Create and start test Postgres in the leaf cluster.
	p.leaf.postgres, err = postgres.MakeTestServer(p.leaf.dbAuthClient, p.leaf.dbService.Name, p.leaf.postgresAddr)
	require.NoError(t, err)
	go p.leaf.postgres.Serve()
	t.Cleanup(func() {
		p.leaf.postgres.Close()
	})

	return p
}

func (p *databasePack) setupUsersAndRoles(t *testing.T) {
	var err error

	p.root.user, p.root.role, err = auth.CreateUserAndRole(p.root.cluster.Process.GetAuthServer(), "root-user", nil)
	require.NoError(t, err)

	p.root.role.SetDatabaseUsers(services.Allow, []string{services.Wildcard})
	p.root.role.SetDatabaseNames(services.Allow, []string{services.Wildcard})
	err = p.root.cluster.Process.GetAuthServer().UpsertRole(context.Background(), p.root.role)
	require.NoError(t, err)

	p.leaf.user, p.leaf.role, err = auth.CreateUserAndRole(p.root.cluster.Process.GetAuthServer(), "leaf-user", nil)
	require.NoError(t, err)

	p.leaf.role.SetDatabaseUsers(services.Allow, []string{services.Wildcard})
	p.leaf.role.SetDatabaseNames(services.Allow, []string{services.Wildcard})
	err = p.leaf.cluster.Process.GetAuthServer().UpsertRole(context.Background(), p.leaf.role)
	require.NoError(t, err)
}
