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
	"fmt"
	"log/slog"
	"maps"
	"net"
	"os"
	"os/user"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	linuxdesktopv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/linuxdesktop/linuxdesktopv1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/recorder"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/desktop/rdp/rdpclient"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
	"github.com/gravitational/teleport/lib/srv/desktop/x11"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/session/reexec"
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

	heartbeat *srv.HeartbeatV2
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
	ChildLogConfig       *srv.ChildLogConfig
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
	if cfg.ChildLogConfig == nil {
		return trace.BadParameter("LinuxServiceConfig is missing ChildLogConfig")
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

	xgb.Logger = slog.NewLogLogger(cfg.Logger.Handler(), logutils.TraceLevel)

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
				ProxyIds: s.cfg.ConnectedProxyGetter.GetProxyIDs(),
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

	s.heartbeat = heartbeat

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

	if s.heartbeat != nil {
		s.heartbeat.Close()
	}

	return nil
}

type tracker struct {
	mu           sync.Mutex
	lastActivity time.Time
	clock        clockwork.Clock
}

func (t *tracker) GetClientLastActive() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastActivity
}

func (t *tracker) UpdateClientActivity() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastActivity = t.clock.Now()
}

// linuxSession encapsulates all state for a single Linux desktop session.
type linuxSession struct {
	service   *LinuxService
	log       *slog.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	tdpConn   *tdp.Conn
	sessionID session.ID
	authCtx   *authz.Context
	identity  tlsca.Identity
	desktop   *linuxdesktopv1pb.LinuxDesktop
	netConfig types.ClusterNetworkingConfig
	authPref  types.AuthPreference

	backend       *x11.Backend
	screenSize    atomic.Pointer[xproto.Rectangle]
	xsessions     map[string]string
	recorder      libevents.SessionPreparerRecorder
	recordSession bool
	audit         *desktopSessionAuditor
	track         tracker
	cmd           *reexec.CommandExecutor

	sessionStarted bool
	username       string
}

func (sess *linuxSession) sendTDPError(message string) {
	if err := sess.tdpConn.WriteMessage(&tdpb.Alert{Message: message, Severity: tdpbv1.AlertSeverity_ALERT_SEVERITY_ERROR}); err != nil {
		sess.log.ErrorContext(sess.ctx, "Failed to send TDPB error message", "error", err, "message", message)
	}
}

func (sess *linuxSession) handleClipboardData(data []byte) {
	sess.tdpConn.WriteMessage(&tdpb.ClipboardData{
		Data: data,
	})
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
			conn.Close()
			return trace.ConnectionProblem(nil, "Got %T from TLS listener, expected *tls.Conn", conn)
		}

		go s.handleConnection(proxyConn)
	}
}

func (s *LinuxService) handleConnection(proxyConn *tls.Conn) {
	log := s.cfg.Logger

	ctx, cancel := context.WithCancel(s.closeCtx)
	defer cancel()

	tdpConn := tdp.NewConn(proxyConn, tdp.DecoderAdapter(tdpb.DecodePermissive))
	defer tdpConn.Close()

	// Inline function to enforce that we are centralizing TDP Error sending in this function.
	sendTDPError := func(message string) {
		if err := tdpConn.WriteMessage(&tdpb.Alert{Message: message, Severity: tdpbv1.AlertSeverity_ALERT_SEVERITY_ERROR}); err != nil {
			log.ErrorContext(ctx, "Failed to send TDPB error message", "error", err, "message", message)
		}
	}

	netConfig, err := s.cfg.AccessPoint.GetClusterNetworkingConfig(ctx)
	if err != nil {
		log.ErrorContext(ctx, "Failed to get cluster networking config", "error", err)
		sendTDPError("Failed to get cluster networking config")
		return
	}

	// Check connection limits.
	remoteAddr, _, err := net.SplitHostPort(proxyConn.RemoteAddr().String())
	if err != nil {
		log.ErrorContext(ctx, "Could not parse client IP", "addr", proxyConn.RemoteAddr().String(), "error", err)
		sendTDPError("Internal error.")
		return
	}
	log = log.With("client_ip", remoteAddr)

	sessionID := session.NewID()
	log = log.With("session_id", sessionID)

	if err := s.cfg.ConnLimiter.AcquireConnection(remoteAddr); err != nil {
		log.WarnContext(ctx, "Connection limit exceeded, rejecting connection")
		sendTDPError("Connection limit exceeded.")
		return
	}
	defer s.cfg.ConnLimiter.ReleaseConnection(remoteAddr)

	// Authenticate the client.
	ctx, err = s.middleware.WrapContextWithUser(ctx, proxyConn)
	if err != nil {
		log.WarnContext(ctx, "mTLS authentication failed for incoming connection", "error", err)
		sendTDPError("Connection authentication failed.")
		return
	}
	log.DebugContext(ctx, "Authenticated Linux desktop connection")

	authCtx, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		log.WarnContext(ctx, "authorization failed for Linux desktop connection", "error", err)
		sendTDPError("Connection authorization failed.")
		return
	}

	authPref, err := s.cfg.AccessPoint.GetAuthPreference(ctx)
	if err != nil {
		log.ErrorContext(ctx, "failed to get auth preference", "error", err)
		sendTDPError("Failed to get auth preference.")
		return
	}

	desktop, err := s.cfg.AuthClient.GetLinuxDesktop(ctx, s.cfg.Heartbeat.HostUUID)
	if err != nil {
		log.ErrorContext(ctx, "failed to get linux desktop", "error", err)
		sendTDPError("Failed to get Linux desktop.")
		return
	}

	sess := &linuxSession{
		service:   s,
		log:       log,
		ctx:       ctx,
		cancel:    cancel,
		tdpConn:   tdpConn,
		sessionID: sessionID,
		authCtx:   authCtx,
		identity:  authCtx.Identity.GetIdentity(),
		desktop:   desktop,
		netConfig: netConfig,
		authPref:  authPref,
	}

	if err := sess.run(); err != nil {
		log.ErrorContext(ctx, "desktop session ended with error", "error", err)
	}
}

