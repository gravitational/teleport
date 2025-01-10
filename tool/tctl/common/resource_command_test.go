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

package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/tctl/common/databaseobject"
	"github.com/gravitational/teleport/tool/tctl/common/databaseobjectimportrule"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

// TestDatabaseServerResource tests tctl db_server rm/get commands.
func TestDatabaseServerResource(t *testing.T) {
	dynAddr := helpers.NewDynamicServiceAddr(t)
	caCertFilePath := filepath.Join(t.TempDir(), "ca-cert.pem")
	require.NoError(t, os.WriteFile(caCertFilePath, []byte(fixtures.TLSCACertPEM), 0644))

	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Databases: config.Databases{
			Service: config.Service{
				EnabledFlag: "true",
			},
			Databases: []*config.Database{
				{
					Name:        "example-rds-us-west-1",
					Description: "Example MySQL",
					Protocol:    "mysql",
					URI:         "localhost:33306",
					StaticLabels: map[string]string{
						// pretend it's been "discovered"
						types.DiscoveredNameLabel: "example",
					},
					TLS: config.DatabaseTLS{
						ServerName: "db.example.com",
						CACertFile: caCertFilePath,
					},
				},
				{
					Name:        "example-rds-us-west-2",
					Description: "Example PostgreSQL",
					Protocol:    "postgres",
					URI:         "localhost:33307",
					AdminUser: config.DatabaseAdminUser{
						Name: "root",
					},
					TLS: config.DatabaseTLS{
						Mode:       "verify-ca",
						ServerName: "db.example.com",
						CACertFile: caCertFilePath,
					},
					StaticLabels: map[string]string{
						// pretend it's been "discovered"
						types.DiscoveredNameLabel: "example",
					},
				},
				{
					Name:        "db3",
					Description: "Example MySQL",
					Protocol:    "mysql",
					URI:         "localhost:33308",
					TLS: config.DatabaseTLS{
						ServerName: "db.example.com",
						CACertFile: caCertFilePath,
					},
				},
			},
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag: "true",
			},
			WebAddr: dynAddr.WebAddr,
			TunAddr: dynAddr.TunnelAddr,
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}

	db1, err := types.NewDatabaseV3(types.Metadata{
		Name:        "example-rds-us-west-1",
		Description: "Example MySQL",
		Labels: map[string]string{
			types.OriginLabel:         types.OriginConfigFile,
			types.DiscoveredNameLabel: "example",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "localhost:33306",
		CACert:   fixtures.TLSCACertPEM,
		AdminUser: &types.DatabaseAdminUser{
			Name: "",
		},
		TLS: types.DatabaseTLS{
			Mode:       types.DatabaseTLSMode_VERIFY_FULL,
			ServerName: "db.example.com",
			CACert:     fixtures.TLSCACertPEM,
		},
	})
	require.NoError(t, err)

	db2, err := types.NewDatabaseV3(types.Metadata{
		Name:        "example-rds-us-west-2",
		Description: "Example PostgreSQL",
		Labels: map[string]string{
			types.OriginLabel:         types.OriginConfigFile,
			types.DiscoveredNameLabel: "example",
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:33307",
		CACert:   fixtures.TLSCACertPEM,
		AdminUser: &types.DatabaseAdminUser{
			Name: "root",
		},
		TLS: types.DatabaseTLS{
			Mode:       types.DatabaseTLSMode_VERIFY_CA,
			ServerName: "db.example.com",
			CACert:     fixtures.TLSCACertPEM,
		},
	})
	require.NoError(t, err)

	db3, err := types.NewDatabaseV3(types.Metadata{
		Name:        "db3",
		Description: "Example MySQL",
		Labels:      map[string]string{types.OriginLabel: types.OriginConfigFile},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "localhost:33308",
		CACert:   fixtures.TLSCACertPEM,
		AdminUser: &types.DatabaseAdminUser{
			Name: "",
		},
		TLS: types.DatabaseTLS{
			Mode:       types.DatabaseTLSMode_VERIFY_FULL,
			ServerName: "db.example.com",
			CACert:     fixtures.TLSCACertPEM,
		},
	})
	require.NoError(t, err)

	process := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))
	clt := testenv.MakeDefaultAuthClient(t, process)

	// get all database servers
	buff, err := runResourceCommand(t, clt, []string{"get", types.KindDatabaseServer, "--format=json"})
	require.NoError(t, err)
	requireGotDatabaseServers(t, buff, db1, db2, db3)

	// get specific database server
	wantServer := fmt.Sprintf("%v/%v", types.KindDatabaseServer, db2.GetName())
	buff, err = runResourceCommand(t, clt, []string{"get", wantServer, "--format=json"})
	require.NoError(t, err)
	requireGotDatabaseServers(t, buff, db2)

	// get database servers by discovered name
	wantServersDiscoveredName := fmt.Sprintf("%v/%v", types.KindDatabaseServer, "example")
	buff, err = runResourceCommand(t, clt, []string{"get", wantServersDiscoveredName, "--format=json"})
	require.NoError(t, err)
	requireGotDatabaseServers(t, buff, db1, db2)

	// remove multiple distinct database servers by discovered name is an error
	_, err = runResourceCommand(t, clt, []string{"rm", wantServersDiscoveredName})
	require.ErrorContains(t, err, "db_server/example matches multiple auto-discovered database servers")

	// remove database server by name
	_, err = runResourceCommand(t, clt, []string{"rm", wantServer})
	require.NoError(t, err)

	_, err = runResourceCommand(t, clt, []string{"get", wantServer, "--format=json"})
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	// remove database server by discovered name.
	_, err = runResourceCommand(t, clt, []string{"rm", wantServersDiscoveredName})
	require.NoError(t, err)

	buff, err = runResourceCommand(t, clt, []string{"get", "db_server/db3", "--format=json"})
	require.NoError(t, err)
	requireGotDatabaseServers(t, buff, db3)
}

// TestDatabaseServiceResource tests tctl db_services get commands.
func TestDatabaseServiceResource(t *testing.T) {
	dynAddr := helpers.NewDynamicServiceAddr(t)

	ctx := context.Background()
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag: "true",
			},
			WebAddr: dynAddr.WebAddr,
			TunAddr: dynAddr.TunnelAddr,
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}

	auth := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))
	clt := testenv.MakeDefaultAuthClient(t, auth)

	// Add a lot of DatabaseServices to test pagination
	dbS, err := types.NewDatabaseServiceV1(
		types.Metadata{Name: uuid.NewString()},
		types.DatabaseServiceSpecV1{
			ResourceMatchers: []*types.DatabaseResourceMatcher{
				{Labels: &types.Labels{"env": []string{"prod"}}},
			},
		},
	)
	require.NoError(t, err)

	randomDBServiceName := ""
	totalDBServices := apidefaults.DefaultChunkSize*2 + 20 // testing partial pages
	for i := 0; i < totalDBServices; i++ {
		dbS.SetName(uuid.NewString())
		if i == apidefaults.DefaultChunkSize { // A "random" database service name
			randomDBServiceName = dbS.GetName()
		}
		_, err = auth.GetAuthServer().UpsertDatabaseService(ctx, dbS)
		require.NoError(t, err)
	}

	t.Run("test pagination of database services ", func(t *testing.T) {
		buff, err := runResourceCommand(t, clt, []string{"get", types.KindDatabaseService, "--format=json"})
		require.NoError(t, err)
		out := mustDecodeJSON[[]*types.DatabaseServiceV1](t, buff)
		require.Len(t, out, totalDBServices)
	})

	service := fmt.Sprintf("%v/%v", types.KindDatabaseService, randomDBServiceName)

	t.Run("get specific database service", func(t *testing.T) {
		buff, err := runResourceCommand(t, clt, []string{"get", service, "--format=json"})
		require.NoError(t, err)
		out := mustDecodeJSON[[]*types.DatabaseServiceV1](t, buff)
		require.Len(t, out, 1)
		require.Equal(t, randomDBServiceName, out[0].GetName())
	})

	t.Run("get unknown database service", func(t *testing.T) {
		unknownService := fmt.Sprintf("%v/%v", types.KindDatabaseService, "unknown")
		_, err := runResourceCommand(t, clt, []string{"get", unknownService, "--format=json"})
		require.True(t, trace.IsNotFound(err), "expected a NotFound error, got %v", err)
	})

	t.Run("get specific database service with human output", func(t *testing.T) {
		buff, err := runResourceCommand(t, clt, []string{"get", service, "--format=text"})
		require.NoError(t, err)
		outputString := buff.String()
		require.Contains(t, outputString, "env=[prod]")
		require.Contains(t, outputString, randomDBServiceName)
	})
}

