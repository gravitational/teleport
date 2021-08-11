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

// Package desktop implements Desktop Access services, like
// windows_desktop_access.
package desktop

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
)

// WindowsService implements the RDP-based Windows desktop access service.
//
// This service accepts mTLS connections from the proxy, establishes RDP
// connections to Windows hosts and translates RDP into Teleport's desktop
// protocol.
type WindowsService struct {
	cfg        WindowsServiceConfig
	middleware *auth.Middleware

	closeCtx context.Context
	close    func()
}

// WindowsServiceConfig contains all necessary configuration values for a
// WindowsService.
type WindowsServiceConfig struct {
	// Log is the logging sink for the service.
	Log logrus.FieldLogger
	// Clock provides current time.
	Clock clockwork.Clock
	// TLS is the TLS server configuration.
	TLS *tls.Config
	// AccessPoint is the Auth API client (with caching).
	AccessPoint auth.AccessPoint
	// ConnLimiter limits the number of active connections per client IP.
	ConnLimiter *limiter.ConnectionsLimiter
	// Heartbeat contains configuration for service heartbeats.
	Heartbeat HeartbeatConfig
}

// HeartbeatConfig contains the configuration for service heartbeats.
type HeartbeatConfig struct {
	// HostUUID is the UUID of the host that this service runs on. Used as the
	// name of the created API object.
	HostUUID string
	// PublicAddr is the public address of this service.
	PublicAddr string
	// OnHeartbeat is called after each heartbeat attempt.
	OnHeartbeat func(error)
	// StaticHosts is an optional list of static Windows hosts to register.
	StaticHosts []utils.NetAddr
}