func (sess *linuxSession) run() error {
	s := sess.service

	xsessions, err := x11.GetAvailableXSessions(nil, nil)
	if err != nil {
		sess.sendTDPError("Couldn't get available xsessions.")
		return trace.Wrap(err)
	}
	sess.xsessions = xsessions

	var recConfig types.SessionRecordingConfig
	if sess.authCtx.Checker.RecordDesktopSession() {
		recConfig, err = s.cfg.AccessPoint.GetSessionRecordingConfig(sess.ctx)
		if err != nil {
			sess.sendTDPError("Couldn't get session recording config")
			return trace.Wrap(err)
		}
		sess.recordSession = recConfig.GetMode() != types.RecordOff
	} else {
		recConfig = types.DefaultSessionRecordingConfig()
		recConfig.SetMode(types.RecordOff)
		sess.log.InfoContext(sess.ctx, "desktop session will not be recorded, user's roles disable recording")
	}
	rec, err := s.newSessionRecorder(recConfig, string(sess.sessionID))
	if err != nil {
		sess.sendTDPError("Couldn't create session recorder")
		return trace.Wrap(err)
	}
	sess.recorder = rec

	// Closing the stream writer is needed to flush all recorded data
	// and trigger the upload. Do it in a goroutine since depending on
	// the session size it can take a while, and we don't want to block
	// the client.
	defer func() {
		go func() {
			if err := sess.recorder.Close(context.Background()); err != nil {
				sess.log.ErrorContext(context.Background(), "closing stream writer for desktop", "session_id", sess.sessionID.String(), "error", err)
			}
		}()
	}()

	sess.audit = s.newSessionAuditor(string(sess.sessionID), &sess.identity, "", sess.desktop)

	delay := timer()
	sess.tdpConn.OnSend = makeTDPSendHandler(sess.ctx, s, s.cfg.Clock, s.cfg.Logger, sess.recorder, delay, sess.tdpConn, sess.audit)
	sess.tdpConn.OnRecv = makeTDPReceiveHandler(sess.ctx, s, s.cfg.Clock, s.cfg.Logger, sess.recorder, delay, sess.tdpConn, sess.audit)

	sess.track = tracker{clock: s.cfg.Clock}
	sess.track.UpdateClientActivity()

	if err := sess.startMonitor(); err != nil {
		startEvent := sess.audit.makeLinuxSessionStart(err)
		s.record(sess.ctx, sess.recorder, startEvent)
		s.emit(sess.ctx, startEvent)
		sess.sendTDPError("Couldn't start connection monitor.")
		return trace.Wrap(err)
	}

	defer func() {
		if sess.sessionStarted {
			endEvent := sess.audit.makeLinuxSessionEnd(sess.recordSession)
			s.record(context.Background(), sess.recorder, endEvent)
			s.emit(context.Background(), endEvent)
		}
		sess.audit.teardown(context.Background())
	}()

	sess.backend, err = x11.NewBackend(sess.ctx, x11.Config{
		ClipboardDataReceiver: sess.handleClipboardData,
		Logger:                sess.log,
	})
	if err != nil {
		sess.sendTDPError("Couldn't create backend.")
		return trace.Wrap(err)
	}
	defer sess.backend.Close()

	defer func() {
		if sess.cmd != nil {
			sess.cmd.Close()
		}
	}()

	return sess.messageLoop()
}

