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

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
)

// TestDatabaseServerResource tests tctl db_server rm/get commands.
func TestDatabaseServerResource(t *testing.T) {
	dynAddr := newDynamicServiceAddr(t)
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
					Name:        "example",
					Description: "Example MySQL",
					Protocol:    "mysql",
					URI:         "localhost:33306",
				},
				{
					Name:        "example2",
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
				},
				{
					Name:        "db3",
					Description: "Example MySQL",
					Protocol:    "mysql",
					URI:         "localhost:33308",
				},
			},
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag: "true",
			},
			WebAddr: dynAddr.webAddr,
			TunAddr: dynAddr.tunnelAddr,
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.authAddr,
			},
		},
	}

	db1, err := types.NewDatabaseV3(types.Metadata{
		Name:        "example",
		Description: "Example MySQL",
		Labels:      map[string]string{types.OriginLabel: types.OriginConfigFile},
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
		Name:        "example2",
		Description: "Example PostgreSQL",
		Labels:      map[string]string{types.OriginLabel: types.OriginConfigFile},
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

	_ = makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.descriptors))

	var out []*types.DatabaseServerV3

	// get all database servers
	buff, err := runResourceCommand(t, fileConfig, []string{"get", types.KindDatabaseServer, "--format=json"})
	require.NoError(t, err)
	mustDecodeJSON(t, buff, &out)
	require.Len(t, out, 3)

	// get specific database server
	wantServer := fmt.Sprintf("%v/%v", types.KindDatabaseServer, db2.GetName())
	buff, err = runResourceCommand(t, fileConfig, []string{"get", wantServer, "--format=json"})
	require.NoError(t, err)
	mustDecodeJSON(t, buff, &out)
	require.Len(t, out, 1)
	gotDB := out[0].GetDatabase()
	require.Empty(t, cmp.Diff([]types.Database{db2}, []types.Database{gotDB},
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Namespace", "Expires"),
	))

	// get database servers by prefix of name
	wantServersPrefix := fmt.Sprintf("%v/%v", types.KindDatabaseServer, "exam")
	buff, err = runResourceCommand(t, fileConfig, []string{"get", wantServersPrefix, "--format=json"})
	require.NoError(t, err)
	mustDecodeJSON(t, buff, &out)
	require.Len(t, out, 2)
	gotDBs := types.DatabaseServers{out[0], out[1]}.ToDatabases()
	require.Empty(t, cmp.Diff([]types.Database{db1, db2}, gotDBs,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Namespace", "Expires"),
	))

	// remove database servers by prefix is an error
	_, err = runResourceCommand(t, fileConfig, []string{"rm", wantServersPrefix})
	require.ErrorContains(t, err, "db_server/exam matches multiple database servers")

	// remove database server by name
	_, err = runResourceCommand(t, fileConfig, []string{"rm", wantServer})
	require.NoError(t, err)

	_, err = runResourceCommand(t, fileConfig, []string{"get", wantServer, "--format=json"})
	require.Error(t, err)
	require.IsType(t, &trace.NotFoundError{}, err.(*trace.TraceErr).OrigError())

	// remove database server by prefix name.
	_, err = runResourceCommand(t, fileConfig, []string{"rm", wantServersPrefix})
	require.NoError(t, err)

	buff, err = runResourceCommand(t, fileConfig, []string{"get", "db_server", "--format=json"})
	require.NoError(t, err)
	mustDecodeJSON(t, buff, &out)
	require.Len(t, out, 1)
	gotDBs = types.DatabaseServers{out[0]}.ToDatabases()
	require.Empty(t, cmp.Diff([]types.Database{db3}, gotDBs,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Namespace", "Expires"),
	))
}

