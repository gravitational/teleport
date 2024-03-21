// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package service

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/gravitational/trace"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/vnet/v1"
	"github.com/gravitational/teleport/lib/teleterm/daemon"
	"github.com/gravitational/teleport/lib/vnet"
)

// Service implements gRPC service for VNet.
type Service struct {
	api.UnimplementedVnetServiceServer

	cfg    Config
	vnet   *vnet.Manager
	mu     sync.Mutex
	closed atomic.Bool
}

// New creates an instance of Service
func New(cfg Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		cfg: cfg,
	}, nil
}

type Config struct {
	DaemonService *daemon.Service
}

// CheckAndSetDefaults checks and sets the defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.DaemonService == nil {
		return trace.BadParameter("missing DaemonService")
	}

	return nil
}

func (s *Service) Start(ctx context.Context, req *api.StartRequest) (*api.StartResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed.Load() {
		return nil, trace.Errorf("VNet service has been closed")
	}

	// TODO: Take req.RootClusterUri into account.
	if s.vnet != nil {
		return nil, trace.CompareFailed("VNet service is already running")
	}

	// adminSubcmdCtx has no effect on the execution of the admin subcommand itself, but it's
	// able to close the prompt for the password if one is shown at the time of cancelation.
	adminSubcmdCtx, cancelAdminSubcmdCtx := context.WithCancel(context.Background())

	tun, cleanup, err := vnet.CreateAndSetupTUNDevice(adminSubcmdCtx)
	if err != nil {
		cancelAdminSubcmdCtx()
		return nil, trace.Wrap(err)
	}

	_, client, err := s.cfg.DaemonService.ResolveCluster(req.RootClusterUri)
	if err != nil {
		cancelAdminSubcmdCtx()
		cleanup()
		return nil, trace.Wrap(err)
	}

	// TODO: Should NewManager take context?
	manager, err := vnet.NewManager(context.TODO(), &vnet.Config{
		Client:    client,
		TUNDevice: tun,
	})
	if err != nil {
		cancelAdminSubcmdCtx()
		cleanup()
		return nil, trace.Wrap(err)
	}

	s.vnet = manager

	go func() {
		defer cleanup()
		defer cancelAdminSubcmdCtx()
		s.vnet.Run()
		// TODO: Log error.
	}()

	return &api.StartResponse{}, nil
}

// Stop closes the VNet instance. req.RootClusterUri must match RootClusterUri of the currently
// active instance.
//
// Intended to be called by the Electron app when the user wants to stop VNet for a particular root
// cluster.
func (s *Service) Stop(ctx context.Context, req *api.StopRequest) (*api.StopResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed.Load() {
		return nil, trace.Errorf("VNet service has been closed")
	}

	if s.vnet == nil {
		return nil, trace.Errorf("VNet service is not running")
	}

	err := s.vnet.Close()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.vnet = nil

	return &api.StopResponse{}, nil
}

// Close stops the current VNet instance and prevents new instances from being started.
//
// Intended for cleanup code when tsh daemon gets terminated.
func (s *Service) Close() error {
	s.closed.Store(true)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.vnet != nil {
		if err := s.vnet.Close(); err != nil {
			return trace.Wrap(err)
		}
	}

	s.vnet = nil

	return nil
}
