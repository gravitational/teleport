// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package desktop

import (
	"cmp"
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"time"

	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	linuxdesktopv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/linuxdesktop/linuxdesktopv1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/desktop/rdp/rdpclient"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/utils"
)

// LinuxService implements the RDP-based Linux desktop access service.
//
// This service accepts mTLS connections from the proxy, establishes RDP
// connections to Linux hosts and translates RDP into Teleport's desktop
// protocol.
type LinuxService struct {
	cfg        LinuxServiceConfig
	middleware *authz.Middleware

	// clusterName is the cached local cluster name, to avoid calling
	// cfg.AccessPoint.GetClusterName multiple times.
	clusterName string

	// auditCache caches information from shared directory
	// TDP messages that are needed for
	// creating shared directory audit events.
	auditCache sharedDirectoryAuditCache

	closeCtx context.Context
	close    func()
}

// LinuxServiceConfig contains all necessary configuration values for a
// LinuxService.
type LinuxServiceConfig struct {
	// Logger is the logger for the service.
	Logger *slog.Logger
	// Clock provides current time.
	Clock        clockwork.Clock
	DataDir      string
	LicenseStore rdpclient.LicenseStore
	// Authorizer is used to authorize requests.
	Authorizer authz.Authorizer
	// LockWatcher is used to monitor for new locks.
	LockWatcher *services.LockWatcher
	// Emitter emits audit log events.
	Emitter events.Emitter
	// TLS is the TLS server configuration.
	TLS *tls.Config
	// AccessPoint is the Auth API client (with caching).
	AccessPoint authclient.LinuxDesktopAccessPoint
	// AuthClient is the Auth API client (without caching).
	AuthClient authclient.ClientI
	// InventoryHandle is used to send linux desktop heartbeats via the inventory control stream.
	InventoryHandle inventory.DownstreamHandle
	// ConnLimiter limits the number of active connections per client IP.
	ConnLimiter *limiter.ConnectionsLimiter
	// Heartbeat contains configuration for service heartbeats.
	Heartbeat HeartbeatConfig
	// Hostname of the Linux desktop service
	Hostname string
	// ConnectedProxyGetter gets the proxies teleport is connected to.
	ConnectedProxyGetter reversetunnelclient.ConnectedProxyGetter
	Labels               map[string]string
}

func (cfg *LinuxServiceConfig) CheckAndSetDefaults() error {
	if cfg.Authorizer == nil {
		return trace.BadParameter("LinuxServiceConfig is missing Authorizer")
	}
	if cfg.LockWatcher == nil {
		return trace.BadParameter("LinuxServiceConfig is missing LockWatcher")
	}
	if cfg.Emitter == nil {
		return trace.BadParameter("LinuxServiceConfig is missing Emitter")
	}
	if cfg.TLS == nil {
		return trace.BadParameter("LinuxServiceConfig is missing TLS")
	}
	if cfg.AccessPoint == nil {
		return trace.BadParameter("LinuxServiceConfig is missing AccessPoint")
	}
	if cfg.AuthClient == nil {
		return trace.BadParameter("LinuxServiceConfig is missing AuthClient")
	}
	if cfg.InventoryHandle == nil {
		return trace.BadParameter("LinuxServiceConfig is missing InventoryHandle")
	}
	if cfg.ConnLimiter == nil {
		return trace.BadParameter("LinuxServiceConfig is missing ConnLimiter")
	}
	if cfg.ConnectedProxyGetter == nil {
		return trace.BadParameter("LinuxServiceConfig is missing ConnectedProxyGetter")
	}

	cfg.Logger = cmp.Or(cfg.Logger, slog.With(teleport.ComponentKey, teleport.ComponentLinuxDesktop))
	cfg.Clock = cmp.Or(cfg.Clock, clockwork.NewRealClock())

	return nil
}

// NewLinuxService initializes a new LinuxService.
//
// To start serving connections, call Serve.
// When done serving connections, call Close.
func NewLinuxService(cfg LinuxServiceConfig) (*LinuxService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName, err := cfg.AccessPoint.GetClusterName(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err, "fetching cluster name")
	}

	ctx, close := context.WithCancel(context.Background())
	s := &LinuxService{
		cfg: cfg,
		middleware: &authz.Middleware{
			ClusterName:   clusterName.GetClusterName(),
			AcceptedUsage: []string{teleport.UsageLinuxDesktopOnly},
		},
		clusterName: clusterName.GetClusterName(),
		closeCtx:    ctx,
		close:       close,
		auditCache:  newSharedDirectoryAuditCache(),
	}

	if err := s.startServiceHeartbeat(); err != nil {
		return nil, trace.Wrap(err)
	}

	return s, nil
}

func (s *LinuxService) startServiceHeartbeat() error {
	heartbeat, err := srv.NewLinuxDesktopHeartbeat(srv.HeartbeatV2Config[*linuxdesktopv1pb.LinuxDesktop]{
		InventoryHandle: s.cfg.InventoryHandle,
		Announcer:       s.cfg.AccessPoint,
		GetResource: func(ctx context.Context) (*linuxdesktopv1pb.LinuxDesktop, error) {
			desktop, err := linuxdesktopv1.NewLinuxDesktop(s.cfg.Heartbeat.HostUUID, &linuxdesktopv1pb.LinuxDesktopSpec{
				Addr:     s.cfg.Heartbeat.PublicAddr,
				Hostname: s.cfg.Hostname,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			desktop.Metadata.Expires = timestamppb.New(s.cfg.Clock.Now().Add(5 * time.Minute))
			desktop.Metadata.Labels = s.cfg.Labels
			return desktop, nil
		},
		AnnounceInterval: apidefaults.ServerAnnounceTTL/2 + utils.RandomDuration(apidefaults.ServerAnnounceTTL/10),
		PollInterval:     defaults.HeartbeatCheckPeriod,
		OnHeartbeat:      s.cfg.Heartbeat.OnHeartbeat,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		if err := heartbeat.Run(); err != nil {
			s.cfg.Logger.ErrorContext(s.closeCtx, "service heartbeat ended", "error", err)
		}
	}()
	return nil
}

// Close instructs the server to stop accepting new connections and abort all
// established ones. Close does not wait for the connections to be finished.
func (s *LinuxService) Close() error {
	s.close()

	return nil
}

// Serve starts serving TLS connections for plainLis. plainLis should be a TCP
// listener and Serve will handle TLS internally.
func (s *LinuxService) Serve(plainLis net.Listener) error {
	lis := tls.NewListener(plainLis, s.cfg.TLS)
	defer lis.Close()
	for {
		select {
		case <-s.closeCtx.Done():
			return trace.Wrap(s.closeCtx.Err())
		default:
		}
		conn, err := lis.Accept()
		if err != nil {
			if utils.IsOKNetworkError(err) || trace.IsConnectionProblem(err) {
				return nil
			}
			return trace.Wrap(err)
		}
		proxyConn, ok := conn.(*tls.Conn)
		if !ok {
			return trace.ConnectionProblem(nil, "Got %T from TLS listener, expected *tls.Conn", conn)
		}

		go s.handleConnection(proxyConn)
	}
}

func (s *LinuxService) handleConnection(conn net.Conn) {
	tdpConn := tdp.NewConn(conn, tdp.DecoderAdapter(tdpb.DecodePermissive))
	defer tdpConn.Close()
	tdpConn.WriteMessage(&tdpb.Alert{
		Message:  "Connection closed gracefully",
		Severity: tdpbv1.AlertSeverity_ALERT_SEVERITY_INFO,
	})
}
