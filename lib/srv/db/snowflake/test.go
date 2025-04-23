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

package snowflake

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

// TestServerOption allows setting test server options.
type TestServerOption func(*TestServer)

// TestForceTokenRefresh makes the test server return sessionExpiredCode
// error on the first query request.
func TestForceTokenRefresh() TestServerOption {
	return func(ts *TestServer) {
		ts.forceTokenRefresh = true
	}
}

type TestServer struct {
	cfg                common.TestServerConfig
	listener           net.Listener
	port               string
	tlsConfig          *tls.Config
	authorizationToken string
	forceTokenRefresh  bool
}

// NewTestServer returns a new instance of a test Snowflake server.
func NewTestServer(config common.TestServerConfig, opts ...TestServerOption) (*TestServer, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig, err := common.MakeTestServerTLSConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.InsecureSkipVerify = true

	port, err := config.Port()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	testServer := &TestServer{
		cfg:                config,
		listener:           config.Listener,
		port:               port,
		tlsConfig:          tlsConfig,
		authorizationToken: "test-token-123",
	}

	for _, opt := range opts {
		opt(testServer)
	}

	return testServer, nil
}

// Serve starts serving client connections.
func (s *TestServer) Serve() error {
	mux := http.NewServeMux()
	mux.HandleFunc(loginRequestPath, s.handleLogin)
	mux.HandleFunc(queryRequestPath, s.handleQuery)
	mux.HandleFunc(sessionRequestPath, s.handleSession)
	mux.HandleFunc(tokenRequestPath, s.handleToken)

	srv := &httptest.Server{
		Listener: s.listener,
		Config:   &http.Server{Handler: mux},
	}

	srv.StartTLS()

	return nil
}

// handleLogin is the test server /session/v1/login-request HTTP handler.
func (s *TestServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	loginReq := &loginRequest{}
	if err := json.NewDecoder(r.Body).Decode(loginReq); err != nil {
		w.WriteHeader(400)
		return
	}
	data := loginReq.Data
	// Verify received JWT and return HTTP 401 if verification fails.
	if err := s.verifyJWT(r.Context(), data.AccountName, data.LoginName, data.Token); err != nil {
		w.WriteHeader(401)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(testLoginResponse)))
	w.Write([]byte(testLoginResponse))
}

// handleQuery is the test server /queries/v1/query-request HTTP handler.
func (s *TestServer) handleQuery(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Header.Get("Authorization") != fmt.Sprintf("Snowflake Token=\"%s\"", s.authorizationToken) {
		w.WriteHeader(401)
		return
	}

	if s.forceTokenRefresh {
		w.Write([]byte(testForceTokenRefresh))
		w.Header().Set("Content-Length", strconv.Itoa(len(testForceTokenRefresh)))
		s.forceTokenRefresh = false
		return
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(testQueryResponse)))
	w.Write([]byte(testQueryResponse))
}

// handleSession is the test server /session HTTP handler.
func (s *TestServer) handleSession(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(testSessionEndResponse)))
	w.Write([]byte(testSessionEndResponse))
}

// handleToken is the test server /session/token-request HTTP handler.
func (s *TestServer) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Snowflake Token=\"master-token-123\"" {
		w.WriteHeader(401)
		return
	}

	renewReq := &renewSessionRequest{}
	if err := json.NewDecoder(r.Body).Decode(renewReq); err != nil {
		w.WriteHeader(400)
		return
	}

	if renewReq.OldSessionToken != "test-token-123" {
		w.WriteHeader(401)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(testSessionTokenResponse)))
	w.Write([]byte(testSessionTokenResponse))
	s.authorizationToken = "sessionToken-123"
}