// TestIntegrationResource tests tctl integration commands.
func TestIntegrationResource(t *testing.T) {
	dynAddr := helpers.NewDynamicServiceAddr(t)

	ctx := context.Background()
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag: "true",
			},
			WebAddr: dynAddr.WebAddr,
			TunAddr: dynAddr.TunnelAddr,
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}

	auth := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))
	clt := testenv.MakeDefaultAuthClient(t, auth)

	t.Run("get", func(t *testing.T) {

		// Add a lot of Integrations to test pagination
		ig1, err := types.NewIntegrationAWSOIDC(
			types.Metadata{Name: uuid.NewString()},
			&types.AWSOIDCIntegrationSpecV1{
				RoleARN: "arn:aws:iam::123456789012:role/OpsTeam",
			},
		)
		require.NoError(t, err)

		randomIntegrationName := ""
		totalIntegrations := apidefaults.DefaultChunkSize*2 + 20 // testing partial pages
		for i := 0; i < totalIntegrations; i++ {
			ig1.SetName(uuid.NewString())
			if i == apidefaults.DefaultChunkSize { // A "random" integration name
				randomIntegrationName = ig1.GetName()
			}
			_, err = auth.GetAuthServer().CreateIntegration(ctx, ig1)
			require.NoError(t, err)
		}

		t.Run("test pagination of integrations ", func(t *testing.T) {
			buff, err := runResourceCommand(t, clt, []string{"get", types.KindIntegration, "--format=json"})
			require.NoError(t, err)
			out := mustDecodeJSON[[]types.IntegrationV1](t, buff)
			require.Len(t, out, totalIntegrations)
		})

		igName := fmt.Sprintf("%v/%v", types.KindIntegration, randomIntegrationName)

		t.Run("get specific integration", func(t *testing.T) {
			buff, err := runResourceCommand(t, clt, []string{"get", igName, "--format=json"})
			require.NoError(t, err)
			out := mustDecodeJSON[[]types.IntegrationV1](t, buff)
			require.Len(t, out, 1)
			require.Equal(t, randomIntegrationName, out[0].GetName())
		})

		t.Run("get unknown integration", func(t *testing.T) {
			unknownIntegration := fmt.Sprintf("%v/%v", types.KindIntegration, "unknown")
			_, err := runResourceCommand(t, clt, []string{"get", unknownIntegration, "--format=json"})
			require.True(t, trace.IsNotFound(err), "expected a NotFound error, got %v", err)
		})

		t.Run("get specific integration with human output", func(t *testing.T) {
			buff, err := runResourceCommand(t, clt, []string{"get", igName, "--format=text"})
			require.NoError(t, err)
			outputString := buff.String()
			require.Contains(t, outputString, "RoleARN=arn:aws:iam::123456789012:role/OpsTeam")
			require.Contains(t, outputString, randomIntegrationName)
		})
	})

	t.Run("create", func(t *testing.T) {
		integrationYAMLPath := filepath.Join(t.TempDir(), "integration.yaml")
		require.NoError(t, os.WriteFile(integrationYAMLPath, []byte(integrationYAML), 0644))
		_, err := runResourceCommand(t, clt, []string{"create", integrationYAMLPath})
		require.NoError(t, err)

		buff, err := runResourceCommand(t, clt, []string{"get", "integration/myawsint", "--format=text"})
		require.NoError(t, err)
		outputString := buff.String()
		require.Contains(t, outputString, "RoleARN=arn:aws:iam::123456789012:role/OpsTeam")
		require.Contains(t, outputString, "myawsint")

		// Update the RoleARN to another role
		integrationYAMLV2 := strings.ReplaceAll(integrationYAML, "OpsTeam", "DevTeam")
		require.NoError(t, os.WriteFile(integrationYAMLPath, []byte(integrationYAMLV2), 0644))

		// Trying to create it again should return an error
		_, err = runResourceCommand(t, clt, []string{"create", integrationYAMLPath})
		require.True(t, trace.IsAlreadyExists(err), "expected already exists error, got %v", err)

		// Using the force should be ok and replace the current object
		_, err = runResourceCommand(t, clt, []string{"create", "--force", integrationYAMLPath})
		require.NoError(t, err)

		// The RoleARN must be updated
		buff, err = runResourceCommand(t, clt, []string{"get", "integration/myawsint", "--format=text"})
		require.NoError(t, err)
		outputString = buff.String()
		require.Contains(t, outputString, "RoleARN=arn:aws:iam::123456789012:role/DevTeam")
	})
}

// TestDiscoveryConfigResource tests tctl discoveryConfig commands.
func TestDiscoveryConfigResource(t *testing.T) {
	dynAddr := helpers.NewDynamicServiceAddr(t)

	ctx := context.Background()
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag: "true",
			},
			WebAddr: dynAddr.WebAddr,
			TunAddr: dynAddr.TunnelAddr,
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}

	auth := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))
	clt := testenv.MakeDefaultAuthClient(t, auth)

	t.Run("get", func(t *testing.T) {
		// Add a lot of DiscoveryConfigs to test pagination
		dc, err := discoveryconfig.NewDiscoveryConfig(
			header.Metadata{
				Name: "mydiscoveryconfig",
			},
			discoveryconfig.Spec{
				DiscoveryGroup: "prod-resources",
			},
		)
		require.NoError(t, err)

		randomDiscoveryConfigName := ""
		totalDiscoveryConfigs := apidefaults.DefaultChunkSize*2 + 20 // testing partial pages
		for i := 0; i < totalDiscoveryConfigs; i++ {
			dc.SetName(uuid.NewString())
			if i == apidefaults.DefaultChunkSize { // A "random" discoveryConfig name
				randomDiscoveryConfigName = dc.GetName()
			}
			_, err = auth.GetAuthServer().CreateDiscoveryConfig(ctx, dc)
			require.NoError(t, err)
		}

		t.Run("test pagination of discovery configs ", func(t *testing.T) {
			buff, err := runResourceCommand(t, clt, []string{"get", types.KindDiscoveryConfig, "--format=json"})
			require.NoError(t, err)
			out := mustDecodeJSON[[]discoveryconfig.DiscoveryConfig](t, buff)
			require.Len(t, out, totalDiscoveryConfigs)
		})

		dcName := fmt.Sprintf("%v/%v", types.KindDiscoveryConfig, randomDiscoveryConfigName)

		t.Run("get specific discovery config", func(t *testing.T) {
			buff, err := runResourceCommand(t, clt, []string{"get", dcName, "--format=json"})
			require.NoError(t, err)
			out := mustDecodeJSON[[]discoveryconfig.DiscoveryConfig](t, buff)
			require.Len(t, out, 1)
			require.Equal(t, randomDiscoveryConfigName, out[0].GetName())
		})

		t.Run("get unknown discovery config", func(t *testing.T) {
			unknownDiscoveryConfig := fmt.Sprintf("%v/%v", types.KindDiscoveryConfig, "unknown")
			_, err := runResourceCommand(t, clt, []string{"get", unknownDiscoveryConfig, "--format=json"})
			require.True(t, trace.IsNotFound(err), "expected a NotFound error, got %v", err)
		})

		t.Run("get specific discovery config with human output", func(t *testing.T) {
			buff, err := runResourceCommand(t, clt, []string{"get", dcName, "--format=text"})
			require.NoError(t, err)
			outputString := buff.String()
			require.Contains(t, outputString, "prod-resources")
			require.Contains(t, outputString, randomDiscoveryConfigName)
		})
	})

	t.Run("create", func(t *testing.T) {
		discoveryConfigYAMLPath := filepath.Join(t.TempDir(), "discoveryConfig.yaml")
		require.NoError(t, os.WriteFile(discoveryConfigYAMLPath, []byte(discoveryConfigYAML), 0644))
		_, err := runResourceCommand(t, clt, []string{"create", discoveryConfigYAMLPath})
		require.NoError(t, err)

		buff, err := runResourceCommand(t, clt, []string{"get", "discovery_config/my-discovery-config", "--format=text"})
		require.NoError(t, err)
		outputString := buff.String()
		require.Contains(t, outputString, "my-discovery-config")
		require.Contains(t, outputString, "mydg1")

		// Update the discovery group to another group
		discoveryConfigYAMLV2 := strings.ReplaceAll(discoveryConfigYAML, "mydg1", "mydg2")
		require.NoError(t, os.WriteFile(discoveryConfigYAMLPath, []byte(discoveryConfigYAMLV2), 0644))

		// Trying to create it again should return an error
		_, err = runResourceCommand(t, clt, []string{"create", discoveryConfigYAMLPath})
		require.True(t, trace.IsAlreadyExists(err), "expected already exists error, got %v", err)

		// Using the force should be ok and replace the current object
		_, err = runResourceCommand(t, clt, []string{"create", "--force", discoveryConfigYAMLPath})
		require.NoError(t, err)

		// The DiscoveryGroup must be updated
		buff, err = runResourceCommand(t, clt, []string{"get", "discovery_config/my-discovery-config", "--format=text"})
		require.NoError(t, err)
		outputString = buff.String()
		require.Contains(t, outputString, "mydg2")
	})
}

func TestCreateLock(t *testing.T) {
	dynAddr := helpers.NewDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag: "true",
			},
			WebAddr: dynAddr.WebAddr,
			TunAddr: dynAddr.TunnelAddr,
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}

	timeNow := time.Now().UTC()
	fakeClock := clockwork.NewFakeClockAt(timeNow)
	process := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors), withFakeClock(fakeClock))
	clt := testenv.MakeDefaultAuthClient(t, process)

	_, err := types.NewLock("test-lock", types.LockSpecV2{
		Target: types.LockTarget{
			User: "bad@actor",
		},
		Message: "I am a message",
	})
	require.NoError(t, err)

	// Ensure there are no locks to start
	buf, err := runResourceCommand(t, clt, []string{"get", types.KindLock, "--format=json"})
	require.NoError(t, err)
	locks := mustDecodeJSON[[]*types.LockV2](t, buf)
	require.Empty(t, locks)

	// Create the locks
	lockYAMLPath := filepath.Join(t.TempDir(), "lock.yaml")
	require.NoError(t, os.WriteFile(lockYAMLPath, []byte(lockYAML), 0644))
	_, err = runResourceCommand(t, clt, []string{"create", lockYAMLPath})
	require.NoError(t, err)

	// Fetch the locks
	buf, err = runResourceCommand(t, clt, []string{"get", types.KindLock, "--format=json"})
	require.NoError(t, err)
	locks = mustDecodeJSON[[]*types.LockV2](t, buf)
	require.Len(t, locks, 1)

	expected, err := types.NewLock("test-lock", types.LockSpecV2{
		Target: types.LockTarget{
			User: "bad@actor",
		},
		Message: "Come see me",
	})
	require.NoError(t, err)
	expected.SetCreatedBy(string(types.RoleAdmin))

	expected.SetCreatedAt(timeNow)

	require.Empty(t, cmp.Diff([]*types.LockV2{expected.(*types.LockV2)}, locks,
		cmpopts.IgnoreFields(types.LockV2{}, "Metadata")))
}

// TestCreateDatabaseInInsecureMode connects to auth server with --insecure mode and creates a DB resource.
func TestCreateDatabaseInInsecureMode(t *testing.T) {
	dynAddr := helpers.NewDynamicServiceAddr(t)

	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Databases: config.Databases{
			Service: config.Service{
				EnabledFlag: "true",
			},
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag: "true",
			},
			WebAddr: dynAddr.WebAddr,
			TunAddr: dynAddr.TunnelAddr,
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}

	process := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))
	clt := testenv.MakeDefaultAuthClient(t, process)

	// Create the databases yaml file.
	dbYAMLPath := filepath.Join(t.TempDir(), "db.yaml")
	require.NoError(t, os.WriteFile(dbYAMLPath, []byte(dbYAML), 0644))

	_, err := runResourceCommand(t, clt, []string{"create", dbYAMLPath})
	require.NoError(t, err)
}

