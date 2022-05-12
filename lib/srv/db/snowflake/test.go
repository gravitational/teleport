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

package snowflake

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
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
	log                logrus.FieldLogger
	authorizationToken string
	forceTokenRefresh  bool
}

// NewTestServer returns a new instance of a test Snowflake server.
func NewTestServer(config common.TestServerConfig, opts ...TestServerOption) (*TestServer, error) {
	address := "localhost:0"
	if config.Address != "" {
		address = config.Address
	}
	tlsConfig, err := common.MakeTestServerTLSConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.InsecureSkipVerify = true

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	testServer := &TestServer{
		cfg:       config,
		listener:  listener,
		port:      port,
		tlsConfig: tlsConfig,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: defaults.ProtocolSnowflake,
			"name":          config.Name,
		}),
		authorizationToken: "test-token-123",
	}

	for _, opt := range opts {
		opt(testServer)
	}

	return testServer, nil
}

const (
	// loginResponse is a successful login response returned by the Snowflake mock server
	// used in tests.
	loginResponse = `
{
  "data": {
    "token": "test-token-123"
  },
  "success": true
}`
	// queryResponse is a successful query response returned by the Snowflake mock server
	// used in tests.
	queryResponse = `{
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
	// sessionEndResponse is the response returned by the Snowflake mock server on the session end request.
	sessionEndResponse = `{
  "data": null,
  "code": null,
  "message": null,
  "success": true
}
`

	// forceTokenRefresh is the response returned by the Snowflake mock server to force client to renew
	// the session token.
	forceTokenRefresh = `{
  "code": "390112",
  "message": null,
  "success": false
}
`
	// sessionTokenResponse is the response returned by the Snowflake mock server on token renew request.
	sessionTokenResponse = `{
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

// Serve starts serving client connections.
func (s *TestServer) Serve() error {
	mux := http.NewServeMux()
	handler := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case loginRequestPath:
			// TODO(jakule): verify JWT
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Length", strconv.Itoa(len(loginResponse)))
			w.Write([]byte(loginResponse))
		case queryRequestPath:
			w.Header().Set("Content-Type", "application/json")

			if r.Header.Get("Authorization") != fmt.Sprintf("Snowflake Token=\"%s\"", s.authorizationToken) {
				w.WriteHeader(401)
				return
			}

			if s.forceTokenRefresh {
				w.Write([]byte(forceTokenRefresh))
				w.Header().Set("Content-Length", strconv.Itoa(len(forceTokenRefresh)))
				s.forceTokenRefresh = false
				return
			}

			w.Header().Set("Content-Length", strconv.Itoa(len(queryResponse)))
			w.Write([]byte(queryResponse))
		case sessionRequestPath:
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Length", strconv.Itoa(len(sessionEndResponse)))
			w.Write([]byte(sessionEndResponse))
		case tokenRequestPath:
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Length", strconv.Itoa(len(sessionTokenResponse)))
			w.Write([]byte(sessionTokenResponse))
			s.authorizationToken = "sessionToken-123"
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
	mux.HandleFunc("/", handler)

	srv := &httptest.Server{
		Listener: s.listener,
		Config:   &http.Server{Handler: mux},
	}

	srv.StartTLS()

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

	db, err := sql.Open("snowflake", fmt.Sprintf("username:password@%s/dbname/schemaname?account=acn123&protocol=http&loginTimeout=5", config.Address))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return db, nil
}
