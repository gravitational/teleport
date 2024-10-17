/*
Copyright 2021 Gravitational, Inc.

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

package common

import (
	"bytes"
	"context"
	"crypto/x509"
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
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tbot/testhelpers"
	"github.com/gravitational/teleport/lib/utils"
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

	_ = makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))

	// get all database servers
	buff, err := runResourceCommand(t, fileConfig, []string{"get", types.KindDatabaseServer, "--format=json"})
	require.NoError(t, err)
	requireGotDatabaseServers(t, buff, db1, db2, db3)

	// get specific database server
	wantServer := fmt.Sprintf("%v/%v", types.KindDatabaseServer, db2.GetName())
	buff, err = runResourceCommand(t, fileConfig, []string{"get", wantServer, "--format=json"})
	require.NoError(t, err)
	requireGotDatabaseServers(t, buff, db2)

	// get database servers by discovered name
	wantServersDiscoveredName := fmt.Sprintf("%v/%v", types.KindDatabaseServer, "example")
	buff, err = runResourceCommand(t, fileConfig, []string{"get", wantServersDiscoveredName, "--format=json"})
	require.NoError(t, err)
	requireGotDatabaseServers(t, buff, db1, db2)

	// remove multiple distinct database servers by discovered name is an error
	_, err = runResourceCommand(t, fileConfig, []string{"rm", wantServersDiscoveredName})
	require.ErrorContains(t, err, "db_server/example matches multiple auto-discovered database servers")

	// remove database server by name
	_, err = runResourceCommand(t, fileConfig, []string{"rm", wantServer})
	require.NoError(t, err)

	_, err = runResourceCommand(t, fileConfig, []string{"get", wantServer, "--format=json"})
	require.Error(t, err)
	require.IsType(t, &trace.NotFoundError{}, err.(*trace.TraceErr).OrigError())

	// remove database server by discovered name.
	_, err = runResourceCommand(t, fileConfig, []string{"rm", wantServersDiscoveredName})
	require.NoError(t, err)

	buff, err = runResourceCommand(t, fileConfig, []string{"get", "db_server/db3", "--format=json"})
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
		buff, err := runResourceCommand(t, fileConfig, []string{"get", types.KindDatabaseService, "--format=json"})
		require.NoError(t, err)
		out := mustDecodeJSON[[]*types.DatabaseServiceV1](t, buff)
		require.Len(t, out, totalDBServices)
	})

	service := fmt.Sprintf("%v/%v", types.KindDatabaseService, randomDBServiceName)

	t.Run("get specific database service", func(t *testing.T) {
		buff, err := runResourceCommand(t, fileConfig, []string{"get", service, "--format=json"})
		require.NoError(t, err)
		out := mustDecodeJSON[[]*types.DatabaseServiceV1](t, buff)
		require.Len(t, out, 1)
		require.Equal(t, randomDBServiceName, out[0].GetName())
	})

	t.Run("get unknown database service", func(t *testing.T) {
		unknownService := fmt.Sprintf("%v/%v", types.KindDatabaseService, "unknown")
		_, err := runResourceCommand(t, fileConfig, []string{"get", unknownService, "--format=json"})
		require.True(t, trace.IsNotFound(err), "expected a NotFound error, got %v", err)
	})

	t.Run("get specific database service with human output", func(t *testing.T) {
		buff, err := runResourceCommand(t, fileConfig, []string{"get", service, "--format=text"})
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
			buff, err := runResourceCommand(t, fileConfig, []string{"get", types.KindIntegration, "--format=json"})
			require.NoError(t, err)
			out := mustDecodeJSON[[]types.IntegrationV1](t, buff)
			require.Len(t, out, totalIntegrations)
		})

		igName := fmt.Sprintf("%v/%v", types.KindIntegration, randomIntegrationName)

		t.Run("get specific integration", func(t *testing.T) {
			buff, err := runResourceCommand(t, fileConfig, []string{"get", igName, "--format=json"})
			require.NoError(t, err)
			out := mustDecodeJSON[[]types.IntegrationV1](t, buff)
			require.Len(t, out, 1)
			require.Equal(t, randomIntegrationName, out[0].GetName())
		})

		t.Run("get unknown integration", func(t *testing.T) {
			unknownIntegration := fmt.Sprintf("%v/%v", types.KindIntegration, "unknown")
			_, err := runResourceCommand(t, fileConfig, []string{"get", unknownIntegration, "--format=json"})
			require.True(t, trace.IsNotFound(err), "expected a NotFound error, got %v", err)
		})

		t.Run("get specific integration with human output", func(t *testing.T) {
			buff, err := runResourceCommand(t, fileConfig, []string{"get", igName, "--format=text"})
			require.NoError(t, err)
			outputString := buff.String()
			require.Contains(t, outputString, "RoleARN=arn:aws:iam::123456789012:role/OpsTeam")
			require.Contains(t, outputString, randomIntegrationName)
		})
	})

	t.Run("create", func(t *testing.T) {
		integrationYAMLPath := filepath.Join(t.TempDir(), "integration.yaml")
		require.NoError(t, os.WriteFile(integrationYAMLPath, []byte(integrationYAML), 0644))
		_, err := runResourceCommand(t, fileConfig, []string{"create", integrationYAMLPath})
		require.NoError(t, err)

		buff, err := runResourceCommand(t, fileConfig, []string{"get", "integration/myawsint", "--format=text"})
		require.NoError(t, err)
		outputString := buff.String()
		require.Contains(t, outputString, "RoleARN=arn:aws:iam::123456789012:role/OpsTeam")
		require.Contains(t, outputString, "myawsint")

		// Update the RoleARN to another role
		integrationYAMLV2 := strings.ReplaceAll(integrationYAML, "OpsTeam", "DevTeam")
		require.NoError(t, os.WriteFile(integrationYAMLPath, []byte(integrationYAMLV2), 0644))

		// Trying to create it again should return an error
		_, err = runResourceCommand(t, fileConfig, []string{"create", integrationYAMLPath})
		require.True(t, trace.IsAlreadyExists(err), "expected already exists error, got %v", err)

		// Using the force should be ok and replace the current object
		_, err = runResourceCommand(t, fileConfig, []string{"create", "--force", integrationYAMLPath})
		require.NoError(t, err)

		// The RoleARN must be updated
		buff, err = runResourceCommand(t, fileConfig, []string{"get", "integration/myawsint", "--format=text"})
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
			buff, err := runResourceCommand(t, fileConfig, []string{"get", types.KindDiscoveryConfig, "--format=json"})
			require.NoError(t, err)
			out := mustDecodeJSON[[]discoveryconfig.DiscoveryConfig](t, buff)
			require.Len(t, out, totalDiscoveryConfigs)
		})

		dcName := fmt.Sprintf("%v/%v", types.KindDiscoveryConfig, randomDiscoveryConfigName)

		t.Run("get specific discovery config", func(t *testing.T) {
			buff, err := runResourceCommand(t, fileConfig, []string{"get", dcName, "--format=json"})
			require.NoError(t, err)
			out := mustDecodeJSON[[]discoveryconfig.DiscoveryConfig](t, buff)
			require.Len(t, out, 1)
			require.Equal(t, randomDiscoveryConfigName, out[0].GetName())
		})

		t.Run("get unknown discovery config", func(t *testing.T) {
			unknownDiscoveryConfig := fmt.Sprintf("%v/%v", types.KindDiscoveryConfig, "unknown")
			_, err := runResourceCommand(t, fileConfig, []string{"get", unknownDiscoveryConfig, "--format=json"})
			require.True(t, trace.IsNotFound(err), "expected a NotFound error, got %v", err)
		})

		t.Run("get specific discovery config with human output", func(t *testing.T) {
			buff, err := runResourceCommand(t, fileConfig, []string{"get", dcName, "--format=text"})
			require.NoError(t, err)
			outputString := buff.String()
			require.Contains(t, outputString, "prod-resources")
			require.Contains(t, outputString, randomDiscoveryConfigName)
		})
	})

	t.Run("create", func(t *testing.T) {
		discoveryConfigYAMLPath := filepath.Join(t.TempDir(), "discoveryConfig.yaml")
		require.NoError(t, os.WriteFile(discoveryConfigYAMLPath, []byte(discoveryConfigYAML), 0644))
		_, err := runResourceCommand(t, fileConfig, []string{"create", discoveryConfigYAMLPath})
		require.NoError(t, err)

		buff, err := runResourceCommand(t, fileConfig, []string{"get", "discovery_config/my-discovery-config", "--format=text"})
		require.NoError(t, err)
		outputString := buff.String()
		require.Contains(t, outputString, "my-discovery-config")
		require.Contains(t, outputString, "mydg1")

		// Update the discovery group to another group
		discoveryConfigYAMLV2 := strings.ReplaceAll(discoveryConfigYAML, "mydg1", "mydg2")
		require.NoError(t, os.WriteFile(discoveryConfigYAMLPath, []byte(discoveryConfigYAMLV2), 0644))

		// Trying to create it again should return an error
		_, err = runResourceCommand(t, fileConfig, []string{"create", discoveryConfigYAMLPath})
		require.True(t, trace.IsAlreadyExists(err), "expected already exists error, got %v", err)

		// Using the force should be ok and replace the current object
		_, err = runResourceCommand(t, fileConfig, []string{"create", "--force", discoveryConfigYAMLPath})
		require.NoError(t, err)

		// The DiscoveryGroup must be updated
		buff, err = runResourceCommand(t, fileConfig, []string{"get", "discovery_config/my-discovery-config", "--format=text"})
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
	makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors), withFakeClock(fakeClock))

	_, err := types.NewLock("test-lock", types.LockSpecV2{
		Target: types.LockTarget{
			User: "bad@actor",
		},
		Message: "I am a message",
	})
	require.NoError(t, err)

	// Ensure there are no locks to start
	buf, err := runResourceCommand(t, fileConfig, []string{"get", types.KindLock, "--format=json"})
	require.NoError(t, err)
	locks := mustDecodeJSON[[]*types.LockV2](t, buf)
	require.Empty(t, locks)

	// Create the locks
	lockYAMLPath := filepath.Join(t.TempDir(), "lock.yaml")
	require.NoError(t, os.WriteFile(lockYAMLPath, []byte(lockYAML), 0644))
	_, err = runResourceCommand(t, fileConfig, []string{"create", lockYAMLPath})
	require.NoError(t, err)

	// Fetch the locks
	buf, err = runResourceCommand(t, fileConfig, []string{"get", types.KindLock, "--format=json"})
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

	makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))

	// Create the databases yaml file.
	dbYAMLPath := filepath.Join(t.TempDir(), "db.yaml")
	require.NoError(t, os.WriteFile(dbYAMLPath, []byte(dbYAML), 0644))

	// Reset RootCertPool and run tctl command with --insecure flag.
	opts := []optionsFunc{
		withRootCertPool(x509.NewCertPool()),
		withInsecure(true),
	}
	_, err := runResourceCommand(t, fileConfig, []string{"create", dbYAMLPath}, opts...)
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
  uri: "localhost1"
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

	makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))

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

			_, err := runResourceCommand(t, fileConfig, []string{"create", "-f", capYAMLPath})
			tt.expectError(t, err)

			if tt.expectSecondFactor != nil {
				buf, err := runResourceCommand(t, fileConfig, []string{"get", "cap", "--format=json"})
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

	makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))

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

			_, err := runResourceCommand(t, fileConfig, []string{"create", "-f", spYAMLPath})
			tt.expectError(t, err)

			if tt.expectEntityID != nil {
				buf, err := runResourceCommand(t, fileConfig, []string{"get", fmt.Sprintf("saml_sp/%s", tt.name), "--format=json"})
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

func (test *dynamicResourceTest[T]) setup(t *testing.T) *config.FileConfig {
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
	_ = makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))
	return fileConfig
}

func (test *dynamicResourceTest[T]) run(t *testing.T) {
	t.Helper()
	fileConfig := test.setup(t)

	// Initially there are no resources.
	buf, err := runResourceCommand(t, fileConfig, []string{"get", test.kind, "--format=json"})
	require.NoError(t, err)
	resources := mustDecodeJSON[[]T](t, buf)
	require.Len(t, resources, 0)

	// Create the resources.
	yamlPath := filepath.Join(t.TempDir(), "resources.yaml")
	require.NoError(t, os.WriteFile(yamlPath, []byte(test.resourceYAML), 0644))
	_, err = runResourceCommand(t, fileConfig, []string{"create", yamlPath})
	require.NoError(t, err)

	// Fetch all resources.
	buf, err = runResourceCommand(t, fileConfig, []string{"get", test.kind, "--format=json"})
	require.NoError(t, err)
	resources = mustDecodeJSON[[]T](t, buf)
	require.Len(t, resources, 3)
	require.Empty(t, cmp.Diff([]T{test.fooResource, test.fooBar1Resource, test.fooBar2Resource}, resources,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision", "Namespace"),
	))

	// Fetch specific resource.
	buf, err = runResourceCommand(t, fileConfig,
		[]string{"get", fmt.Sprintf("%v/%v", test.kind, test.fooResource.GetName()), "--format=json"})
	require.NoError(t, err)
	resources = mustDecodeJSON[[]T](t, buf)
	require.Len(t, resources, 1)
	require.Empty(t, cmp.Diff([]T{test.fooResource}, resources,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision", "Namespace"),
	))

	// Remove a resource.
	_, err = runResourceCommand(t, fileConfig, []string{"rm", fmt.Sprintf("%v/%v", test.kind, test.fooResource.GetName())})
	require.NoError(t, err)

	// Fetch all resources again.
	buf, err = runResourceCommand(t, fileConfig, []string{"get", test.kind, "--format=json"})
	require.NoError(t, err)
	resources = mustDecodeJSON[[]T](t, buf)
	require.Len(t, resources, 2)
	require.Empty(t, cmp.Diff([]T{test.fooBar1Resource, test.fooBar2Resource}, resources,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision", "Namespace"),
	))

	if !test.runDiscoveredNameChecks {
		return
	}

	// Test discovered name behavior.
	// Fetching multiple resources ("foo-bar-1" and "foo-bar-2") by discovered name is ok.
	buf, err = runResourceCommand(t, fileConfig, []string{"get", fmt.Sprintf("%v/%v", test.kind, "foo-bar"), "--format=json"})
	require.NoError(t, err)
	resources = mustDecodeJSON[[]T](t, buf)
	require.Len(t, resources, 2)
	require.Empty(t, cmp.Diff([]T{test.fooBar1Resource, test.fooBar2Resource}, resources,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision", "Namespace"),
	))

	// Removing multiple resources ("foo-bar-1" and "foo-bar-2") by discovered name is an error.
	_, err = runResourceCommand(t, fileConfig, []string{"rm", fmt.Sprintf("%v/%v", test.kind, "foo-bar")})
	require.ErrorContains(t, err, "matches multiple")

	// Remove "foo-bar-2" resource by full name.
	_, err = runResourceCommand(t, fileConfig, []string{"rm", fmt.Sprintf("%v/%v", test.kind, test.fooBar2Resource.GetName())})
	require.NoError(t, err)

	// Fetch all resources again.
	buf, err = runResourceCommand(t, fileConfig, []string{"get", test.kind, "--format=json"})
	require.NoError(t, err)
	resources = mustDecodeJSON[[]T](t, buf)
	require.Len(t, resources, 1)
	require.Empty(t, cmp.Diff([]T{test.fooBar1Resource}, resources,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision", "Namespace"),
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
		URI: "localhost1",
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
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Namespace", "Expires"),
	))
}

// TestCreateResources asserts that tctl create and tctl create -f
// operate as expected when a resource does and does not already exist.
func TestCreateResources(t *testing.T) {
	t.Parallel()

	fc, fds := testhelpers.DefaultConfig(t)
	_ = testhelpers.MakeAndRunTestAuthServer(t, utils.NewLoggerForTests(), fc, fds)

	tests := []struct {
		kind   string
		create func(t *testing.T, fc *config.FileConfig)
	}{
		{
			kind:   types.KindServerInfo,
			create: testCreateServerInfo,
		},
		{
			kind:   types.KindAutoUpdateConfig,
			create: testCreateAutoUpdateConfig,
		},
		{
			kind:   types.KindAutoUpdateVersion,
			create: testCreateAutoUpdateVersion,
		},
	}

	for _, test := range tests {
		t.Run(test.kind, func(t *testing.T) {
			test.create(t, fc)
		})
	}
}

func testCreateServerInfo(t *testing.T, fc *config.FileConfig) {
	// Ensure that our test server info does not exist
	_, err := runResourceCommand(t, fc, []string{"get", types.KindServerInfo + "/test-server-info", "--format=json"})
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
	_, err = runResourceCommand(t, fc, []string{"create", serverInfoYAMLPath})
	require.NoError(t, err)

	// Fetch the server info
	buf, err := runResourceCommand(t, fc, []string{"get", types.KindServerInfo + "/test-server-info", "--format=json"})
	require.NoError(t, err)
	serverInfos := mustDecodeJSON[[]*types.ServerInfoV1](t, buf)
	require.Len(t, serverInfos, 1)

	var expected types.ServerInfoV1
	err = yaml.Unmarshal([]byte(serverInfoYAML), &expected)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(
		[]*types.ServerInfoV1{&expected},
		serverInfos,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Explicitly change the revision and try creating the resource with and without
	// the force flag.
	expected.SetRevision(uuid.NewString())
	newRevisionServerInfo, err := services.MarshalServerInfo(&expected, services.PreserveResourceID())
	require.NoError(t, err)
	err = os.WriteFile(serverInfoYAMLPath, newRevisionServerInfo, 0644)
	require.NoError(t, err)

	_, err = runResourceCommand(t, fc, []string{"create", serverInfoYAMLPath})
	require.True(t, trace.IsAlreadyExists(err))

	_, err = runResourceCommand(t, fc, []string{"create", "-f", serverInfoYAMLPath})
	require.NoError(t, err)
}

func testCreateAutoUpdateConfig(t *testing.T, fc *config.FileConfig) {
	const resourceYAML = `kind: autoupdate_config
metadata:
  name: autoupdate-config
  revision: 3a43b44a-201e-4d7f-aef1-ae2f6d9811ed
spec:
  tools:
    mode: enabled
version: v1
`
	_, err := runResourceCommand(t, fc, []string{"get", types.KindAutoUpdateConfig, "--format=json"})
	require.ErrorContains(t, err, "doesn't exist")

	// Create the resource.
	resourceYAMLPath := filepath.Join(t.TempDir(), "resource.yaml")
	require.NoError(t, os.WriteFile(resourceYAMLPath, []byte(resourceYAML), 0644))
	_, err = runResourceCommand(t, fc, []string{"create", resourceYAMLPath})
	require.NoError(t, err)

	// Get the resource
	buf, err := runResourceCommand(t, fc, []string{"get", types.KindAutoUpdateConfig, "--format=json"})
	require.NoError(t, err)
	resources := mustDecodeJSON[[]*autoupdate.AutoUpdateConfig](t, buf)
	require.Len(t, resources, 1)

	var expected autoupdate.AutoUpdateConfig
	require.NoError(t, yaml.Unmarshal([]byte(resourceYAML), &expected))

	require.Empty(t, cmp.Diff(
		[]*autoupdate.AutoUpdateConfig{&expected},
		resources,
		protocmp.IgnoreFields(&headerv1.Metadata{}, "id", "revision"),
		protocmp.Transform(),
	))
}

func testCreateAutoUpdateVersion(t *testing.T, fc *config.FileConfig) {
	const resourceYAML = `kind: autoupdate_version
metadata:
  name: autoupdate-version
  revision: 3a43b44a-201e-4d7f-aef1-ae2f6d9811ed
spec:
  tools:
    target_version: 1.2.3
version: v1
`
	_, err := runResourceCommand(t, fc, []string{"get", types.KindAutoUpdateVersion, "--format=json"})
	require.ErrorContains(t, err, "doesn't exist")

	// Create the resource.
	resourceYAMLPath := filepath.Join(t.TempDir(), "resource.yaml")
	require.NoError(t, os.WriteFile(resourceYAMLPath, []byte(resourceYAML), 0644))
	_, err = runResourceCommand(t, fc, []string{"create", resourceYAMLPath})
	require.NoError(t, err)

	// Get the resource
	buf, err := runResourceCommand(t, fc, []string{"get", types.KindAutoUpdateVersion, "--format=json"})
	require.NoError(t, err)
	resources := mustDecodeJSON[[]*autoupdate.AutoUpdateVersion](t, buf)
	require.Len(t, resources, 1)

	var expected autoupdate.AutoUpdateVersion
	require.NoError(t, yaml.Unmarshal([]byte(resourceYAML), &expected))

	require.Empty(t, cmp.Diff(
		[]*autoupdate.AutoUpdateVersion{&expected},
		resources,
		protocmp.IgnoreFields(&headerv1.Metadata{}, "id", "revision"),
		protocmp.Transform(),
	))
}