// TestDatabaseServiceResource tests tctl db_services get commands.
func TestDatabaseServiceResource(t *testing.T) {
	dynAddr := newDynamicServiceAddr(t)

	ctx := context.Background()
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag: "true",
			},
			WebAddr: dynAddr.webAddr,
			TunAddr: dynAddr.tunnelAddr,
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.authAddr,
			},
		},
	}

	auth := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.descriptors))

	var out []*types.DatabaseServiceV1

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
		mustDecodeJSON(t, buff, &out)
		require.Len(t, out, totalDBServices)
	})

	service := fmt.Sprintf("%v/%v", types.KindDatabaseService, randomDBServiceName)

	t.Run("get specific database service", func(t *testing.T) {
		buff, err := runResourceCommand(t, fileConfig, []string{"get", service, "--format=json"})
		require.NoError(t, err)
		mustDecodeJSON(t, buff, &out)
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
	dynAddr := newDynamicServiceAddr(t)

	ctx := context.Background()
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag: "true",
			},
			WebAddr: dynAddr.webAddr,
			TunAddr: dynAddr.tunnelAddr,
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.authAddr,
			},
		},
	}

	auth := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.descriptors))

	t.Run("get", func(t *testing.T) {

		var out []types.IntegrationV1

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
			mustDecodeJSON(t, buff, &out)
			require.Len(t, out, totalIntegrations)
		})

		igName := fmt.Sprintf("%v/%v", types.KindIntegration, randomIntegrationName)

		t.Run("get specific integration", func(t *testing.T) {
			buff, err := runResourceCommand(t, fileConfig, []string{"get", igName, "--format=json"})
			require.NoError(t, err)
			mustDecodeJSON(t, buff, &out)
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

func TestCreateLock(t *testing.T) {
	dynAddr := newDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag: "true",
			},
			WebAddr: dynAddr.webAddr,
			TunAddr: dynAddr.tunnelAddr,
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.authAddr,
			},
		},
	}

	timeNow := time.Now().UTC()
	fakeClock := clockwork.NewFakeClockAt(timeNow)
	makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.descriptors), withFakeClock(fakeClock))

	_, err := types.NewLock("test-lock", types.LockSpecV2{
		Target: types.LockTarget{
			User: "bad@actor",
		},
		Message: "I am a message",
	})
	require.NoError(t, err)

	var locks []*types.LockV2

	// Ensure there are no locks to start
	buf, err := runResourceCommand(t, fileConfig, []string{"get", types.KindLock, "--format=json"})
	require.NoError(t, err)
	mustDecodeJSON(t, buf, &locks)
	require.Empty(t, locks)

	// Create the locks
	lockYAMLPath := filepath.Join(t.TempDir(), "lock.yaml")
	require.NoError(t, os.WriteFile(lockYAMLPath, []byte(lockYAML), 0644))
	_, err = runResourceCommand(t, fileConfig, []string{"create", lockYAMLPath})
	require.NoError(t, err)

	// Fetch the locks
	buf, err = runResourceCommand(t, fileConfig, []string{"get", types.KindLock, "--format=json"})
	require.NoError(t, err)
	mustDecodeJSON(t, buf, &locks)
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
	dynAddr := newDynamicServiceAddr(t)

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
			WebAddr: dynAddr.webAddr,
			TunAddr: dynAddr.tunnelAddr,
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.authAddr,
			},
		},
	}

	makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.descriptors))

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
  name: foo-bar
spec:
  protocol: "postgres"
  uri: "localhost:5433"
  tls:
    mode: "verify-full"
---
kind: db
version: v3
metadata:
  name: foo-bar-baz
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
  name: foo-bar
spec:
  uri: "localhost2"
---
kind: app
version: v3
metadata:
  name: foo-bar-baz
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
  name: foo-bar
spec: {}
---
kind: kube_cluster
version: v3
metadata:
  name: foo-bar-baz
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
)

func TestCreateClusterAuthPreference_WithSupportForSecondFactorWithoutQuotes(t *testing.T) {
	dynAddr := newDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.authAddr,
			},
		},
	}

	makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.descriptors))

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
				var authPreferences []types.AuthPreferenceV2
				mustDecodeJSON(t, buf, &authPreferences)
				require.NotZero(t, len(authPreferences))
				tt.expectSecondFactor(t, authPreferences[0].Spec.SecondFactor)
			}
		})
	}
}

func TestCreateSAMLIdPServiceProvider(t *testing.T) {
	dynAddr := newDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.authAddr,
			},
		},
	}

	makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.descriptors))

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
				sps := []*types.SAMLIdPServiceProviderV1{}
				mustDecodeJSON(t, buf, &sps)
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
	kind                string
	resourceYAML        string
	fooResource         T
	fooBarResource      T
	fooBarBazResource   T
	runPrefixNameChecks bool
}

