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

package elasticsearch

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

// TestServerOption allows setting test server options.
type TestServerOption func(*TestServer)

type TestServer struct {
	cfg       common.TestServerConfig
	listener  net.Listener
	port      string
	tlsConfig *tls.Config
	log       logrus.FieldLogger
}

// NewTestServer returns a new instance of a test Elasticsearch server.
func NewTestServer(config common.TestServerConfig, opts ...TestServerOption) (svr *TestServer, err error) {

	err = config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer config.CloseOnError(&err)

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
		cfg:       config,
		listener:  config.Listener,
		port:      port,
		tlsConfig: tlsConfig,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: defaults.ProtocolElasticsearch,
			"name":          config.Name,
		}),
	}

	for _, opt := range opts {
		opt(testServer)
	}

	return testServer, nil
}

// Serve starts serving client connections.
func (s *TestServer) Serve() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/_sql", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(testSQLResponse)))
		w.Write([]byte(testSQLResponse))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(testQueryResponse)))
		w.Write([]byte(testQueryResponse))
	})

	srv := &httptest.Server{
		Listener: s.listener,
		Config:   &http.Server{Handler: mux},
		TLS:      s.tlsConfig,
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

// MakeTestClient returns Redis client connection according to the provided
// parameters.
func MakeTestClient(_ context.Context, config common.TestClientConfig) (*elasticsearch.Client, error) {
	es, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"http://" + config.Address},
	})

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return es, nil
}

const (
	// testQueryResponse is a default successful query response.
	testQueryResponse = `
{
  "name" : "es01",
  "cluster_name" : "docker-cluster",
  "cluster_uuid" : "4--PI31pREC7AnXs_3pchA",
  "version" : {
    "number" : "8.3.3",
    "build_flavor" : "default",
    "build_type" : "docker",
    "build_hash" : "801fed82df74dbe537f89b71b098ccaff88d2c56",
    "build_date" : "2022-07-23T19:30:09.227964828Z",
    "build_snapshot" : false,
    "lucene_version" : "9.2.0",
    "minimum_wire_compatibility_version" : "7.17.0",
    "minimum_index_compatibility_version" : "7.0.0"
  },
  "tagline" : "You Know, for Search"
}`

	// testSQLResponse is returned from SQL endpoint.
	testSQLResponse = `{"columns":[{"name":"42","type":"integer"}],"rows":[[42]]}`
)