func (sess *linuxSession) startMonitor() error {
	s := sess.service
	monitorCfg := srv.MonitorConfig{
		Context:               sess.ctx,
		Conn:                  sess.tdpConn,
		Clock:                 s.cfg.Clock,
		ClientIdleTimeout:     sess.authCtx.Checker.AdjustClientIdleTimeout(sess.netConfig.GetClientIdleTimeout()),
		DisconnectExpiredCert: sess.authCtx.GetDisconnectCertExpiry(sess.authPref),
		Logger:                s.cfg.Logger,
		Emitter:               s.cfg.Emitter,
		EmitterContext:        s.closeCtx,
		LockWatcher:           s.cfg.LockWatcher,
		LockingMode:           sess.authCtx.Checker.LockingMode(sess.authPref.GetLockingMode()),
		LockTargets:           append(services.LockTargetsFromTLSIdentity(sess.identity), types.LockTarget{LinuxDesktop: sess.desktop.GetMetadata().GetName()}),
		Tracker:               &sess.track,
		TeleportUser:          sess.identity.Username,
		UserOriginClusterName: sess.identity.OriginClusterName,
		ServerID:              s.cfg.Heartbeat.HostUUID,
		IdleTimeoutMessage:    sess.netConfig.GetClientIdleTimeoutMessage(),
		MessageWriter:         &monitorErrorSender{tdpConn: sess.tdpConn},
	}
	return srv.StartMonitor(monitorCfg)
}

func (sess *linuxSession) messageLoop() error {
	for {
		msg, err := sess.tdpConn.ReadMessage()
		if utils.IsOKNetworkError(err) || trace.IsConnectionProblem(err) {
			return nil
		}
		if err != nil {
			return trace.ConnectionProblem(err, "failed to read message")
		}

		// If the message was due to user input, then we update client activity
		// in order to refresh the client_idle_timeout checks.
		//
		// Note: we count some of the directory sharing messages as client activity
		// because we don't want a session to be closed due to inactivity during a large
		// file transfer.
		switch msg.(type) {
		case *tdpb.KeyboardButton, *tdpb.MouseMove, *tdpb.MouseButton, *tdpb.MouseWheel,
			*tdpb.SharedDirectoryAnnounce, *tdpb.SharedDirectoryResponse:

			sess.track.UpdateClientActivity()
		}

		switch m := msg.(type) {
		case *tdpb.ClientHello:
			if err := sess.handleClientHello(m); err != nil {
				return trace.Wrap(err)
			}
		case *tdpb.SessionSelection:
			if err := sess.handleSessionSelection(m); err != nil {
				return trace.Wrap(err)
			}
		case *tdpb.MouseMove:
			if err := sess.backend.SendMouseMove(int16(m.X), int16(m.Y)); err != nil {
				sess.log.ErrorContext(sess.ctx, "failed to send mouse move", "error", err)
				sess.sendTDPError("Couldn't send mouse move.")
				return trace.Wrap(err)
			}
		case *tdpb.MouseButton:
			if err := sess.backend.SendMouseButton(byte(m.Button-1), m.Pressed); err != nil {
				sess.log.ErrorContext(sess.ctx, "failed to send mouse button", "error", err)
				sess.sendTDPError("Couldn't send mouse button.")
				return trace.Wrap(err)
			}
		case *tdpb.MouseWheel:
			if err := sess.backend.SendMouseWheel(int(m.Delta)); err != nil {
				sess.log.ErrorContext(sess.ctx, "failed to send mouse wheel", "error", err)
				sess.sendTDPError("Couldn't send mouse wheel event.")
				return trace.Wrap(err)
			}
		case *tdpb.KeyboardButton:
			if err := sess.backend.SendKeyboardButton(byte(m.KeyCode), m.Pressed); err != nil {
				sess.log.ErrorContext(sess.ctx, "failed to send keyboard button", "error", err)
				sess.sendTDPError("Couldn't send keyboard button.")
				return trace.Wrap(err)
			}
		case *tdpb.Ping:
			if err := sess.tdpConn.WriteMessage(m); err != nil {
				sess.log.ErrorContext(sess.ctx, "failed to send ping message", "error", err)
				return trace.Wrap(err)
			}
		case *tdpb.ClipboardData:
			if err := sess.backend.SetClipboardData(m.Data); err != nil {
				sess.log.ErrorContext(sess.ctx, "failed to set clipboard data", "error", err)
				sess.sendTDPError("Couldn't set clipboard data.")
				return trace.Wrap(err)
			}
		case *tdpb.ClientScreenSpec:
			if err := sess.handleClientScreenSpec(m); err != nil {
				return trace.Wrap(err)
			}
		default:
			sess.log.InfoContext(sess.service.closeCtx, "Ignoring message", "message", fmt.Sprintf("%T", msg))
		}
	}
}