func (test *dynamicResourceTest[T]) setup(t *testing.T) *config.FileConfig {
	t.Helper()
	requireResource := func(t *testing.T, r T, name string) {
		t.Helper()
		require.NotNil(t, r, "dynamicResourceTest requires a resource named %q", name)
		require.Equal(t, r.GetName(), name, "dynamicResourceTest requires a resource named %q", name)
	}
	requireResource(t, test.fooResource, "foo")
	requireResource(t, test.fooBarResource, "foo-bar")
	requireResource(t, test.fooBarBazResource, "foo-bar-baz")
	dynAddr := newDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag: "true",
			},
			WebAddr: dynAddr.webAddr,
			TunAddr: dynAddr.tunnelAddr,
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.authAddr,
			},
		},
	}
	_ = makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.descriptors))
	return fileConfig
}

func (test *dynamicResourceTest[T]) run(t *testing.T) {
	t.Helper()
	fileConfig := test.setup(t)
	var out []T

	// Initially there are no resources.
	buf, err := runResourceCommand(t, fileConfig, []string{"get", test.kind, "--format=json"})
	require.NoError(t, err)
	mustDecodeJSON(t, buf, &out)
	require.Len(t, out, 0)

	// Create the resources.
	yamlPath := filepath.Join(t.TempDir(), "resources.yaml")
	require.NoError(t, os.WriteFile(yamlPath, []byte(test.resourceYAML), 0644))
	_, err = runResourceCommand(t, fileConfig, []string{"create", yamlPath})
	require.NoError(t, err)

	// Fetch all resources.
	buf, err = runResourceCommand(t, fileConfig, []string{"get", test.kind, "--format=json"})
	require.NoError(t, err)
	mustDecodeJSON(t, buf, &out)
	require.Len(t, out, 3)
	require.Empty(t, cmp.Diff([]T{test.fooResource, test.fooBarResource, test.fooBarBazResource}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Namespace"),
	))

	// Fetch specific resource.
	buf, err = runResourceCommand(t, fileConfig,
		[]string{"get", fmt.Sprintf("%v/%v", test.kind, test.fooResource.GetName()), "--format=json"})
	require.NoError(t, err)
	mustDecodeJSON(t, buf, &out)
	require.Len(t, out, 1)
	require.Empty(t, cmp.Diff([]T{test.fooResource}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Namespace"),
	))

	// Remove a resource.
	_, err = runResourceCommand(t, fileConfig, []string{"rm", fmt.Sprintf("%v/%v", test.kind, test.fooBarResource.GetName())})
	require.NoError(t, err)

	// Fetch all resources again.
	buf, err = runResourceCommand(t, fileConfig, []string{"get", test.kind, "--format=json"})
	require.NoError(t, err)
	mustDecodeJSON(t, buf, &out)
	require.Len(t, out, 2)
	require.Empty(t, cmp.Diff([]T{test.fooResource, test.fooBarBazResource}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Namespace"),
	))

	if !test.runPrefixNameChecks {
		return
	}

	// Test prefix name behavior.
	// Removing multiple resources ("foo" and "foo-bar-baz")by prefix name is an error.
	_, err = runResourceCommand(t, fileConfig, []string{"rm", fmt.Sprintf("%v/%v", test.kind, "f")})
	require.ErrorContains(t, err, "matches multiple")

	// Remove "foo-bar-baz" resource by a prefix of its name.
	_, err = runResourceCommand(t, fileConfig, []string{"rm", fmt.Sprintf("%v/%v", test.kind, "foo-bar-b")})
	require.NoError(t, err)

	// Fetch all resources again.
	buf, err = runResourceCommand(t, fileConfig, []string{"get", test.kind, "--format=json"})
	require.NoError(t, err)
	mustDecodeJSON(t, buf, &out)
	require.Len(t, out, 1)
	require.Empty(t, cmp.Diff([]T{test.fooResource}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Namespace"),
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
		Name:   "foo-bar",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5433",
		TLS: types.DatabaseTLS{
			Mode: types.DatabaseTLSMode_VERIFY_FULL,
		},
	})
	require.NoError(t, err)
	dbFooBarBaz, err := types.NewDatabaseV3(types.Metadata{
		Name:   "foo-bar-baz",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)

	require.NoError(t, err)
	test := dynamicResourceTest[*types.DatabaseV3]{
		kind:                types.KindDatabase,
		resourceYAML:        dbYAML,
		fooResource:         dbFoo,
		fooBarResource:      dbFooBar,
		fooBarBazResource:   dbFooBarBaz,
		runPrefixNameChecks: true,
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
	kubeFooBar, err := types.NewKubernetesClusterV3(types.Metadata{
		Name:   "foo-bar",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)
	kubeFooBarBaz, err := types.NewKubernetesClusterV3(types.Metadata{
		Name:   "foo-bar-baz",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)
	test := dynamicResourceTest[*types.KubernetesClusterV3]{
		kind:                types.KindKubernetesCluster,
		resourceYAML:        kubeYAML,
		fooResource:         kubeFoo,
		fooBarResource:      kubeFooBar,
		fooBarBazResource:   kubeFooBarBaz,
		runPrefixNameChecks: true,
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
	appFooBar, err := types.NewAppV3(types.Metadata{
		Name:   "foo-bar",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.AppSpecV3{
		URI: "localhost2",
	})
	require.NoError(t, err)
	appFooBarBaz, err := types.NewAppV3(types.Metadata{
		Name:   "foo-bar-baz",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.AppSpecV3{
		URI: "localhost3",
	})
	require.NoError(t, err)
	test := dynamicResourceTest[*types.AppV3]{
		kind:              types.KindApp,
		resourceYAML:      appYAML,
		fooResource:       appFoo,
		fooBarResource:    appFooBar,
		fooBarBazResource: appFooBarBaz,
	}
	test.run(t)
}

func TestGetOneResourceNameToDelete(t *testing.T) {
	foo1 := mustCreateNewKubeServer(t, "foo", "host-foo", nil)
	foo2 := mustCreateNewKubeServer(t, "foo", "host-foo", nil)
	fooBar := mustCreateNewKubeServer(t, "foo-bar", "host-foo-bar", nil)
	baz := mustCreateNewKubeServer(t, "baz", "host-baz", nil)
	tests := []struct {
		desc            string
		refName         string
		wantErrContains string
		resources       []types.KubeServer
		wantName        string
	}{
		{
			desc:      "one resource is ok",
			refName:   "baz",
			resources: []types.KubeServer{baz},
			wantName:  "baz",
		},
		{
			desc:      "multiple resources with same name is ok",
			refName:   "foo",
			resources: []types.KubeServer{foo1, foo2},
			wantName:  "foo",
		},
		{
			desc:            "zero resources is an error",
			refName:         "xxx",
			wantErrContains: `kubernetes server "xxx" not found`,
		},
		{
			desc:            "multiple resources with different names is an error",
			refName:         "f",
			resources:       []types.KubeServer{foo1, foo2, fooBar},
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

func TestFilterByNameOrPrefix(t *testing.T) {
	foo1 := mustCreateNewKubeServer(t, "foo", "host-foo", nil)
	foo2 := mustCreateNewKubeServer(t, "foo", "host-foo", nil)
	fooBar := mustCreateNewKubeServer(t, "foo-bar", "host-foo-bar", nil)
	baz := mustCreateNewKubeServer(t, "baz", "host-baz", nil)
	resources := []types.KubeServer{
		foo1, foo2, fooBar, baz,
	}
	hostNameGetter := func(ks types.KubeServer) string { return ks.GetHostname() }
	tests := []struct {
		desc           string
		filter         string
		altNameGetters []altNameFn[types.KubeServer]
		want           []types.KubeServer
	}{
		{
			desc:   "filters by exact name first",
			filter: "foo",
			want:   []types.KubeServer{foo1, foo2},
		},
		{
			desc:   "filters by prefix name",
			filter: "fo",
			want:   []types.KubeServer{foo1, foo2, fooBar},
		},
		{
			desc:           "checks alt names for exact matches first",
			filter:         "host-foo",
			altNameGetters: []altNameFn[types.KubeServer]{hostNameGetter},
			want:           []types.KubeServer{foo1, foo2},
		},
		{
			desc:           "checks alt names for prefix matches",
			filter:         "host-f",
			altNameGetters: []altNameFn[types.KubeServer]{hostNameGetter},
			want:           []types.KubeServer{foo1, foo2, fooBar},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			got := filterByNameOrPrefix(resources, test.filter, test.altNameGetters...)
			require.Empty(t, cmp.Diff(test.want, got))
		})
	}
}

func TestFormatAmbiguousDeleteMessage(t *testing.T) {
	ref := services.Ref{Kind: types.KindDatabase, Name: "x"}
	resDesc := "database"
	names := []string{"xbbb", "xaaa", "xccc", "xb"}
	got := formatAmbiguousDeleteMessage(ref, resDesc, names)
	require.Contains(t, got, "db/x matches multiple databases", "should have formated the ref used and pluralized the resource description")
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
