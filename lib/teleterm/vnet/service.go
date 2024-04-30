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

package vnet

import (
	"context"
	"crypto/tls"
	"errors"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	vnetproto "github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/api/types"
	apiteleterm "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/vnet/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/daemon"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/vnet"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "term:vnet")

type status int

const (
	statusNotRunning status = iota
	statusRunning
	statusClosed
)

// Service implements gRPC service for VNet.
type Service struct {
	api.UnimplementedVnetServiceServer

	cfg            Config
	mu             sync.Mutex
	status         status
	processManager *vnet.ProcessManager
}

// New creates an instance of Service.
func New(cfg Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		cfg: cfg,
	}, nil
}

type Config struct {
	DaemonService      *daemon.Service
	InsecureSkipVerify bool
	// InstallationID used for event reporting.
	InstallationID string
}

// CheckAndSetDefaults checks and sets the defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.DaemonService == nil {
		return trace.BadParameter("missing DaemonService")
	}

	if c.InstallationID == "" {
		return trace.BadParameter("missing InstallationID")
	}

	return nil
}

func (s *Service) Start(ctx context.Context, req *api.StartRequest) (*api.StartResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == statusClosed {
		return nil, trace.CompareFailed("VNet service has been closed")
	}

	if s.status == statusRunning {
		return &api.StartResponse{}, nil
	}

	appProvider := &appProvider{
		daemonService:      s.cfg.DaemonService,
		insecureSkipVerify: s.cfg.InsecureSkipVerify,
	}

	processManager, err := vnet.SetupAndRun(ctx, appProvider)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		err := processManager.Wait()
		if err != nil && !errors.Is(err, context.Canceled) {
			log.ErrorContext(ctx, "VNet closed with an error", "error", err)
		} else {
			log.DebugContext(ctx, "VNet closed")
		}

		// TODO(ravicious): Notify the Electron app about change of VNet state, but only if it's
		// running. If it's not running, then the Start RPC has already failed and forwarded the error
		// to the user.

		s.mu.Lock()
		defer s.mu.Unlock()

		if s.status == statusRunning {
			s.status = statusNotRunning
		}
	}()

	s.processManager = processManager
	s.status = statusRunning
	return &api.StartResponse{}, nil
}

// Stop stops VNet and cleans up used resources. Blocks until VNet stops or ctx is canceled.
func (s *Service) Stop(ctx context.Context, req *api.StopRequest) (*api.StopResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.stopLocked()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.StopResponse{}, nil
}

func (s *Service) stopLocked() error {
	if s.status == statusClosed {
		return trace.CompareFailed("VNet service has been closed")
	}

	if s.status == statusNotRunning {
		return nil
	}

	s.processManager.Close()
	err := s.processManager.Wait()
	if err != nil && !errors.Is(err, context.Canceled) {
		return trace.Wrap(err)
	}

	s.status = statusNotRunning
	return nil
}

// Close stops VNet service and prevents it from being started again. Blocks until VNet stops.
// Intended for cleanup code when tsh daemon gets terminated.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.stopLocked()
	if err != nil {
		return trace.Wrap(err)
	}

	s.status = statusClosed
	return nil
}

type appProvider struct {
	daemonService      *daemon.Service
	insecureSkipVerify bool
}

func (p *appProvider) ListProfiles() ([]string, error) {
	profiles, err := p.daemonService.ListProfileNames()
	return profiles, trace.Wrap(err)
}

func (p *appProvider) GetCachedClient(ctx context.Context, profileName, leafClusterName string) (vnet.ClusterClient, error) {
	return p.getCachedClient(ctx, profileName, leafClusterName)
}

func (p *appProvider) getCachedClient(ctx context.Context, profileName, leafClusterName string) (*client.ClusterClient, error) {
	uri := uri.NewClusterURI(profileName).AppendLeafCluster(leafClusterName)
	client, err := p.daemonService.GetCachedClient(ctx, uri)
	return client, trace.Wrap(err)
}

func (p *appProvider) ReissueAppCert(ctx context.Context, profileName, leafClusterName string, app types.Application) (tls.Certificate, error) {
	clusterURI := uri.NewClusterURI(profileName).AppendLeafCluster(leafClusterName)
	appURI := clusterURI.AppendApp(app.GetName())

	reloginReq := &apiteleterm.ReloginRequest{
		RootClusterUri: clusterURI.GetRootClusterURI().String(),
		Reason: &apiteleterm.ReloginRequest_VnetCertExpired{
			VnetCertExpired: &apiteleterm.VnetCertExpired{
				TargetUri: appURI.String(),
			},
		},
	}

	var cert tls.Certificate

	reissueCert := func() error {
		cluster, _, err := p.daemonService.ResolveClusterURI(clusterURI)
		if err != nil {
			return trace.Wrap(err)
		}

		client, err := p.daemonService.GetCachedClient(ctx, clusterURI)
		if err != nil {
			return trace.Wrap(err)
		}

		cert, err = cluster.ReissueAppCert(ctx, client, app)
		return trace.Wrap(err)
	}

	if err := p.daemonService.RetryWithRelogin(ctx, reloginReq, reissueCert); err != nil {
		notifyErr := p.daemonService.NotifyApp(ctx, &apiteleterm.SendNotificationRequest{
			Subject: &apiteleterm.SendNotificationRequest_CannotProxyVnetConnection{
				CannotProxyVnetConnection: &apiteleterm.CannotProxyVnetConnection{
					TargetUri: appURI.String(),
					Error:     err.Error(),
				},
			},
		})
		if notifyErr != nil {
			log.ErrorContext(ctx, "Failed to send a notification for an error encountered during VNet cert reissue",
				"cert_reissue_error", err, "notify_error", notifyErr)
		}

		return tls.Certificate{}, trace.Wrap(err)
	}

	return cert, nil
}

// GetDialOptions returns ALPN dial options for the profile.
func (p *appProvider) GetDialOptions(ctx context.Context, profileName string) (*vnet.DialOptions, error) {
	cluster, tc, err := p.daemonService.ResolveClusterURI(uri.NewClusterURI(profileName))
	if err != nil {
		return nil, trace.Wrap(err, "resolving cluster by URI")
	}

	dialOpts := &vnet.DialOptions{
		WebProxyAddr:            cluster.GetProxyHost(),
		ALPNConnUpgradeRequired: tc.TLSRoutingConnUpgradeRequired,
		InsecureSkipVerify:      p.insecureSkipVerify,
	}
	if dialOpts.ALPNConnUpgradeRequired {
		dialOpts.RootClusterCACertPool, err = tc.RootClusterCACertPool(ctx)
		if err != nil {
			return nil, trace.Wrap(err, "loading root cluster CA cert pool")
		}
	}
	return dialOpts, nil
}

func (p *appProvider) GetVnetConfig(ctx context.Context, profileName, leafClusterName string) (*vnetproto.VnetConfig, error) {
	clusterClient, err := p.getCachedClient(ctx, profileName, leafClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	vnetConfigClient := clusterClient.AuthClient.VnetConfigServiceClient()
	vnetConfig, err := vnetConfigClient.GetVnetConfig(ctx, &vnetproto.GetVnetConfigRequest{})
	return vnetConfig, trace.Wrap(err)
}

func (p *appProvider) OnNewConnection(ctx context.Context, profileName, leafClusterName string, app types.Application) error {
	return nil
}