func (sess *linuxSession) handleClientHello(m *tdpb.ClientHello) error {
	s := sess.service
	sess.username = m.Username
	sess.audit.targetUser = m.Username
	sess.log = sess.log.With("username", m.Username)

	state := sess.authCtx.GetAccessState(sess.authPref)
	if err := sess.authCtx.Checker.CheckAccess(
		types.Resource153ToResourceWithLabels(sess.desktop),
		state,
		services.NewLinuxDesktopLoginMatcher(m.Username)); err != nil {
		startEvent := sess.audit.makeLinuxSessionStart(err)
		s.record(sess.ctx, sess.recorder, startEvent)
		s.emit(sess.ctx, startEvent)
		sess.log.WarnContext(sess.ctx, "authorization failed for Linux desktop connection", "error", err)
		sess.sendTDPError("Connection authorization failed.")
		return trace.Wrap(err)
	}

	if err := sess.changeAuthorityFileOwnership(m); err != nil {
		return trace.Wrap(err)
	}

	if err := s.trackSession(sess.ctx, &sess.identity, m.Username, string(sess.sessionID)); err != nil {
		sess.log.ErrorContext(sess.ctx, "failed to track session", "error", err)
		sess.sendTDPError("Failed to track session.")
		return trace.Wrap(err)
	}

	startEvent := sess.audit.makeLinuxSessionStart(nil)
	s.record(context.Background(), sess.recorder, startEvent)
	s.emit(context.Background(), startEvent)

	sess.sessionStarted = true

	if m.ScreenSpec == nil {
		sess.log.ErrorContext(sess.ctx, "missing screen spec")
		sess.sendTDPError("Missing screen specification.")
		return trace.BadParameter("missing screen spec")
	}

	if m.ScreenSpec.Width > types.MaxRDPScreenWidth || m.ScreenSpec.Height > types.MaxRDPScreenHeight {
		sess.log.ErrorContext(sess.ctx, "invalid screen size", "width", m.ScreenSpec.Width, "height", m.ScreenSpec.Height)
		sess.sendTDPError(fmt.Sprintf("Screen is too large. Maximum is %dx%d", types.MaxRDPScreenWidth, types.MaxRDPScreenHeight))
		return trace.BadParameter("invalid screen size")
	}

	width := uint16(m.ScreenSpec.Width)
	height := uint16(m.ScreenSpec.Height)
	sess.screenSize.Store(&xproto.Rectangle{
		Width:  width,
		Height: height,
	})
	if err := sess.backend.Resize(width, height); err != nil {
		sess.log.ErrorContext(sess.ctx, "failed to resize screen", "error", err)
		sess.sendTDPError("Couldn't resize backend.")
		return trace.Wrap(err)
	}
	if err := sess.tdpConn.WriteMessage(&tdpb.ServerHello{
		ActivationSpec: &tdpbv1.ConnectionActivated{
			IoChannelId:   0,
			UserChannelId: 0,
			ScreenWidth:   m.ScreenSpec.Width,
			ScreenHeight:  m.ScreenSpec.Height,
		},
		ClipboardEnabled: true,
		Sessions:         slices.Collect(maps.Keys(sess.xsessions)),
	}); err != nil {
		sess.log.WarnContext(sess.ctx, "failed to send server hello", "error", err)
		return trace.Wrap(err)
	}
	go sess.processScreenChanges()
	return nil
}

