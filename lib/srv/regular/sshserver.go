/*
Copyright 2015 Gravitational, Inc.

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

// Package regular implements SSH server that supports multiplexing
// tunneling, SSH connections proxying and only supports Key based auth
package regular

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentNode,
})

// Server implements SSH server that uses configuration backend and
// certificate-based authentication
type Server struct {
	sync.Mutex

	*logrus.Entry

	namespace string
	addr      utils.NetAddr
	hostname  string

	srv           *sshutils.Server
	shell         string
	getRotation   RotationGetter
	authService   auth.AccessPoint
	reg           *srv.SessionRegistry
	sessionServer rsession.Service
	limiter       *limiter.Limiter

	labels      map[string]string                //static server labels
	cmdLabels   map[string]services.CommandLabel //dymanic server labels
	labelsMutex *sync.Mutex

	proxyMode bool
	proxyTun  reversetunnel.Server

	advertiseAddr   *utils.NetAddr
	proxyPublicAddr utils.NetAddr

	// server UUID gets generated once on the first start and never changes
	// usually stored in a file inside the data dir
	uuid string

	// this gets set to true for unit testing
	isTestStub bool

	// cancel cancels all operations
	cancel context.CancelFunc
	// ctx is broadcasting context closure
	ctx context.Context

	// alog points to the AuditLog this server uses to report
	// auditable events
	alog events.IAuditLog

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
	pamConfig *pam.Config

	// dataDir is a server local data directory
	dataDir string

	// heartbeat sends updates about this server
	// back to auth server
	heartbeat *srv.Heartbeat

	// useTunnel is used to inform other components that this server is
	// requesting connections to it come over a reverse tunnel.
	useTunnel bool

	// fips means Teleport started in a FedRAMP/FIPS 140-2 compliant
	// configuration.
	fips bool

	// ebpf is the service used for enhanced session recording.
	ebpf bpf.BPF
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

func (s *Server) GetAuditLog() events.IAuditLog {
	if s.isAuditedAtProxy() {
		return events.NewDiscardAuditLog()
	}
	return s.alog
}

func (s *Server) GetAccessPoint() auth.AccessPoint {
	return s.authService
}

func (s *Server) GetSessionServer() rsession.Service {
	if s.isAuditedAtProxy() {
		return rsession.NewDiscardSessionServer()
	}
	return s.sessionServer
}

// GetPAM returns the PAM configuration for this server.
func (s *Server) GetPAM() (*pam.Config, error) {
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

// isAuditedAtProxy returns true if sessions are being recorded at the proxy
// and this is a Teleport node.
func (s *Server) isAuditedAtProxy() bool {
	// always be safe, better to double record than not record at all
	clusterConfig, err := s.GetAccessPoint().GetClusterConfig()
	if err != nil {
		return false
	}

	isRecordAtProxy := clusterConfig.GetSessionRecording() == services.RecordAtProxy
	isTeleportNode := s.Component() == teleport.ComponentNode

	if isRecordAtProxy && isTeleportNode {
		return true
	}
	return false
}

// ServerOption is a functional option passed to the server
type ServerOption func(s *Server) error

// Close closes listening socket and stops accepting connections
func (s *Server) Close() error {
	s.cancel()
	s.reg.Close()
	if s.heartbeat != nil {
		if err := s.heartbeat.Close(); err != nil {
			s.Warningf("Failed to close heartbeat: %v", err)
		}
		s.heartbeat = nil
	}
	return s.srv.Close()
}

// Shutdown performs graceful shutdown
func (s *Server) Shutdown(ctx context.Context) error {
	// wait until connections drain off
	err := s.srv.Shutdown(ctx)
	s.cancel()
	s.reg.Close()
	if s.heartbeat != nil {
		if err := s.heartbeat.Close(); err != nil {
			s.Warningf("Failed to close heartbeat: %v.", err)
		}
		s.heartbeat = nil
	}
	return err
}

// Start starts server
func (s *Server) Start() error {
	if len(s.getCommandLabels()) > 0 {
		s.updateLabels()
	}

	// If the server requested connections to it arrive over a reverse tunnel,
	// don't call Start() which listens on a socket, return right away.
	if s.useTunnel {
		go s.heartbeat.Run()
		return nil
	}
	if err := s.srv.Start(); err != nil {
		return trace.Wrap(err)
	}
	// Heartbeat should start only after s.srv.Start.
	// If the server is configured to listen on port 0 (such as in tests),
	// it'll only populate its actual listening address during s.srv.Start.
	// Heartbeat uses this address to announce. Avoid announcing an empty
	// address on first heartbeat.
	go s.heartbeat.Run()
	return nil
}

// Serve servers service on started listener
func (s *Server) Serve(l net.Listener) error {
	if len(s.getCommandLabels()) > 0 {
		s.updateLabels()
	}
	go s.heartbeat.Run()
	return s.srv.Serve(l)
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

// RotationGetter returns rotation state
type RotationGetter func(role teleport.Role) (*services.Rotation, error)

// SetRotationGetter sets rotation state getter
func SetRotationGetter(getter RotationGetter) ServerOption {
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

// SetSessionServer represents realtime session registry server
func SetSessionServer(sessionServer rsession.Service) ServerOption {
	return func(s *Server) error {
		s.sessionServer = sessionServer
		return nil
	}
}

// SetProxyMode starts this server in SSH proxying mode
func SetProxyMode(tsrv reversetunnel.Server) ServerOption {
	return func(s *Server) error {
		// always set proxy mode to true,
		// because in some tests reverse tunnel is disabled,
		// but proxy is still used without it.
		s.proxyMode = true
		s.proxyTun = tsrv
		return nil
	}
}

// SetLabels sets dynamic and static labels that server will report to the
// auth servers
func SetLabels(labels map[string]string,
	cmdLabels services.CommandLabels) ServerOption {
	return func(s *Server) error {
		// make sure to clone labels to avoid
		// concurrent writes to the map during reloads
		cmdLabels = cmdLabels.Clone()
		for name, label := range cmdLabels {
			if label.GetPeriod() < time.Second {
				label.SetPeriod(time.Second)
				cmdLabels[name] = label
				log.Warningf("label period can't be less that 1 second. Period for label '%v' was set to 1 second", name)
			}
		}
		s.labels = labels
		s.cmdLabels = cmdLabels
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

// SetAuditLog assigns an audit log interfaces to this server
func SetAuditLog(alog events.IAuditLog) ServerOption {
	return func(s *Server) error {
		s.alog = alog
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

func SetPAMConfig(pamConfig *pam.Config) ServerOption {
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

// New returns an unstarted server
func New(addr utils.NetAddr,
	hostname string,
	signers []ssh.Signer,
	authService auth.AccessPoint,
	dataDir string,
	advertiseAddr string,
	proxyPublicAddr utils.NetAddr,
	options ...ServerOption) (*Server, error) {

	// read the host UUID:
	uuid, err := utils.ReadOrMakeHostUUID(dataDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(context.TODO())
	s := &Server{
		addr:            addr,
		authService:     authService,
		hostname:        hostname,
		labelsMutex:     &sync.Mutex{},
		proxyPublicAddr: proxyPublicAddr,
		uuid:            uuid,
		cancel:          cancel,
		ctx:             ctx,
		clock:           clockwork.NewRealClock(),
		dataDir:         dataDir,
	}
	s.limiter, err = limiter.NewLimiter(limiter.LimiterConfig{})
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
	if s.alog == nil {
		return nil, trace.BadParameter("setup valid AuditLog parameter using SetAuditLog")
	}

	if s.namespace == "" {
		return nil, trace.BadParameter("setup valid namespace parameter using SetNamespace")
	}

	var component string
	if s.proxyMode {
		component = teleport.ComponentProxy
	} else {
		component = teleport.ComponentNode
	}

	s.Entry = logrus.WithFields(logrus.Fields{
		trace.Component:       component,
		trace.ComponentFields: logrus.Fields{},
	})

	s.reg, err = srv.NewSessionRegistry(s)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// add in common auth handlers
	s.authHandlers = &srv.AuthHandlers{
		Entry: logrus.WithFields(logrus.Fields{
			trace.Component:       component,
			trace.ComponentFields: logrus.Fields{},
		}),
		Server:      s,
		Component:   component,
		AuditLog:    s.alog,
		AccessPoint: s.authService,
		FIPS:        s.fips,
	}

	// common term handlers
	s.termHandlers = &srv.TermHandlers{
		SessionRegistry: s.reg,
	}

	server, err := sshutils.NewServer(
		component,
		addr, s, signers,
		sshutils.AuthMethods{PublicKey: s.authHandlers.UserKeyAuth},
		sshutils.SetLimiter(s.limiter),
		sshutils.SetRequestHandler(s),
		sshutils.SetCiphers(s.ciphers),
		sshutils.SetKEXAlgorithms(s.kexAlgorithms),
		sshutils.SetMACAlgorithms(s.macAlgorithms),
		sshutils.SetFIPS(s.fips),
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
	heartbeat, err := srv.NewHeartbeat(srv.HeartbeatConfig{
		Mode:            heartbeatMode,
		Context:         ctx,
		Component:       component,
		Announcer:       s.authService,
		GetServerInfo:   s.getServerInfo,
		KeepAlivePeriod: defaults.ServerKeepAliveTTL,
		AnnouncePeriod:  defaults.ServerAnnounceTTL/2 + utils.RandomDuration(defaults.ServerAnnounceTTL/10),
		ServerTTL:       defaults.ServerAnnounceTTL,
		CheckPeriod:     defaults.HeartbeatCheckPeriod,
		Clock:           s.clock,
	})
	if err != nil {
		s.srv.Close()
		return nil, trace.Wrap(err)
	}
	s.heartbeat = heartbeat
	return s, nil
}

func (s *Server) getNamespace() string {
	return services.ProcessNamespace(s.namespace)
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
		log.Warningf("Failed to parse advertise address %q, %v, using default value %q.", advertiseAddr, err, listenAddr)
		return listenAddr
	}
	if aport == "" {
		aport = port
	}
	return net.JoinHostPort(ahost, aport)
}

func (s *Server) getRole() teleport.Role {
	if s.proxyMode {
		return teleport.RoleProxy
	}
	return teleport.RoleNode
}

// GetInfo returns a services.Server that represents this server.
func (s *Server) GetInfo() services.Server {
	// Only set the address for non-tunnel nodes.
	var addr string
	if !s.useTunnel {
		addr = s.AdvertiseAddr()
	}

	return &services.ServerV2{
		Kind:    services.KindNode,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      s.ID(),
			Namespace: s.getNamespace(),
			Labels:    s.labels,
		},
		Spec: services.ServerSpecV2{
			CmdLabels: services.LabelsToV2(s.getCommandLabels()),
			Addr:      addr,
			Hostname:  s.hostname,
			UseTunnel: s.useTunnel,
			Version:   teleport.Version,
		},
	}
}

func (s *Server) getServerInfo() (services.Server, error) {
	server := s.GetInfo()
	if s.getRotation != nil {
		rotation, err := s.getRotation(s.getRole())
		if err != nil {
			if !trace.IsNotFound(err) {
				log.Warningf("Failed to get rotation state: %v", err)
			}
		} else {
			server.SetRotation(*rotation)
		}
	}
	server.SetTTL(s.clock, defaults.ServerAnnounceTTL)
	server.SetPublicAddr(s.proxyPublicAddr.String())
	return server, nil
}

func (s *Server) updateLabels() {
	for name, label := range s.getCommandLabels() {
		go s.periodicUpdateLabel(name, label.Clone())
	}
}

func (s *Server) syncUpdateLabels() {
	for name, label := range s.getCommandLabels() {
		s.updateLabel(name, label)
	}
}

func (s *Server) updateLabel(name string, label services.CommandLabel) {
	out, err := exec.Command(label.GetCommand()[0], label.GetCommand()[1:]...).Output()
	if err != nil {
		log.Errorf(err.Error())
		label.SetResult(err.Error() + " output: " + string(out))
	} else {
		label.SetResult(strings.TrimSpace(string(out)))
	}
	s.setCommandLabel(name, label)
}

func (s *Server) periodicUpdateLabel(name string, label services.CommandLabel) {
	t := time.NewTicker(label.GetPeriod())
	defer t.Stop()
	for {
		s.updateLabel(name, label.Clone())
		select {
		case <-t.C:
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Server) setCommandLabel(name string, value services.CommandLabel) {
	s.labelsMutex.Lock()
	defer s.labelsMutex.Unlock()
	s.cmdLabels[name] = value
}

func (s *Server) getCommandLabels() map[string]services.CommandLabel {
	s.labelsMutex.Lock()
	defer s.labelsMutex.Unlock()
	out := make(map[string]services.CommandLabel, len(s.cmdLabels))
	for key, val := range s.cmdLabels {
		out[key] = val.Clone()
	}
	return out
}

// serveAgent will build the a sock path for this user and serve an SSH agent on unix socket.
func (s *Server) serveAgent(ctx *srv.ServerContext) error {
	// gather information about user and process. this will be used to set the
	// socket path and permissions
	systemUser, err := user.Lookup(ctx.Identity.Login)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	uid, err := strconv.Atoi(systemUser.Uid)
	if err != nil {
		return trace.Wrap(err)
	}
	gid, err := strconv.Atoi(systemUser.Gid)
	if err != nil {
		return trace.Wrap(err)
	}
	pid := os.Getpid()

	// build the socket path and set permissions
	socketDir, err := ioutil.TempDir(os.TempDir(), "teleport-")
	if err != nil {
		return trace.Wrap(err)
	}
	dirCloser := &utils.RemoveDirCloser{Path: socketDir}
	socketPath := filepath.Join(socketDir, fmt.Sprintf("teleport-%v.socket", pid))
	if err := os.Chown(socketDir, uid, gid); err != nil {
		if err := dirCloser.Close(); err != nil {
			log.Warnf("failed to remove directory: %v", err)
		}
		return trace.ConvertSystemError(err)
	}

	// start an agent server on a unix socket.  each incoming connection
	// will result in a separate agent request.
	agentServer := teleagent.NewServer(ctx.Parent().StartAgentChannel)
	err = agentServer.ListenUnixSocket(socketPath, uid, gid, 0600)
	if err != nil {
		return trace.Wrap(err)
	}
	ctx.Parent().SetEnv(teleport.SSHAuthSock, socketPath)
	ctx.Parent().SetEnv(teleport.SSHAgentPID, fmt.Sprintf("%v", pid))
	ctx.Parent().AddCloser(agentServer)
	ctx.Parent().AddCloser(dirCloser)
	ctx.Debugf("Starting agent server for Teleport user %v and socket %v.", ctx.Identity.TeleportUser, socketPath)
	go func() {
		if err := agentServer.Serve(); err != nil {
			ctx.Errorf("agent server for user %q stopped: %v", ctx.Identity.TeleportUser, err)
		}
	}()

	return nil
}

// EmitAuditEvent logs a given event to the audit log attached to the
// server who owns these sessions
func (s *Server) EmitAuditEvent(event events.Event, fields events.EventFields) {
	log.Debugf("server.EmitAuditEvent(%v)", event.Name)
	alog := s.alog
	if alog != nil {
		// record the event time with ms precision
		fields[events.EventTime] = s.clock.Now().In(time.UTC).Round(time.Millisecond)
		if err := alog.EmitAuditEvent(event, fields); err != nil {
			log.Error(trace.DebugReport(err))
		}
	} else {
		log.Warn("SSH server has no audit log")
	}
}

// HandleRequest processes global out-of-band requests. Global out-of-band
// requests are processed in order (this way the originator knows which
// request we are responding to). If Teleport does not support the request
// type or an error occurs while processing that request Teleport will reply
// req.Reply(false, nil).
//
// For more details: https://tools.ietf.org/html/rfc4254.html#page-4
func (s *Server) HandleRequest(r *ssh.Request) {
	switch r.Type {
	case teleport.KeepAliveReqType:
		s.handleKeepAlive(r)
	case teleport.RecordingProxyReqType:
		s.handleRecordingProxy(r)
	case teleport.VersionRequest:
		s.handleVersionRequest(r)
	default:
		if r.WantReply {
			if err := r.Reply(false, nil); err != nil {
				log.Warnf("Failed to reply to %q request: %v", r.Type, err)
			}
		}
		log.Debugf("Discarding %q global request: %+v", r.Type, r)
	}
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
				log.Errorf("Failed to parse request data: %v, err: %v.", string(nch.ExtraData()), err)
				rejectChannel(nch, ssh.UnknownChannelType, "failed to parse direct-tcpip request")
				return
			}
			ch, _, err := nch.Accept()
			if err != nil {
				log.Warnf("Unable to accept channel: %v.", err)
				rejectChannel(nch, ssh.ConnectionFailed, fmt.Sprintf("unable to accept channel: %v", err))
				return
			}
			go s.handleProxyJump(ctx, ccx, identityContext, ch, *req)
			return
		// Channels of type "session" handle requests that are involved in running
		// commands on a server. In the case of proxy mode subsystem and agent
		// forwarding requests occur over the "session" channel.
		case teleport.ChanSession:
			ch, requests, err := nch.Accept()
			if err != nil {
				log.Warnf("Unable to accept channel: %v.", err)
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
		ch, requests, err := nch.Accept()
		if err != nil {
			log.Warnf("Unable to accept channel: %v.", err)
			rejectChannel(nch, ssh.ConnectionFailed, fmt.Sprintf("unable to accept channel: %v", err))
			return
		}
		go s.handleSessionRequests(ctx, ccx, identityContext, ch, requests)
	// Channels of type "direct-tcpip" handles request for port forwarding.
	case teleport.ChanDirectTCPIP:
		req, err := sshutils.ParseDirectTCPIPReq(nch.ExtraData())
		if err != nil {
			log.Errorf("Failed to parse request data: %v, err: %v.", string(nch.ExtraData()), err)
			rejectChannel(nch, ssh.UnknownChannelType, "failed to parse direct-tcpip request")
			return
		}
		ch, _, err := nch.Accept()
		if err != nil {
			log.Warnf("Unable to accept channel: %v.", err)
			rejectChannel(nch, ssh.ConnectionFailed, fmt.Sprintf("unable to accept channel: %v", err))
			return
		}
		go s.handleDirectTCPIPRequest(ctx, ccx, identityContext, ch, req)
	default:
		rejectChannel(nch, ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %v", channelType))
	}
}

// handleDirectTCPIPRequest handles port forwarding requests.
func (s *Server) handleDirectTCPIPRequest(ctx context.Context, ccx *sshutils.ConnectionContext, identityContext srv.IdentityContext, channel ssh.Channel, req *sshutils.DirectTCPIPReq) {
	// Create context for this channel. This context will be closed when
	// forwarding is complete.
	ctx, scx, err := srv.NewServerContext(ctx, ccx, s, identityContext)
	if err != nil {
		log.Errorf("Unable to create connection context: %v.", err)
		writeStderr(channel, "Unable to create connection context.")
		return
	}
	scx.IsTestStub = s.isTestStub
	scx.AddCloser(channel)
	scx.ChannelType = teleport.ChanDirectTCPIP
	scx.SrcAddr = net.JoinHostPort(req.Orig, strconv.Itoa(int(req.OrigPort)))
	scx.DstAddr = net.JoinHostPort(req.Host, strconv.Itoa(int(req.Port)))
	defer scx.Close()

	// Check if the role allows port forwarding for this user.
	err = s.authHandlers.CheckPortForward(scx.DstAddr, scx)
	if err != nil {
		writeStderr(channel, err.Error())
		return
	}

	scx.Debugf("Opening direct-tcpip channel from %v to %v.", scx.SrcAddr, scx.DstAddr)
	defer scx.Debugf("Closing direct-tcpip channel from %v to %v.", scx.SrcAddr, scx.DstAddr)

	// Create command to re-exec Teleport which will perform a net.Dial. The
	// reason it's not done directly is because the PAM stack needs to be called
	// from another process.
	cmd, err := srv.ConfigureCommand(scx)
	if err != nil {
		writeStderr(channel, err.Error())
	}
	// Propagate stderr from the spawned Teleport process to log any errors.
	cmd.Stderr = os.Stderr

	// Create a pipe for std{in,out} that will be used to transfer data between
	// parent and child.
	pr, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	pw, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}

	// Start the child process that will be used to make the actual connection
	// to the target host.
	err = cmd.Start()
	if err != nil {
		writeStderr(channel, err.Error())
		return
	}

	// Start copy routines that copy from channel to stdin pipe and from stdout
	// pipe to channel.
	errorCh := make(chan error, 2)
	go func() {
		defer channel.Close()
		defer pw.Close()
		defer pr.Close()

		_, err := io.Copy(pw, channel)
		errorCh <- err
	}()
	go func() {
		defer channel.Close()
		defer pw.Close()
		defer pr.Close()

		_, err := io.Copy(channel, pr)
		errorCh <- err
	}()

	// Block until copy is complete and the child process is done executing.
	for i := 0; i < 2; i++ {
		select {
		case err := <-errorCh:
			if err != nil && err != io.EOF {
				log.Warnf("Connection problem in \"direct-tcpip\" channel: %v %T.", trace.DebugReport(err), err)
			}
		case <-ctx.Done():
			break
		case <-s.ctx.Done():
			break
		}
	}
	err = cmd.Wait()
	if err != nil {
		writeStderr(channel, err.Error())
		return
	}

	// Emit a port forwarding event.
	s.EmitAuditEvent(events.PortForward, events.EventFields{
		events.PortForwardAddr:    scx.DstAddr,
		events.PortForwardSuccess: true,
		events.EventLogin:         scx.Identity.Login,
		events.EventUser:          scx.Identity.TeleportUser,
		events.LocalAddr:          scx.ServerConn.LocalAddr().String(),
		events.RemoteAddr:         scx.ServerConn.RemoteAddr().String(),
	})
}

// handleSessionRequests handles out of band session requests once the session
// channel has been created this function's loop handles all the "exec",
// "subsystem" and "shell" requests.
func (s *Server) handleSessionRequests(ctx context.Context, ccx *sshutils.ConnectionContext, identityContext srv.IdentityContext, ch ssh.Channel, in <-chan *ssh.Request) {
	// Create context for this channel. This context will be closed when the
	// session request is complete.
	ctx, scx, err := srv.NewServerContext(ctx, ccx, s, identityContext)
	if err != nil {
		log.Errorf("Unable to create connection context: %v.", err)
		writeStderr(ch, "Unable to create connection context.")
		return
	}
	scx.IsTestStub = s.isTestStub
	scx.AddCloser(ch)
	scx.ChannelType = teleport.ChanSession
	defer scx.Close()

	clusterConfig, err := s.GetAccessPoint().GetClusterConfig()
	if err != nil {
		log.Errorf("Unable to fetch cluster config: %v.", err)
		writeStderr(ch, "Unable to fetch cluster configuration.")
		return
	}

	// The keep-alive loop will keep pinging the remote server and after it has
	// missed a certain number of keep-alive requests it will cancel the
	// closeContext which signals the server to shutdown.
	go srv.StartKeepAliveLoop(srv.KeepAliveParams{
		Conns: []srv.RequestSender{
			scx.ServerConn,
		},
		Interval:     clusterConfig.GetKeepAliveInterval(),
		MaxCount:     clusterConfig.GetKeepAliveCountMax(),
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
			if err := s.dispatch(ch, req, scx); err != nil {
				s.replyError(ch, req, err)
				return
			}
			if req.WantReply {
				if err := req.Reply(true, nil); err != nil {
					log.Warnf("Failed to reply to %q request: %v", req.Type, err)
				}
			}
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
			log.Debugf("Closing session due to cancellation.")
			return
		}
	}
}

// dispatch receives an SSH request for a subsystem and disptaches the request to the
// appropriate subsystem implementation
func (s *Server) dispatch(ch ssh.Channel, req *ssh.Request, ctx *srv.ServerContext) error {
	ctx.Debugf("Handling request %v, want reply %v.", req.Type, req.WantReply)

	// If this SSH server is configured to only proxy, we do not support anything
	// other than our own custom "subsystems" and environment manipulation.
	if s.proxyMode {
		switch req.Type {
		case sshutils.SubsystemRequest:
			return s.handleSubsystem(ch, req, ctx)
		case sshutils.EnvRequest:
			// we currently ignore setting any environment variables via SSH for security purposes
			return s.handleEnv(ch, req, ctx)
		case sshutils.AgentForwardRequest:
			// process agent forwarding, but we will only forward agent to proxy in
			// recording proxy mode.
			err := s.handleAgentForwardProxy(req, ctx)
			if err != nil {
				log.Debug(err)
			}
			return nil
		default:
			return trace.BadParameter(
				"(%v) proxy doesn't support request type '%v'", s.Component(), req.Type)
		}
	}

	switch req.Type {
	case sshutils.ExecRequest:
		return s.termHandlers.HandleExec(ch, req, ctx)
	case sshutils.PTYRequest:
		return s.termHandlers.HandlePTYReq(ch, req, ctx)
	case sshutils.ShellRequest:
		return s.termHandlers.HandleShell(ch, req, ctx)
	case sshutils.WindowChangeRequest:
		return s.termHandlers.HandleWinChange(ch, req, ctx)
	case sshutils.EnvRequest:
		return s.handleEnv(ch, req, ctx)
	case sshutils.SubsystemRequest:
		// subsystems are SSH subsystems defined in http://tools.ietf.org/html/rfc4254 6.6
		// they are in essence SSH session extensions, allowing to implement new SSH commands
		return s.handleSubsystem(ch, req, ctx)
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
		err := s.handleAgentForwardNode(req, ctx)
		if err != nil {
			log.Debug(err)
		}
		return nil
	default:
		return trace.BadParameter(
			"%v doesn't support request type '%v'", s.Component(), req.Type)
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
	if ctx.ClusterConfig.GetSessionRecording() != services.RecordAtProxy {
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

func (s *Server) handleSubsystem(ch ssh.Channel, req *ssh.Request, ctx *srv.ServerContext) error {
	sb, err := s.parseSubsystemRequest(req, ctx)
	if err != nil {
		ctx.Warnf("Failed to parse subsystem request: %v: %v.", req, err)
		return trace.Wrap(err)
	}
	ctx.Debugf("Subsystem request: %v.", sb)
	// starting subsystem is blocking to the client,
	// while collecting its result and waiting is not blocking
	if err := sb.Start(ctx.ServerConn, ch, req, ctx); err != nil {
		ctx.Warnf("Subsystem request %v failed: %v.", sb, err)
		ctx.SendSubsystemResult(srv.SubsystemResult{Err: trace.Wrap(err)})
		return trace.Wrap(err)
	}
	go func() {
		err := sb.Wait()
		log.Debugf("Subsystem %v finished with result: %v.", sb, err)
		ctx.SendSubsystemResult(srv.SubsystemResult{Err: trace.Wrap(err)})
	}()
	return nil
}

// handleEnv accepts environment variables sent by the client and stores them
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

// handleKeepAlive accepts and replies to keepalive@openssh.com requests.
func (s *Server) handleKeepAlive(req *ssh.Request) {
	log.Debugf("Received %q: WantReply: %v", req.Type, req.WantReply)

	// only reply if the sender actually wants a response
	if req.WantReply {
		err := req.Reply(true, nil)
		if err != nil {
			log.Warnf("Unable to reply to %q request: %v", req.Type, err)
			return
		}
	}

	log.Debugf("Replied to %q", req.Type)
}

// handleRecordingProxy responds to global out-of-band with a bool which
// indicates if it is in recording mode or not.
func (s *Server) handleRecordingProxy(req *ssh.Request) {
	var recordingProxy bool

	log.Debugf("Global request (%v, %v) received", req.Type, req.WantReply)

	if req.WantReply {
		// get the cluster config, if we can't get it, reply false
		clusterConfig, err := s.authService.GetClusterConfig()
		if err != nil {
			err := req.Reply(false, nil)
			if err != nil {
				log.Warnf("Unable to respond to global request (%v, %v): %v", req.Type, req.WantReply, err)
			}
			return
		}

		// reply true that we were able to process the message and reply with a
		// bool if we are in recording mode or not
		recordingProxy = clusterConfig.GetSessionRecording() == services.RecordAtProxy
		err = req.Reply(true, []byte(strconv.FormatBool(recordingProxy)))
		if err != nil {
			log.Warnf("Unable to respond to global request (%v, %v): %v: %v", req.Type, req.WantReply, recordingProxy, err)
			return
		}
	}

	log.Debugf("Replied to global request (%v, %v): %v", req.Type, req.WantReply, recordingProxy)
}

// handleVersionRequest replies with the Teleport version of the server.
func (s *Server) handleVersionRequest(req *ssh.Request) {
	err := req.Reply(true, []byte(teleport.Version))
	if err != nil {
		log.Debugf("Failed to reply to version request: %v.", err)
	}
}

// handleProxyJump handles ProxyJump request that is executed via direct tcp-ip dial on the proxy
func (s *Server) handleProxyJump(ctx context.Context, ccx *sshutils.ConnectionContext, identityContext srv.IdentityContext, ch ssh.Channel, req sshutils.DirectTCPIPReq) {
	// Create context for this channel. This context will be closed when the
	// session request is complete.
	ctx, scx, err := srv.NewServerContext(ctx, ccx, s, identityContext)
	if err != nil {
		log.Errorf("Unable to create connection context: %v.", err)
		writeStderr(ch, "Unable to create connection context.")
		return
	}
	scx.IsTestStub = s.isTestStub
	scx.AddCloser(ch)
	defer scx.Close()

	clusterConfig, err := s.GetAccessPoint().GetClusterConfig()
	if err != nil {
		log.Errorf("Unable to fetch cluster config: %v.", err)
		writeStderr(ch, "Unable to fetch cluster configuration.")
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
	if clusterConfig.GetSessionRecording() == services.RecordAtProxy {
		err = s.handleAgentForwardProxy(&ssh.Request{}, scx)
		if err != nil {
			log.Warningf("Failed to request agent in recording mode: %v", err)
			writeStderr(ch, "Failed to request agent")
			return
		}
	}

	// The keep-alive loop will keep pinging the remote server and after it has
	// missed a certain number of keep-alive requests it will cancel the
	// closeContext which signals the server to shutdown.
	go srv.StartKeepAliveLoop(srv.KeepAliveParams{
		Conns: []srv.RequestSender{
			scx.ServerConn,
		},
		Interval:     clusterConfig.GetKeepAliveInterval(),
		MaxCount:     clusterConfig.GetKeepAliveCountMax(),
		CloseContext: ctx,
		CloseCancel:  scx.CancelFunc(),
	})

	subsys, err := newProxySubsys(scx, s, proxySubsysRequest{
		host: req.Host,
		port: fmt.Sprintf("%v", req.Port),
	})
	if err != nil {
		log.Errorf("Unable instantiate proxy subsystem: %v.", err)
		writeStderr(ch, "Unable to instantiate proxy subsystem.")
		return
	}

	if err := subsys.Start(scx.ServerConn, ch, &ssh.Request{}, scx); err != nil {
		log.Errorf("Unable to start proxy subsystem: %v.", err)
		writeStderr(ch, "Unable to start proxy subsystem.")
		return
	}

	wch := make(chan struct{})
	go func() {
		defer close(wch)
		if err := subsys.Wait(); err != nil {
			log.Errorf("Proxy subsystem failed: %v.", err)
			writeStderr(ch, "Proxy subsystem failed.")
		}
	}()
	select {
	case <-wch:
	case <-ctx.Done():
	}
}

func (s *Server) replyError(ch ssh.Channel, req *ssh.Request, err error) {
	log.Error(err)
	message := trace.UserMessage(err)
	writeStderr(ch, message)
	if req.WantReply {
		if err := req.Reply(false, []byte(message)); err != nil {
			log.Warnf("Failed to reply to %q request: %v", req.Type, err)
		}
	}
}

func (s *Server) parseSubsystemRequest(req *ssh.Request, ctx *srv.ServerContext) (srv.Subsystem, error) {
	var r sshutils.SubsystemReq
	if err := ssh.Unmarshal(req.Payload, &r); err != nil {
		return nil, trace.BadParameter("failed to parse subsystem request: %v", err)
	}
	if s.proxyMode && strings.HasPrefix(r.Name, "proxy:") {
		return parseProxySubsys(r.Name, s, ctx)
	}
	if s.proxyMode && strings.HasPrefix(r.Name, "proxysites") {
		return parseProxySitesSubsys(r.Name, s)
	}
	return nil, trace.BadParameter("unrecognized subsystem: %v", r.Name)
}

func writeStderr(ch ssh.Channel, msg string) {
	if _, err := fmt.Fprint(ch.Stderr(), msg); err != nil {
		log.Warnf("Failed writing to ssh.Channel.Stderr(): %v", err)
	}
}

func rejectChannel(ch ssh.NewChannel, reason ssh.RejectionReason, msg string) {
	if err := ch.Reject(reason, msg); err != nil {
		log.Warnf("Failed to reject new ssh.Channel: %v", err)
	}
}
