/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package podman

import (
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
)

// FakeServer is a fake implementation of the Podman REST API that can be used
// in tests.
type FakeServer struct {
	srv        *http.Server
	lis        net.Listener
	addr       string
	containers map[string]*Container
	pods       map[string]*Pod
}

// NewFakeServer creates a test server. Call Start to run the server and Close to stop it.
func NewFakeServer(socketAddr string, opts ...TestServerOption) (*FakeServer, error) {
	lis, err := net.Listen("unix", filepath.Join(socketAddr, "podman.sock"))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := &FakeServer{
		containers: make(map[string]*Container),
		pods:       make(map[string]*Pod),
		addr:       "unix://" + lis.Addr().String(),
		lis:        lis,
	}
	for _, opt := range opts {
		opt(s)
	}

	s.srv = &http.Server{
		Handler: s,
	}

	return s, nil
}

func (s *FakeServer) Start() {
	go func() { _ = s.srv.Serve(s.lis) }()
}

func (s *FakeServer) Close() error {
	return trace.Wrap(s.srv.Close())
}

// ServeHTTP satisfies the http.Handler interface.
func (s *FakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path, hasPrefix := strings.CutPrefix(r.URL.Path, "/v4.0.0/libpod")
	if !hasPrefix {
		http.NotFound(w, r)
		return
	}

	path, hasSuffix := strings.CutSuffix(path, "/json")
	if !hasSuffix {
		http.NotFound(w, r)
		return
	}

	if id, hasPrefix := strings.CutPrefix(path, "/containers/"); hasPrefix {
		container, ok := s.containers[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(container)
		return
	}

	if id, hasPrefix := strings.CutPrefix(path, "/pods/"); hasPrefix {
		pod, ok := s.pods[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(pod)
		return
	}

	http.NotFound(w, r)
}

// Addr returns the address on which the test server can be reached.
func (s *FakeServer) Addr() string { return s.addr }

// TestServerOption configures the test server.
type TestServerOption func(*FakeServer)

// WithContainer adds a container to the test server's mock data.
func WithContainer(c Container) TestServerOption {
	return func(s *FakeServer) {
		s.containers[c.ID] = &c
	}
}

// WithPod adds a pod to the test server's mock data.
func WithPod(p Pod) TestServerOption {
	return func(s *FakeServer) {
		s.pods[p.ID] = &p
	}
}