const (
	dbYAML = `kind: db
version: v3
metadata:
  name: foo
spec:
  protocol: "mysql"
  uri: "localhost:3306"
  tls:
    mode: "verify-ca"
---
kind: db
version: v3
metadata:
  name: foo-bar-1
  labels:
    teleport.internal/discovered-name: "foo-bar"
spec:
  protocol: "postgres"
  uri: "localhost:5433"
  tls:
    mode: "verify-full"
---
kind: db
version: v3
metadata:
  name: foo-bar-2
  labels:
    teleport.internal/discovered-name: "foo-bar"
spec:
  protocol: "postgres"
  uri: "localhost:5432"`

	appYAML = `kind: app
version: v3
metadata:
  name: foo
spec:
  uri: "tcp://localhost1"
  tcp_ports:
  - port: 1234
  - port: 30000
    end_port: 30768
---
kind: app
version: v3
metadata:
  name: foo-bar-1
  labels:
    teleport.internal/discovered-name: "foo-bar"
spec:
  uri: "localhost2"
---
kind: app
version: v3
metadata:
  name: foo-bar-2
  labels:
    teleport.internal/discovered-name: "foo-bar"
spec:
  uri: "localhost3"`

	kubeYAML = `
kind: kube_cluster
version: v3
metadata:
  name: foo
spec: {}
---
kind: kube_cluster
version: v3
metadata:
  name: foo-bar-1
  labels:
    teleport.internal/discovered-name: "foo-bar"
spec: {}
---
kind: kube_cluster
version: v3
metadata:
  name: foo-bar-2
  labels:
    teleport.internal/discovered-name: "foo-bar"
spec: {}`

	lockYAML = `kind: lock
version: v2
metadata:
  name: "test-lock"
spec:
  target:
    user: "bad@actor"
  message: "Come see me"`

	integrationYAML = `kind: integration
sub_kind: aws-oidc
version: v1
metadata:
  name: myawsint
spec:
  aws_oidc:
    role_arn: "arn:aws:iam::123456789012:role/OpsTeam"
`

	discoveryConfigYAML = `kind: discovery_config
version: v1
metadata:
  name: my-discovery-config
spec:
  discovery_group: mydg1
  aws:
  - types: ["ec2"]
    regions: ["eu-west-2"]
`
)

func TestCreateClusterAuthPreference_WithSupportForSecondFactorWithoutQuotes(t *testing.T) {
	dynAddr := helpers.NewDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}

	process := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))
	clt := testenv.MakeDefaultAuthClient(t, process)

	tests := []struct {
		desc               string
		input              string
		expectError        require.ErrorAssertionFunc
		expectSecondFactor require.ValueAssertionFunc
	}{
		{desc: "handle off with quotes", input: `
kind: cluster_auth_preference
metadata:
  name: cluster-auth-preference
spec:
  second_factor: "off"
  type: local
version: v2`,
			expectError:        require.NoError,
			expectSecondFactor: requireEqual(constants.SecondFactorOff)},
		{desc: "handle off without quotes", input: `
kind: cluster_auth_preference
metadata:
  name: cluster-auth-preference
spec:
  second_factor: off
  type: local
version: v2`,
			expectError:        require.NoError,
			expectSecondFactor: requireEqual(constants.SecondFactorOff)},
		{desc: "handle on without quotes", input: `
kind: cluster_auth_preference
metadata:
  name: cluster-auth-preference
spec:
  second_factor: on
  webauthn:
    rp_id: localhost
  type: local
version: v2`,
			expectError:        require.NoError,
			expectSecondFactor: requireEqual(constants.SecondFactorOn)},
		{desc: "unsupported numeric type as second_factor", input: `
kind: cluster_auth_preference
metadata:
  name: cluster-auth-preference
spec:
  second_factor: 4.3
  type: local
version: v2`,
			expectError: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			capYAMLPath := filepath.Join(t.TempDir(), "cap.yaml")
			require.NoError(t, os.WriteFile(capYAMLPath, []byte(tt.input), 0644))

			_, err := runResourceCommand(t, clt, []string{"create", "-f", capYAMLPath})
			tt.expectError(t, err)

			if tt.expectSecondFactor != nil {
				buf, err := runResourceCommand(t, clt, []string{"get", "cap", "--format=json"})
				require.NoError(t, err)
				authPreferences := mustDecodeJSON[[]types.AuthPreferenceV2](t, buf)
				require.NotEmpty(t, authPreferences)
				tt.expectSecondFactor(t, authPreferences[0].Spec.SecondFactor)
			}
		})
	}
}

func TestCreateSAMLIdPServiceProvider(t *testing.T) {
	dynAddr := helpers.NewDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}

	process := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))
	clt := testenv.MakeDefaultAuthClient(t, process)

	tests := []struct {
		desc           string
		input          string
		name           string
		expectError    require.ErrorAssertionFunc
		expectEntityID require.ValueAssertionFunc
	}{
		{
			desc: "handle no supplied entity ID",
			input: `
kind: saml_idp_service_provider
version: v1
metadata:
  name: test1
spec:
  entity_descriptor: |
    <?xml version="1.0" encoding="UTF-8"?>
    <md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" entityID="IAMShowcase" validUntil="2025-12-09T09:13:31.006Z">
       <md:SPSSODescriptor AuthnRequestsSigned="false" WantAssertionsSigned="true" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
          <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
          <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
          <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://sptest.iamshowcase.com/acs" index="0" isDefault="true"/>
       </md:SPSSODescriptor>
    </md:EntityDescriptor>
`,
			name:           "test1",
			expectError:    require.NoError,
			expectEntityID: requireEqual("IAMShowcase"),
		},
		{
			desc: "handle overwrite entity ID",
			input: `
kind: saml_idp_service_provider
version: v1
metadata:
  name: test1
spec:
  entity_descriptor: |
    <?xml version="1.0" encoding="UTF-8"?>
    <md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" entityID="IAMShowcase" validUntil="2025-12-09T09:13:31.006Z">
       <md:SPSSODescriptor AuthnRequestsSigned="false" WantAssertionsSigned="true" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
          <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
          <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
          <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://sptest.iamshowcase.com/acs" index="0" isDefault="true"/>
       </md:SPSSODescriptor>
    </md:EntityDescriptor>
  entity_id: never-seen-entity-id
`,
			name:        "test1",
			expectError: require.Error,
		},
		{
			desc: "handle invalid entity descriptor",
			input: `
kind: saml_idp_service_provider
version: v1
metadata:
  name: test1
spec:
  entity_descriptor: |
    <?xml version="1.0" encoding="UTF-8"?>
    <md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" entityID="IAMShowcase" validUntil="2025-12-09T09:13:31.006Z">
`,
			name:        "test1",
			expectError: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			spYAMLPath := filepath.Join(t.TempDir(), "sp.yaml")
			require.NoError(t, os.WriteFile(spYAMLPath, []byte(tt.input), 0644))

			_, err := runResourceCommand(t, clt, []string{"create", "-f", spYAMLPath})
			tt.expectError(t, err)

			if tt.expectEntityID != nil {
				buf, err := runResourceCommand(t, clt, []string{"get", fmt.Sprintf("saml_sp/%s", tt.name), "--format=json"})
				require.NoError(t, err)
				sps := mustDecodeJSON[[]*types.SAMLIdPServiceProviderV1](t, buf)
				tt.expectEntityID(t, sps[0].GetEntityID())
			}
		})
	}
}

func TestUpsertVerb(t *testing.T) {
	tests := []struct {
		name     string
		exists   bool
		force    bool
		expected string
	}{
		{
			name:     "exists && force",
			exists:   true,
			force:    true,
			expected: "created",
		},
		{
			name:     "!exists && force",
			exists:   false,
			force:    true,
			expected: "created",
		},
		{
			name:     "exists && !force",
			exists:   true,
			force:    false,
			expected: "updated",
		},
		{
			name:     "!exists && !force",
			exists:   false,
			force:    false,
			expected: "created",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := UpsertVerb(test.exists, test.force)
			require.Equal(t, test.expected, actual)
		})
	}
}

type dynamicResourceTest[T types.ResourceWithLabels] struct {
	kind                    string
	resourceYAML            string
	fooResource             T
	fooBar1Resource         T
	fooBar2Resource         T
	runDiscoveredNameChecks bool
}

func (test *dynamicResourceTest[T]) setup(t *testing.T) *authclient.Client {
	t.Helper()
	requireResource := func(t *testing.T, r T, name string) {
		t.Helper()
		require.NotNil(t, r, "dynamicResourceTest requires a resource named %q", name)
		require.Equal(t, r.GetName(), name, "dynamicResourceTest requires a resource named %q", name)
	}
	requireResource(t, test.fooResource, "foo")
	requireResource(t, test.fooBar1Resource, "foo-bar-1")
	requireResource(t, test.fooBar2Resource, "foo-bar-2")
	dynAddr := helpers.NewDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag: "true",
			},
			WebAddr: dynAddr.WebAddr,
			TunAddr: dynAddr.TunnelAddr,
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}
	process := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))
	clt := testenv.MakeDefaultAuthClient(t, process)
	return clt
}

func (test *dynamicResourceTest[T]) run(t *testing.T) {
	t.Helper()
	clt := test.setup(t)

	// Initially there are no resources.
	buf, err := runResourceCommand(t, clt, []string{"get", test.kind, "--format=json"})
	require.NoError(t, err)
	resources := mustDecodeJSON[[]T](t, buf)
	require.Empty(t, resources)

	// Create the resources.
	yamlPath := filepath.Join(t.TempDir(), "resources.yaml")
	require.NoError(t, os.WriteFile(yamlPath, []byte(test.resourceYAML), 0644))
	_, err = runResourceCommand(t, clt, []string{"create", yamlPath})
	require.NoError(t, err)

	// Fetch all resources.
	buf, err = runResourceCommand(t, clt, []string{"get", test.kind, "--format=json"})
	require.NoError(t, err)
	resources = mustDecodeJSON[[]T](t, buf)
	require.Len(t, resources, 3)
	require.Empty(t, cmp.Diff([]T{test.fooResource, test.fooBar1Resource, test.fooBar2Resource}, resources,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Namespace"),
	))

	// Fetch specific resource.
	buf, err = runResourceCommand(t, clt,
		[]string{"get", fmt.Sprintf("%v/%v", test.kind, test.fooResource.GetName()), "--format=json"})
	require.NoError(t, err)
	resources = mustDecodeJSON[[]T](t, buf)
	require.Len(t, resources, 1)
	require.Empty(t, cmp.Diff([]T{test.fooResource}, resources,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Namespace"),
	))

	// Remove a resource.
	_, err = runResourceCommand(t, clt, []string{"rm", fmt.Sprintf("%v/%v", test.kind, test.fooResource.GetName())})
	require.NoError(t, err)

	// Fetch all resources again.
	buf, err = runResourceCommand(t, clt, []string{"get", test.kind, "--format=json"})
	require.NoError(t, err)
	resources = mustDecodeJSON[[]T](t, buf)
	require.Len(t, resources, 2)
	require.Empty(t, cmp.Diff([]T{test.fooBar1Resource, test.fooBar2Resource}, resources,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Namespace"),
	))

	if !test.runDiscoveredNameChecks {
		return
	}

	// Test discovered name behavior.
	// Fetching multiple resources ("foo-bar-1" and "foo-bar-2") by discovered name is ok.
	buf, err = runResourceCommand(t, clt, []string{"get", fmt.Sprintf("%v/%v", test.kind, "foo-bar"), "--format=json"})
	require.NoError(t, err)
	resources = mustDecodeJSON[[]T](t, buf)
	require.Len(t, resources, 2)
	require.Empty(t, cmp.Diff([]T{test.fooBar1Resource, test.fooBar2Resource}, resources,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Namespace"),
	))

	// Removing multiple resources ("foo-bar-1" and "foo-bar-2") by discovered name is an error.
	_, err = runResourceCommand(t, clt, []string{"rm", fmt.Sprintf("%v/%v", test.kind, "foo-bar")})
	require.ErrorContains(t, err, "matches multiple")

	// Remove "foo-bar-2" resource by full name.
	_, err = runResourceCommand(t, clt, []string{"rm", fmt.Sprintf("%v/%v", test.kind, test.fooBar2Resource.GetName())})
	require.NoError(t, err)

	// Fetch all resources again.
	buf, err = runResourceCommand(t, clt, []string{"get", test.kind, "--format=json"})
	require.NoError(t, err)
	resources = mustDecodeJSON[[]T](t, buf)
	require.Len(t, resources, 1)
	require.Empty(t, cmp.Diff([]T{test.fooBar1Resource}, resources,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Namespace"),
	))
}

