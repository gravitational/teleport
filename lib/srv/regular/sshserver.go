/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

// Package regular implements SSH server that supports multiplexing
// tunneling, SSH connections proxying and only supports Key based auth
package regular

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"os"
	"os/user"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/observability/tracing"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/proxy"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/ingress"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/uds"
)

var log = logrus.WithFields(logrus.Fields{
	teleport.ComponentKey: teleport.ComponentNode,
})

// Server implements SSH server that uses configuration backend and
// certificate-based authentication
type Server struct {
	sync.Mutex

	*logrus.Entry

	namespace string
	addr      utils.NetAddr
	hostname  string

	srv         *sshutils.Server
	shell       string
	getRotation services.RotationGetter
	authService srv.AccessPoint
	reg         *srv.SessionRegistry
	limiter     *limiter.Limiter

	inventoryHandle inventory.DownstreamHandle

	// labels are static labels.
	labels map[string]string

	// dynamicLabels are the result of command execution.
	dynamicLabels *labels.Dynamic

	// cloudLabels are the labels imported from a cloud provider.
	cloudLabels labels.Importer

	proxyMode        bool
	proxyTun         reversetunnelclient.Tunnel
	proxyAccessPoint authclient.ReadProxyAccessPoint
	peerAddr         string

	advertiseAddr   *utils.NetAddr
	proxyPublicAddr utils.NetAddr
	publicAddrs     []utils.NetAddr

	// server UUID gets generated once on the first start and never changes
	// usually stored in a file inside the data dir
	uuid string

	// this gets set to true for unit testing
	isTestStub bool

	// cancel cancels all operations
	cancel context.CancelFunc
	// ctx is broadcasting context closure
	ctx context.Context

	// StreamEmitter points to the auth service and emits audit events
	events.StreamEmitter

	// clock is a system clock
	clock clockwork.Clock

	// permitUserEnvironment controls if this server will read ~/.tsh/environment
	// before creating a new session.
	permitUserEnvironment bool

	// ciphers is a list of ciphers that the server supports. If omitted,
	// the defaults will be used.
	ciphers []string

	// kexAlgorithms is a list of key exchange (KEX) algorithms that the
	// server supports. If omitted, the defaults will be used.
	kexAlgorithms []string

	// macAlgorithms is a list of message authentication codes (MAC) that
	// the server supports. If omitted the defaults will be used.
	macAlgorithms []string

	// authHandlers are common authorization and authentication related handlers.
	authHandlers *srv.AuthHandlers

	// termHandlers are common terminal related handlers.
	termHandlers *srv.TermHandlers

	// pamConfig holds configuration for PAM.
	pamConfig *servicecfg.PAMConfig

	// dataDir is a server local data directory
	dataDir string

	// heartbeat sends updates about this server
	// back to auth server
	heartbeat srv.HeartbeatI

	// useTunnel is used to inform other components that this server is
	// requesting connections to it come over a reverse tunnel.
	useTunnel bool

	// fips means Teleport started in a FedRAMP/FIPS 140-2 compliant
	// configuration.
	fips bool

	// ebpf is the service used for enhanced session recording.
	ebpf bpf.BPF

	// onHeartbeat is a callback for heartbeat status.
	onHeartbeat func(error)

	// utmpPath is the path to the user accounting database.
	utmpPath string

	// wtmpPath is the path to the user accounting s.Logger.
	wtmpPath string

	// btmpPath is the path to the user accounting failed login log.
	btmpPath string

	// allowTCPForwarding indicates whether the ssh server is allowed to offer
	// TCP port forwarding.
	allowTCPForwarding bool

	// x11 is the X11 forwarding configuration for the server
	x11 *x11.ServerConfig

	// allowFileCopying indicates whether the ssh server is allowed to handle
	// remote file operations via SCP or SFTP.
	allowFileCopying bool

	// lockWatcher is the server's lock watcher.
	lockWatcher *services.LockWatcher

	// connectedProxyGetter gets the proxies teleport is connected to.
	connectedProxyGetter *reversetunnel.ConnectedProxyGetter

	// nodeWatcher is the server's node watcher.
	nodeWatcher *services.NodeWatcher

	// createHostUser configures whether a host should allow host user
	// creation
	createHostUser bool

	storage *local.PresenceService

	// users is used to start the automatic user deletion loop
	users srv.HostUsers

	// sudoers is used to manage sudoers file provisioning
	sudoers srv.HostSudoers

	// tracerProvider is used to create tracers capable
	// of starting spans.
	tracerProvider oteltrace.TracerProvider

	// router used by subsystem requests to connect to nodes
	// and clusters
	router *proxy.Router

	// sessionController is used to restrict new sessions
	// based on locks and cluster preferences
	sessionController *srv.SessionController

	// ingressReporter reports new and active connections.
	ingressReporter *ingress.Reporter
	// ingressService the service name passed to the ingress reporter.
	ingressService string

	// proxySigner is used to generate signed PROXYv2 header so we can securely propagate client IP
	proxySigner PROXYHeaderSigner
	// caGetter is used to get host CA of the cluster to verify signed PROXY headers
	caGetter CertAuthorityGetter

	// remoteForwardingMap holds the remote port forwarding listeners that need
	// to be closed when forwarding finishes, keyed by listen addr.
	remoteForwardingMap utils.SyncMap[string, io.Closer]
}

// TargetMetadata returns metadata about the server.
func (s *Server) TargetMetadata() apievents.ServerMetadata {
	return apievents.ServerMetadata{
		ServerVersion:   teleport.Version,
		ServerNamespace: s.GetNamespace(),
		ServerID:        s.ID(),
		ServerAddr:      s.Addr(),
		ServerLabels:    s.getAllLabels(),
		ServerHostname:  s.hostname,
	}
}

// GetClock returns server clock implementation
func (s *Server) GetClock() clockwork.Clock {
	return s.clock
}

// GetDataDir returns server data dir
func (s *Server) GetDataDir() string {
	return s.dataDir
}

func (s *Server) GetNamespace() string {
	return s.namespace
}

func (s *Server) GetAccessPoint() srv.AccessPoint {
	return s.authService
}

// GetUserAccountingPaths returns the optional override of the utmp, wtmp, and btmp paths.
func (s *Server) GetUserAccountingPaths() (string, string, string) {
	return s.utmpPath, s.wtmpPath, s.btmpPath
}

// GetPAM returns the PAM configuration for this server.
func (s *Server) GetPAM() (*servicecfg.PAMConfig, error) {
	return s.pamConfig, nil
}

// UseTunnel used to determine if this node has connected to this cluster
// using reverse tunnel.
func (s *Server) UseTunnel() bool {
	return s.useTunnel
}

// GetBPF returns the BPF service used by enhanced session recording.
func (s *Server) GetBPF() bpf.BPF {
	return s.ebpf
}

// GetLockWatcher gets the server's lock watcher.
func (s *Server) GetLockWatcher() *services.LockWatcher {
	return s.lockWatcher
}

// GetCreateHostUser determines whether users should be created on the
// host automatically
func (s *Server) GetCreateHostUser() bool {
	return s.createHostUser
}

// GetHostUsers returns the HostUsers instance being used to manage
// host user provisioning
func (s *Server) GetHostUsers() srv.HostUsers {
	return s.users
}

// GetHostSudoers returns the HostSudoers instance being used to manage
// sudoers file provisioning
func (s *Server) GetHostSudoers() srv.HostSudoers {
	if s.sudoers == nil {
		return &srv.HostSudoersNotImplemented{}
	}
	return s.sudoers
}

// ServerOption is a functional option passed to the server
type ServerOption func(s *Server) error

func (s *Server) close() {
	s.cancel()
	s.reg.Close()
	if s.heartbeat != nil {
		if err := s.heartbeat.Close(); err != nil {
			s.Warningf("Failed to close heartbeat: %v", err)
		}
	}
	if s.dynamicLabels != nil {
		s.dynamicLabels.Close()
	}

	if s.users != nil {
		s.users.Shutdown()
	}
}

// Close closes listening socket and stops accepting connections
func (s *Server) Close() error {
	s.close()
	// Close the server first so we don't accept any new forwarding connections
	// after we've closed them all.
	errors := []error{s.srv.Close()}
	s.remoteForwardingMap.Range(func(_ string, closer io.Closer) bool {
		if closer != nil {
			if err := closer.Close(); err != nil {
				errors = append(errors, err)
			}
		}
		return true
	})
	return trace.NewAggregate(errors...)
}

// Shutdown performs graceful shutdown
func (s *Server) Shutdown(ctx context.Context) error {
	// Stop heart beating immediately to prevent active connections
	// from making the server appear alive and well.
	if s.heartbeat != nil {
		if err := s.heartbeat.Close(); err != nil {
			s.Warningf("Failed to close heartbeat: %v", err)
		}
	}

	// wait until connections drain off
	err := s.srv.Shutdown(ctx)
	return trace.NewAggregate(err, s.Close())
}

// Start starts server
func (s *Server) Start() error {
	// Only call srv.Start() which listens on a socket if the server did not
	// request connections to it arrive over a reverse tunnel.
	if !s.useTunnel {
		if err := s.srv.Start(); err != nil {
			return trace.Wrap(err)
		}
	}
	// Heartbeat should start only after s.srv.Start.
	// If the server is configured to listen on port 0 (such as in tests),
	// it'll only populate its actual listening address during s.srv.Start.
	// Heartbeat uses this address to announce. Avoid announcing an empty
	// address on first heartbeat.
	s.startPeriodicOperations()
	return nil
}

// Serve servers service on started listener
func (s *Server) Serve(l net.Listener) error {
	s.startPeriodicOperations()
	return trace.Wrap(s.srv.Serve(l))
}

func (s *Server) startPeriodicOperations() {
	// If the server has dynamic labels defined, start a loop that will
	// asynchronously keep them updated.
	if s.dynamicLabels != nil {
		go s.dynamicLabels.Start()
	}
	// If the server allows host user provisioning, this will start an
	// automatic cleanup process for any temporary leftover users.
	if s.GetCreateHostUser() && s.users != nil {
		go s.users.UserCleanup()
	}
	if s.cloudLabels != nil {
		s.cloudLabels.Start(s.Context())
	}
	if s.heartbeat != nil {
		go s.heartbeat.Run()
	}
}