// verifyJWT verifies the provided JWT token. It checks if the token was signed with the Database Client CA
// and asserts token claims.
func (s *TestServer) verifyJWT(ctx context.Context, accName, loginName, token string) error {
	clusterName := "root.example.com"

	caCert, err := s.cfg.AuthClient.GetCertAuthority(ctx, types.CertAuthID{
		DomainName: clusterName,
		Type:       types.DatabaseClientCA,
	}, false)
	if err != nil {
		return trace.Wrap(err)
	}

	clock := clockwork.NewRealClock()
	publicKey := caCert.GetActiveKeys().TLS[0].Cert

	block, _ := pem.Decode(publicKey)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return trace.Wrap(err)
	}

	key, err := jwt.New(&jwt.Config{
		Clock:       clock,
		PublicKey:   cert.PublicKey,
		ClusterName: clusterName,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = key.VerifySnowflake(jwt.SnowflakeVerifyParams{
		RawToken:    token,
		LoginName:   loginName,
		AccountName: accName,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *TestServer) Port() string {
	return s.port
}

// Close starts serving client connections.
func (s *TestServer) Close() error {
	return s.listener.Close()
}

// ClientOptionsParams is a struct for client configuration options.
type ClientOptionsParams struct {
}

// ClientOptions allows setting test client options.
type ClientOptions func(*ClientOptionsParams)

// MakeTestClient returns Redis client connection according to the provided
// parameters.
func MakeTestClient(_ context.Context, config common.TestClientConfig, opts ...ClientOptions) (*sql.DB, error) {
	clientOptions := &ClientOptionsParams{}

	for _, opt := range opts {
		opt(clientOptions)
	}

	snowflakeDSN := fmt.Sprintf("%s:password@%s/dbname/schemaname?account=acn123&protocol=http&loginTimeout=5",
		config.RouteToDatabase.Username, config.Address)
	db, err := sql.Open("snowflake", snowflakeDSN)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return db, nil
}

const (
	// testLoginResponse is a successful login response returned by the Snowflake mock server
	// used in tests.
	testLoginResponse = `
{
  "data": {
    "token": "test-token-123",
	"validityInSeconds": 3600,
    "masterToken": "master-token-123",
	"masterValidityInSeconds": 14400
  },
  "success": true
}`
	// testQueryResponse is a successful query response returned by the Snowflake mock server
	// used in tests.
	testQueryResponse = `{
  "data": {
    "parameters": [
      {
        "name": "CLIENT_PREFETCH_THREADS",
        "value": 4
      },
      {
        "name": "TIMESTAMP_OUTPUT_FORMAT",
        "value": "YYYY-MM-DD HH24:MI:SS.FF3 TZHTZM"
      },
      {
        "name": "TIME_OUTPUT_FORMAT",
        "value": "HH24:MI:SS"
      },
      {
        "name": "TIMESTAMP_TZ_OUTPUT_FORMAT",
        "value": ""
      },
      {
        "name": "CLIENT_RESULT_CHUNK_SIZE",
        "value": 160
      },
      {
        "name": "CLIENT_SESSION_KEEP_ALIVE",
        "value": false
      },
      {
        "name": "CLIENT_OUT_OF_BAND_TELEMETRY_ENABLED",
        "value": false
      },
      {
        "name": "CLIENT_METADATA_USE_SESSION_DATABASE",
        "value": false
      },
      {
        "name": "ENABLE_STAGE_S3_PRIVATELINK_FOR_US_EAST_1",
        "value": false
      },
      {
        "name": "CLIENT_RESULT_PREFETCH_THREADS",
        "value": 1
      },
      {
        "name": "TIMESTAMP_NTZ_OUTPUT_FORMAT",
        "value": "YYYY-MM-DD HH24:MI:SS.FF3"
      },
      {
        "name": "CLIENT_METADATA_REQUEST_USE_CONNECTION_CTX",
        "value": false
      },
      {
        "name": "CLIENT_HONOR_CLIENT_TZ_FOR_TIMESTAMP_NTZ",
        "value": true
      },
      {
        "name": "CLIENT_MEMORY_LIMIT",
        "value": 1536
      },
      {
        "name": "CLIENT_TIMESTAMP_TYPE_MAPPING",
        "value": "TIMESTAMP_LTZ"
      },
      {
        "name": "TIMEZONE",
        "value": "America/Los_Angeles"
      },
      {
        "name": "CLIENT_RESULT_PREFETCH_SLOTS",
        "value": 2
      },
      {
        "name": "CLIENT_TELEMETRY_ENABLED",
        "value": true
      },
      {
        "name": "CLIENT_USE_V1_QUERY_API",
        "value": true
      },
      {
        "name": "CLIENT_DISABLE_INCIDENTS",
        "value": true
      },
      {
        "name": "CLIENT_RESULT_COLUMN_CASE_INSENSITIVE",
        "value": false
      },
      {
        "name": "BINARY_OUTPUT_FORMAT",
        "value": "HEX"
      },
      {
        "name": "CLIENT_ENABLE_LOG_INFO_STATEMENT_PARAMETERS",
        "value": false
      },
      {
        "name": "CLIENT_TELEMETRY_SESSIONLESS_ENABLED",
        "value": true
      },
      {
        "name": "DATE_OUTPUT_FORMAT",
        "value": "YYYY-MM-DD"
      },
      {
        "name": "CLIENT_FORCE_PROTECT_ID_TOKEN",
        "value": true
      },
      {
        "name": "CLIENT_CONSENT_CACHE_ID_TOKEN",
        "value": false
      },
      {
        "name": "CLIENT_STAGE_ARRAY_BINDING_THRESHOLD",
        "value": 65280
      },
      {
        "name": "CLIENT_SESSION_KEEP_ALIVE_HEARTBEAT_FREQUENCY",
        "value": 3600
      },
      {
        "name": "CLIENT_SESSION_CLONE",
        "value": false
      },
      {
        "name": "AUTOCOMMIT",
        "value": true
      },
      {
        "name": "TIMESTAMP_LTZ_OUTPUT_FORMAT",
        "value": ""
      }
    ],
    "rowtype": [
      {
        "name": "42",
        "database": "",
        "schema": "",
        "table": "",
        "byteLength": null,
        "scale": 0,
        "precision": 2,
        "type": "fixed",
        "nullable": false,
        "collation": null,
        "length": null
      }
    ],
    "rowset": [
      [
        "42"
      ]
    ],
    "total": 1,
    "returned": 1,
    "queryId": "01a423a5-0401-6d90-005f-7d030001802e",
    "databaseProvider": null,
    "finalDatabaseName": null,
    "finalSchemaName": null,
    "finalWarehouseName": null,
    "finalRoleName": "PUBLIC",
    "numberOfBinds": 0,
    "arrayBindSupported": false,
    "statementTypeId": 4096,
    "version": 1,
    "sendResultTime": 1652054723686,
    "queryResultFormat": "json"
  },
  "code": null,
  "message": null,
  "success": true
}
`
	// testSessionEndResponse is the response returned by the Snowflake mock server on the session end request.
	testSessionEndResponse = `{
  "data": null,
  "code": null,
  "message": null,
  "success": true
}
`

	// testForceTokenRefresh is the response returned by the Snowflake mock server to force client to renew
	// the session token.
	testForceTokenRefresh = `{
  "code": "390112",
  "message": null,
  "success": false
}
`
	// testSessionTokenResponse is the response returned by the Snowflake mock server on token renew request.
	testSessionTokenResponse = `{
  "data": {
    "sessionToken": "sessionToken-123",
    "validityInSecondsST": 3600,
    "masterToken": "masterToken-123",
    "validityInSecondsMT": 14400,
    "sessionId": 2325132
  },
  "success": true
}
`
)