// TestDatabaseResource tests tctl commands that manage database resources.
func TestDatabaseResource(t *testing.T) {
	t.Parallel()
	dbFoo, err := types.NewDatabaseV3(types.Metadata{
		Name:   "foo",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "localhost:3306",
		TLS: types.DatabaseTLS{
			Mode: types.DatabaseTLSMode_VERIFY_CA,
		},
	})
	require.NoError(t, err)
	dbFooBar, err := types.NewDatabaseV3(types.Metadata{
		Name:   "foo-bar-1",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic, types.DiscoveredNameLabel: "foo-bar"},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5433",
		TLS: types.DatabaseTLS{
			Mode: types.DatabaseTLSMode_VERIFY_FULL,
		},
	})
	require.NoError(t, err)
	dbFooBar2, err := types.NewDatabaseV3(types.Metadata{
		Name:   "foo-bar-2",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic, types.DiscoveredNameLabel: "foo-bar"},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)

	require.NoError(t, err)
	test := dynamicResourceTest[*types.DatabaseV3]{
		kind:                    types.KindDatabase,
		resourceYAML:            dbYAML,
		fooResource:             dbFoo,
		fooBar1Resource:         dbFooBar,
		fooBar2Resource:         dbFooBar2,
		runDiscoveredNameChecks: true,
	}
	test.run(t)
}

// TestKubeClusterResource tests tctl commands that manage dynamic kube cluster resources.
func TestKubeClusterResource(t *testing.T) {
	t.Parallel()
	kubeFoo, err := types.NewKubernetesClusterV3(types.Metadata{
		Name:   "foo",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)
	kubeFooBar1, err := types.NewKubernetesClusterV3(types.Metadata{
		Name:   "foo-bar-1",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic, types.DiscoveredNameLabel: "foo-bar"},
	}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)
	kubeFooBar2, err := types.NewKubernetesClusterV3(types.Metadata{
		Name:   "foo-bar-2",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic, types.DiscoveredNameLabel: "foo-bar"},
	}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)
	test := dynamicResourceTest[*types.KubernetesClusterV3]{
		kind:                    types.KindKubernetesCluster,
		resourceYAML:            kubeYAML,
		fooResource:             kubeFoo,
		fooBar1Resource:         kubeFooBar1,
		fooBar2Resource:         kubeFooBar2,
		runDiscoveredNameChecks: true,
	}
	test.run(t)
}

// TestAppResource tests tctl commands that manage application resources.
func TestAppResource(t *testing.T) {
	t.Parallel()
	appFoo, err := types.NewAppV3(types.Metadata{
		Name:   "foo",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.AppSpecV3{
		URI: "tcp://localhost1",
		TCPPorts: []*types.PortRange{
			&types.PortRange{Port: 1234},
			&types.PortRange{Port: 30000, EndPort: 30768},
		},
	})
	require.NoError(t, err)
	appFooBar1, err := types.NewAppV3(types.Metadata{
		Name:   "foo-bar-1",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic, types.DiscoveredNameLabel: "foo-bar"},
	}, types.AppSpecV3{
		URI: "localhost2",
	})
	require.NoError(t, err)
	appFooBar2, err := types.NewAppV3(types.Metadata{
		Name:   "foo-bar-2",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic, types.DiscoveredNameLabel: "foo-bar"},
	}, types.AppSpecV3{
		URI: "localhost3",
	})
	require.NoError(t, err)
	test := dynamicResourceTest[*types.AppV3]{
		kind:            types.KindApp,
		resourceYAML:    appYAML,
		fooResource:     appFoo,
		fooBar1Resource: appFooBar1,
		fooBar2Resource: appFooBar2,
	}
	test.run(t)
}

func TestGetOneResourceNameToDelete(t *testing.T) {
	foo1 := mustCreateNewKubeServer(t, "foo-eks", "host-foo1", "foo", nil)
	foo2 := mustCreateNewKubeServer(t, "foo-eks", "host-foo2", "foo", nil)
	fooBar1 := mustCreateNewKubeServer(t, "foo-bar-eks-us-west-1", "host-foo-bar1", "foo-bar", nil)
	fooBar2 := mustCreateNewKubeServer(t, "foo-bar-eks-us-west-2", "host-foo-bar2", "foo-bar", nil)
	tests := []struct {
		desc            string
		refName         string
		wantErrContains string
		resources       []types.KubeServer
		wantName        string
	}{
		{
			desc:      "one resource is ok",
			refName:   "foo-bar-eks-us-west-1",
			resources: []types.KubeServer{fooBar1},
			wantName:  "foo-bar-eks-us-west-1",
		},
		{
			desc:      "multiple resources with same name is ok",
			refName:   "foo",
			resources: []types.KubeServer{foo1, foo2},
			wantName:  "foo-eks",
		},
		{
			desc:            "zero resources is an error",
			refName:         "xxx",
			wantErrContains: `kubernetes server "xxx" not found`,
		},
		{
			desc:            "multiple resources with different names is an error",
			refName:         "foo-bar",
			resources:       []types.KubeServer{fooBar1, fooBar2},
			wantErrContains: "matches multiple",
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			ref := services.Ref{Kind: types.KindKubeServer, Name: test.refName}
			resDesc := "kubernetes server"
			name, err := getOneResourceNameToDelete(test.resources, ref, resDesc)
			if test.wantErrContains != "" {
				require.ErrorContains(t, err, test.wantErrContains)
				return
			}
			require.Equal(t, test.wantName, name)
		})
	}
}

func TestFilterByNameOrDiscoveredName(t *testing.T) {
	foo1 := mustCreateNewKubeServer(t, "foo-eks-us-west-1", "host-foo", "foo", nil)
	foo2 := mustCreateNewKubeServer(t, "foo-eks-us-west-2", "host-foo", "foo", nil)
	fooBar1 := mustCreateNewKubeServer(t, "foo-bar", "host-foo-bar1", "", nil)
	fooBar2 := mustCreateNewKubeServer(t, "foo-bar-eks-us-west-2", "host-foo-bar2", "foo-bar", nil)
	resources := []types.KubeServer{
		foo1, foo2, fooBar1, fooBar2,
	}
	hostNameGetter := func(ks types.KubeServer) string { return ks.GetHostname() }
	tests := []struct {
		desc           string
		filter         string
		altNameGetters []altNameFn[types.KubeServer]
		want           []types.KubeServer
	}{
		{
			desc:   "filters by exact name",
			filter: "foo-eks-us-west-1",
			want:   []types.KubeServer{foo1},
		},
		{
			desc:   "filters by exact name over discovered names",
			filter: "foo-bar",
			want:   []types.KubeServer{fooBar1},
		},
		{
			desc:   "filters by discovered name",
			filter: "foo",
			want:   []types.KubeServer{foo1, foo2},
		},
		{
			desc:           "checks alt names for exact matches",
			filter:         "host-foo",
			altNameGetters: []altNameFn[types.KubeServer]{hostNameGetter},
			want:           []types.KubeServer{foo1, foo2},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			got := filterByNameOrDiscoveredName(resources, test.filter, test.altNameGetters...)
			require.Empty(t, cmp.Diff(test.want, got))
		})
	}
}

func TestFormatAmbiguousDeleteMessage(t *testing.T) {
	ref := services.Ref{Kind: types.KindDatabase, Name: "x"}
	resDesc := "database"
	names := []string{"xbbb", "xaaa", "xccc", "xb"}
	got := formatAmbiguousDeleteMessage(ref, resDesc, names)
	require.Contains(t, got, "db/x matches multiple auto-discovered databases",
		"should have formatted the ref used and pluralized the resource description")
	wantSortedNames := strings.Join([]string{"xaaa", "xb", "xbbb", "xccc"}, "\n")
	require.Contains(t, got, wantSortedNames, "should have sorted the matching names")
	require.Contains(t, got, "$ tctl rm db/xaaa", "should have contained an example command")
}

// requireEqual creates an assertion function with a bound `expected` value
// for use with table-driven tests
func requireEqual(expected interface{}) require.ValueAssertionFunc {
	return func(t require.TestingT, actual interface{}, msgAndArgs ...interface{}) {
		require.Equal(t, expected, actual, msgAndArgs...)
	}
}

// helper for decoding the output of runResourceCommand and checking we got
// the databases expected.
func requireGotDatabaseServers(t *testing.T, buf *bytes.Buffer, want ...types.Database) {
	t.Helper()
	servers := mustDecodeJSON[[]*types.DatabaseServerV3](t, buf)
	require.Len(t, servers, len(want))
	databases := types.Databases{}
	for _, server := range servers {
		databases = append(databases, server.GetDatabase())
	}
	require.Empty(t, cmp.Diff(types.Databases(want).ToMap(), databases.ToMap(),
		cmpopts.IgnoreFields(types.Metadata{}, "Namespace", "Expires"),
	))
}