// Wait waits until server stops
func (s *Server) Wait() {
	s.srv.Wait(context.TODO())
}

// HandleConnection is called after a connection has been accepted and starts
// to perform the SSH handshake immediately.
func (s *Server) HandleConnection(conn net.Conn) {
	s.srv.HandleConnection(conn)
}

// SetUserAccountingPaths is a functional server option to override the user accounting database and log path.
func SetUserAccountingPaths(utmpPath, wtmpPath, btmpPath string) ServerOption {
	return func(s *Server) error {
		s.utmpPath = utmpPath
		s.wtmpPath = wtmpPath
		s.btmpPath = btmpPath
		return nil
	}
}

// SetClock is a functional server option to override the internal
// clock
func SetClock(clock clockwork.Clock) ServerOption {
	return func(s *Server) error {
		s.clock = clock
		return nil
	}
}

// SetRotationGetter sets rotation state getter
func SetRotationGetter(getter services.RotationGetter) ServerOption {
	return func(s *Server) error {
		s.getRotation = getter
		return nil
	}
}

// SetShell sets default shell that will be executed for interactive
// sessions
func SetShell(shell string) ServerOption {
	return func(s *Server) error {
		s.shell = shell
		return nil
	}
}

// SetProxyMode starts this server in SSH proxying mode
func SetProxyMode(peerAddr string, tsrv reversetunnelclient.Tunnel, ap authclient.ReadProxyAccessPoint, router *proxy.Router) ServerOption {
	return func(s *Server) error {
		// always set proxy mode to true,
		// because in some tests reverse tunnel is disabled,
		// but proxy is still used without it.
		s.proxyMode = true
		s.proxyTun = tsrv
		s.proxyAccessPoint = ap
		s.peerAddr = peerAddr
		s.router = router
		return nil
	}
}

// SetIngressReporter sets the reporter for reporting new and active connections.
func SetIngressReporter(service string, r *ingress.Reporter) ServerOption {
	return func(s *Server) error {
		s.ingressReporter = r
		s.ingressService = service
		return nil
	}
}

// SetLabels sets dynamic and static labels that server will report to the
// auth servers.
func SetLabels(staticLabels map[string]string, cmdLabels services.CommandLabels, cloudLabels labels.Importer) ServerOption {
	return func(s *Server) error {
		var err error

		// clone and validate labels and cmdLabels.  in theory,
		// only cmdLabels should experience concurrent writes,
		// but this operation is only run once on startup
		// so a little defensive cloning is harmless.
		labelsClone := make(map[string]string, len(staticLabels))
		for name, label := range staticLabels {
			if !types.IsValidLabelKey(name) {
				return trace.BadParameter("invalid label key: %q", name)
			}
			labelsClone[name] = label
		}
		s.labels = labelsClone

		if len(cmdLabels) > 0 {
			// Create dynamic labels.
			s.dynamicLabels, err = labels.NewDynamic(s.ctx, &labels.DynamicConfig{
				Labels: cmdLabels,
			})
			if err != nil {
				return trace.Wrap(err)
			}
		}

		s.cloudLabels = cloudLabels
		return nil
	}
}

// SetLimiter sets rate and connection limiter for this server
func SetLimiter(limiter *limiter.Limiter) ServerOption {
	return func(s *Server) error {
		s.limiter = limiter
		return nil
	}
}

// SetEmitter assigns an audit event emitter for this server
func SetEmitter(emitter events.StreamEmitter) ServerOption {
	return func(s *Server) error {
		s.StreamEmitter = emitter
		return nil
	}
}

// SetUUID sets server unique ID
func SetUUID(uuid string) ServerOption {
	return func(s *Server) error {
		s.uuid = uuid
		return nil
	}
}

func SetNamespace(namespace string) ServerOption {
	return func(s *Server) error {
		s.namespace = namespace
		return nil
	}
}

// SetPermitUserEnvironment allows you to set the value of permitUserEnvironment.
func SetPermitUserEnvironment(permitUserEnvironment bool) ServerOption {
	return func(s *Server) error {
		s.permitUserEnvironment = permitUserEnvironment
		return nil
	}
}

func SetCiphers(ciphers []string) ServerOption {
	return func(s *Server) error {
		s.ciphers = ciphers
		return nil
	}
}

func SetKEXAlgorithms(kexAlgorithms []string) ServerOption {
	return func(s *Server) error {
		s.kexAlgorithms = kexAlgorithms
		return nil
	}
}

func SetMACAlgorithms(macAlgorithms []string) ServerOption {
	return func(s *Server) error {
		s.macAlgorithms = macAlgorithms
		return nil
	}
}

func SetPAMConfig(pamConfig *servicecfg.PAMConfig) ServerOption {
	return func(s *Server) error {
		s.pamConfig = pamConfig
		return nil
	}
}

func SetUseTunnel(useTunnel bool) ServerOption {
	return func(s *Server) error {
		s.useTunnel = useTunnel
		return nil
	}
}

func SetFIPS(fips bool) ServerOption {
	return func(s *Server) error {
		s.fips = fips
		return nil
	}
}

func SetBPF(ebpf bpf.BPF) ServerOption {
	return func(s *Server) error {
		s.ebpf = ebpf
		return nil
	}
}

func SetOnHeartbeat(fn func(error)) ServerOption {
	return func(s *Server) error {
		s.onHeartbeat = fn
		return nil
	}
}

// SetCreateHostUser configures host user creation on a server
func SetCreateHostUser(createUser bool) ServerOption {
	return func(s *Server) error {
		s.createHostUser = createUser && runtime.GOOS == constants.LinuxOS
		return nil
	}
}

// SetStoragePresenceService configures host user creation on a server
func SetStoragePresenceService(service *local.PresenceService) ServerOption {
	return func(s *Server) error {
		s.storage = service
		return nil
	}
}

// SetAllowTCPForwarding sets the TCP port forwarding mode that this server is
// allowed to offer. The default value is SSHPortForwardingModeAll, i.e. port
// forwarding is allowed.
func SetAllowTCPForwarding(allow bool) ServerOption {
	return func(s *Server) error {
		s.allowTCPForwarding = allow
		return nil
	}
}

// SetLockWatcher sets the server's lock watcher.
func SetLockWatcher(lockWatcher *services.LockWatcher) ServerOption {
	return func(s *Server) error {
		s.lockWatcher = lockWatcher
		return nil
	}
}

// SetNodeWatcher sets the server's node watcher.
func SetNodeWatcher(nodeWatcher *services.NodeWatcher) ServerOption {
	return func(s *Server) error {
		s.nodeWatcher = nodeWatcher
		return nil
	}
}

// SetX11ForwardingConfig sets the server's X11 forwarding configuration
func SetX11ForwardingConfig(xc *x11.ServerConfig) ServerOption {
	return func(s *Server) error {
		s.x11 = xc
		return nil
	}
}

// SetAllowFileCopying sets whether the server is allowed to handle
// SCP/SFTP requests.
func SetAllowFileCopying(allow bool) ServerOption {
	return func(s *Server) error {
		s.allowFileCopying = allow
		return nil
	}
}

// SetConnectedProxyGetter sets the ConnectedProxyGetter.
func SetConnectedProxyGetter(getter *reversetunnel.ConnectedProxyGetter) ServerOption {
	return func(s *Server) error {
		s.connectedProxyGetter = getter
		return nil
	}
}

// SetInventoryControlHandle sets the server's downstream inventory control
// handle.
func SetInventoryControlHandle(handle inventory.DownstreamHandle) ServerOption {
	return func(s *Server) error {
		s.inventoryHandle = handle
		return nil
	}
}

// SetTracerProvider sets the tracer provider.
func SetTracerProvider(provider oteltrace.TracerProvider) ServerOption {
	return func(s *Server) error {
		s.tracerProvider = provider
		return nil
	}
}

// SetSessionController sets the session controller.
func SetSessionController(controller *srv.SessionController) ServerOption {
	return func(s *Server) error {
		s.sessionController = controller
		return nil
	}
}

// SetPROXYSigner sets the PROXY headers signer
func SetPROXYSigner(proxySigner PROXYHeaderSigner) ServerOption {
	return func(s *Server) error {
		s.proxySigner = proxySigner
		return nil
	}
}

// SetCAGetter sets the cert authority getter
func SetCAGetter(caGetter CertAuthorityGetter) ServerOption {
	return func(s *Server) error {
		s.caGetter = caGetter
		return nil
	}
}

// SetPublicAddrs sets the server's public addresses
func SetPublicAddrs(addrs []utils.NetAddr) ServerOption {
	return func(s *Server) error {
		s.publicAddrs = addrs
		return nil
	}
}

