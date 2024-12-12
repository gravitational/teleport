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

package opensearch

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"

	"github.com/gravitational/trace"
	"github.com/opensearch-project/opensearch-go/v2"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// TestServerOption allows setting test server options.
type TestServerOption func(*TestServer)

type TestServer struct {
	cfg       common.TestServerConfig
	listener  net.Listener
	port      string
	tlsConfig *tls.Config
	log       *slog.Logger
}

// NewTestServer returns a new instance of a test OpenSearch server.
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
		log: utils.NewSlogLoggerForTests().With(
			teleport.ComponentKey, defaults.ProtocolOpenSearch,
			"name", config.Name,
		),
	}

	for _, opt := range opts {
		opt(testServer)
	}

	return testServer, nil
}

// Serve starts serving client connections.
func (s *TestServer) Serve() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/_count", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(testCountResponse)))
		w.Write([]byte(testCountResponse))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		s.log.DebugContext(r.Context(), "Handling request", "url", logutils.StringerAttr(r.URL))
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
func MakeTestClient(_ context.Context, config common.TestClientConfig) (*opensearch.Client, error) {
	clt, err := opensearch.NewClient(opensearch.Config{
		Addresses: []string{"http://" + config.Address},
	})
	return clt, trace.Wrap(err)
}

const (
	// testQueryResponse is a default successful query response.
	testQueryResponse = `
{
  "name" : "5b1234425a372fe585532acf1da64323",
  "cluster_name" : "123476220453:test",
  "cluster_uuid" : "G_uZbXf-TgKJaGU8sEc70g",
  "version" : {
    "distribution" : "opensearch",
    "number" : "2.3.0",
    "build_type" : "tar",
    "build_hash" : "unknown",
    "build_date" : "2022-11-10T22:04:34.357368Z",
    "build_snapshot" : false,
    "lucene_version" : "9.3.0",
    "minimum_wire_compatibility_version" : "7.10.0",
    "minimum_index_compatibility_version" : "7.0.0"
  },
  "tagline" : "The OpenSearch Project: https://opensearch.org/"
}`

	// testCountResponse is returned from Count endpoint.
	testCountResponse = `{"count":31874,"_shards":{"total":6,"successful":6,"skipped":0,"failed":0}}`
)
