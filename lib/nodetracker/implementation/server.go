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

package implementation

import (
	"context"
	"net"
	"time"

	"github.com/gravitational/teleport/lib/nodetracker"
	"github.com/gravitational/teleport/lib/nodetracker/api"

	"github.com/gravitational/trace"

	"google.golang.org/grpc"
)

// Server is a node tracker server implementation of api.Server
// This might be moved to teleport/e or somewhere else private
type Server struct {
	listener       net.Listener
	server         *grpc.Server
	trackerService *trackerService
}

// NewServer initializes a node tracker service grpc server
func NewServer(listener net.Listener, offlineThreshold time.Duration) {
	tracker := NewTracker(&Config{OfflineThreshold: offlineThreshold})
	trackerService := NewTrackerService(tracker)

	server := grpc.NewServer()
	api.RegisterNodeTrackerServiceServer(server, trackerService)

	s := Server{
		listener:       listener,
		server:         server,
		trackerService: trackerService,
	}

	nodetracker.SetServer(&s)
}

// Serve starts the grpc server
func (s *Server) Start() error {
	if err := s.server.Serve(s.listener); err != nil && err != grpc.ErrServerStopped {
		return trace.Wrap(err)
	}
	return nil
}

// Stop stops the grpc server
func (s *Server) Stop() {
	s.server.GracefulStop()
	s.trackerService.tracker.Stop()
}

type trackerService struct {
	tracker api.Tracker
}

// NewTrackerService returns a tracker service handler that is responsible
// tracking node to proxy relations
func NewTrackerService(tracker api.Tracker) *trackerService {
	return &trackerService{
		tracker: tracker,
	}
}

// AddNode is a wrapper around the tracker's AddNode implementation
func (t *trackerService) AddNode(ctx context.Context, request *api.AddNodeRequest) (*api.AddNodeResponse, error) {
	t.tracker.AddNode(
		ctx,
		request.NodeID,
		request.ProxyID,
		request.ClusterName,
		request.Addr,
	)
	return &api.AddNodeResponse{}, nil
}

// RemoveNode is a wrapper around the tracker's RemoveNode implementation
func (t *trackerService) RemoveNode(ctx context.Context, request *api.RemoveNodeRequest) (*api.RemoveNodeResponse, error) {
	t.tracker.RemoveNode(ctx, request.NodeID)
	return &api.RemoveNodeResponse{}, nil
}

// GetProxies is a wrapper around the tracker's GetProxies implementation
func (t *trackerService) GetProxies(ctx context.Context, request *api.GetProxiesRequest) (*api.GetProxiesResponse, error) {
	proxyDetails := t.tracker.GetProxies(ctx, request.NodeID)
	return &api.GetProxiesResponse{ProxyDetails: proxyDetails}, nil
}