// TestCreateResources asserts that tctl create and tctl create -f
// operate as expected when a resource does and does not already exist.
func TestCreateResources(t *testing.T) {
	t.Parallel()

	process := testenv.MakeTestServer(t, testenv.WithLogger(utils.NewSlogLoggerForTests()))
	rootClient := testenv.MakeDefaultAuthClient(t, process)

	// tctlGetAllValidations allows tests to register post-test validations to validate
	// that their resource is present in "tctl get all" output.
	// This allows running test rows instead of the whole test table.
	var tctlGetAllValidations []func(t *testing.T, out string)

	tests := []struct {
		kind        string
		create      func(t *testing.T, clt *authclient.Client)
		getAllCheck func(t *testing.T, out string)
	}{
		{
			kind:   types.KindGithubConnector,
			create: testCreateGithubConnector,
			getAllCheck: func(t *testing.T, s string) {
				assert.Contains(t, s, "kind: github")
			},
		},
		{
			kind:   types.KindRole,
			create: testCreateRole,
			getAllCheck: func(t *testing.T, s string) {
				assert.Contains(t, s, "kind: role")
			},
		},
		{
			kind:   types.KindServerInfo,
			create: testCreateServerInfo,
		},
		{
			kind:   types.KindUser,
			create: testCreateUser,
			getAllCheck: func(t *testing.T, s string) {
				assert.Contains(t, s, "kind: user")
			},
		},
		{
			kind:   types.KindDatabaseObjectImportRule,
			create: testCreateDatabaseObjectImportRule,
		},
		{
			kind:   types.KindDatabaseObject,
			create: testCreateDatabaseObject,
		},
		{
			kind:   types.KindClusterNetworkingConfig,
			create: testCreateClusterNetworkingConfig,
			getAllCheck: func(t *testing.T, s string) {
				assert.Contains(t, s, "kind: cluster_networking_config")
			},
		},
		{
			kind:   types.KindClusterAuthPreference,
			create: testCreateAuthPreference,
			getAllCheck: func(t *testing.T, s string) {
				assert.Contains(t, s, "kind: cluster_auth_preference")
			},
		},
		{
			kind:   types.KindSessionRecordingConfig,
			create: testCreateSessionRecordingConfig,
		},
		{
			kind:   types.KindAppServer,
			create: testCreateAppServer,
		},
		{
			kind:   types.KindStaticHostUser,
			create: testCreateStaticHostUser,
		},
		{
			kind:   types.KindAutoUpdateConfig,
			create: testCreateAutoUpdateConfig,
		},
		{
			kind:   types.KindAutoUpdateVersion,
			create: testCreateAutoUpdateVersion,
		},
		{
			kind:   types.KindAutoUpdateAgentRollout,
			create: testCreateAutoUpdateAgentRollout,
		},
		{
			kind:   types.KindDynamicWindowsDesktop,
			create: testCreateDynamicWindowsDesktop,
		},
	}

	for _, test := range tests {
		t.Run(test.kind, func(t *testing.T) {
			test.create(t, rootClient)
			if test.getAllCheck != nil {
				tctlGetAllValidations = append(tctlGetAllValidations, test.getAllCheck)
			}
		})
	}

	// Verify that the created resources appear in tctl get all
	out, err := runResourceCommand(t, rootClient, []string{"get", "all"})
	require.NoError(t, err)
	s := out.String()
	for _, validateGetAll := range tctlGetAllValidations {
		validateGetAll(t, s)
	}
}

func testCreateGithubConnector(t *testing.T, clt *authclient.Client) {
	// Ensure there are no connectors to start
	buf, err := runResourceCommand(t, clt, []string{"get", types.KindGithubConnector, "--format=json"})
	require.NoError(t, err)
	connectors := mustDecodeJSON[[]*types.GithubConnectorV3](t, buf)
	require.Empty(t, connectors)

	const connectorYAML = `kind: github
metadata:
  name: github
spec:
  client_id: "12345"
  client_secret: "678910"
  display: Github
  redirect_url: https://proxy.example.com/v1/webapi/github/callback
  teams_to_roles:
  - organization: acme
    roles:
    - access
    - editor
    - auditor
    team: users
version: v3`

	// Create the connector
	connectorYAMLPath := filepath.Join(t.TempDir(), "connector.yaml")
	require.NoError(t, os.WriteFile(connectorYAMLPath, []byte(connectorYAML), 0644))
	_, err = runResourceCommand(t, clt, []string{"create", connectorYAMLPath})
	require.NoError(t, err)

	// Fetch the connector
	buf, err = runResourceCommand(t, clt, []string{"get", types.KindGithubConnector, "--format=json"})
	require.NoError(t, err)
	connectors = mustDecodeJSON[[]*types.GithubConnectorV3](t, buf)
	require.Len(t, connectors, 1)

	var expected types.GithubConnectorV3
	require.NoError(t, yaml.Unmarshal([]byte(connectorYAML), &expected))

	require.Empty(t, cmp.Diff(
		[]*types.GithubConnectorV3{&expected},
		connectors,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Namespace"),
		cmpopts.IgnoreFields(types.GithubConnectorSpecV3{}, "ClientSecret"), // get retrieves the connector without secrets
	))

	// Explicitly change the revision and try creating the user with and without
	// the force flag.
	expected.SetRevision(uuid.NewString())
	connectorBytes, err := services.MarshalGithubConnector(&expected, services.PreserveRevision())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(connectorYAMLPath, connectorBytes, 0644))

	_, err = runResourceCommand(t, clt, []string{"create", connectorYAMLPath})
	require.True(t, trace.IsAlreadyExists(err))

	_, err = runResourceCommand(t, clt, []string{"create", "-f", connectorYAMLPath})
	require.NoError(t, err)
}

func testCreateRole(t *testing.T, clt *authclient.Client) {
	// Ensure that our test role does not exist
	_, err := runResourceCommand(t, clt, []string{"get", types.KindRole + "/test-role", "--format=json"})
	require.True(t, trace.IsNotFound(err), "expected test-role to not exist prior to being created")

	const roleYAML = `kind: role
metadata:
  name: test-role
spec:
  allow:
    app_labels:
      '*': '*'
    db_labels:
      '*': '*'
    kubernetes_labels:
      '*': '*'
    kubernetes_resources:
    - kind: pod
      name: '*'
      namespace: '*'
    logins:
    - test
    node_labels:
      '*': '*'
  deny: {}
  options:
    cert_format: standard
    create_db_user: false
    create_desktop_user: false
    desktop_clipboard: true
    desktop_directory_sharing: true
    enhanced_recording:
    - command
    - network
    forward_agent: false
    idp:
      saml:
        enabled: true
    max_session_ttl: 30h0m0s
    pin_source_ip: false
    port_forwarding: true
    record_session:
      default: best_effort
      desktop: true
    ssh_file_copy: true
version: v7
`

	// Create the role
	roleYAMLPath := filepath.Join(t.TempDir(), "role.yaml")
	require.NoError(t, os.WriteFile(roleYAMLPath, []byte(roleYAML), 0644))
	_, err = runResourceCommand(t, clt, []string{"create", roleYAMLPath})
	require.NoError(t, err)

	// Fetch the role
	buf, err := runResourceCommand(t, clt, []string{"get", types.KindRole + "/test-role", "--format=json"})
	require.NoError(t, err)
	roles := mustDecodeJSON[[]*types.RoleV6](t, buf)
	require.Len(t, roles, 1)

	var expected types.RoleV6
	require.NoError(t, yaml.Unmarshal([]byte(roleYAML), &expected))

	require.Empty(t, cmp.Diff(
		[]*types.RoleV6{&expected},
		roles,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Explicitly change the revision and try creating the role with and without
	// the force flag.
	expected.SetRevision(uuid.NewString())
	connectorBytes, err := services.MarshalRole(&expected, services.PreserveRevision())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(roleYAMLPath, connectorBytes, 0644))

	_, err = runResourceCommand(t, clt, []string{"create", roleYAMLPath})
	require.True(t, trace.IsAlreadyExists(err))

	_, err = runResourceCommand(t, clt, []string{"create", "-f", roleYAMLPath})
	require.NoError(t, err)
}

func testCreateServerInfo(t *testing.T, clt *authclient.Client) {
	// Ensure that our test server info does not exist
	_, err := runResourceCommand(t, clt, []string{"get", types.KindServerInfo + "/test-server-info", "--format=json"})
	require.True(t, trace.IsNotFound(err), "expected test-role to not exist prior to being created")

	const serverInfoYAML = `---
kind: server_info
sub_kind: cloud_info
version: v1
metadata:
  name: test-server-info
spec:
  new_labels:
    'a': '1'
    'b': '2'
`

	// Create the server info
	serverInfoYAMLPath := filepath.Join(t.TempDir(), "server-info.yaml")
	err = os.WriteFile(serverInfoYAMLPath, []byte(serverInfoYAML), 0644)
	require.NoError(t, err)
	_, err = runResourceCommand(t, clt, []string{"create", serverInfoYAMLPath})
	require.NoError(t, err)

	// Fetch the server info
	buf, err := runResourceCommand(t, clt, []string{"get", types.KindServerInfo + "/test-server-info", "--format=json"})
	require.NoError(t, err)
	serverInfos := mustDecodeJSON[[]*types.ServerInfoV1](t, buf)
	require.Len(t, serverInfos, 1)

	var expected types.ServerInfoV1
	err = yaml.Unmarshal([]byte(serverInfoYAML), &expected)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(
		[]*types.ServerInfoV1{&expected},
		serverInfos,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Explicitly change the revision and try creating the resource with and without
	// the force flag.
	expected.SetRevision(uuid.NewString())
	newRevisionServerInfo, err := services.MarshalServerInfo(&expected, services.PreserveRevision())
	require.NoError(t, err)
	err = os.WriteFile(serverInfoYAMLPath, newRevisionServerInfo, 0644)
	require.NoError(t, err)

	_, err = runResourceCommand(t, clt, []string{"create", serverInfoYAMLPath})
	require.True(t, trace.IsAlreadyExists(err))

	_, err = runResourceCommand(t, clt, []string{"create", "-f", serverInfoYAMLPath})
	require.NoError(t, err)
}

func testCreateUser(t *testing.T, clt *authclient.Client) {
	// Ensure that our test user does not exist
	_, err := runResourceCommand(t, clt, []string{"get", types.KindUser + "/llama", "--format=json"})
	require.True(t, trace.IsNotFound(err), "expected llama user to not exist prior to being created")

	const userYAML = `kind: user
version: v2
metadata:
  name: llama
spec:
  roles: ["access"]
`

	// Create the user
	userYAMLPath := filepath.Join(t.TempDir(), "user.yaml")
	require.NoError(t, os.WriteFile(userYAMLPath, []byte(userYAML), 0644))
	_, err = runResourceCommand(t, clt, []string{"create", userYAMLPath})
	require.NoError(t, err)

	// Fetch the user
	buf, err := runResourceCommand(t, clt, []string{"get", types.KindUser + "/llama", "--format=json"})
	require.NoError(t, err)
	users := mustDecodeJSON[[]*types.UserV2](t, buf)
	require.Len(t, users, 1)

	var expected types.UserV2
	require.NoError(t, yaml.Unmarshal([]byte(userYAML), &expected))

	require.Empty(t, cmp.Diff(
		[]*types.UserV2{&expected},
		users,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		cmpopts.IgnoreFields(types.UserSpecV2{}, "CreatedBy"),
		cmpopts.IgnoreFields(types.UserV2{}, "Status"),
	))

	// Explicitly change the revision and try creating the user with and without
	// the force flag.
	expected.SetRevision(uuid.NewString())
	connectorBytes, err := services.MarshalUser(&expected, services.PreserveRevision())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(userYAMLPath, connectorBytes, 0644))

	_, err = runResourceCommand(t, clt, []string{"create", userYAMLPath})
	require.True(t, trace.IsAlreadyExists(err))

	_, err = runResourceCommand(t, clt, []string{"create", "-f", userYAMLPath})
	require.NoError(t, err)
}

func testCreateDatabaseObjectImportRule(t *testing.T, clt *authclient.Client) {
	const resourceYAML = `kind: db_object_import_rule
metadata:
  expires: "2034-03-22T18:06:35.161162Z"
  id: 1711129895244889000
  name: import_all_staging_tables
  namespace: default
spec:
  database_labels:
  - name: env
    values:
    - staging
    - prod
  - name: owner_org
    values:
    - trading
  mappings:
  - add_labels:
      custom_label: my_custom_value
      env: staging
    match:
      procedure_names:
      - aaa
      - bbb
      - ccc
      table_names:
      - '*'
      view_names:
      - "1"
      - "2"
      - "3"
    scope:
      database_names:
      - foo
      - bar
      - baz
      schema_names:
      - public
  priority: 30
version: v1
`

	// Verify there is no matching resource
	const resourceKey = "db_object_import_rule/import_all_staging_tables"
	_, err := runResourceCommand(t, clt, []string{"get", resourceKey, "--format=json"})
	require.Error(t, err)

	// Create the resource
	resourceYAMLPath := filepath.Join(t.TempDir(), "resource.yaml")
	require.NoError(t, os.WriteFile(resourceYAMLPath, []byte(resourceYAML), 0644))
	_, err = runResourceCommand(t, clt, []string{"create", resourceYAMLPath})
	require.NoError(t, err)

	// Fetch the resource
	buf, err := runResourceCommand(t, clt, []string{"get", resourceKey, "--format=json"})
	require.NoError(t, err)
	resources := mustDecodeJSON[[]databaseobjectimportrule.Resource](t, buf)
	require.Len(t, resources, 1)

	// Compare with baseline
	cmpOpts := []cmp.Option{
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		protocmp.Transform(),
	}

	var expected databaseobjectimportrule.Resource
	require.NoError(t, yaml.Unmarshal([]byte(resourceYAML), &expected))

	require.Equal(t, "", cmp.Diff(expected, resources[0], cmpOpts...))
	require.Equal(t, "", cmp.Diff(databaseobjectimportrule.ResourceToProto(&expected), databaseobjectimportrule.ResourceToProto(&resources[0]), cmpOpts...))
}

func testCreateClusterNetworkingConfig(t *testing.T, clt *authclient.Client) {
	// Get the initial cnc.
	buf, err := runResourceCommand(t, clt, []string{"get", types.KindClusterNetworkingConfig, "--format=json"})
	require.NoError(t, err)

	cnc := mustDecodeJSON[[]*types.ClusterNetworkingConfigV2](t, buf)
	require.Len(t, cnc, 1)
	initial := cnc[0]

	const cncYAML = `kind: cluster_networking_config
metadata:
  name: cluster-networking-config
spec:
  client_idle_timeout: 0s
  idle_timeout_message: ""
  keep_alive_count_max: 300
  case_insensitive_routing: true
  keep_alive_interval: 5m0s
  proxy_listener_mode: 1
  session_control_timeout: 0s
  tunnel_strategy:
    type: agent_mesh
  web_idle_timeout: 0s
version: v2
`

	// Create the cnc
	cncYAMLPath := filepath.Join(t.TempDir(), "cnc.yaml")
	require.NoError(t, os.WriteFile(cncYAMLPath, []byte(cncYAML), 0644))
	_, err = runResourceCommand(t, clt, []string{"create", cncYAMLPath})
	require.NoError(t, err)

	// Fetch the cnc
	buf, err = runResourceCommand(t, clt, []string{"get", types.KindClusterNetworkingConfig, "--format=json"})
	require.NoError(t, err)
	cnc = mustDecodeJSON[[]*types.ClusterNetworkingConfigV2](t, buf)
	require.Len(t, cnc, 1)

	var expected types.ClusterNetworkingConfigV2
	require.NoError(t, yaml.Unmarshal([]byte(cncYAML), &expected))

	require.NotEqual(t, int64(300), initial.GetKeepAliveCountMax())
	require.False(t, initial.GetCaseInsensitiveRouting())
	require.True(t, expected.GetCaseInsensitiveRouting())
	require.Equal(t, int64(300), expected.GetKeepAliveCountMax())

	// Explicitly change the revision and try creating the cnc with and without
	// the force flag.
	expected.SetRevision(uuid.NewString())
	raw, err := services.MarshalClusterNetworkingConfig(&expected, services.PreserveRevision())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cncYAMLPath, raw, 0644))

	_, err = runResourceCommand(t, clt, []string{"create", cncYAMLPath})
	require.True(t, trace.IsAlreadyExists(err))

	_, err = runResourceCommand(t, clt, []string{"create", "-f", cncYAMLPath})
	require.NoError(t, err)
}

