/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package bigquery

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/srv/db/common"
)

// TestServerOption allows setting test server options.
type TestServerOption func(*TestServer)

// TestServer is a BigQuery test server that mocks the BigQuery REST API.
type TestServer struct {
	cfg    common.TestServerConfig
	port   string
	server *httptest.Server

	mu sync.Mutex
}

// NewTestServer returns a new instance of a test BigQuery server.
func NewTestServer(config common.TestServerConfig, opts ...TestServerOption) (*TestServer, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	port, err := config.Port()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/queries"):
			handleQuery(w, r)
		case strings.Contains(r.URL.Path, "/jobs"):
			handleJob(w, r)
		default:
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"kind": "bigquery#response"}`))
		}
	})

	server := &TestServer{
		cfg:  config,
		port: port,
		server: &httptest.Server{
			Listener: config.Listener,
			Config:   &http.Server{Handler: mux},
		},
	}

	for _, opt := range opts {
		opt(server)
	}

	return server, nil
}

// handleQuery handles POST to /queries endpoints (jobs.query).
func handleQuery(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.Write([]byte(testQueryResponse))
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	w.Write([]byte(testQueryResponse))
}

// handleJob handles POST to /jobs endpoints (jobs.insert).
func handleJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(testJobResponse))
}

// Serve starts serving client connections.
func (s *TestServer) Serve() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.server.Start()
	return nil
}

// Close closes the server.
func (s *TestServer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.server.Close()
	return nil
}

// Port returns the port the server is listening on.
func (s *TestServer) Port() string {
	return s.port
}

// Client is a test BigQuery client that sends HTTP requests.
type Client struct {
	addr string
}

// MakeTestClient returns a BigQuery test client.
// The client connects to the Teleport ALPN local proxy via plain HTTP.
// The engine handles HTTPS to the upstream BigQuery API.
func MakeTestClient(_ context.Context, config common.TestClientConfig) (*Client, error) {
	return &Client{
		addr: config.Address,
	}, nil
}

// Query sends a query request to the BigQuery proxy.
func (c *Client) Query(projectID, query string) (*http.Response, error) {
	url := "http://" + c.addr + "/bigquery/v2/projects/" + projectID + "/queries"
	body, _ := json.Marshal(map[string]interface{}{
		"query":        query,
		"useLegacySql": false,
	})
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))

	return http.DefaultClient.Do(req)
}

// ListDatasets sends a list datasets request to the BigQuery proxy.
func (c *Client) ListDatasets(projectID string) (*http.Response, error) {
	url := "http://" + c.addr + "/bigquery/v2/projects/" + projectID + "/datasets"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return http.DefaultClient.Do(req)
}

const (
	testQueryResponse = `{
    "kind": "bigquery#queryResponse",
    "schema": {
        "fields": [{"name": "f0_", "type": "INTEGER", "mode": "NULLABLE"}]
    },
    "jobReference": {"projectId": "test-project", "jobId": "test-job-id"},
    "totalRows": "1",
    "rows": [{"f": [{"v": "1"}]}],
    "totalBytesProcessed": "0",
    "jobComplete": true,
    "cacheHit": false
}`

	testJobResponse = `{
    "kind": "bigquery#job",
    "status": {"state": "DONE"},
    "jobReference": {"projectId": "test-project", "jobId": "test-job-id"},
    "configuration": {"query": {"query": "SELECT 1"}}
}`
)