// New returns an unstarted server
func New(
	ctx context.Context,
	addr utils.NetAddr,
	hostname string,
	getHostSigners sshutils.GetHostSignersFunc,
	authService srv.AccessPoint,
	dataDir string,
	advertiseAddr string,
	proxyPublicAddr utils.NetAddr,
	auth authclient.ClientI,
	options ...ServerOption,
) (*Server, error) {
	// read the host UUID:
	uuid, err := utils.ReadOrMakeHostUUID(dataDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	s := &Server{
		addr:               addr,
		authService:        authService,
		hostname:           hostname,
		proxyPublicAddr:    proxyPublicAddr,
		uuid:               uuid,
		cancel:             cancel,
		ctx:                ctx,
		clock:              clockwork.NewRealClock(),
		dataDir:            dataDir,
		allowTCPForwarding: true,
	}
	s.limiter, err = limiter.NewLimiter(limiter.Config{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if advertiseAddr != "" {
		s.advertiseAddr, err = utils.ParseAddr(advertiseAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	for _, o := range options {
		if err := o(s); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// TODO(klizhentas): replace function arguments with struct
	if s.StreamEmitter == nil {
		return nil, trace.BadParameter("setup valid Emitter parameter using SetEmitter")
	}

	if s.namespace == "" {
		return nil, trace.BadParameter("setup valid namespace parameter using SetNamespace")
	}

	if s.lockWatcher == nil {
		return nil, trace.BadParameter("setup valid LockWatcher parameter using SetLockWatcher")
	}

	if s.sessionController == nil {
		return nil, trace.BadParameter("setup valid SessionControl parameter using SetSessionControl")
	}

	if s.connectedProxyGetter == nil {
		s.connectedProxyGetter = reversetunnel.NewConnectedProxyGetter()
	}

	if s.tracerProvider == nil {
		s.tracerProvider = tracing.DefaultProvider()
	}

	var component string
	if s.proxyMode {
		component = teleport.ComponentProxy
	} else {
		component = teleport.ComponentNode
	}

	s.Entry = logrus.WithFields(logrus.Fields{
		teleport.ComponentKey:    component,
		teleport.ComponentFields: logrus.Fields{},
	})

	if s.GetCreateHostUser() {
		s.users = srv.NewHostUsers(ctx, s.storage, s.ID())
	}
	s.sudoers = srv.NewHostSudoers(s.ID())

	s.reg, err = srv.NewSessionRegistry(srv.SessionRegistryConfig{
		Srv:                   s,
		SessionTrackerService: auth,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// add in common auth handlers
	authHandlerConfig := srv.AuthHandlerConfig{
		Server:      s,
		Component:   component,
		AccessPoint: s.authService,
		FIPS:        s.fips,
		Emitter:     s.StreamEmitter,
		Clock:       s.clock,
	}

	s.authHandlers, err = srv.NewAuthHandlers(&authHandlerConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// common term handlers
	s.termHandlers = &srv.TermHandlers{
		SessionRegistry: s.reg,
	}

	clusterName, err := s.GetAccessPoint().GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	server, err := sshutils.NewServer(
		component,
		addr, s,
		getHostSigners,
		sshutils.AuthMethods{PublicKey: s.authHandlers.UserKeyAuth},
		sshutils.SetLimiter(s.limiter),
		sshutils.SetRequestHandler(s),
		sshutils.SetNewConnHandler(s),
		sshutils.SetCiphers(s.ciphers),
		sshutils.SetKEXAlgorithms(s.kexAlgorithms),
		sshutils.SetMACAlgorithms(s.macAlgorithms),
		sshutils.SetFIPS(s.fips),
		sshutils.SetClock(s.clock),
		sshutils.SetIngressReporter(s.ingressService, s.ingressReporter),
		sshutils.SetClusterName(clusterName.GetClusterName()),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.srv = server

	var heartbeatMode srv.HeartbeatMode
	if s.proxyMode {
		heartbeatMode = srv.HeartbeatModeProxy
	} else {
		heartbeatMode = srv.HeartbeatModeNode
	}

	var heartbeat srv.HeartbeatI
	if heartbeatMode == srv.HeartbeatModeNode && s.inventoryHandle != nil {
		s.Logger.Debug("starting control-stream based heartbeat.")
		heartbeat, err = srv.NewSSHServerHeartbeat(srv.HeartbeatV2Config[*types.ServerV2]{
			InventoryHandle: s.inventoryHandle,
			GetResource:     s.getServerInfo,
			OnHeartbeat:     s.onHeartbeat,
		})
	} else {
		s.Logger.Debug("starting legacy heartbeat.")
		heartbeat, err = srv.NewHeartbeat(srv.HeartbeatConfig{
			Mode:            heartbeatMode,
			Context:         ctx,
			Component:       component,
			Announcer:       s.authService,
			GetServerInfo:   s.getServerResource,
			KeepAlivePeriod: apidefaults.ServerKeepAliveTTL(),
			AnnouncePeriod:  apidefaults.ServerAnnounceTTL/2 + utils.RandomDuration(apidefaults.ServerAnnounceTTL/10),
			ServerTTL:       apidefaults.ServerAnnounceTTL,
			CheckPeriod:     defaults.HeartbeatCheckPeriod,
			Clock:           s.clock,
			OnHeartbeat:     s.onHeartbeat,
		})
	}
	if err != nil {
		s.srv.Close()
		return nil, trace.Wrap(err)
	}

	s.heartbeat = heartbeat

	return s, nil
}

func (s *Server) getNamespace() string {
	return types.ProcessNamespace(s.namespace)
}

func (s *Server) tunnelWithAccessChecker(ctx *srv.ServerContext) (reversetunnelclient.Tunnel, error) {
	clusterName, err := s.GetAccessPoint().GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return reversetunnelclient.NewTunnelWithRoles(s.proxyTun, clusterName.GetClusterName(), ctx.Identity.AccessChecker, s.proxyAccessPoint), nil
}

// Context returns server shutdown context
func (s *Server) Context() context.Context {
	return s.ctx
}

func (s *Server) Component() string {
	if s.proxyMode {
		return teleport.ComponentProxy
	}
	return teleport.ComponentNode
}

// Addr returns server address
func (s *Server) Addr() string {
	return s.srv.Addr()
}

// ActiveConnections returns the number of connections that are
// being served.
func (s *Server) ActiveConnections() int32 {
	return s.srv.ActiveConnections()
}

// ID returns server ID
func (s *Server) ID() string {
	return s.uuid
}

// HostUUID is the ID of the server. This value is the same as ID, it is
// different from the forwarding server.
func (s *Server) HostUUID() string {
	return s.uuid
}

// PermitUserEnvironment returns if ~/.tsh/environment will be read before a
// session is created by this server.
func (s *Server) PermitUserEnvironment() bool {
	return s.permitUserEnvironment
}

func (s *Server) setAdvertiseAddr(addr *utils.NetAddr) {
	s.Lock()
	defer s.Unlock()
	s.advertiseAddr = addr
}

func (s *Server) getAdvertiseAddr() *utils.NetAddr {
	s.Lock()
	defer s.Unlock()
	return s.advertiseAddr
}

// AdvertiseAddr returns an address this server should be publicly accessible
// as, in "ip:host" form
func (s *Server) AdvertiseAddr() string {
	// set if we have explicit --advertise-ip option
	advertiseAddr := s.getAdvertiseAddr()
	listenAddr := s.Addr()
	if advertiseAddr == nil {
		return listenAddr
	}
	_, port, _ := net.SplitHostPort(listenAddr)
	ahost, aport, err := utils.ParseAdvertiseAddr(advertiseAddr.String())
	if err != nil {
		s.Logger.Warningf("Failed to parse advertise address %q, %v, using default value %q.", advertiseAddr, err, listenAddr)
		return listenAddr
	}
	if aport == "" {
		aport = port
	}
	return net.JoinHostPort(ahost, aport)
}

func (s *Server) getRole() types.SystemRole {
	if s.proxyMode {
		return types.RoleProxy
	}
	return types.RoleNode
}

// getStaticLabels gets the labels that the server should present as static,
// which includes EC2 labels if available.
func (s *Server) getStaticLabels() map[string]string {
	labels := make(map[string]string, len(s.labels))
	if s.cloudLabels != nil {
		maps.Copy(labels, s.cloudLabels.Get())
	}
	// Let labels sent over ics override labels from instance metadata.
	if s.inventoryHandle != nil {
		maps.Copy(labels, s.inventoryHandle.GetUpstreamLabels(proto.LabelUpdateKind_SSHServerCloudLabels))
	}

	// Let static labels override any other labels.
	maps.Copy(labels, s.labels)

	return labels
}

// getDynamicLabels returns all dynamic labels. If no dynamic labels are
// defined, return an empty set.
func (s *Server) getDynamicLabels() map[string]types.CommandLabelV2 {
	if s.dynamicLabels == nil {
		return make(map[string]types.CommandLabelV2)
	}
	return types.LabelsToV2(s.dynamicLabels.Get())
}

// getAllLabels return a combination of static and dynamic labels.
func (s *Server) getAllLabels() map[string]string {
	lmap := make(map[string]string)
	for key, value := range s.getStaticLabels() {
		lmap[key] = value
	}
	for key, cmd := range s.getDynamicLabels() {
		lmap[key] = cmd.Result
	}
	return lmap
}

// GetInfo returns a services.Server that represents this server.
func (s *Server) GetInfo() types.Server {
	return s.getBasicInfo()
}

func (s *Server) getBasicInfo() *types.ServerV2 {
	// Only set the address for non-tunnel nodes.
	var addr string
	if !s.useTunnel {
		addr = s.AdvertiseAddr()
	}

	srv := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      s.ID(),
			Namespace: s.getNamespace(),
			Labels:    s.getStaticLabels(),
		},
		Spec: types.ServerSpecV2{
			CmdLabels: s.getDynamicLabels(),
			Addr:      addr,
			Hostname:  s.hostname,
			UseTunnel: s.useTunnel,
			Version:   teleport.Version,
			ProxyIDs:  s.connectedProxyGetter.GetProxyIDs(),
		},
	}
	srv.SetPublicAddrs(utils.NetAddrsToStrings(s.publicAddrs))

	return srv
}

func (s *Server) getServerInfo() *types.ServerV2 {
	server := s.getBasicInfo()
	if s.getRotation != nil {
		rotation, err := s.getRotation(s.getRole())
		if err != nil {
			if !trace.IsNotFound(err) {
				s.Logger.Warningf("Failed to get rotation state: %v", err)
			}
		} else {
			server.SetRotation(*rotation)
		}
	}

	server.SetExpiry(s.clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL))
	server.SetPeerAddr(s.peerAddr)
	return server
}

func (s *Server) getServerResource() (types.Resource, error) {
	return s.getServerInfo(), nil
}

// getDirectTCPIPForwarder sets up a connection-level subprocess that handles forwarding connections. Subsequent
// calls from the same connection context reuse the same forwarder.
func (s *Server) getDirectTCPIPForwardDialer(scx *srv.ServerContext) (sshutils.TCPIPForwardDialer, error) {
	if d, ok := scx.Parent().GetDirectTCPIPForwardDialer(); ok {
		return d, nil
	}

	proc, err := s.startForwardingSubprocess(scx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// set up a dial function that sends the address + fd as a unix datagram message. the forwarder subprocess
	// interprets all such messages in this way, and will dial the specified address and proxy all traffic
	// to the desired endpoint.
	dialer := func(addr string) (net.Conn, error) {
		local, remote, err := uds.NewSocketpair(uds.SocketTypeStream)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer remote.Close()

		remoteFD, err := remote.File()
		if err != nil {
			local.Close()
			return nil, trace.Wrap(err)
		}
		defer remoteFD.Close()

		_, _, err = proc.Conn.WriteWithFDs([]byte(addr), []*os.File{remoteFD})
		if err != nil {
			local.Close()
			return nil, trace.Wrap(err)
		}

		return local, nil
	}

	// try to register with the parent context.
	if other, ok := scx.Parent().TrySetDirectTCPIPForwardDialer(dialer); !ok {
		// another forwarder was concurrently created. this isn't actually a problem, multiple forwarders
		// being registered is harmless, but it does result in slightly higher resource utilization, so its
		// preferable to use the existing forwarder and close ours in the background.
		go proc.Close()
		return other, nil
	}

	// successfully registered this dialer, add closer to context.
	scx.Parent().AddCloser(proc)

	return dialer, nil
}

// listenTCPIP creates a new listener in the forwarding process.
func (s *Server) listenTCPIP(scx *srv.ServerContext, addr string) (*net.TCPListener, error) {
	proc, err := s.getTCPIPForwardProcess(scx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a socket to receive new connections on.
	localConn, remoteConn, err := uds.NewSocketpair(uds.SocketTypeDatagram)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer localConn.Close()
	defer remoteConn.Close()
	remoteFD, err := remoteConn.File()
	if err != nil {
		localConn.Close()
		return nil, trace.Wrap(err)
	}
	defer remoteFD.Close()
	_, _, err = proc.Conn.WriteWithFDs([]byte(addr), []*os.File{remoteFD})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The forwarding process may have chosen its own port, so we need to get the
	// new listen address.
	fileCh := make(chan *os.File)
	// Read addr in another goroutine so we can cancel it if the forwarding process
	// stops.
	go func() {
		defer close(fileCh)

		fbuf := make([]*os.File, 1)
		if _, fn, _ := localConn.ReadWithFDs(nil, fbuf); fn == 0 {
			fileCh <- nil
		}
		select {
		case fileCh <- fbuf[0]:
		case <-proc.Done:
			fbuf[0].Close()
		}
	}()

	var listenerFD *os.File
	select {
	case <-proc.Done:
		localConn.Close()
		return nil, trace.Errorf("forwarding process is closed")
	case listenerFD = <-fileCh:
	}

	if listenerFD == nil {
		return nil, trace.BadParameter("forwarding process did not return a listener")
	}
	if err := validateListenerSocket(scx, localConn.UnixConn, listenerFD); err != nil {
		return nil, trace.Wrap(err)
	}

	listener, err := net.FileListener(listenerFD)
	tcpListener, ok := listener.(*net.TCPListener)
	if !ok {
		return nil, trace.BadParameter("listener is not a TCPListener")
	}
	return tcpListener, trace.Wrap(err)
}

func controlSyscallConn(conn syscall.Conn, f func(fd uintptr) error) error {
	syscallConn, err := conn.SyscallConn()
	if err != nil {
		return trace.Wrap(err)
	}
	if cErr := syscallConn.Control(func(fd uintptr) {
		err = f(fd)
	}); cErr != nil {
		return trace.Wrap(cErr)
	}
	return trace.Wrap(err)
}

// getDirectTCPIPForwarder sets up a connection-level subprocess that handles
// remote forwarding connections. Subsequent calls from the same connection
// context reuse the same forwarder.
func (s *Server) getTCPIPForwardProcess(scx *srv.ServerContext) (*sshutils.TCPIPForwardProcess, error) {
	if proc, ok := scx.Parent().GetTCPIPForwardProcess(); ok {
		return proc, nil
	}

	proc, err := s.startForwardingSubprocess(scx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Try to register with the parent context.
	if otherProc, ok := scx.Parent().TrySetTCPIPForwardProcess(proc); !ok {
		// Another forwarder was concurrently created. this isn't actually a problem, multiple forwarders
		// being registered is harmless, but it does result in slightly higher resource utilization, so its
		// preferable to use the existing forwarder and close ours in the background.
		go proc.Close()
		return otherProc, nil
	}

	scx.Parent().AddCloser(proc)
	return proc, nil
}

// startForwardingSubprocess launches the forwarding process. It returns a
// conn to communicate with the process and a close func to close the process
// when finished.
func (s *Server) startForwardingSubprocess(scx *srv.ServerContext) (*sshutils.TCPIPForwardProcess, error) {
	// Create the socket to communicate over.
	remoteConn, localConn, err := uds.NewSocketpair(uds.SocketTypeDatagram)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer remoteConn.Close()
	remoteFD, err := remoteConn.File()
	if err != nil {
		localConn.Close()
		return nil, trace.Wrap(err)
	}
	defer remoteFD.Close()

	// Create command to re-exec Teleport which will handle forwarding. The
	// reason it's not done directly is because the PAM stack needs to be called
	// from the child process.
	cmd, err := srv.ConfigureCommand(scx, remoteFD)
	if err != nil {
		localConn.Close()
		return nil, trace.Wrap(err)
	}

	// Propagate stderr from the spawned Teleport process to log any errors.
	cmd.Stderr = os.Stderr

	// Start the child process that will be used to listen for connections.
	if err := cmd.Start(); err != nil {
		localConn.Close()
		return nil, trace.Wrap(err)
	}

	cdone := make(chan struct{})
	var explicitlyClosed atomic.Bool

	go func() {
		defer close(cdone)
		defer localConn.Close()
		// Ensure unexpected cmd failures get logged.
		if err := cmd.Wait(); err != nil && !explicitlyClosed.Load() {
			s.Logger.Warnf("Remote forwarder process exited early with unexpected error: %v", err)
		}
	}()

	processCloser := utils.CloseFunc(func() error {
		// Set flag indicating that the exit of the child is expected (changes logging behavior).
		explicitlyClosed.Store(true)
		localConn.Close()

		// We expect closing the conn to cause the child process to exit, but it's
		// best to verify.
		select {
		case <-cdone:
		case <-time.After(time.Second * 3):
			// forcibly kill the child.
			s.Logger.Warn("Forcibly terminating remote forwarder subprocess.")
			cmd.Process.Kill()
		}
		return nil
	})

	return &sshutils.TCPIPForwardProcess{
		Conn:   localConn,
		Done:   cdone,
		Closer: processCloser,
	}, nil
}

// serveAgent will build the a sock path for this user and serve an SSH agent on unix socket.
func (s *Server) serveAgent(ctx *srv.ServerContext) error {
	// gather information about user and process. this will be used to set the
	// socket path and permissions
	systemUser, err := user.Lookup(ctx.Identity.Login)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	pid := os.Getpid()

	socketDir := "teleport"
	socketName := fmt.Sprintf("teleport-%v.socket", pid)

	// start an agent server on a unix socket.  each incoming connection
	// will result in a separate agent request.
	agentServer := teleagent.NewServer(ctx.Parent().StartAgentChannel)
	err = agentServer.ListenUnixSocket(socketDir, socketName, systemUser)
	if err != nil {
		return trace.Wrap(err)
	}

	ctx.Parent().SetEnv(teleport.SSHAuthSock, agentServer.Path)
	ctx.Parent().SetEnv(teleport.SSHAgentPID, fmt.Sprintf("%v", pid))
	ctx.Parent().AddCloser(agentServer)
	ctx.Debugf("Starting agent server for Teleport user %v and socket %v.", ctx.Identity.TeleportUser, agentServer.Path)
	go func() {
		if err := agentServer.Serve(); err != nil {
			ctx.Errorf("agent server for user %q stopped: %v", ctx.Identity.TeleportUser, err)
		}
	}()

	return nil
}

// HandleRequest processes global out-of-band requests. Global out-of-band
// requests are processed in order (this way the originator knows which
// request we are responding to). If Teleport does not support the request
// type or an error occurs while processing that request Teleport will reply
// req.Reply(false, nil).
//
// For more details: https://tools.ietf.org/html/rfc4254.html#page-4
func (s *Server) HandleRequest(ctx context.Context, ccx *sshutils.ConnectionContext, r *ssh.Request) {
	switch r.Type {
	case teleport.KeepAliveReqType:
		s.handleKeepAlive(r)
	case teleport.ClusterDetailsReqType:
		s.handleClusterDetails(ctx, r)
	case teleport.VersionRequest:
		s.handleVersionRequest(r)
	case teleport.TerminalSizeRequest:
		if err := s.termHandlers.HandleTerminalSize(r); err != nil {
			s.Logger.WithError(err).Warn("failed to handle terminal size request")
			if r.WantReply {
				if err := r.Reply(false, nil); err != nil {
					s.Logger.Warnf("Failed to reply to %q request: %v", r.Type, err)
				}
			}
		}
	case teleport.TCPIPForwardRequest:
		if err := s.handleTCPIPForwardRequest(ctx, ccx, r); err != nil {
			s.Logger.WithError(err).Warn("failed to handle tcpip forward request")
			if err := r.Reply(false, nil); err != nil {
				s.Logger.Warnf("Failed to reply to %q request: %v", r.Type, err)
			}
		}
	case teleport.CancelTCPIPForwardRequest:
		if err := s.handleCancelTCPIPForwardRequest(ctx, ccx, r); err != nil {
			s.Logger.WithError(err).Warn("failed to handle cancel tcpip forward request")
			if err := r.Reply(false, nil); err != nil {
				s.Logger.Warnf("Failed to reply to %q request: %v", r.Type, err)
			}
		}
	default:
		if r.WantReply {
			if err := r.Reply(false, nil); err != nil {
				s.Logger.Warnf("Failed to reply to %q request: %v", r.Type, err)
			}
		}
		s.Logger.Debugf("Discarding %q global request: %+v", r.Type, r)
	}
}

// HandleNewConn is called by sshutils.Server once for each new incoming connection,
// prior to handling any channels or requests.  Currently this callback's only
// function is to apply session control restrictions.
func (s *Server) HandleNewConn(ctx context.Context, ccx *sshutils.ConnectionContext) (context.Context, error) {
	identityContext, err := s.authHandlers.CreateIdentityContext(ccx.ServerConn)
	if err != nil {
		return ctx, trace.Wrap(err)
	}

	ctx, err = s.sessionController.AcquireSessionContext(ctx, identityContext, ccx.ServerConn.LocalAddr().String(), ccx.ServerConn.RemoteAddr().String(), ccx)
	return ctx, trace.Wrap(err)
}

// HandleNewChan is called when new channel is opened
func (s *Server) HandleNewChan(ctx context.Context, ccx *sshutils.ConnectionContext, nch ssh.NewChannel) {
	identityContext, err := s.authHandlers.CreateIdentityContext(ccx.ServerConn)
	if err != nil {
		rejectChannel(nch, ssh.Prohibited, fmt.Sprintf("Unable to create identity from connection: %v", err))
		return
	}

	channelType := nch.ChannelType()
	if s.proxyMode {
		switch channelType {
		// Channels of type "direct-tcpip", for proxies, it's equivalent
		// of teleport proxy: subsystem
		case teleport.ChanDirectTCPIP:
			req, err := sshutils.ParseDirectTCPIPReq(nch.ExtraData())
			if err != nil {
				s.Logger.Errorf("Failed to parse request data: %v, err: %v.", string(nch.ExtraData()), err)
				rejectChannel(nch, ssh.UnknownChannelType, "failed to parse direct-tcpip request")
				return
			}
			ch, reqC, err := nch.Accept()
			if err != nil {
				s.Logger.Warnf("Unable to accept channel: %v.", err)
				rejectChannel(nch, ssh.ConnectionFailed, fmt.Sprintf("unable to accept channel: %v", err))
				return
			}
			go ssh.DiscardRequests(reqC)
			go s.handleProxyJump(ctx, ccx, identityContext, ch, *req)
			return
		// Channels of type "session" handle requests that are involved in running
		// commands on a server. In the case of proxy mode subsystem and agent
		// forwarding requests occur over the "session" channel.
		case teleport.ChanSession:
			ch, requests, err := nch.Accept()
			if err != nil {
				s.Logger.Warnf("Unable to accept channel: %v.", err)
				rejectChannel(nch, ssh.ConnectionFailed, fmt.Sprintf("unable to accept channel: %v", err))
				return
			}
			go s.handleSessionRequests(ctx, ccx, identityContext, ch, requests)
			return
		default:
			rejectChannel(nch, ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %v", channelType))
			return
		}
	}

	switch channelType {
	// Channels of type "session" handle requests that are involved in running
	// commands on a server, subsystem requests, and agent forwarding.
	case teleport.ChanSession:
		var decr func()
		if max := identityContext.AccessChecker.MaxSessions(); max != 0 {
			d, ok := ccx.IncrSessions(max)
			if !ok {
				// user has exceeded their max concurrent ssh sessions.
				if err := s.EmitAuditEvent(s.ctx, &apievents.SessionReject{
					Metadata: apievents.Metadata{
						Type: events.SessionRejectedEvent,
						Code: events.SessionRejectedCode,
					},
					UserMetadata: identityContext.GetUserMetadata(),
					ConnectionMetadata: apievents.ConnectionMetadata{
						Protocol:   events.EventProtocolSSH,
						LocalAddr:  ccx.ServerConn.LocalAddr().String(),
						RemoteAddr: ccx.ServerConn.RemoteAddr().String(),
					},
					ServerMetadata: apievents.ServerMetadata{
						ServerVersion:   teleport.Version,
						ServerID:        s.uuid,
						ServerNamespace: s.GetNamespace(),
					},
					Reason:  events.SessionRejectedReasonMaxSessions,
					Maximum: max,
				}); err != nil {
					s.Logger.WithError(err).Warn("Failed to emit session reject event.")
				}
				rejectChannel(nch, ssh.Prohibited, fmt.Sprintf("too many session channels for user %q (max=%d)", identityContext.TeleportUser, max))
				return
			}
			decr = d
		}
		ch, requests, err := nch.Accept()
		if err != nil {
			s.Logger.Warnf("Unable to accept channel: %v.", err)
			rejectChannel(nch, ssh.ConnectionFailed, fmt.Sprintf("unable to accept channel: %v", err))
			if decr != nil {
				decr()
			}
			return
		}
		go func() {
			s.handleSessionRequests(ctx, ccx, identityContext, ch, requests)
			if decr != nil {
				decr()
			}
		}()
	// Channels of type "direct-tcpip" handles request for port forwarding.
	case teleport.ChanDirectTCPIP:
		// On regular server in "normal" mode "direct-tcpip" channels from
		// SessionJoinPrincipal should be rejected, otherwise it's possible
		// to use the "-teleport-internal-join" user to bypass RBAC.
		if identityContext.Login == teleport.SSHSessionJoinPrincipal {
			s.Logger.Error("Connection rejected, direct-tcpip with SessionJoinPrincipal in regular node must be blocked")
			rejectChannel(
				nch, ssh.Prohibited,
				fmt.Sprintf("attempted %v channel open in join-only mode", channelType))
			return
		}
		req, err := sshutils.ParseDirectTCPIPReq(nch.ExtraData())
		if err != nil {
			s.Logger.Errorf("Failed to parse request data: %v, err: %v.", string(nch.ExtraData()), err)
			rejectChannel(nch, ssh.UnknownChannelType, "failed to parse direct-tcpip request")
			return
		}
		ch, reqC, err := nch.Accept()
		if err != nil {
			s.Logger.Warnf("Unable to accept channel: %v.", err)
			rejectChannel(nch, ssh.ConnectionFailed, fmt.Sprintf("unable to accept channel: %v", err))
			return
		}
		go ssh.DiscardRequests(reqC)
		go s.handleDirectTCPIPRequest(ctx, ccx, identityContext, ch, req)
	default:
		rejectChannel(nch, ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %v", channelType))
	}
}

// canPortForward determines if port forwarding is allowed for the current
// user/role/node combo. Returns nil if port forwarding is allowed, non-nil
// if denied.
func (s *Server) canPortForward(scx *srv.ServerContext) error {
	// Is the node configured to allow port forwarding?
	if !s.allowTCPForwarding {
		return trace.AccessDenied("node does not allow port forwarding")
	}

	// Check if the role allows port forwarding for this user.
	err := s.authHandlers.CheckPortForward(scx.DstAddr, scx)
	if err != nil {
		return err
	}

	return nil
}

// stderrWriter wraps an ssh.Channel in an implementation of io.StringWriter
// that sends anything written back the client over its stderr stream
type stderrWriter struct {
	channel ssh.Channel
}

func (w *stderrWriter) WriteString(s string) (int, error) {
	writeStderr(w.channel, s)
	return len(s), nil
}

// handleDirectTCPIPRequest handles port forwarding requests.
func (s *Server) handleDirectTCPIPRequest(ctx context.Context, ccx *sshutils.ConnectionContext, identityContext srv.IdentityContext, channel ssh.Channel, req *sshutils.DirectTCPIPReq) {
	// Create context for this channel. This context will be closed when
	// forwarding is complete.
	ctx, scx, err := srv.NewServerContext(ctx, ccx, s, identityContext)
	if err != nil {
		s.Logger.WithError(err).Error("Unable to create connection context.")
		writeStderr(channel, "Unable to create connection context.")
		if err := channel.Close(); err != nil {
			s.Logger.WithError(err).Warn("Failed to close channel.")
		}
		return
	}
	scx.IsTestStub = s.isTestStub
	scx.AddCloser(channel)
	scx.ExecType = teleport.ChanDirectTCPIP
	scx.SrcAddr = sshutils.JoinHostPort(req.Orig, req.OrigPort)
	scx.DstAddr = sshutils.JoinHostPort(req.Host, req.Port)
	scx.SetAllowFileCopying(s.allowFileCopying)
	defer scx.Close()

	channel = scx.TrackActivity(channel)

	// Bail out now if TCP port forwarding is not allowed for this node/user/role
	// combo
	if err = s.canPortForward(scx); err != nil {
		writeStderr(channel, err.Error())
		return
	}

	dialFunc, err := s.getDirectTCPIPForwardDialer(scx)
	if err != nil {
		if errors.Is(err, trace.NotFound(user.UnknownUserError(scx.Identity.Login).Error())) || errors.Is(err, trace.BadParameter("unknown user")) {
			// user does not exist for the provided login. Terminate the connection.
			s.Logger.Warnf("Forwarding data via direct-tcpip channel failed. Terminating connection because user %q does not exist", scx.Identity.Login)
			if err := ccx.ServerConn.Close(); err != nil {
				s.Logger.Warnf("Unable to terminate connection: %v", err)
			}
			return
		}

		s.Logger.WithError(err).Error("Forwarding data via direct-tcpip channel failed")
		writeStderr(channel, err.Error())
		return
	}

	scx.Debugf("Opening direct-tcpip channel from %v to %v.", scx.SrcAddr, scx.DstAddr)
	defer scx.Debugf("Closing direct-tcpip channel from %v to %v.", scx.SrcAddr, scx.DstAddr)

	conn, err := dialFunc(scx.DstAddr)
	if err != nil {
		writeStderr(channel, err.Error())
		return
	}

	if err := utils.ProxyConn(ctx, conn, channel); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
		s.Logger.Warnf("Connection problem in direct-tcpip channel: %v %T.", trace.DebugReport(err), err)
	}

	if err := s.EmitAuditEvent(s.ctx, &apievents.PortForward{
		Metadata: apievents.Metadata{
			Type: events.PortForwardEvent,
			Code: events.PortForwardCode,
		},
		UserMetadata: scx.Identity.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			LocalAddr:  scx.ServerConn.LocalAddr().String(),
			RemoteAddr: scx.ServerConn.RemoteAddr().String(),
		},
		Addr: scx.DstAddr,
		Status: apievents.Status{
			Success: true,
		},
	}); err != nil {
		s.Logger.WithError(err).Warn("Failed to emit port forward event.")
	}
}

// handleSessionRequests handles out of band session requests once the session
// channel has been created this function's loop handles all the "exec",
// "subsystem" and "shell" requests.
func (s *Server) handleSessionRequests(ctx context.Context, ccx *sshutils.ConnectionContext, identityContext srv.IdentityContext, ch ssh.Channel, in <-chan *ssh.Request) {
	netConfig, err := s.GetAccessPoint().GetClusterNetworkingConfig(ctx)
	if err != nil {
		s.Logger.Errorf("Unable to fetch cluster networking config: %v.", err)
		writeStderr(ch, "Unable to fetch cluster networking configuration.")
		return
	}

	// Create context for this channel. This context will be closed when the
	// session request is complete.
	ctx, scx, err := srv.NewServerContext(ctx, ccx, s, identityContext, func(cfg *srv.MonitorConfig) {
		cfg.IdleTimeoutMessage = netConfig.GetClientIdleTimeoutMessage()
		cfg.MessageWriter = &stderrWriter{channel: ch}
	})
	if err != nil {
		s.Logger.WithError(err).Error("Unable to create connection context.")
		writeStderr(ch, "Unable to create connection context.")
		if err := ch.Close(); err != nil {
			s.Logger.WithError(err).Warn("Failed to close channel.")
		}
		return
	}
	scx.IsTestStub = s.isTestStub
	scx.AddCloser(ch)
	scx.ExecType = teleport.ChanSession
	scx.SetAllowFileCopying(s.allowFileCopying)
	defer scx.Close()

	ch = scx.TrackActivity(ch)

	// The keep-alive loop will keep pinging the remote server and after it has
	// missed a certain number of keep-alive requests it will cancel the
	// closeContext which signals the server to shutdown.
	go srv.StartKeepAliveLoop(srv.KeepAliveParams{
		Conns: []srv.RequestSender{
			scx.ServerConn,
		},
		Interval:     netConfig.GetKeepAliveInterval(),
		MaxCount:     netConfig.GetKeepAliveCountMax(),
		CloseContext: ctx,
		CloseCancel:  scx.CancelFunc(),
	})

	for {
		// update scx with the session ID:
		if !s.proxyMode {
			err := scx.CreateOrJoinSession(s.reg)
			if err != nil {
				errorMessage := fmt.Sprintf("unable to update context: %v", err)
				scx.Errorf("Unable to update context: %v.", errorMessage)

				// write the error to channel and close it
				writeStderr(ch, errorMessage)
				_, err := ch.SendRequest("exit-status", false, ssh.Marshal(struct{ C uint32 }{C: teleport.RemoteCommandFailure}))
				if err != nil {
					scx.Errorf("Failed to send exit status %v.", errorMessage)
				}
				return
			}
		}
		select {
		case creq := <-scx.SubsystemResultCh:
			// this means that subsystem has finished executing and
			// want us to close session and the channel
			scx.Debugf("Close session request: %v.", creq.Err)
			return
		case req := <-in:
			if req == nil {
				// this will happen when the client closes/drops the connection
				scx.Debugf("Client %v disconnected.", scx.ServerConn.RemoteAddr())
				return
			}

			reqCtx := tracessh.ContextFromRequest(req)
			ctx, span := s.tracerProvider.Tracer("ssh").Start(
				oteltrace.ContextWithRemoteSpanContext(ctx, oteltrace.SpanContextFromContext(reqCtx)),
				fmt.Sprintf("ssh.Regular.SessionRequest/%s", req.Type),
				oteltrace.WithSpanKind(oteltrace.SpanKindServer),
				oteltrace.WithAttributes(
					semconv.RPCServiceKey.String("ssh.RegularServer"),
					semconv.RPCMethodKey.String("SessionRequest"),
					semconv.RPCSystemKey.String("ssh"),
				),
			)
			// some functions called inside dispatch() may handle replies to SSH channel requests internally,
			// rather than leaving the reply to be handled inside this loop. in that case, those functions must
			// set req.WantReply to false so that two replies are not sent.
			if err := s.dispatch(ctx, ch, req, scx); err != nil {
				s.replyError(ch, req, err)
				span.End()
				return
			}
			if req.WantReply {
				if err := req.Reply(true, nil); err != nil {
					s.Logger.Warnf("Failed to reply to %q request: %v", req.Type, err)
				}
			}
			span.End()
		case result := <-scx.ExecResultCh:
			scx.Debugf("Exec request (%q) complete: %v", result.Command, result.Code)

			// The exec process has finished and delivered the execution result, send
			// the result back to the client, and close the session and channel.
			_, err := ch.SendRequest("exit-status", false, ssh.Marshal(struct{ C uint32 }{C: uint32(result.Code)}))
			if err != nil {
				scx.Infof("Failed to send exit status for %v: %v", result.Command, err)
			}

			return
		case <-ctx.Done():
			s.Logger.Debugf("Closing session due to cancellation.")
			return
		}
	}
}

// dispatch receives an SSH request for a subsystem and dispatches the request to the
// appropriate subsystem implementation
func (s *Server) dispatch(ctx context.Context, ch ssh.Channel, req *ssh.Request, serverContext *srv.ServerContext) error {
	serverContext.Debugf("Handling request %v, want reply %v.", req.Type, req.WantReply)

	// If this SSH server is configured to only proxy, we do not support anything
	// other than our own custom "subsystems" and environment manipulation.
	if s.proxyMode {
		switch req.Type {
		case sshutils.SubsystemRequest:
			return s.handleSubsystem(ctx, ch, req, serverContext)
		case sshutils.EnvRequest:
			return s.handleEnv(ch, req, serverContext)
		case tracessh.EnvsRequest:
			return s.handleEnvs(ch, req, serverContext)
		case sshutils.AgentForwardRequest:
			// process agent forwarding, but we will only forward agent to proxy in
			// recording proxy mode.
			err := s.handleAgentForwardProxy(req, serverContext)
			if err != nil {
				s.Logger.Warn(err)
			}
			return nil
		case sshutils.PuTTYSimpleRequest:
			// PuTTY automatically requests a named 'simple@putty.projects.tartarus.org' channel any time it connects to a server
			// as a proxy to indicate that it's in "simple" node and won't be requesting any other channels.
			// As we don't support this request, we ignore it.
			// https://the.earth.li/~sgtatham/putty/0.76/htmldoc/AppendixG.html#sshnames-channel
			s.Logger.Debugf("%v: deliberately ignoring request for '%v' channel", s.Component(), sshutils.PuTTYSimpleRequest)
			return nil
		default:
			s.Logger.Warnf("(%v) proxy doesn't support request type '%v'", s.Component(), req.Type)
			if req.WantReply {
				if err := req.Reply(false, nil); err != nil {
					s.Logger.Errorf("sending error reply on SSH channel: %v", err)
				}
			}
			return nil
		}
	}

	// Certs with a join-only principal can only use a
	// subset of all the possible request types.
	if serverContext.JoinOnly {
		switch req.Type {
		case sshutils.PTYRequest:
			return s.termHandlers.HandlePTYReq(ctx, ch, req, serverContext)
		case sshutils.ShellRequest:
			return s.termHandlers.HandleShell(ctx, ch, req, serverContext)
		case sshutils.WindowChangeRequest:
			return s.termHandlers.HandleWinChange(ctx, ch, req, serverContext)
		case teleport.ForceTerminateRequest:
			return s.termHandlers.HandleForceTerminate(ch, req, serverContext)
		case sshutils.EnvRequest, tracessh.EnvsRequest:
		case constants.FileTransferDecision:
			return s.termHandlers.HandleFileTransferDecision(ctx, ch, req, serverContext)
			// We ignore all SSH setenv requests for join-only principals.
			// SSH will send them anyway but it seems fine to silently drop them.
		case sshutils.SubsystemRequest:
			return s.handleSubsystem(ctx, ch, req, serverContext)
		case sshutils.AgentForwardRequest:
			// This happens when SSH client has agent forwarding enabled, in this case
			// client sends a special request, in return SSH server opens new channel
			// that uses SSH protocol for agent drafted here:
			// https://tools.ietf.org/html/draft-ietf-secsh-agent-02
			// the open ssh proto spec that we implement is here:
			// http://cvsweb.openbsd.org/cgi-bin/cvsweb/src/usr.bin/ssh/PROTOCOL.agent

			// to maintain interoperability with OpenSSH, agent forwarding requests
			// should never fail, all errors should be logged and we should continue
			// processing requests.
			err := s.handleAgentForwardNode(req, serverContext)
			if err != nil {
				s.Logger.Warn(err)
			}
			return nil
		case sshutils.PuTTYWinadjRequest:
			return s.handlePuTTYWinadj(ch, req)
		default:
			return trace.AccessDenied("attempted %v request in join-only mode", req.Type)
		}
	}
	switch req.Type {
	case sshutils.ExecRequest:
		if err := s.termHandlers.SessionRegistry.TryCreateHostUser(serverContext); err != nil {
			return trace.Wrap(err)
		}
		if err := s.termHandlers.SessionRegistry.TryWriteSudoersFile(serverContext); err != nil {
			return trace.Wrap(err)
		}
		return s.termHandlers.HandleExec(ctx, ch, req, serverContext)
	case sshutils.PTYRequest:
		return s.termHandlers.HandlePTYReq(ctx, ch, req, serverContext)
	case sshutils.ShellRequest:
		if err := s.termHandlers.SessionRegistry.TryCreateHostUser(serverContext); err != nil {
			return trace.Wrap(err)
		}
		if err := s.termHandlers.SessionRegistry.TryWriteSudoersFile(serverContext); err != nil {
			return trace.Wrap(err)
		}
		return s.termHandlers.HandleShell(ctx, ch, req, serverContext)
	case constants.InitiateFileTransfer:
		return s.termHandlers.HandleFileTransferRequest(ctx, ch, req, serverContext)
	case constants.FileTransferDecision:
		return s.termHandlers.HandleFileTransferDecision(ctx, ch, req, serverContext)
	case sshutils.WindowChangeRequest:
		return s.termHandlers.HandleWinChange(ctx, ch, req, serverContext)
	case teleport.ForceTerminateRequest:
		return s.termHandlers.HandleForceTerminate(ch, req, serverContext)
	case sshutils.EnvRequest:
		return s.handleEnv(ch, req, serverContext)
	case tracessh.EnvsRequest:
		return s.handleEnvs(ch, req, serverContext)
	case sshutils.SubsystemRequest:
		// subsystems are SSH subsystems defined in http://tools.ietf.org/html/rfc4254 6.6
		// they are in essence SSH session extensions, allowing to implement new SSH commands
		return s.handleSubsystem(ctx, ch, req, serverContext)
	case sshutils.X11ForwardRequest:
		return s.handleX11Forward(ch, req, serverContext)
	case sshutils.AgentForwardRequest:
		// This happens when SSH client has agent forwarding enabled, in this case
		// client sends a special request, in return SSH server opens new channel
		// that uses SSH protocol for agent drafted here:
		// https://tools.ietf.org/html/draft-ietf-secsh-agent-02
		// the open ssh proto spec that we implement is here:
		// http://cvsweb.openbsd.org/cgi-bin/cvsweb/src/usr.bin/ssh/PROTOCOL.agent
		if err := s.termHandlers.SessionRegistry.TryCreateHostUser(serverContext); err != nil {
			s.Logger.Warn(err)
			return nil
		}
		if err := s.termHandlers.SessionRegistry.TryWriteSudoersFile(serverContext); err != nil {
			s.Logger.Warn(err)
			return nil
		}

		// to maintain interoperability with OpenSSH, agent forwarding requests
		// should never fail, all errors should be logged and we should continue
		// processing requests.
		err := s.handleAgentForwardNode(req, serverContext)
		if err != nil {
			s.Logger.Warn(err)
		}
		return nil
	case sshutils.PuTTYWinadjRequest:
		return s.handlePuTTYWinadj(ch, req)
	default:
		s.Logger.Warnf("%v doesn't support request type '%v'", s.Component(), req.Type)
		if req.WantReply {
			if err := req.Reply(false, nil); err != nil {
				s.Logger.Errorf("sending error reply on SSH channel: %v", err)
			}
		}
		return nil
	}
}

// handleAgentForwardNode will create a unix socket and serve the agent running
// on the client on it.
func (s *Server) handleAgentForwardNode(req *ssh.Request, ctx *srv.ServerContext) error {
	// check if the user's RBAC role allows agent forwarding
	err := s.authHandlers.CheckAgentForward(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Enable agent forwarding for the broader connection-level
	// context.
	ctx.Parent().SetForwardAgent(true)

	// serve an agent on a unix socket on this node
	err = s.serveAgent(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// handleAgentForwardProxy will forward the clients agent to the proxy (when
// the proxy is running in recording mode). When running in normal mode, this
// request will do nothing. To maintain interoperability, agent forwarding
// requests should never fail, all errors should be logged and we should
// continue processing requests.
func (s *Server) handleAgentForwardProxy(req *ssh.Request, ctx *srv.ServerContext) error {
	// Forwarding an agent to the proxy is only supported when the proxy is in
	// recording mode.
	if !services.IsRecordAtProxy(ctx.SessionRecordingConfig.GetMode()) {
		return trace.BadParameter("agent forwarding to proxy only supported in recording mode")
	}

	// Check if the user's RBAC role allows agent forwarding.
	err := s.authHandlers.CheckAgentForward(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Enable agent forwarding for the broader connection-level
	// context.
	ctx.Parent().SetForwardAgent(true)

	return nil
}

// handleX11Forward handles an X11 forwarding request from the client.
func (s *Server) handleX11Forward(ch ssh.Channel, req *ssh.Request, ctx *srv.ServerContext) (err error) {
	event := &apievents.X11Forward{
		Metadata: apievents.Metadata{
			Type: events.X11ForwardEvent,
			Code: events.X11ForwardCode,
		},
		UserMetadata: ctx.Identity.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			LocalAddr:  ctx.ServerConn.LocalAddr().String(),
			RemoteAddr: ctx.ServerConn.RemoteAddr().String(),
		},
		Status: apievents.Status{
			Success: true,
		},
	}

	defer func() {
		if err != nil {
			event.Metadata.Code = events.X11ForwardFailureCode
			event.Status.Success = false
			event.Status.Error = err.Error()
		}
		if trace.IsAccessDenied(err) {
			// denied X11 requests are ok from a protocol perspective so we
			// don't return them, just reply over ssh and emit the audit s.Logger.
			s.replyError(ch, req, err)
			err = nil
		}
		if err := s.EmitAuditEvent(s.ctx, event); err != nil {
			s.Logger.WithError(err).Warn("Failed to emit x11-forward event.")
		}
	}()

	// check if X11 forwarding is disabled, or if xauth can't be handled.
	if !s.x11.Enabled || x11.CheckXAuthPath() != nil {
		return trace.AccessDenied("X11 forwarding is not enabled")
	}

	// Check if the user's RBAC role allows X11 forwarding.
	if err := s.authHandlers.CheckX11Forward(ctx); err != nil {
		return trace.Wrap(err)
	}

	var x11Req x11.ForwardRequestPayload
	if err := ssh.Unmarshal(req.Payload, &x11Req); err != nil {
		return trace.Wrap(err)
	}

	if err := ctx.OpenXServerListener(x11Req, s.x11.DisplayOffset, s.x11.MaxDisplay); err != nil {
		if trace.IsLimitExceeded(err) {
			return trace.AccessDenied("The server cannot support any more X11 forwarding sessions at this time")
		}
		return trace.Wrap(err)
	}

	return nil
}

func (s *Server) handleSubsystem(ctx context.Context, ch ssh.Channel, req *ssh.Request, serverContext *srv.ServerContext) error {
	sb, err := s.parseSubsystemRequest(req, serverContext)
	if err != nil {
		serverContext.Warnf("Failed to parse subsystem request: %v: %v.", req, err)
		return trace.Wrap(err)
	}
	serverContext.Debugf("Subsystem request: %v.", sb)
	// starting subsystem is blocking to the client,
	// while collecting its result and waiting is not blocking
	if err := sb.Start(ctx, serverContext.ServerConn, ch, req, serverContext); err != nil {
		serverContext.Warnf("Subsystem request %v failed: %v.", sb, err)
		serverContext.SendSubsystemResult(srv.SubsystemResult{Err: trace.Wrap(err)})
		return trace.Wrap(err)
	}
	go func() {
		err := sb.Wait()
		s.Logger.Debugf("Subsystem %v finished with result: %v.", sb, err)
		serverContext.SendSubsystemResult(srv.SubsystemResult{Err: trace.Wrap(err)})
	}()
	return nil
}

// handleEnv accepts an environment variable sent by the client and stores it
// in connection context
func (s *Server) handleEnv(ch ssh.Channel, req *ssh.Request, ctx *srv.ServerContext) error {
	var e sshutils.EnvReqParams
	if err := ssh.Unmarshal(req.Payload, &e); err != nil {
		ctx.Error(err)
		return trace.Wrap(err, "failed to parse env request")
	}
	ctx.SetEnv(e.Name, e.Value)
	return nil
}

// handleEnvs accepts environment variables sent by the client and stores them
// in connection context
func (s *Server) handleEnvs(ch ssh.Channel, req *ssh.Request, ctx *srv.ServerContext) error {
	var raw tracessh.EnvsReq
	if err := ssh.Unmarshal(req.Payload, &raw); err != nil {
		ctx.Error(err)
		return trace.Wrap(err, "failed to parse envs request")
	}

	var envs map[string]string
	if err := json.Unmarshal(raw.EnvsJSON, &envs); err != nil {
		return trace.Wrap(err, "failed to unmarshal envs")
	}

	for k, v := range envs {
		ctx.SetEnv(k, v)
	}

	return nil
}

// handleKeepAlive accepts and replies to keepalive@openssh.com requests.
func (s *Server) handleKeepAlive(req *ssh.Request) {
	// only reply if the sender actually wants a response
	if !req.WantReply {
		return
	}

	if err := req.Reply(true, nil); err != nil {
		s.Logger.Warnf("Unable to reply to %q request: %v", req.Type, err)
		return
	}

	s.Logger.Debugf("Replied to %q", req.Type)
}

// handleClusterDetails responds to global out-of-band with details about the cluster.
func (s *Server) handleClusterDetails(ctx context.Context, req *ssh.Request) {
	s.Logger.Debugf("Global request (%v, %v) received", req.Type, req.WantReply)

	if !req.WantReply {
		return
	}
	// get the cluster config, if we can't get it, reply false
	recConfig, err := s.authService.GetSessionRecordingConfig(ctx)
	if err != nil {
		if err := req.Reply(false, nil); err != nil {
			s.Logger.Warnf("Unable to respond to global request (%v, %v): %v", req.Type, req.WantReply, err)
		}
		return
	}

	details := sshutils.ClusterDetails{
		RecordingProxy: services.IsRecordAtProxy(recConfig.GetMode()),
		FIPSEnabled:    s.fips,
	}

	if err = req.Reply(true, ssh.Marshal(details)); err != nil {
		s.Logger.Warnf("Unable to respond to global request (%v, %v): %v: %v", req.Type, req.WantReply, details, err)
		return
	}

	s.Logger.Debugf("Replied to global request (%v, %v): %v", req.Type, req.WantReply, details)
}

// handleVersionRequest replies with the Teleport version of the server.
func (s *Server) handleVersionRequest(req *ssh.Request) {
	err := req.Reply(true, []byte(teleport.Version))
	if err != nil {
		s.Logger.Debugf("Failed to reply to version request: %v.", err)
	}
}

// handleProxyJump handles ProxyJump request that is executed via direct tcp-ip dial on the proxy
func (s *Server) handleProxyJump(ctx context.Context, ccx *sshutils.ConnectionContext, identityContext srv.IdentityContext, ch ssh.Channel, req sshutils.DirectTCPIPReq) {
	// Create context for this channel. This context will be closed when the
	// session request is complete.
	ctx, scx, err := srv.NewServerContext(ctx, ccx, s, identityContext)
	if err != nil {
		s.Logger.WithError(err).Error("Unable to create connection context.")
		writeStderr(ch, "Unable to create connection context.")
		if err := ch.Close(); err != nil {
			s.Logger.WithError(err).Warn("Failed to close channel.")
		}
		return
	}
	scx.IsTestStub = s.isTestStub
	scx.AddCloser(ch)
	scx.SetAllowFileCopying(s.allowFileCopying)
	defer scx.Close()

	ch = scx.TrackActivity(ch)

	recConfig, err := s.GetAccessPoint().GetSessionRecordingConfig(ctx)
	if err != nil {
		s.Logger.Errorf("Unable to fetch session recording config: %v.", err)
		writeStderr(ch, "Unable to fetch session recording configuration.")
		return
	}

	// force agent forward, because in recording mode proxy needs
	// client's agent to authenticate to the target server
	//
	// When proxy is in "Recording mode" the following will happen with SSH:
	//
	// $ ssh -J user@teleport.proxy:3023 -p 3022 user@target -F ./forward.config
	//
	// Where forward.config enables agent forwarding:
	//
	// Host teleport.proxy
	//     ForwardAgent yes
	//
	// This will translate to ProxyCommand:
	//
	// exec ssh -l user -p 3023 -F ./forward.config -vvv -W 'target:3022' teleport.proxy
	//
	// -W means establish direct tcp-ip, and in SSH 2.0 session implementation,
	// this gets called before agent forwarding is requested:
	//
	// https://github.com/openssh/openssh-portable/blob/master/ssh.c#L1884
	//
	// so in recording mode, proxy is forced to request agent forwarding
	// "out of band", before SSH client actually asks for it
	// which is a hack, but the only way we can think of making it work,
	// ideas are appreciated.
	if services.IsRecordAtProxy(recConfig.GetMode()) {
		err = s.handleAgentForwardProxy(&ssh.Request{}, scx)
		if err != nil {
			s.Logger.Warningf("Failed to request agent in recording mode: %v", err)
			writeStderr(ch, "Failed to request agent")
			return
		}
	}

	netConfig, err := s.GetAccessPoint().GetClusterNetworkingConfig(ctx)
	if err != nil {
		s.Logger.Errorf("Unable to fetch cluster networking config: %v.", err)
		writeStderr(ch, "Unable to fetch cluster networking configuration.")
		return
	}

	// The keep-alive loop will keep pinging the remote server and after it has
	// missed a certain number of keep-alive requests it will cancel the
	// closeContext which signals the server to shutdown.
	go srv.StartKeepAliveLoop(srv.KeepAliveParams{
		Conns: []srv.RequestSender{
			scx.ServerConn,
		},
		Interval:     netConfig.GetKeepAliveInterval(),
		MaxCount:     netConfig.GetKeepAliveCountMax(),
		CloseContext: ctx,
		CloseCancel:  scx.CancelFunc(),
	})

	subsys, err := newProxySubsys(scx, s, proxySubsysRequest{
		host: req.Host,
		port: fmt.Sprintf("%v", req.Port),
	})
	if err != nil {
		s.Logger.Errorf("Unable instantiate proxy subsystem: %v.", err)
		writeStderr(ch, "Unable to instantiate proxy subsystem.")
		return
	}

	if err := subsys.Start(ctx, scx.ServerConn, ch, &ssh.Request{}, scx); err != nil {
		s.Logger.Errorf("Unable to start proxy subsystem: %v.", err)
		writeStderr(ch, "Unable to start proxy subsystem.")
		return
	}

	wch := make(chan struct{})
	go func() {
		defer close(wch)
		if err := subsys.Wait(); err != nil {
			s.Logger.Errorf("Proxy subsystem failed: %v.", err)
			writeStderr(ch, "Proxy subsystem failed.")
		}
	}()
	select {
	case <-wch:
	case <-ctx.Done():
	}
}

// createForwardingContext creates a server context for a user intending to
// port forward. It returns an error if the user is not allowed to port forward.
func (s *Server) createForwardingContext(ctx context.Context, ccx *sshutils.ConnectionContext, r *ssh.Request) (context.Context, *srv.ServerContext, error) {
	req, err := sshutils.ParseTCPIPForwardReq(r.Payload)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	identityContext, err := s.authHandlers.CreateIdentityContext(ccx.ServerConn)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// On regular server in "normal" mode "tcpip-forward" requests from
	// SessionJoinPrincipal should be rejected, otherwise it's possible to use
	// the "-teleport-internal-join" user to bypass RBAC.
	if identityContext.Login == teleport.SSHSessionJoinPrincipal {
		log.Errorf("Request rejected, %q with SessionJoinPrincipal in forward node must be blocked", r.Type)
		err := trace.AccessDenied("attempted %q request in join-only mode", r.Type)
		if replyErr := r.Reply(false, []byte(utils.FormatErrorWithNewline(err))); replyErr != nil {
			s.Logger.Warnf("Failed to reply to %q request: %v", r.Type, err)
		}
		// Disable default reply by caller, we already handled it.
		r.WantReply = false
		return nil, nil, err
	}

	// Create context for this request.
	ctx, scx, err := srv.NewServerContext(ctx, ccx, s, identityContext)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	listenAddr := sshutils.JoinHostPort(req.Addr, req.Port)
	scx.IsTestStub = s.isTestStub
	scx.ExecType = teleport.TCPIPForwardRequest
	scx.SrcAddr = listenAddr
	scx.DstAddr = ccx.NetConn.RemoteAddr().String()
	scx.SetAllowFileCopying(s.allowFileCopying)

	if err := s.canPortForward(scx); err != nil {
		scx.Close()
		return nil, nil, trace.Wrap(err)
	}
	return ctx, scx, nil
}

// handleTCPIPForwardRequest handles remote port forwarding requests.
func (s *Server) handleTCPIPForwardRequest(ctx context.Context, ccx *sshutils.ConnectionContext, r *ssh.Request) error {
	ctx, scx, err := s.createForwardingContext(ctx, ccx, r)
	if err != nil {
		return trace.Wrap(err)
	}
	defer scx.Close()
	listener, err := s.listenTCPIP(scx, scx.SrcAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	// Set the src addr again since it may have been updated with a new port.
	scx.SrcAddr = listener.Addr().String()
	event := scx.GetPortForwardEvent()
	if err := s.EmitAuditEvent(ctx, &event); err != nil {
		s.Logger.WithError(err).Warn("Failed to emit audit event.")
	}
	if err := sshutils.StartRemoteListener(ctx, scx.ConnectionContext.ServerConn, scx.SrcAddr, listener); err != nil {
		return trace.Wrap(err)
	}

	// Report addr back to the client.
	if r.WantReply {
		var payload []byte
		req, err := sshutils.ParseTCPIPForwardReq(r.Payload)
		if err != nil {
			return trace.Wrap(err)
		}
		if req.Port == 0 {
			payload = ssh.Marshal(struct {
				Port uint32
			}{Port: uint32(listener.Addr().(*net.TCPAddr).Port)})
		}

		if err := r.Reply(true, payload); err != nil {
			s.Logger.Warnf("Failed to reply to %q request: %v", r.Type, err)
		}
	}

	s.remoteForwardingMap.Store(scx.SrcAddr, listener)
	return nil
}

// handleCancelTCPIPForwardRequest handles canceling a previously requested
// remote forwarded port.
func (s *Server) handleCancelTCPIPForwardRequest(ctx context.Context, ccx *sshutils.ConnectionContext, r *ssh.Request) error {
	_, scx, err := s.createForwardingContext(ctx, ccx, r)
	if err != nil {
		return trace.Wrap(err)
	}
	defer scx.Close()

	listener, ok := s.remoteForwardingMap.LoadAndDelete(scx.SrcAddr)
	if !ok {
		return trace.NotFound("no remote forwarding listener at %v", scx.SrcAddr)
	}
	if err := r.Reply(true, nil); err != nil {
		s.Logger.Warnf("Failed to reply to %q request: %v", r.Type, err)
	}
	return trace.Wrap(listener.Close())
}

func (s *Server) replyError(ch ssh.Channel, req *ssh.Request, err error) {
	s.Logger.WithError(err).Errorf("failure handling SSH %q request", req.Type)
	// Terminate the error with a newline when writing to remote channel's
	// stderr so the output does not mix with the rest of the output if the remote
	// side is not doing additional formatting for extended data.
	// See github.com/gravitational/teleport/issues/4542
	message := utils.FormatErrorWithNewline(err)
	writeStderr(ch, message)
	if req.WantReply {
		if err := req.Reply(false, []byte(message)); err != nil {
			s.Logger.Warnf("Failed to reply to %q request: %v", req.Type, err)
		}
	}
}

func (s *Server) parseSubsystemRequest(req *ssh.Request, ctx *srv.ServerContext) (srv.Subsystem, error) {
	var r sshutils.SubsystemReq
	if err := ssh.Unmarshal(req.Payload, &r); err != nil {
		return nil, trace.BadParameter("failed to parse subsystem request: %v", err)
	}

	if s.proxyMode {
		switch {
		case strings.HasPrefix(r.Name, "proxy:"):
			return parseProxySubsys(r.Name, s, ctx)
		case strings.HasPrefix(r.Name, "proxysites"):
			return parseProxySitesSubsys(r.Name, s)
		default:
			return nil, trace.BadParameter("unrecognized subsystem: %v", r.Name)
		}
	}

	switch {
	// DELETE IN 15.0.0 (deprecated, tsh will not be using this anymore)
	case r.Name == teleport.GetHomeDirSubsystem:
		return newHomeDirSubsys(), nil
	case r.Name == teleport.SFTPSubsystem:
		err := ctx.CheckSFTPAllowed(s.reg)
		if err != nil {
			s.EmitAuditEvent(context.Background(), &apievents.SFTP{
				Metadata: apievents.Metadata{
					Code: events.SFTPDisallowedCode,
					Type: events.SFTPEvent,
					Time: time.Now(),
				},
				UserMetadata:   ctx.Identity.GetUserMetadata(),
				ServerMetadata: ctx.GetServerMetadata(),
				Error:          err.Error(),
			})
			return nil, trace.Wrap(err)
		}

		return newSFTPSubsys(ctx.ConsumeApprovedFileTransferRequest())
	default:
		return nil, trace.BadParameter("unrecognized subsystem: %v", r.Name)
	}
}

func writeStderr(ch ssh.Channel, msg string) {
	if _, err := io.WriteString(ch.Stderr(), msg); err != nil {
		log.Warnf("Failed writing to ssh.Channel.Stderr(): %v", err)
	}
}

func rejectChannel(ch ssh.NewChannel, reason ssh.RejectionReason, msg string) {
	if err := ch.Reject(reason, msg); err != nil {
		log.Warnf("Failed to reject new ssh.Channel: %v", err)
	}
}

// handlePuTTYWinadj replies with failure to a PuTTY winadj request as required.
// it returns an error if the reply fails. context from the PuTTY documentation:
// PuTTY sends this request along with some SSH_MSG_CHANNEL_WINDOW_ADJUST messages as part of its window-size
// tuning. It can be sent on any type of channel. There is no message-specific data. Servers MUST treat it
// as an unrecognized request and respond with SSH_MSG_CHANNEL_FAILURE.
// https://the.earth.li/~sgtatham/putty/0.76/htmldoc/AppendixG.html#sshnames-channel
func (s *Server) handlePuTTYWinadj(ch ssh.Channel, req *ssh.Request) error {
	if err := req.Reply(false, nil); err != nil {
		s.Logger.Warnf("Failed to reply to %q request: %v", req.Type, err)
		return err
	}
	// the reply has been handled inside this function (rather than relying on the standard behavior
	// of leaving handleSessionRequests to do it) so set the WantReply flag to false here.
	req.WantReply = false
	return nil
}
