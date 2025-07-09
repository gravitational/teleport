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

package dynamodb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// Client alias for easier use.
type Client = dynamodb.DynamoDB

// ClientOptionsParams is a struct for client configuration options.
type ClientOptionsParams struct {
	Username string
}

// ClientOptions allows setting test client options.
type ClientOptions func(*ClientOptionsParams)

// MakeTestClient returns DynamoDB client connection according to the provided
// parameters.
func MakeTestClient(_ context.Context, config common.TestClientConfig, opts ...ClientOptions) (*Client, error) {
	provider := session.Must(session.NewSession(&aws.Config{
		Credentials: credentials.NewCredentials(&credentials.StaticProvider{Value: credentials.Value{
			AccessKeyID:     "fakeClientKeyID",
			SecretAccessKey: "fakeClientSecret",
		}}),
		Region: aws.String("local"),
	}))
	dynamoClient := dynamodb.New(provider, &aws.Config{
		Endpoint:   aws.String("http://" + config.Address),
		MaxRetries: aws.Int(0), // disable automatic retries in tests
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	})
	return dynamoClient, nil
}

// TestServerOption allows setting test server options.
type TestServerOption func(*TestServer)

// TestServer is a DynamoDB test server that mocks AWS signature checking and API.
type TestServer struct {
	cfg    common.TestServerConfig
	log    logrus.FieldLogger
	port   string
	server *httptest.Server

	// mu is needed to guard starting/closing the server.
	mu sync.Mutex
}

// NewTestServer returns a new instance of a test DynamoDB server.
func NewTestServer(config common.TestServerConfig, opts ...TestServerOption) (*TestServer, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log := logrus.WithFields(logrus.Fields{
		teleport.ComponentKey: defaults.ProtocolDynamoDB,
		"name":                config.Name,
	})
	tlsConfig, err := common.MakeTestServerTLSConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.InsecureSkipVerify = true // DynamoDB verifies requests with AWS sig v4, not mTLS.

	port, err := config.Port()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		err := awsutils.VerifyAWSSignature(r, credentials.NewStaticCredentials("AKIDl", "SECRET", "SESSION"))
		if err != nil {
			code := trace.ErrorToCode(err)
			body, _ := json.Marshal(jsonErr{
				Code:    strconv.Itoa(code),
				Message: err.Error(),
			})
			http.Error(w, string(body), code)
			return
		}
		w.Header().Set("Content-Type", awsutils.AmzJSON1_1)
		w.Header().Set("Content-Length", strconv.Itoa(len(testListTablesResponse)))
		w.Write([]byte(testListTablesResponse))
	})

	server := &TestServer{
		cfg:  config,
		log:  log,
		port: port,
		server: &httptest.Server{
			Listener: config.Listener,
			Config:   &http.Server{Handler: mux},
			TLS:      tlsConfig,
		},
	}

	for _, opt := range opts {
		opt(server)
	}

	return server, nil
}

// Serve starts serving client connections.
func (s *TestServer) Serve() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.server.StartTLS()
	return nil
}

// Close closes the server.
func (s *TestServer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.server.Close()
	return nil
}

func (s *TestServer) Port() string {
	return s.port
}

const (
	// testListTablesResponse is a default successful ListTables JSON response.
	testListTablesResponse = `
{
    "TableNames": [
        "table-one",
        "table-two"
    ]
}`
)