func testCreateAuthPreference(t *testing.T, clt *authclient.Client) {
	// Get the initial CAP.
	buf, err := runResourceCommand(t, clt, []string{"get", types.KindClusterAuthPreference, "--format=json"})
	require.NoError(t, err)

	cap := mustDecodeJSON[[]*types.AuthPreferenceV2](t, buf)
	require.Len(t, cap, 1)
	initial := cap[0]

	const capYAML = `kind: cluster_auth_preference
metadata:
  name: cluster-auth-preference
spec:
  second_factors: [otp, sso]
  type: local
version: v2
`

	// Create the cap
	capYAMLPath := filepath.Join(t.TempDir(), "cap.yaml")
	require.NoError(t, os.WriteFile(capYAMLPath, []byte(capYAML), 0644))
	_, err = runResourceCommand(t, clt, []string{"create", capYAMLPath})
	require.NoError(t, err)

	// Fetch the cap
	buf, err = runResourceCommand(t, clt, []string{"get", types.KindClusterAuthPreference, "--format=json"})
	require.NoError(t, err)
	cap = mustDecodeJSON[[]*types.AuthPreferenceV2](t, buf)
	require.Len(t, cap, 1)

	expectInitialSecondFactors := []types.SecondFactorType{types.SecondFactorType_SECOND_FACTOR_TYPE_OTP} // second factors defaults to [otp]
	require.Equal(t, expectInitialSecondFactors, initial.GetSecondFactors())

	var revised types.AuthPreferenceV2
	require.NoError(t, yaml.Unmarshal([]byte(capYAML), &revised))
	expectRevisedSecondFactors := []types.SecondFactorType{types.SecondFactorType_SECOND_FACTOR_TYPE_OTP, types.SecondFactorType_SECOND_FACTOR_TYPE_SSO}
	require.Equal(t, expectRevisedSecondFactors, revised.GetSecondFactors())

	// Explicitly change the revision and try creating the cap with and without
	// the force flag.
	revised.SetRevision(uuid.NewString())
	raw, err := services.MarshalAuthPreference(&revised, services.PreserveRevision())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(capYAMLPath, raw, 0644))

	_, err = runResourceCommand(t, clt, []string{"create", capYAMLPath})
	require.True(t, trace.IsAlreadyExists(err))

	_, err = runResourceCommand(t, clt, []string{"create", "-f", capYAMLPath})
	require.NoError(t, err)
}

func testCreateSessionRecordingConfig(t *testing.T, clt *authclient.Client) {
	// Get the initial recording config.
	buf, err := runResourceCommand(t, clt, []string{"get", types.KindSessionRecordingConfig, "--format=json"})
	require.NoError(t, err)

	src := mustDecodeJSON[[]*types.SessionRecordingConfigV2](t, buf)
	require.Len(t, src, 1)
	initial := src[0]

	const srcYAML = `kind: session_recording_config
metadata:
  labels:
    teleport.dev/origin: defaults
  name: session-recording-config
spec:
  mode: proxy
version: v2
`

	// Create the src
	srcYAMLPath := filepath.Join(t.TempDir(), "src.yaml")
	require.NoError(t, os.WriteFile(srcYAMLPath, []byte(srcYAML), 0644))
	_, err = runResourceCommand(t, clt, []string{"create", srcYAMLPath})
	require.NoError(t, err)

	// Fetch the cap
	buf, err = runResourceCommand(t, clt, []string{"get", types.KindSessionRecordingConfig, "--format=json"})
	require.NoError(t, err)
	src = mustDecodeJSON[[]*types.SessionRecordingConfigV2](t, buf)
	require.Len(t, src, 1)

	var expected types.SessionRecordingConfigV2
	require.NoError(t, yaml.Unmarshal([]byte(srcYAML), &expected))

	require.Equal(t, types.RecordOff, initial.GetMode())
	require.Equal(t, types.RecordAtProxy, expected.GetMode())

	// Explicitly change the revision and try creating the src with and without
	// the force flag.
	expected.SetRevision(uuid.NewString())
	raw, err := services.MarshalSessionRecordingConfig(&expected, services.PreserveRevision())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(srcYAMLPath, raw, 0644))

	_, err = runResourceCommand(t, clt, []string{"create", srcYAMLPath})
	require.True(t, trace.IsAlreadyExists(err))

	_, err = runResourceCommand(t, clt, []string{"create", "-f", srcYAMLPath})
	require.NoError(t, err)
}