func (sess *linuxSession) changeAuthorityFileOwnership(m *tdpb.ClientHello) error {
	currentUser, err := user.Current()
	if err != nil {
		sess.log.ErrorContext(sess.ctx, "failed to get current user", "error", err)
		sess.sendTDPError("Internal server error")
		return trace.Wrap(err)
	}
	targetUser, err := user.Lookup(m.Username)
	if err != nil {
		sess.log.WarnContext(sess.ctx, "couldn't lookup user", "error", err)
		sess.sendTDPError(fmt.Sprintf("Couldn't find user: %s", m.Username))
		return trace.Wrap(err)
	}
	if currentUser.Uid != targetUser.Uid {
		uid, err := strconv.Atoi(targetUser.Uid)
		if err != nil {
			sess.log.ErrorContext(sess.ctx, "couldn't convert uid to int", "error", err)
			sess.sendTDPError("Internal server error")
			return trace.Wrap(err)
		}
		gid, err := strconv.Atoi(targetUser.Gid)
		if err != nil {
			sess.log.ErrorContext(sess.ctx, "couldn't convert gid to int", "error", err)
			sess.sendTDPError("Internal server error")
			return trace.Wrap(err)
		}

		if err := os.Chown(sess.backend.AuthorityFile, uid, gid); err != nil {
			sess.log.ErrorContext(sess.ctx, "couldn't change Xauthority file ownership", "error", err)
			sess.sendTDPError("Internal server error")
			return trace.Wrap(err)
		}
	}
	return nil
}

func (sess *linuxSession) handleSessionSelection(m *tdpb.SessionSelection) error {
	xsession, ok := sess.xsessions[m.Name]
	if !ok {
		sess.log.WarnContext(sess.ctx, "failed to get xsession", "name", m.Name)
		sess.sendTDPError(fmt.Sprintf("Couldn't find xsession %s.", m.Name))
		return trace.NotFound("xsession %s not found", m.Name)
	}
	cmd, err := x11.StartTeleportExecXSession(sess.ctx, &x11.XSessionConfig{
		Logger:         sess.log,
		Command:        xsession,
		Username:       sess.identity.Username,
		Login:          sess.username,
		ChildLogConfig: sess.service.cfg.ChildLogConfig,
		Display:        sess.backend.Display,
		AuthorityFile:  sess.backend.AuthorityFile,
		RemoteAddr:     reexec.NetAddrFromAddr(sess.tdpConn.RemoteAddr()),
	})
	if err != nil {
		sess.log.ErrorContext(sess.ctx, "failed to start Xsession", "error", err)
		sess.sendTDPError("Couldn't start Xsession.")
		return trace.Wrap(err)
	}
	sess.cmd = cmd
	go func() {
		err := cmd.Wait()
		if sess.ctx.Err() != nil {
			return
		}
		if err == nil {
			sess.sendTDPError("Xsession was terminated")
		} else {
			sess.log.Error("Xsession was terminated", "error", err)
			sess.sendTDPError("Xsession was terminated with error")
		}
	}()
	return nil
}

func (sess *linuxSession) handleClientScreenSpec(m *tdpb.ClientScreenSpec) error {
	if m.Width > types.MaxRDPScreenWidth || m.Height > types.MaxRDPScreenHeight {
		sess.log.ErrorContext(sess.ctx, "invalid screen size", "width", m.Width, "height", m.Height)
		sess.sendTDPError(fmt.Sprintf("Screen is too large. Maximum is %dx%d", types.MaxRDPScreenWidth, types.MaxRDPScreenHeight))
		return trace.BadParameter("invalid screen size")
	}
	sess.screenSize.Store(&xproto.Rectangle{
		Width:  uint16(m.Width),
		Height: uint16(m.Height),
	})
	if err := sess.backend.Resize(uint16(m.Width), uint16(m.Height)); err != nil {
		sess.log.ErrorContext(sess.ctx, "failed to resize screen", "error", err)
		sess.sendTDPError("Couldn't resize backend.")
		return trace.Wrap(err)
	}
	if err := sess.tdpConn.WriteMessage(&tdpb.ServerHello{
		ActivationSpec: &tdpbv1.ConnectionActivated{
			ScreenWidth:  m.Width,
			ScreenHeight: m.Height,
		},
		ClipboardEnabled: true,
	}); err != nil {
		sess.log.ErrorContext(sess.ctx, "failed to send server-hello message", "error", err)
		return trace.Wrap(err)
	}
	return nil
}