func (cfg WindowsServiceConfig) CheckAndSetDefaults() error {
	if cfg.Log == nil {
		cfg.Log = logrus.New().WithField(trace.Component, teleport.ComponentWindowsDesktop)
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.TLS == nil {
		return trace.BadParameter("WindowsServiceConfig is missing TLS")
	}
	if cfg.AccessPoint == nil {
		return trace.BadParameter("WindowsServiceConfig is missing AccessPoint")
	}
	if cfg.ConnLimiter == nil {
		return trace.BadParameter("WindowsServiceConfig is missing ConnLimiter")
	}
	if err := cfg.Heartbeat.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (cfg HeartbeatConfig) CheckAndSetDefaults() error {
	if cfg.HostUUID == "" {
		return trace.BadParameter("HeartbeatConfig is missing HostUUID")
	}
	if cfg.PublicAddr == "" {
		return trace.BadParameter("HeartbeatConfig is missing PublicAddr")
	}
	if cfg.OnHeartbeat == nil {
		return trace.BadParameter("HeartbeatConfig is missing OnHeartbeat")
	}
	return nil
}

// NewWindowsService initializes a new WindowsService.
//
// To start serving connections, call Serve.
// When done serving connections, call Close.
func NewWindowsService(cfg WindowsServiceConfig) (*WindowsService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, close := context.WithCancel(context.Background())
	s := &WindowsService{
		cfg: cfg,
		middleware: &auth.Middleware{
			AccessPoint:   cfg.AccessPoint,
			AcceptedUsage: []string{teleport.UsageWindowsDesktopOnly},
		},
		closeCtx: ctx,
		close:    close,
	}

	// TODO(awly): session recording.
	// TODO(awly): user locking.

	if err := s.startServiceHeartbeat(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.startStaticHostHeartbeats(); err != nil {
		return nil, trace.Wrap(err)
	}

	return s, nil
}

func (s *WindowsService) startServiceHeartbeat() error {
	heartbeat, err := srv.NewHeartbeat(srv.HeartbeatConfig{
		Context:         s.closeCtx,
		Component:       teleport.ComponentWindowsDesktop,
		Mode:            srv.HeartbeatModeWindowsDesktopService,
		Announcer:       s.cfg.AccessPoint,
		GetServerInfo:   s.getServiceHeartbeatInfo,
		KeepAlivePeriod: apidefaults.ServerKeepAliveTTL,
		AnnouncePeriod:  apidefaults.ServerAnnounceTTL/2 + utils.RandomDuration(apidefaults.ServerAnnounceTTL/10),
		CheckPeriod:     defaults.HeartbeatCheckPeriod,
		ServerTTL:       apidefaults.ServerAnnounceTTL,
		OnHeartbeat:     s.cfg.Heartbeat.OnHeartbeat,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		if err := heartbeat.Run(); err != nil {
			s.cfg.Log.WithError(err).Error("Heartbeat ended with error")
		}
	}()
	return nil
}

// startStaticHostHeartbeats spawns heartbeat routines for all static hosts in
// this service. We use heartbeats instead of registering once at startup to
// support expiration.
//
// When a WindowsService with a list of static hosts disappears, those hosts
// should eventually get cleaned up. But they should exist as long as the
// service itself is running.
func (s *WindowsService) startStaticHostHeartbeats() error {
	for _, host := range s.cfg.Heartbeat.StaticHosts {
		heartbeat, err := srv.NewHeartbeat(srv.HeartbeatConfig{
			Context:         s.closeCtx,
			Component:       teleport.ComponentWindowsDesktop,
			Mode:            srv.HeartbeatModeWindowsDesktop,
			Announcer:       s.cfg.AccessPoint,
			GetServerInfo:   s.getHostHeartbeatInfo(host),
			KeepAlivePeriod: apidefaults.ServerKeepAliveTTL,
			AnnouncePeriod:  apidefaults.ServerAnnounceTTL/2 + utils.RandomDuration(apidefaults.ServerAnnounceTTL/10),
			CheckPeriod:     defaults.HeartbeatCheckPeriod,
			ServerTTL:       apidefaults.ServerAnnounceTTL,
			OnHeartbeat:     s.cfg.Heartbeat.OnHeartbeat,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		go func() {
			if err := heartbeat.Run(); err != nil {
				s.cfg.Log.WithError(err).Error("Heartbeat ended with error")
			}
		}()
	}
	return nil
}

// Close instructs the server to stop accepting new connections and abort all
// established ones. Close does not wait for the connections to be finished.
func (s *WindowsService) Close() error {
	s.close()
	return nil
}

// Serve starts serving TLS connections fro plainLis. plainLis should be a TCP
// listener and Serve will handle TLS internally.
func (s *WindowsService) Serve(plainLis net.Listener) error {
	lis := tls.NewListener(plainLis, s.cfg.TLS)
	defer lis.Close()
	for {
		select {
		case <-s.closeCtx.Done():
			return trace.Wrap(s.closeCtx.Err())
		default:
		}

		con, err := lis.Accept()
		if err != nil {
			if utils.IsOKNetworkError(err) || trace.IsConnectionProblem(err) {
				return nil
			}
			return trace.Wrap(err)
		}

		go s.handleConnection(con)
	}
}

func (s *WindowsService) handleConnection(con net.Conn) {
	defer con.Close()

	// Check connection limits.
	remoteAddr, _, err := net.SplitHostPort(con.RemoteAddr().String())
	if err != nil {
		s.cfg.Log.WithError(err).Errorf("Could not parse client IP from %q", con.RemoteAddr().String())
		return
	}
	if err := s.cfg.ConnLimiter.AcquireConnection(remoteAddr); err != nil {
		s.cfg.Log.WithError(err).Warningf("Connection limit exceeded, rejecting connection from %q", remoteAddr)
		return
	}
	defer s.cfg.ConnLimiter.ReleaseConnection(remoteAddr)

	// Authenticate the client.
	tlsCon, ok := con.(*tls.Conn)
	if !ok {
		s.cfg.Log.Errorf("Got %T from TLS listener, expected *tls.Conn", con)
		return
	}
	ctx, err := s.middleware.WrapContextWithUser(s.closeCtx, tlsCon)
	if err != nil {
		s.cfg.Log.WithError(err).Warningf("mTLS authentication failed for inbound connection from %q", remoteAddr)
		return
	}
	s.cfg.Log.Debugf("Authenticated Windows desktop connection from %q", remoteAddr)

	// TODO(awly): connect to the target Windows host over RDP and proxy
	// messages.
	s.cfg.Log.Error("Windows desktop support not implemented beyond authentication.")
	_ = ctx
}

func (s *WindowsService) getServiceHeartbeatInfo() (types.Resource, error) {
	srv, err := types.NewWindowsDesktopServiceV3(types.Metadata{
		Name: s.cfg.Heartbeat.HostUUID,
	}, types.WindowsDesktopServiceSpecV3{
		Addr:            s.cfg.Heartbeat.PublicAddr,
		TeleportVersion: teleport.Version,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	srv.SetExpiry(s.cfg.Clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL))
	return srv, nil
}

func (s *WindowsService) getHostHeartbeatInfo(netAddr utils.NetAddr) func() (types.Resource, error) {
	return func() (types.Resource, error) {
		addr := netAddr.String()
		name, err := s.nameForStaticHost(addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		desktop, err := types.NewWindowsDesktopV3(types.Metadata{
			Name: name,
		}, types.WindowsDesktopSpecV3{
			Addr: addr,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		desktop.SetExpiry(s.cfg.Clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL))
		return desktop, nil
	}
}

// nameForStaticHost attempts to find the UUID of an existing Windows desktop
// with the same address. If no matching address is found, a new UUID is
// generated.
//
// The list of WindowsDesktop objects should be read from the local cache. It
// should be reasonably fast to do this scan on every heartbeat. However, with
// a very large number of desktops in the cluster, this may use up a lot of CPU
// time.
//
// TODO(awly): think of an alternative way to not duplicate desktop objects
// coming from different windows_desktop_services.
func (s *WindowsService) nameForStaticHost(addr string) (string, error) {
	desktops, err := s.cfg.AccessPoint.GetWindowsDesktops(s.closeCtx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	for _, d := range desktops {
		if d.GetAddr() == addr {
			return d.GetName(), nil
		}
	}
	return uuid.New().String(), nil
}
