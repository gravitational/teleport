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
	"errors"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	prehogv1alpha "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	teletermv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/vnet/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusteridcache"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/daemon"
	"github.com/gravitational/teleport/lib/vnet"
)

// Service implements gRPC service for VNet.
type Service struct {
	api.UnimplementedVnetServiceServer

	cfg    Config
	log    *slog.Logger
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
		// TODO: Replace with logutils.NewPackageLogger after rebasing on top of master.
		log: slog.With(teleport.ComponentKey, "term:vnet"),
	}, nil
}

type Config struct {
	DaemonService  *daemon.Service
	ClusterIDCache *clusteridcache.Cache
	// InstallationID used for event reporting.
	InstallationID string
	ReportUsage    bool
}

// CheckAndSetDefaults checks and sets the defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.DaemonService == nil {
		return trace.BadParameter("missing DaemonService")
	}

	if c.ClusterIDCache == nil {
		return trace.BadParameter("missing cluster ID cache")
	}

	if c.InstallationID == "" {
		return trace.BadParameter("missing installation ID")
	}

	return nil
}

func (s *Service) Start(ctx context.Context, req *api.StartRequest) (*api.StartResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed.Load() {
		return nil, trace.Errorf("VNet service has been closed")
	}

	if s.vnet != nil {
		return nil, trace.CompareFailed("VNet service is already running")
	}

	// TODO(ravicious): Support multiple root clusters.
	rootClusters, err := s.cfg.DaemonService.ListRootClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	i := slices.IndexFunc(rootClusters, func(cluster *clusters.Cluster) bool {
		return cluster.Connected()
	})
	if i == -1 {
		return nil, trace.Errorf("no active root cluster found")
	}
	rootCluster := rootClusters[i]

	_, client, err := s.cfg.DaemonService.ResolveClusterURI(rootCluster.URI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// vnet.CreateAndSetupTUNDevice passes the provided context to exec.CommandContext which executes
	// a long running admin subcommand. The context itself has little effect on the execution of the
	// admin subcommand once it's started, but it's able to close the prompt for the password if one
	// is shown at the time of cancelation.
	adminSubcmdCtx, cancelAdminSubcmdCtx := context.WithCancel(context.Background())

	ipv6Prefix, err := vnet.IPv6Prefix()
	if err != nil {
		cancelAdminSubcmdCtx()
		return nil, trace.Wrap(err)
	}

	customDNSZones := []string{}
	tun, cleanup, err := vnet.CreateAndSetupTUNDevice(adminSubcmdCtx, ipv6Prefix.String(), customDNSZones)
	if err != nil {
		cancelAdminSubcmdCtx()
		return nil, trace.Wrap(err)
	}

	config := &vnet.Config{
		Client:     client,
		TUNDevice:  tun,
		IPv6Prefix: ipv6Prefix,
	}

	if s.cfg.ReportUsage {
		config.Middleware = &UsageReportingMiddleware{
			daemonService:  s.cfg.DaemonService,
			log:            s.log,
			clusterIDCache: s.cfg.ClusterIDCache,
			reportedApps:   make(map[string]struct{}),
			installationID: s.cfg.InstallationID,
		}
	}

	// TODO: Should NewManager take context?
	manager, err := vnet.NewManager(context.TODO(), config)
	if err != nil {
		cancelAdminSubcmdCtx()
		cleanup()
		return nil, trace.Wrap(err)
	}

	s.vnet = manager

	go func() {
		defer cleanup()
		defer cancelAdminSubcmdCtx()
		err := s.vnet.Run()
		if err != nil && !errors.Is(err, context.Canceled) {
			s.log.ErrorContext(ctx, "VNet manager did not exit successfully", "error", err)
		}
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

type UsageReportingMiddleware struct {
	daemonService *daemon.Service
	// reportedApps contains a set of URIs for apps which usage has been already reported.
	// App gateways (local proxies) in Connect report a single event per gateway created per app. VNet
	// needs to replicate this behavior, hence why it keeps track of reported apps to report only one
	// event per app per VNet's lifespan.
	reportedApps map[string]struct{}
	// mu protects access to reportedApps.
	mu sync.Mutex
	// clusterIDCache stores cluster ID that needs to be included with each usage event. It's updated
	// outside of UsageReportingMiddleware – the middleware merely reads data from it. If the cache
	// does not contain the given cluster ID, the middleware drops the event.
	clusterIDCache *clusteridcache.Cache
	log            *slog.Logger
	installationID string
}

func (m *UsageReportingMiddleware) OnNewConnection(ctx context.Context, tc *client.TeleportClient, app types.Application) {
	go func() {
		err := m.onNewConnection(ctx, tc, app)
		if err != nil {
			m.log.ErrorContext(ctx, "Failed to submit usage event", "error", err, "app", app.GetName())
		}
	}()
}

func (m *UsageReportingMiddleware) onNewConnection(ctx context.Context, tc *client.TeleportClient, app types.Application) error {
	clusterURI, rootClusterName, err := getClusterURIAndRootClusterName(ctx, tc)
	if err != nil {
		return trace.Wrap(err)
	}
	rootClusterURI := clusterURI.GetRootClusterURI()
	appURI := clusterURI.AppendApp(app.GetName())

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, hasAppBeenReported := m.reportedApps[appURI.String()]; hasAppBeenReported {
		m.log.DebugContext(ctx, "App was already reported", "app", appURI.String())
		return nil
	}

	clusterID, ok := m.clusterIDCache.Load(rootClusterURI)
	if !ok {
		return trace.Errorf("cluster ID for %q not found", rootClusterURI)
	}

	m.log.DebugContext(ctx, "Reporting usage event", "app", appURI.String())

	err = m.daemonService.ReportUsageEvent(&teletermv1.ReportUsageEventRequest{
		AuthClusterId: clusterID,
		PrehogReq: &prehogv1alpha.SubmitConnectEventRequest{
			DistinctId: m.installationID,
			Timestamp:  timestamppb.Now(),
			Event: &prehogv1alpha.SubmitConnectEventRequest_ProtocolUse{
				ProtocolUse: &prehogv1alpha.ConnectProtocolUseEvent{
					ClusterName: rootClusterName,
					UserName:    tc.Username,
					Protocol:    "app",
					Origin:      "vnet",
					// TODO: Add AccessThrough: "vnet"
				},
			},
		},
	})
	if err != nil {
		return trace.Wrap(err, "adding event to queue")
	}

	m.reportedApps[appURI.String()] = struct{}{}

	return nil
}

func getClusterURIAndRootClusterName(ctx context.Context, tc *client.TeleportClient) (uri.ResourceURI, string, error) {
	profileName := tc.Profile().Name()
	siteName := tc.SiteName
	rootClusterName, err := tc.RootClusterName(ctx)
	if err != nil {
		return uri.ResourceURI{}, "", trace.Wrap(err)
	}
	clusterURI := uri.NewClusterURI(profileName)

	if siteName != "" && siteName != rootClusterName {
		clusterURI = clusterURI.AppendLeafCluster(siteName)
	}

	return clusterURI, rootClusterName, nil
}