func (sess *linuxSession) processScreenChanges() {
	var lastScreenSize *xproto.Rectangle
	i := 0
	os.Mkdir("/tmp/img", 0755)
	for {
		start := time.Now()
		size := 0
		changes, err := sess.backend.GetChanges()
		if err != nil && !utils.IsOKNetworkError(err) {
			sess.log.ErrorContext(sess.ctx, "failed to get changes from backend", "error", err)
			return
		}
		currentScreenSize := sess.screenSize.Load()
		if lastScreenSize != currentScreenSize && currentScreenSize != nil {
			lastScreenSize = currentScreenSize
			changes = []xproto.Rectangle{*currentScreenSize}
		}
		for _, change := range changes {
			changeSize := int(change.Width) * int(change.Height)
			if changeSize == 0 {
				continue
			}
			size += int(change.Width) * int(change.Height)
			img, err := sess.backend.GetImage(change)
			if err != nil {
				sess.log.ErrorContext(sess.ctx, "failed to get image from backend", "error", err)
				return
			}

			os.WriteFile(fmt.Sprintf("/tmp/img/img%d_%dx%d", i, change.Width, change.Height), img, 0644)
			i++
			frames, err := rdpclient.EncodeQOIZ(img, uint16(change.X), uint16(change.Y), change.Width, change.Height)
			if err != nil {
				sess.log.ErrorContext(sess.ctx, "failed to encode FastPathPDUs", "error", err)
				return
			}
			framesSize := 0
			for _, frame := range frames {
				framesSize += len(frame.Pdu)
				if err := sess.tdpConn.WriteMessage(frame); err != nil {
					sess.log.ErrorContext(sess.ctx, "failed to send frame", "error", err)
					return
				}
			}
			sess.log.DebugContext(sess.ctx, "frames", "size", size, "frames", framesSize)
		}
		delta := time.Since(start)
		if size > 0 {
			sess.log.Log(sess.ctx, logutils.TraceLevel, "Frame encoded", "time", delta, "size", size)
		}
		select {
		case <-sess.ctx.Done():
			return
			// We want to keep 25fps, so we want to wait ~40ms between frames
		case <-sess.service.cfg.Clock.After(40*time.Millisecond - delta):
		}
	}
}

func (s *LinuxService) newSessionRecorder(recConfig types.SessionRecordingConfig, sessionID string) (libevents.SessionPreparerRecorder, error) {
	return recorder.New(recorder.Config{
		SessionID:    session.ID(sessionID),
		ServerID:     s.cfg.Heartbeat.HostUUID,
		Namespace:    apidefaults.Namespace,
		Clock:        s.cfg.Clock,
		ClusterName:  s.clusterName,
		RecordingCfg: recConfig,
		SyncStreamer: s.cfg.AuthClient,
		DataDir:      s.cfg.DataDir,
		Component:    teleport.Component(teleport.ComponentSession, teleport.ComponentLinuxDesktop),
		// Session stream is using server context, not session context,
		// to make sure that session is uploaded even after it is closed
		Context: s.closeCtx,
	})
}

// trackSession creates a session tracker for the given sessionID and
// attributes, and starts a goroutine to continually extend the tracker
// expiration while the session is active. Once the given ctx is closed,
// the tracker will be marked as terminated.
func (s *LinuxService) trackSession(ctx context.Context, id *tlsca.Identity, linuxUser string, sessionID string) error {
	trackerSpec := types.SessionTrackerSpecV1{
		SessionID:   sessionID,
		Kind:        string(types.LinuxDesktopSessionKind),
		State:       types.SessionState_SessionStateRunning,
		Hostname:    s.cfg.Hostname,
		Address:     s.cfg.Heartbeat.HostUUID,
		ClusterName: s.clusterName,
		Login:       linuxUser,
		Participants: []types.Participant{{
			User:    id.Username,
			Cluster: id.OriginClusterName,
		}},
		HostUser: id.Username,
		Created:  s.cfg.Clock.Now(),
		HostID:   s.cfg.Heartbeat.HostUUID,
	}

	s.cfg.Logger.DebugContext(ctx, "Creating session tracker", "session_id", sessionID)
	tracker, err := srv.NewSessionTracker(ctx, trackerSpec, s.cfg.AuthClient)
	if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		if err := tracker.UpdateExpirationLoop(ctx, s.cfg.Clock); err != nil {
			s.cfg.Logger.WarnContext(ctx, "Failed to update session tracker expiration", "session_id", sessionID, "error", err)
		}
	}()

	go func() {
		<-ctx.Done()
		if err := tracker.Close(s.closeCtx); err != nil {
			s.cfg.Logger.DebugContext(s.closeCtx, "Failed to close session tracker", "session_id", sessionID)
		}
	}()

	return nil
}
