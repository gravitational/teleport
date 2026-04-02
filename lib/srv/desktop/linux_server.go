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
	"os/user"
	"slices"
	"strconv"
	"sync/atomic"
	"time"

	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/teleport/api/types"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/recorder"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
	"github.com/gravitational/teleport/lib/srv/desktop/x11"
	"github.com/gravitational/teleport/lib/tlsca"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/trace"
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
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

func (s *LinuxService) handleConnection(proxyConn *tls.Conn) {
	log := s.cfg.Logger

	ctx, cancel := context.WithCancel(s.closeCtx)
	defer cancel()

	tdpConn := tdp.NewConn(proxyConn, tdp.DecoderAdapter(tdpb.DecodePermissive))
	defer tdpConn.Close()

	// Inline function to enforce that we are centralizing TDP Error sending in this function.
	sendTDPError := func(message string) {
		if err := tdpConn.WriteMessage(&tdpb.Alert{Message: message, Severity: tdpbv1.AlertSeverity_ALERT_SEVERITY_ERROR}); err != nil {
			log.ErrorContext(context.Background(), "Failed to send TDPB error message", "error", err, "message", message)
		}
	}

	// Check connection limits.
	remoteAddr, _, err := net.SplitHostPort(proxyConn.RemoteAddr().String())
	if err != nil {
		log.ErrorContext(context.Background(), "Could not parse client IP", "addr", proxyConn.RemoteAddr().String(), "error", err)
		sendTDPError("Internal error.")
		return
	}
	log = log.With("client_ip", remoteAddr)

	sessionID := session.NewID()
	log = log.With("session_id", sessionID)

	if err := s.cfg.ConnLimiter.AcquireConnection(remoteAddr); err != nil {
		log.WarnContext(context.Background(), "Connection limit exceeded, rejecting connection")
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

	backend, err := x11.NewBackend(ctx, x11.Config{
		ClipboardDataReceiver: func(data []byte) {
			tdpConn.WriteMessage(&tdpb.ClipboardData{
				Data: data,
			})
		},
		Logger: s.cfg.Logger,
	})
	if err != nil {
		log.WarnContext(ctx, "backend creation failed", "error", err)
		sendTDPError("Couldn't create backend.")
		return
	}
	defer backend.Close()

	var screenSize atomic.Pointer[xproto.Rectangle]

	xsessions, err := x11.GetAvailableXSessions(nil, nil)
	if err != nil {
		log.ErrorContext(ctx, "failed to get available xsessions", "error", err)
		sendTDPError("Couldn't get available xsessions.")
		return
	}

	var username string

	// in order for the session to be recorded, the cluster's session recording mode must
	// not be "off" and the user's roles must enable recording
	var recConfig types.SessionRecordingConfig
	var recordSession bool
	if !authCtx.Checker.RecordDesktopSession() {
		recConfig = types.DefaultSessionRecordingConfig()
		recConfig.SetMode(types.RecordOff)
		log.InfoContext(ctx, "desktop session will not be recorded, user's roles disable recording")
	} else {
		recConfig, err = s.cfg.AccessPoint.GetSessionRecordingConfig(ctx)
		if err != nil {
			log.ErrorContext(ctx, "failed to get session recording config", "error", err)
			sendTDPError("Couldn't get session recording config")
			return
		}
		recordSession = recConfig.GetMode() != types.RecordOff
	}
	recorder, err := s.newSessionRecorder(recConfig, string(sessionID))
	if err != nil {
		log.ErrorContext(ctx, "failed to create session recorder", "error", err)
		sendTDPError("Couldn't create session recorder")
		return
	}

	// Closing the stream writer is needed to flush all recorded data
	// and trigger the upload. Do it in a goroutine since depending on
	// the session size it can take a while, and we don't want to block
	// the client.
	defer func() {
		go func() {
			if err := recorder.Close(context.Background()); err != nil {
				log.ErrorContext(context.Background(), "closing stream writer for desktop", "session_id", sessionID.String())
			}
		}()
	}()

	identity := authCtx.Identity.GetIdentity()
	audit := s.newSessionAuditor(string(sessionID), &identity, "", desktop)

	delay := timer()
	tdpConn.OnSend = makeTDPSendHandler(ctx, s, s.cfg.Clock, s.cfg.Logger, recorder, delay, tdpConn, audit)
	tdpConn.OnRecv = makeTDPReceiveHandler(ctx, s, s.cfg.Clock, s.cfg.Logger, recorder, delay, tdpConn, audit)

	sessionStarted := false

	defer func() {
		if sessionStarted {
			endEvent := audit.makeLinuxSessionEnd(recordSession)
			s.record(context.Background(), recorder, endEvent)
			s.emit(context.Background(), endEvent)
		}
		audit.teardown(context.Background())
	}()

	for {
		msg, err := tdpConn.ReadMessage()
		if utils.IsOKNetworkError(err) || trace.IsConnectionProblem(err) {
			return
		}
		if err != nil {
			log.ErrorContext(ctx, "got error reading message", "error", err)
			return
		}
		switch m := msg.(type) {
		case *tdpb.ClientHello:
			username = m.Username
			audit.targetUser = username
			log = log.With("username", username)

			state := authCtx.GetAccessState(authPref)
			if err := authCtx.Checker.CheckAccess(
				types.Resource153ToResourceWithLabels(desktop),
				state,
				services.NewLinuxDesktopLoginMatcher(username)); err != nil {
				startEvent := audit.makeLinuxSessionStart(err)
				s.record(ctx, recorder, startEvent)
				log.WarnContext(ctx, "authorization failed for Linux desktop connection", "error", err)
				sendTDPError("Connection authorization failed.")
				return
			}
			currentUser, err := user.Current()
			if err != nil {
				log.ErrorContext(ctx, "failed to get current user", "error", err)
				sendTDPError("Internal server error")
			}
			targetUser, err := user.Lookup(username)
			if err != nil {
				log.WarnContext(ctx, "couldn't lookup user", "error", err)
				sendTDPError(fmt.Sprintf("Couldn't find user: %s", username))
				return
			}
			if currentUser.Uid != targetUser.Uid {
				uid, err := strconv.Atoi(targetUser.Uid)
				if err != nil {
					log.ErrorContext(ctx, "couldn't convert uid to int", "error", err)
					sendTDPError("Internal server error")
					return
				}
				gid, err := strconv.Atoi(targetUser.Gid)
				if err != nil {
					log.ErrorContext(ctx, "couldn't convert gid to int", "error", err)
					sendTDPError("Internal server error")
					return
				}

				if err := backend.AuthorityFile.Chown(uid, gid); err != nil {
					log.ErrorContext(ctx, "couldn't change Xauthority file ownership", "error", err)
					sendTDPError("Internal server error")
					return
				}
			}

			if err := s.trackSession(ctx, &identity, username, string(sessionID)); err != nil {
				log.ErrorContext(ctx, "failed to track session", "error", err)
				sendTDPError("Failed to track session.")
				return
			}

			startEvent := audit.makeLinuxSessionStart(nil)
			s.record(context.Background(), recorder, startEvent)
			s.emit(context.Background(), startEvent)

			sessionStarted = true

			width := uint16(m.ScreenSpec.Width)
			height := uint16(m.ScreenSpec.Height)
			screenSize.Store(&xproto.Rectangle{
				Width:  width,
				Height: height,
			})
			if err := backend.Resize(width, height); err != nil {
				log.ErrorContext(ctx, "failed to resize screen", "error", err)
				sendTDPError("Couldn't resize backend.")
				return
			}
			if err := tdpConn.WriteMessage(&tdpb.ServerHello{
				ActivationSpec: &tdpbv1.ConnectionActivated{
					IoChannelId:   0,
					UserChannelId: 0,
					ScreenWidth:   m.ScreenSpec.Width,
					ScreenHeight:  m.ScreenSpec.Height,
				},
				ClipboardEnabled: true,
				Sessions:         slices.Collect(maps.Keys(xsessions)),
			}); err != nil {
				log.WarnContext(ctx, "failed to send server hello", "error", err)
				return
			}
			go s.processScreenChanges(backend, log, ctx, &screenSize, tdpConn)
		case *tdpb.SessionSelection:
			xsession, ok := xsessions[m.Name]
			if !ok {
				log.WarnContext(ctx, "failed to get xsession", "error", err)
				sendTDPError(fmt.Sprintf("Couldn't find xsession %s.", m.Name))
				return
			}
			cmd, err := x11.StartTeleportExecXSession(ctx, &x11.XSessionConfig{
				Logger:         log,
				Command:        xsession,
				Username:       identity.Username,
				Login:          username,
				ChildLogConfig: s.cfg.ChildLogConfig,
				Display:        backend.Display,
				AuthorityFile:  backend.AuthorityFile.Name(),
			})
			go func() {
				err := cmd.Wait()
				if ctx.Err() != nil {
					return
				}
				if err == nil {
					sendTDPError("Xsession was terminated")
				} else {
					sendTDPError("Xsession was terminated with error")
				}
			}()
			if err != nil {
				log.ErrorContext(ctx, "failed to start Xsession", "error", err)
				sendTDPError("Couldn't start Xsession.")
			}
		case *tdpb.MouseMove:
			if err := backend.SendMouseMove(int16(m.X), int16(m.Y)); err != nil {
				log.ErrorContext(ctx, "failed to send mouse move", "error", err)
				sendTDPError("Couldn't send mouse move.")
				return
			}
		case *tdpb.MouseButton:
			if err := backend.SendMouseButton(byte(m.Button-1), m.Pressed); err != nil {
				log.ErrorContext(ctx, "failed to send mouse button", "error", err)
				sendTDPError("Couldn't send mouse button.")
				return
			}
		case *tdpb.MouseWheel:
			if err := backend.SendMouseWheel(int(m.Delta)); err != nil {
				log.ErrorContext(ctx, "failed to send mouse wheel", "error", err)
				sendTDPError("Couldn't send mouse wheel event.")
				return
			}
		case *tdpb.KeyboardButton:
			if err := backend.SendKeyboardButton(byte(m.KeyCode), m.Pressed); err != nil {
				log.ErrorContext(ctx, "failed to send keyboard button", "error", err)
				sendTDPError("Couldn't send keyboard button.")
				return
			}
		case *tdpb.Ping:
			if err := tdpConn.WriteMessage(m); err != nil {
				log.ErrorContext(ctx, "failed to send ping message", "error", err)
				return
			}
		case *tdpb.ClipboardData:
			if err := backend.SetClipboardData(m.Data); err != nil {
				log.ErrorContext(ctx, "failed to set clipboard data", "error", err)
				sendTDPError("Couldn't set clipboard data.")
				return
			}
		case *tdpb.ClientScreenSpec:
			screenSize.Store(&xproto.Rectangle{
				Width:  uint16(m.Width),
				Height: uint16(m.Height),
			})
			if err := backend.Resize(uint16(m.Width), uint16(m.Height)); err != nil {
				log.ErrorContext(ctx, "failed to resize screen", "error", err)
				sendTDPError("Couldn't resize backend.")
				return
			}
			if err := tdpConn.WriteMessage(&tdpb.ServerHello{
				ActivationSpec: &tdpbv1.ConnectionActivated{
					ScreenWidth:  m.Width,
					ScreenHeight: m.Height,
				},
				ClipboardEnabled: true,
			}); err != nil {
				log.ErrorContext(ctx, "failed to send server-hello message", "error", err)
				return
			}
		default:
			log.InfoContext(s.closeCtx, "Ignoring message", "message", fmt.Sprintf("%T", msg))
		}
	}
}

func (s *LinuxService) processScreenChanges(backend *x11.Backend, log *slog.Logger, ctx context.Context, screenSize *atomic.Pointer[xproto.Rectangle], tdpConn *tdp.Conn) {
	var lastScreenSize *xproto.Rectangle
	for {
		start := time.Now()
		qoiz := time.Duration(0)
		writing := time.Duration(0)
		image := time.Duration(0)
		size := 0
		changes, err := backend.GetChanges()
		if err != nil {
			log.ErrorContext(ctx, "failed to get changes from backend", "error", err)
			return
		}
		currentScreenSize := screenSize.Load()
		if lastScreenSize != currentScreenSize && currentScreenSize != nil {
			lastScreenSize = currentScreenSize
			changes = []xproto.Rectangle{*currentScreenSize}
		}
		for _, change := range changes {
			size += int(change.Width) * int(change.Height)
			bi := time.Now()
			img, err := backend.GetImage(change)
			if err != nil {
				log.ErrorContext(ctx, "failed to get image from backend", "error", err)
				return
			}
			image += time.Since(bi)
			fs := time.Now()
			frames, err := rdpclient.EncodeQOIZ(img, uint16(change.X), uint16(change.Y), change.Width, change.Height)
			if err != nil {
				log.ErrorContext(ctx, "failed to encode FastPathPDUs", "error", err)
				return
			}
			qoiz += time.Since(fs)
			for _, frame := range frames {
				bi = time.Now()
				if err := tdpConn.WriteMessage(frame); err != nil {
					log.ErrorContext(ctx, "failed to send frame", "error", err)
					return
				}
				writing += time.Since(bi)
			}
		}
		delta := time.Since(start)
		log.Log(ctx, logutils.TraceLevel, "Frame encoding", "delta", delta, "qoiz", qoiz, "writing", writing, "image", image, "size", size)
		select {
		case <-ctx.Done():
			return
		case <-s.cfg.Clock.After(40*time.Millisecond - delta):
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
		Kind:        string(types.WindowsDesktopSessionKind),
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