func testCreateAppServer(t *testing.T, clt *authclient.Client) {
	const appServerWithIntegrationYAML = `---
kind: app_server
metadata:
  name: my-integration
  labels:
    account_id: "123456789012"
spec:
  app:
    kind: app
    metadata:
      name: my-integration
      labels:
        account_id: "123456789012"
    spec:
      uri: https://console.aws.amazon.com
      integration: my-integration
      public_addr: integration.example.com
    version: v3
  host_id: c6cfe5c2-653f-4e5d-a914-bfac5a7baf38
version: v3
`

	const appServerWithoutIntegrationYAML = `---
kind: app_server
metadata:
  name: my-integration
spec:
  app:
    kind: app
    metadata:
      name: my-integration
    spec:
      uri: https://console.aws.amazon.com
      public_addr: integration.example.com
    version: v3
  host_id: c6cfe5c2-653f-4e5d-a914-bfac5a7baf38
version: v3
`

	// Creating an AppServer with integration is valid.
	srcYAMLPath := filepath.Join(t.TempDir(), "appServerWithIntegrationYAML.yaml")
	require.NoError(t, os.WriteFile(srcYAMLPath, []byte(appServerWithIntegrationYAML), 0644))
	_, err := runResourceCommand(t, clt, []string{"create", srcYAMLPath})
	require.NoError(t, err)

	// Creating an AppServer without integration is invalid.
	srcYAMLPath = filepath.Join(t.TempDir(), "appServerWithoutIntegrationYAML.yaml")
	require.NoError(t, os.WriteFile(srcYAMLPath, []byte(appServerWithoutIntegrationYAML), 0644))
	_, err = runResourceCommand(t, clt, []string{"create", srcYAMLPath})
	require.ErrorContains(t, err, "integration")

	buf, err := runResourceCommand(t, clt, []string{"get", types.KindAppServer, "--format=json"})
	require.NoError(t, err)
	appServers := mustDecodeJSON[[]*types.AppServerV3](t, buf)
	require.Len(t, appServers, 1)

	expectedAppServer, err := types.NewAppServerForAWSOIDCIntegration("my-integration", "c6cfe5c2-653f-4e5d-a914-bfac5a7baf38", "integration.example.com", map[string]string{"account_id": "123456789012"})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(
		expectedAppServer,
		appServers[0],
		cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Namespace"),
	))
}

func testCreateDatabaseObject(t *testing.T, clt *authclient.Client) {
	const resourceYAML = `kind: db_object
metadata:
  expires: "2034-03-22T18:06:35.161162Z"
  id: 1711129895244889000
  labels:
    database: foo
    kind: table
    name: page_views
    protocol: postgres
    schema: web_metrics
    service_name: pg-docker
  name: test_table
  revision: 066f87d9-02cf-4062-9419-96523664c082
spec:
  database: foo
  database_service_name: pg-docker
  name: page_views
  object_kind: table
  protocol: postgres
  schema: web_metrics
version: v1
`

	// Verify there are no pre-existing objects
	buf, err := runResourceCommand(t, clt, []string{"get", types.KindDatabaseObject, "--format=json"})
	require.NoError(t, err)

	resources := mustDecodeJSON[[]databaseobject.Resource](t, buf)
	require.Empty(t, resources)

	// Create the resource
	resourceYAMLPath := filepath.Join(t.TempDir(), "resource.yaml")
	require.NoError(t, os.WriteFile(resourceYAMLPath, []byte(resourceYAML), 0644))
	_, err = runResourceCommand(t, clt, []string{"create", resourceYAMLPath})
	require.NoError(t, err)

	// Fetch the resource
	buf, err = runResourceCommand(t, clt, []string{"get", types.KindDatabaseObject, "--format=json"})
	require.NoError(t, err)
	resources = mustDecodeJSON[[]databaseobject.Resource](t, buf)
	require.Len(t, resources, 1)

	// Compare with baseline
	cmpOpts := []cmp.Option{
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		protocmp.Transform(),
	}

	var expected databaseobject.Resource
	require.NoError(t, yaml.Unmarshal([]byte(resourceYAML), &expected))

	require.Equal(t, "", cmp.Diff(expected, resources[0], cmpOpts...))
	require.Equal(t, "", cmp.Diff(databaseobject.ResourceToProto(&expected), databaseobject.ResourceToProto(&resources[0]), cmpOpts...))
}

// TestCreateEnterpriseResources asserts that tctl create
// behaves as expected for enterprise resources. These resources cannot
// be tested in parallel because they alter the modules to enable features.
// The tests are grouped to amortize the cost of creating and auth server since
// that is the most expensive part of testing editing the resource.
func TestCreateEnterpriseResources(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.OIDC: {Enabled: true},
				entitlements.SAML: {Enabled: true},
			},
		},
	})

	process := testenv.MakeTestServer(t)
	clt := testenv.MakeDefaultAuthClient(t, process)

	tests := []struct {
		kind   string
		create func(t *testing.T, clt *authclient.Client)
	}{
		{
			kind:   types.KindOIDCConnector,
			create: testCreateOIDCConnector,
		},
		{
			kind:   types.KindSAMLConnector,
			create: testCreateSAMLConnector,
		},
	}

	for _, test := range tests {
		t.Run(test.kind, func(t *testing.T) {
			test.create(t, clt)
		})
	}

	// Verify that the created resources appear in tctl get all
	out, err := runResourceCommand(t, clt, []string{"get", "all"})
	require.NoError(t, err)
	s := out.String()
	require.NotEmpty(t, s)
	assert.Contains(t, s, "kind: saml")
	assert.Contains(t, s, "kind: oidc")
}

func testCreateOIDCConnector(t *testing.T, clt *authclient.Client) {
	// Ensure there are no connectors to start
	buf, err := runResourceCommand(t, clt, []string{"get", types.KindOIDCConnector, "--format=json"})
	require.NoError(t, err)
	connectors := mustDecodeJSON[[]*types.OIDCConnectorV3](t, buf)
	require.Empty(t, connectors)

	const connectorYAML = `kind: oidc
version: v3
metadata:
  name: oidc
spec:
  redirect_url: "https://proxy.example.com/v1/webapi/oidc/callback"
  client_id: "12345"
  client_secret: "678910"
  display: OIDC
  scope: [roles]
  claims_to_roles:
    - {claim: "test", value: "test", roles: ["access", "editor", "auditor"]}`

	// Create the connector
	connectorYAMLPath := filepath.Join(t.TempDir(), "connector.yaml")
	require.NoError(t, os.WriteFile(connectorYAMLPath, []byte(connectorYAML), 0644))
	_, err = runResourceCommand(t, clt, []string{"create", connectorYAMLPath})
	require.NoError(t, err)

	// Fetch the connector
	buf, err = runResourceCommand(t, clt, []string{"get", types.KindOIDCConnector, "--format=json"})
	require.NoError(t, err)
	connectors = mustDecodeJSON[[]*types.OIDCConnectorV3](t, buf)
	require.Len(t, connectors, 1)

	var expected types.OIDCConnectorV3
	require.NoError(t, yaml.Unmarshal([]byte(connectorYAML), &expected))

	require.Empty(t, cmp.Diff(
		[]*types.OIDCConnectorV3{&expected},
		connectors,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Namespace"),
		cmpopts.IgnoreFields(types.OIDCConnectorSpecV3{}, "ClientSecret"), // get retrieves the connector without secrets
	))

	// Explicitly change the revision and try creating the user with and without
	// the force flag.
	expected.SetRevision(uuid.NewString())
	connectorBytes, err := services.MarshalOIDCConnector(&expected, services.PreserveRevision())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(connectorYAMLPath, connectorBytes, 0644))

	_, err = runResourceCommand(t, clt, []string{"create", connectorYAMLPath})
	require.True(t, trace.IsAlreadyExists(err))

	_, err = runResourceCommand(t, clt, []string{"create", "-f", connectorYAMLPath})
	require.NoError(t, err)
}

func testCreateSAMLConnector(t *testing.T, clt *authclient.Client) {
	// Ensure there are no connectors to start
	buf, err := runResourceCommand(t, clt, []string{"get", types.KindSAMLConnector, "--format=json"})
	require.NoError(t, err)
	connectors := mustDecodeJSON[[]*types.SAMLConnectorV2](t, buf)
	require.Empty(t, connectors)

	const connectorYAML = `kind: saml
version: v2
metadata:
  name: saml
spec:
  acs: test
  audience: test
  issuer: test
  sso: test
  service_provider_issuer: test
  display: SAML
  attributes_to_roles:
  - name: test
    roles:
    - access
    value: test
  entity_descriptor: |
    <?xml version="1.0" encoding="UTF-8"?>
    <md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="test">
      <md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
        <md:KeyDescriptor use="signing">
          <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
            <ds:X509Data>
              <ds:X509Certificate></ds:X509Certificate>
            </ds:X509Data>
          </ds:KeyInfo>
        </md:KeyDescriptor>
        <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
        <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
        <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="test" />
        <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="test" />
      </md:IDPSSODescriptor>
    </md:EntityDescriptor>` + "\n"

	// Create the connector
	connectorYAMLPath := filepath.Join(t.TempDir(), "connector.yaml")
	require.NoError(t, os.WriteFile(connectorYAMLPath, []byte(connectorYAML), 0644))
	_, err = runResourceCommand(t, clt, []string{"create", connectorYAMLPath})
	require.NoError(t, err)

	// Fetch the connector
	buf, err = runResourceCommand(t, clt, []string{"get", types.KindSAMLConnector, "--format=json"})
	require.NoError(t, err)
	connectors = mustDecodeJSON[[]*types.SAMLConnectorV2](t, buf)
	require.Len(t, connectors, 1)

	var expected types.SAMLConnectorV2
	require.NoError(t, yaml.Unmarshal([]byte(connectorYAML), &expected))

	require.Empty(t, cmp.Diff(
		[]*types.SAMLConnectorV2{&expected},
		connectors,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Namespace"),
		cmpopts.IgnoreFields(types.SAMLConnectorSpecV2{}, "SigningKeyPair"), // get retrieves the connector without secrets
	))

	// Explicitly change the revision and try creating the user with and without
	// the force flag.
	expected.SetRevision(uuid.NewString())
	connectorBytes, err := services.MarshalSAMLConnector(&expected, services.PreserveRevision())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(connectorYAMLPath, connectorBytes, 0644))

	_, err = runResourceCommand(t, clt, []string{"create", connectorYAMLPath})
	require.True(t, trace.IsAlreadyExists(err))

	_, err = runResourceCommand(t, clt, []string{"create", "-f", connectorYAMLPath})
	require.NoError(t, err)
}

