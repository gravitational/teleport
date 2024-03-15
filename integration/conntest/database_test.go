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

package conntest

import (
	"context"
	"encoding/base32"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/conntest"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/web/ui"
)

func startPostgresTestServer(t *testing.T, authServer *auth.Server) *postgres.TestServer {
	postgresTestServer, err := postgres.NewTestServer(common.TestServerConfig{
		AuthClient: authServer,
	})
	require.NoError(t, err)

	go func() {
		t.Logf("Postgres Fake server running at %s port", postgresTestServer.Port())
		assert.NoError(t, postgresTestServer.Serve())
	}()
	t.Cleanup(func() {
		postgresTestServer.Close()
	})

	return postgresTestServer
}

func TestDiagnoseConnectionForPostgresDatabases(t *testing.T) {
	ctx := context.Background()
	diagnoseConnectionEndpoint := strings.Join([]string{"sites", "$site", "diagnostics", "connections"}, "/")

	// Start Teleport Auth and Proxy services
	authProcess, proxyProcess, provisionToken := helpers.MakeTestServers(t)
	authServer := authProcess.GetAuthServer()
	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	// Start Fake Postgres Database
	postgresTestServer := startPostgresTestServer(t, authServer)

	// Start Teleport Database Service
	databaseResourceName := "mypsqldb"
	databaseDBName := "dbname"
	databaseDBUser := "dbuser"
	helpers.MakeTestDatabaseServer(t, *proxyAddr, provisionToken, nil /* resource matchers */, servicecfg.Database{
		Name:     databaseResourceName,
		Protocol: defaults.ProtocolPostgres,
		URI:      net.JoinHostPort("localhost", postgresTestServer.Port()),
	})
	// Wait for the Database Server to be registered
	waitForDatabases(t, authServer, []string{databaseResourceName})

	roleWithFullAccess, err := types.NewRole("fullaccess", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Namespaces:     []string{apidefaults.Namespace},
			DatabaseLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			Rules: []types.Rule{
				types.NewRule(types.KindConnectionDiagnostic, services.RW()),
			},
			DatabaseUsers: []string{databaseDBUser},
			DatabaseNames: []string{databaseDBName},
		},
	})
	require.NoError(t, err)
	roleWithFullAccess, err = authServer.UpsertRole(ctx, roleWithFullAccess)
	require.NoError(t, err)

	for _, tt := range []struct {
		name         string
		teleportUser string

		reqResourceName string
		reqDBUser       string
		reqDBName       string

		expectedSuccess bool
		expectedMessage string
		expectedTraces  []types.ConnectionDiagnosticTrace
	}{
		{
			name:         "success",
			teleportUser: "success",

			reqResourceName: databaseResourceName,
			reqDBUser:       databaseDBUser,
			reqDBName:       databaseDBName,

			expectedSuccess: true,
			expectedMessage: "success",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_RBAC_DATABASE,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "A Database Agent is available to proxy the connection to the Database.",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_CONNECTIVITY,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "Database is accessible from the Database Agent.",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_RBAC_DATABASE_LOGIN,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "Access to Database User and Database Name granted.",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_DATABASE_DB_USER,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "Database User exists in the Database.",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_DATABASE_DB_NAME,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "Database Name exists in the Database.",
				},
			},
		},

		{
			name:         "database not found",
			teleportUser: "dbnotfound",

			reqResourceName: "dbnotfound",
			reqDBUser:       databaseDBUser,
			reqDBName:       databaseDBName,

			expectedSuccess: false,
			expectedMessage: "failed",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:   types.ConnectionDiagnosticTrace_RBAC_DATABASE,
					Status: types.ConnectionDiagnosticTrace_FAILED,
					Details: "Database not found. " +
						"Ensure your role grants access by adding it to the 'db_labels' property. " +
						"This can also happen when you don't have a Database Agent proxying the database - " +
						"you can fix that by adding the database labels to the 'db_service.resources.labels' in 'teleport.yaml' file of the database agent.",
				},
			},
		},
		{
			name:         "no access to db user/name",
			teleportUser: "deniedlogin",

			reqResourceName: databaseResourceName,
			reqDBUser:       "root",
			reqDBName:       "system",

			expectedSuccess: false,
			expectedMessage: "failed",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_RBAC_DATABASE_LOGIN,
					Status:  types.ConnectionDiagnosticTrace_FAILED,
					Details: "Access denied when accessing Database. Please check the Error message for more information.",
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt

			// Set up User
			user, err := types.NewUser(tt.teleportUser)
			require.NoError(t, err)

			user.AddRole(roleWithFullAccess.GetName())
			_, err = authServer.UpsertUser(ctx, user)
			require.NoError(t, err)

			userPassword := uuid.NewString()
			require.NoError(t, authServer.UpsertPassword(tt.teleportUser, []byte(userPassword)))

			webPack := helpers.LoginWebClient(t, proxyAddr.String(), tt.teleportUser, userPassword)

			diagnoseReq := conntest.TestConnectionRequest{
				ResourceKind: types.KindDatabase,
				ResourceName: tt.reqResourceName,
				DatabaseUser: tt.reqDBUser,
				DatabaseName: tt.reqDBName,
				// Default is 30 seconds but since tests run locally, we can reduce this value to also improve test responsiveness
				DialTimeout:        time.Second,
				InsecureSkipVerify: true,
			}
			respStatusCode, respBody := webPack.DoRequest(t, http.MethodPost, diagnoseConnectionEndpoint, diagnoseReq)
			require.Equal(t, http.StatusOK, respStatusCode, string(respBody))

			var connectionDiagnostic ui.ConnectionDiagnostic
			require.NoError(t, json.Unmarshal(respBody, &connectionDiagnostic))

			gotFailedTraces := 0
			expectedFailedTraces := 0

			for i, trace := range connectionDiagnostic.Traces {
				if trace.Status == types.ConnectionDiagnosticTrace_FAILED.String() {
					gotFailedTraces++
				}

				t.Logf("%d status='%s' type='%s' details='%s' error='%s'\n", i, trace.Status, trace.TraceType, trace.Details, trace.Error)
			}

			require.Equal(t, tt.expectedSuccess, connectionDiagnostic.Success)
			require.Equal(t, tt.expectedMessage, connectionDiagnostic.Message)
			for _, expectedTrace := range tt.expectedTraces {
				if expectedTrace.Status == types.ConnectionDiagnosticTrace_FAILED {
					expectedFailedTraces++
				}

				foundTrace := false
				for _, returnedTrace := range connectionDiagnostic.Traces {
					if expectedTrace.Type.String() != returnedTrace.TraceType {
						continue
					}

					foundTrace = true
					require.Equal(t, expectedTrace.Status.String(), returnedTrace.Status)
					require.Equal(t, expectedTrace.Details, returnedTrace.Details)
					require.Contains(t, returnedTrace.Error, expectedTrace.Error)
				}

				require.True(t, foundTrace, expectedTrace)
			}

			require.Equal(t, expectedFailedTraces, gotFailedTraces)
		})
	}

	// Test success with per-session MFA.

	// Set up user.
	user, err := types.NewUser("llama")
	require.NoError(t, err)
	user.AddRole(roleWithFullAccess.GetName())
	_, err = authServer.UpsertUser(ctx, user)
	require.NoError(t, err)
	userPassword := uuid.NewString()
	require.NoError(t, authServer.UpsertPassword("llama", []byte(userPassword)))
	webPack := helpers.LoginWebClient(t, proxyAddr.String(), "llama", userPassword)

	// Require per-session mfa.
	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:           constants.Local,
		SecondFactor:   constants.SecondFactorOTP,
		RequireMFAType: types.RequireMFAType_SESSION,
	})
	require.NoError(t, err)
	_, err = authServer.UpsertAuthPreference(ctx, ap)
	require.NoError(t, err)

	// Set up otp device.
	otpSecret := base32.StdEncoding.EncodeToString([]byte("abc123"))
	dev, err := services.NewTOTPDevice("otp", otpSecret, authServer.GetClock().Now())
	require.NoError(t, err)
	err = authServer.UpsertMFADevice(ctx, "llama", dev)
	require.NoError(t, err)
	validToken, err := totp.GenerateCode(otpSecret, authServer.GetClock().Now())
	require.NoError(t, err)

	diagnoseReq := conntest.TestConnectionRequest{
		ResourceKind: types.KindDatabase,
		ResourceName: databaseResourceName,
		DatabaseUser: databaseDBUser,
		DatabaseName: databaseDBName,
		// Default is 30 seconds but since tests run locally, we can reduce this value to also improve test responsiveness
		DialTimeout:        time.Second,
		InsecureSkipVerify: true,
		MFAResponse: client.MFAChallengeResponse{
			TOTPCode: validToken,
		},
	}
	respStatusCode, respBody := webPack.DoRequest(t, http.MethodPost, diagnoseConnectionEndpoint, diagnoseReq)
	require.Equal(t, http.StatusOK, respStatusCode, string(respBody))

	var connectionDiagnostic ui.ConnectionDiagnostic
	require.NoError(t, json.Unmarshal(respBody, &connectionDiagnostic))
	require.True(t, connectionDiagnostic.Success)
}

func waitForDatabases(t *testing.T, authServer *auth.Server, dbNames []string) {
	ctx := context.Background()

	require.Eventually(t, func() bool {
		all, err := authServer.GetDatabaseServers(ctx, apidefaults.Namespace)
		assert.NoError(t, err)

		if len(dbNames) > len(all) {
			return false
		}

		registered := 0
		for _, db := range dbNames {
			for _, a := range all {
				if a.GetName() == db {
					registered++
					break
				}
			}
		}
		return registered == len(dbNames)
	}, 30*time.Second, 100*time.Millisecond)
}