func testCreateStaticHostUser(t *testing.T, clt *authclient.Client) {
	// Ensure that our test user does not exist
	resourceName := "alice"
	resourceKey := types.KindStaticHostUser + "/" + resourceName
	_, err := runResourceCommand(t, clt, []string{"get", resourceKey, "--format=json"})
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err), "unexpected error: %v", err)

	const userYAML = `kind: static_host_user
version: v2
metadata:
  name: alice
spec:
  matchers:
    - node_labels:
      - name: foo
        values: ["bar"]
      groups:
        - foo
        - bar
      uid: 1234
      gid: 5678
    - node_labels_expression: 'labels["foo"] == labels["bar"]'
      groups:
        - baz
        - quux
      sudoers: ["abc1234"]
`

	// Create the host user
	userYAMLPath := filepath.Join(t.TempDir(), "host_user.yaml")
	require.NoError(t, os.WriteFile(userYAMLPath, []byte(userYAML), 0644))
	_, err = runResourceCommand(t, clt, []string{"create", userYAMLPath})
	require.NoError(t, err)

	// Fetch the user
	buf, err := runResourceCommand(t, clt, []string{"get", resourceKey, "--format=json"})
	require.NoError(t, err)
	hostUsers := mustDecodeJSON[[]*userprovisioningpb.StaticHostUser](t, buf)
	require.Len(t, hostUsers, 1)

	var expected userprovisioningpb.StaticHostUser
	require.NoError(t, yaml.Unmarshal([]byte(userYAML), &expected))

	require.Empty(t, cmp.Diff(
		[]*userprovisioningpb.StaticHostUser{&expected},
		hostUsers,
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		protocmp.Transform(),
	))

	// Explicitly change the revision and try creating the user with and without
	// the force flag.
	expected.GetMetadata().Revision = uuid.NewString()
	hostUserBytes, err := services.MarshalProtoResource(&expected, services.PreserveRevision())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(userYAMLPath, hostUserBytes, 0644))

	_, err = runResourceCommand(t, clt, []string{"create", userYAMLPath})
	require.Error(t, err)
	require.True(t, trace.IsAlreadyExists(err), "unexpected error: %v", err)

	_, err = runResourceCommand(t, clt, []string{"create", "-f", userYAMLPath})
	require.NoError(t, err)
}

func testCreateAutoUpdateConfig(t *testing.T, clt *authclient.Client) {
	const resourceYAML = `kind: autoupdate_config
metadata:
  name: autoupdate-config
  revision: 3a43b44a-201e-4d7f-aef1-ae2f6d9811ed
spec:
  tools:
    mode: enabled
version: v1
`
	_, err := runResourceCommand(t, clt, []string{"get", types.KindAutoUpdateConfig, "--format=json"})
	require.ErrorContains(t, err, "doesn't exist")

	// Create the resource.
	resourceYAMLPath := filepath.Join(t.TempDir(), "resource.yaml")
	require.NoError(t, os.WriteFile(resourceYAMLPath, []byte(resourceYAML), 0644))
	_, err = runResourceCommand(t, clt, []string{"create", resourceYAMLPath})
	require.NoError(t, err)

	buf, err := runResourceCommand(t, clt, []string{"get", types.KindAutoUpdateConfig, "--format=json"})
	require.NoError(t, err)

	rawResources := mustDecodeJSON[[]services.UnknownResource](t, buf)
	require.Len(t, rawResources, 1)
	var resource autoupdate.AutoUpdateConfig
	require.NoError(t, protojson.Unmarshal(rawResources[0].Raw, &resource))

	var expected autoupdate.AutoUpdateConfig
	expectedJSON := mustTranscodeYAMLToJSON(t, bytes.NewReader([]byte(resourceYAML)))
	require.NoError(t, protojson.Unmarshal(expectedJSON, &expected))

	require.Empty(t, cmp.Diff(
		&expected,
		&resource,
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		protocmp.Transform(),
	))

	// Delete the resource
	_, err = runResourceCommand(t, clt, []string{"rm", types.KindAutoUpdateConfig})
	require.NoError(t, err)
	_, err = runResourceCommand(t, clt, []string{"get", types.KindAutoUpdateConfig})
	require.ErrorContains(t, err, "autoupdate_config \"autoupdate-config\" doesn't exist")
}

func testCreateAutoUpdateVersion(t *testing.T, clt *authclient.Client) {
	const resourceYAML = `kind: autoupdate_version
metadata:
  name: autoupdate-version
  revision: 3a43b44a-201e-4d7f-aef1-ae2f6d9811ed
spec:
  tools:
    target_version: 1.2.3
version: v1
`
	_, err := runResourceCommand(t, clt, []string{"get", types.KindAutoUpdateVersion, "--format=json"})
	require.ErrorContains(t, err, "doesn't exist")

	// Create the resource.
	resourceYAMLPath := filepath.Join(t.TempDir(), "resource.yaml")
	require.NoError(t, os.WriteFile(resourceYAMLPath, []byte(resourceYAML), 0644))
	_, err = runResourceCommand(t, clt, []string{"create", resourceYAMLPath})
	require.NoError(t, err)

	buf, err := runResourceCommand(t, clt, []string{"get", types.KindAutoUpdateVersion, "--format=json"})
	require.NoError(t, err)

	rawResources := mustDecodeJSON[[]services.UnknownResource](t, buf)
	require.Len(t, rawResources, 1)
	var resource autoupdate.AutoUpdateVersion
	require.NoError(t, protojson.Unmarshal(rawResources[0].Raw, &resource))

	var expected autoupdate.AutoUpdateVersion
	expectedJSON := mustTranscodeYAMLToJSON(t, bytes.NewReader([]byte(resourceYAML)))
	require.NoError(t, protojson.Unmarshal(expectedJSON, &expected))

	require.Empty(t, cmp.Diff(
		&expected,
		&resource,
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		protocmp.Transform(),
	))

	// Delete the resource
	_, err = runResourceCommand(t, clt, []string{"rm", types.KindAutoUpdateVersion})
	require.NoError(t, err)
	_, err = runResourceCommand(t, clt, []string{"get", types.KindAutoUpdateVersion})
	require.ErrorContains(t, err, "autoupdate_version \"autoupdate-version\" doesn't exist")
}

func testCreateAutoUpdateAgentRollout(t *testing.T, clt *authclient.Client) {
	const resourceYAML = `kind: autoupdate_agent_rollout
metadata:
  name: autoupdate-agent-rollout
  revision: 3a43b44a-201e-4d7f-aef1-ae2f6d9811ed
spec:
  start_version: 1.2.3
  target_version: 1.2.3
  autoupdate_mode: "suspended"
  schedule: "regular"
  strategy: "halt-on-error"
status:
  groups:
    - name: my-group
      state: 1
      config_days: ["*"]
      config_start_hour: 12
      config_wait_hours: 0
version: v1
`
	_, err := runResourceCommand(t, clt, []string{"get", types.KindAutoUpdateAgentRollout, "--format=json"})
	require.ErrorContains(t, err, "doesn't exist")

	// Create the resource.
	resourceYAMLPath := filepath.Join(t.TempDir(), "resource.yaml")
	require.NoError(t, os.WriteFile(resourceYAMLPath, []byte(resourceYAML), 0644))
	_, err = runResourceCommand(t, clt, []string{"create", resourceYAMLPath})
	require.NoError(t, err)

	// Get the resource
	buf, err := runResourceCommand(t, clt, []string{"get", types.KindAutoUpdateAgentRollout, "--format=json"})
	require.NoError(t, err)

	rawResources := mustDecodeJSON[[]services.UnknownResource](t, buf)
	require.Len(t, rawResources, 1)
	var resource autoupdate.AutoUpdateAgentRollout
	require.NoError(t, protojson.Unmarshal(rawResources[0].Raw, &resource))

	var expected autoupdate.AutoUpdateAgentRollout
	expectedJSON := mustTranscodeYAMLToJSON(t, bytes.NewReader([]byte(resourceYAML)))
	require.NoError(t, protojson.Unmarshal(expectedJSON, &expected))

	require.Empty(t, cmp.Diff(
		&expected,
		&resource,
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		protocmp.Transform(),
	))

	// Delete the resource
	_, err = runResourceCommand(t, clt, []string{"rm", types.KindAutoUpdateAgentRollout})
	require.NoError(t, err)
	_, err = runResourceCommand(t, clt, []string{"get", types.KindAutoUpdateAgentRollout})
	require.ErrorContains(t, err, "autoupdate_agent_rollout \"autoupdate-agent-rollout\" doesn't exist")
}

func testCreateDynamicWindowsDesktop(t *testing.T, clt *authclient.Client) {
	const resourceYAML = `kind: dynamic_windows_desktop
metadata:
  name: test
  revision: 3a43b44a-201e-4d7f-aef1-ae2f6d9811ed
spec:
  addr: test
version: v1
`

	// Create the resource.
	resourceYAMLPath := filepath.Join(t.TempDir(), "resource.yaml")
	require.NoError(t, os.WriteFile(resourceYAMLPath, []byte(resourceYAML), 0644))
	_, err := runResourceCommand(t, clt, []string{"create", resourceYAMLPath})
	require.NoError(t, err)

	// Get the resource
	buf, err := runResourceCommand(t, clt, []string{"get", types.KindDynamicWindowsDesktop, "--format=json"})
	require.NoError(t, err)
	resources := mustDecodeJSON[[]types.DynamicWindowsDesktopV1](t, buf)
	require.Len(t, resources, 1)

	var expected types.DynamicWindowsDesktopV1
	require.NoError(t, yaml.Unmarshal([]byte(resourceYAML), &expected))
	expected.SetRevision(resources[0].GetRevision())

	require.Empty(t, cmp.Diff([]types.DynamicWindowsDesktopV1{expected}, resources, protocmp.Transform()))
}

func TestPluginResourceWrapper(t *testing.T) {
	tests := []struct {
		name   string
		plugin types.PluginV1
	}{
		{
			name: "okta",
			plugin: types.PluginV1{
				Metadata: types.Metadata{
					Name: "okta",
				},
				Spec: types.PluginSpecV1{
					Settings: &types.PluginSpecV1_Okta{
						Okta: &types.PluginOktaSettings{
							OrgUrl: "https://oktaorg.okta.com",
							SyncSettings: &types.PluginOktaSyncSettings{
								SyncUsers:       true,
								SsoConnectorId:  "connectorID",
								SyncAccessLists: true,
							},
						},
					},
				},
				Credentials: &types.PluginCredentialsV1{
					Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
						StaticCredentialsRef: &types.PluginStaticCredentialsRef{Labels: map[string]string{"label": "value"}},
					},
				},
			},
		},
		{
			name: "slack",
			plugin: types.PluginV1{
				Metadata: types.Metadata{
					Name: "okta",
				},
				Spec: types.PluginSpecV1{
					Settings: &types.PluginSpecV1_SlackAccessPlugin{
						SlackAccessPlugin: &types.PluginSlackAccessSettings{
							FallbackChannel: "#channel",
						},
					},
				},
				Credentials: &types.PluginCredentialsV1{
					Credentials: &types.PluginCredentialsV1_Oauth2AccessToken{
						Oauth2AccessToken: &types.PluginOAuth2AccessTokenCredentials{
							AccessToken:  "token",
							RefreshToken: "refresh_token",
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buff, err := json.Marshal(&tc.plugin)
			require.NoError(t, err)
			var failItem types.PluginV1
			err = json.Unmarshal(buff, &failItem)
			require.Error(t, err)
			var item pluginResourceWrapper
			err = json.Unmarshal(buff, &item)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(tc.plugin, item.PluginV1))
		})
	}
}
