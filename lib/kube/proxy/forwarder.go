/*
Copyright 2018-2019 Gravitational, Inc.

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

package proxy

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/oxy/forward"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"
	utilexec "k8s.io/client-go/util/exec"
)

// ForwarderConfig specifies configuration for proxy forwarder
type ForwarderConfig struct {
	// Tunnel is the teleport reverse tunnel server
	Tunnel reversetunnel.Server
	// ClusterName is a local cluster name
	ClusterName string
	// Keygen points to a key generator implementation
	Keygen sshca.Authority
	// Auth authenticates user
	Auth auth.Authorizer
	// Client is a proxy client
	Client auth.ClientI
	// DataDir is a data dir to store logs
	DataDir string
	// Namespace is a namespace of the proxy server (not a K8s namespace)
	Namespace string
	// AccessPoint is a caching access point to auth server
	// for caching common requests to the backend
	AccessPoint auth.AccessPoint
	// AuditLog is audit log to send events to
	AuditLog events.IAuditLog
	// ServerID is a unique ID of a proxy server
	ServerID string
	// ClusterOverride if set, routes all requests
	// to the cluster name, used in tests
	ClusterOverride string
	// Context passes the optional external context
	// passing global close to all forwarder operations
	Context context.Context
	// KubeconfigPath is a path to kubernetes configuration
	KubeconfigPath string
	// Clock is a server clock, could be overridden in tests
	Clock clockwork.Clock
}

// CheckAndSetDefaults checks and sets default values
func (f *ForwarderConfig) CheckAndSetDefaults() error {
	if f.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	if f.AccessPoint == nil {
		return trace.BadParameter("missing parameter AccessPoint")
	}
	if f.Auth == nil {
		return trace.BadParameter("missing parameter Auth")
	}
	if f.Tunnel == nil {
		return trace.BadParameter("missing parameter Tunnel")
	}
	if f.ClusterName == "" {
		return trace.BadParameter("missing parameter LocalCluster")
	}
	if f.Keygen == nil {
		return trace.BadParameter("missing parameter Keygen")
	}
	if f.DataDir == "" {
		return trace.BadParameter("missing parameter DataDir")
	}
	if f.ServerID == "" {
		return trace.BadParameter("missing parameter ServerID")
	}
	if f.Namespace == "" {
		f.Namespace = defaults.Namespace
	}
	if f.Context == nil {
		f.Context = context.TODO()
	}
	if f.Clock == nil {
		f.Clock = clockwork.NewRealClock()
	}
	return nil
}

// NewForwarder returns new instance of Kubernetes request
// forwarding proxy.
func NewForwarder(cfg ForwarderConfig) (*Forwarder, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		trace.Component: teleport.Component(teleport.ComponentKube),
	})

	creds, err := getKubeCreds(log, cfg.KubeconfigPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterSessions, err := ttlmap.New(defaults.ClientCacheSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	closeCtx, close := context.WithCancel(cfg.Context)
	fwd := &Forwarder{
		creds:           creds,
		Entry:           log,
		Router:          *httprouter.New(),
		ForwarderConfig: cfg,
		clusterSessions: clusterSessions,
		activeRequests:  make(map[string]context.Context),
		ctx:             closeCtx,
		close:           close,
	}

	fwd.POST("/api/:ver/namespaces/:podNamespace/pods/:podName/exec", fwd.withAuth(fwd.exec))
	fwd.GET("/api/:ver/namespaces/:podNamespace/pods/:podName/exec", fwd.withAuth(fwd.exec))

	fwd.POST("/api/:ver/namespaces/:podNamespace/pods/:podName/attach", fwd.withAuth(fwd.exec))
	fwd.GET("/api/:ver/namespaces/:podNamespace/pods/:podName/attach", fwd.withAuth(fwd.exec))

	fwd.POST("/api/:ver/namespaces/:podNamespace/pods/:podName/portforward", fwd.withAuth(fwd.portForward))
	fwd.GET("/api/:ver/namespaces/:podNamespace/pods/:podName/portforward", fwd.withAuth(fwd.portForward))

	fwd.NotFound = fwd.withAuthStd(fwd.catchAll)

	if cfg.ClusterOverride != "" {
		fwd.Debugf("Cluster override is set, forwarder will send all requests to remote cluster %v.", cfg.ClusterOverride)
	}
	return fwd, nil
}

// Forwarder intercepts kubernetes requests, acting as Kubernetes API proxy.
// it blindly forwards most of the requests on HTTPS protocol layer,
// however some requests like exec sessions it intercepts and records.
type Forwarder struct {
	sync.Mutex
	*log.Entry
	httprouter.Router
	ForwarderConfig
	// clusterSessions is an expiring cache associated with authenticated
	// user connected to a remote cluster, session is invalidated
	// if user changes kubernetes groups via RBAC or cache has expired
	// TODO(klizhentas): flush certs on teleport CA rotation?
	clusterSessions *ttlmap.TTLMap
	// activeRequests is a map used to serialize active CSR requests to the auth server
	activeRequests map[string]context.Context
	// close is a close function
	close context.CancelFunc
	// ctx is a global context signalling exit
	ctx context.Context
	// creds contain kubernetes credentials shared with a proxy process,
	// could be a service account token or client X509 credentials.
	//
	// Note: creds can be nil.
	creds *kubeCreds
}

// Close signals close to all outstanding or background operations
// to complete
func (f *Forwarder) Close() error {
	f.close()
	return nil
}

// authContext is a context of authenticated user,
// contains information about user, target cluster and authenticated groups
type authContext struct {
	auth.AuthContext
	kubeGroups    map[string]struct{}
	kubeUsers     map[string]struct{}
	cluster       cluster
	clusterConfig services.ClusterConfig
	// clientIdleTimeout sets information on client idle timeout
	clientIdleTimeout time.Duration
	// disconnectExpiredCert if set, controls the time when the connection
	// should be disconnected because the client cert expires
	disconnectExpiredCert time.Time
	// sessionTTL specifies the duration of the user's session
	sessionTTL time.Duration
}

func (c authContext) String() string {
	return fmt.Sprintf("user: %v, users: %v, groups: %v, cluster: %v", c.User.GetName(), c.kubeUsers, c.kubeGroups, c.cluster.GetName())
}

func (c *authContext) key() string {
	// it is important that the context key contains user, kubernetes groups and certificate expiry,
	// so that new logins with different parameters will not reuse this context
	return fmt.Sprintf("%v:%v:%v:%v:%v", c.cluster.GetName(), c.User.GetName(), c.kubeUsers, c.kubeGroups, c.disconnectExpiredCert.UTC().Unix())
}

// cluster represents cluster information, name of the cluster
// target address and custom dialer
type cluster struct {
	remoteAddr utils.NetAddr
	reversetunnel.RemoteSite
	targetAddr string
	isRemote   bool
}

func (c *cluster) Dial(_, _ string) (net.Conn, error) {
	return c.RemoteSite.DialTCP(reversetunnel.DialParams{
		From: &c.remoteAddr,
		To:   &utils.NetAddr{AddrNetwork: "tcp", Addr: c.targetAddr},
	})
}

func (c *cluster) DialWithContext(ctx context.Context, _, _ string) (net.Conn, error) {
	return c.RemoteSite.DialTCP(reversetunnel.DialParams{
		From: &c.remoteAddr,
		To:   &utils.NetAddr{AddrNetwork: "tcp", Addr: c.targetAddr},
	})
}

// handlerWithAuthFunc is http handler with passed auth context
type handlerWithAuthFunc func(ctx *authContext, w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error)

// handlerWithAuthFuncStd is http handler with passed auth context
type handlerWithAuthFuncStd func(ctx *authContext, w http.ResponseWriter, r *http.Request) (interface{}, error)

// authenticate function authenticates request
func (f *Forwarder) authenticate(req *http.Request) (*authContext, error) {
	const accessDeniedMsg = "[00] access denied"

	var isRemoteUser bool
	userTypeI := req.Context().Value(auth.ContextUser)
	switch userTypeI.(type) {
	case auth.LocalUser:

	case auth.RemoteUser:
		isRemoteUser = true
	case auth.BuiltinRole:
		f.Warningf("Denying proxy access to unauthenticated user of type %T - this can sometimes be caused by inadvertently using an HTTP load balancer instead of a TCP load balancer on the Kubernetes port.", userTypeI)
		return nil, trace.AccessDenied(accessDeniedMsg)
	default:
		f.Warningf("Denying proxy access to unsupported user type: %T.", userTypeI)
		return nil, trace.AccessDenied(accessDeniedMsg)
	}

	userContext, err := f.Auth.Authorize(req.Context())
	if err != nil {
		switch {
		// propagate connection problem error so we can differentiate
		// between connection failed and access denied
		case trace.IsConnectionProblem(err):
			return nil, trace.ConnectionProblem(err, "[07] failed to connect to the database")
		case trace.IsAccessDenied(err):
			// don't print stack trace, just log the warning
			f.Warn(err)
			return nil, trace.AccessDenied(accessDeniedMsg)
		default:
			f.Warn(trace.DebugReport(err))
			return nil, trace.AccessDenied(accessDeniedMsg)
		}
	}
	peers := req.TLS.PeerCertificates
	if len(peers) > 1 {
		// when turning intermediaries on, don't forget to verify
		// https://github.com/kubernetes/kubernetes/pull/34524/files#diff-2b283dde198c92424df5355f39544aa4R59
		return nil, trace.AccessDenied("access denied: intermediaries are not supported")
	}
	if len(peers) == 0 {
		return nil, trace.AccessDenied("access denied: only mutual TLS authentication is supported")
	}
	clientCert := peers[0]
	authContext, err := f.setupContext(*userContext, req, isRemoteUser, clientCert.NotAfter)
	if err != nil {
		f.Warn(err.Error())
		return nil, trace.AccessDenied(accessDeniedMsg)
	}
	return authContext, nil
}

func (f *Forwarder) withAuthStd(handler handlerWithAuthFuncStd) http.HandlerFunc {
	return httplib.MakeStdHandler(func(w http.ResponseWriter, req *http.Request) (interface{}, error) {
		authContext, err := f.authenticate(req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return handler(authContext, w, req)
	})
}

func (f *Forwarder) withAuth(handler handlerWithAuthFunc) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, req *http.Request, p httprouter.Params) (interface{}, error) {
		authContext, err := f.authenticate(req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return handler(authContext, w, req, p)
	})
}

func (f *Forwarder) setupContext(ctx auth.AuthContext, req *http.Request, isRemoteUser bool, certExpires time.Time) (*authContext, error) {
	roles := ctx.Checker

	clusterConfig, err := f.AccessPoint.GetClusterConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// adjust session ttl to the smaller of two values: the session
	// ttl requested in tsh or the session ttl for the role.
	sessionTTL := roles.AdjustSessionTTL(time.Hour)

	// check signing TTL and return a list of allowed logins
	kubeGroups, kubeUsers, err := roles.CheckKubeGroupsAndUsers(sessionTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// By default, if no kubernetes_users is set (which will be a majority),
	// user will impersonate themselves, which is the backwards-compatible behavior.
	if len(kubeUsers) == 0 {
		kubeUsers = append(kubeUsers, ctx.User.GetName())
	}

	// KubeSystemAuthenticated is a builtin group that allows
	// any user to access common API methods, e.g. discovery methods
	// required for initial client usage, without it, restricted user's
	// kubectl clients will not work
	if !utils.SliceContainsStr(kubeGroups, teleport.KubeSystemAuthenticated) {
		kubeGroups = append(kubeGroups, teleport.KubeSystemAuthenticated)
	}

	var isRemoteCluster bool
	targetCluster, err := f.Tunnel.GetSite(f.ClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ctx.Identity.RouteToCluster != "" {
		targetCluster, err = f.Tunnel.GetSite(ctx.Identity.RouteToCluster)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		isRemoteCluster = targetCluster.GetName() != f.ClusterName
	} else {
		// DELETE IN(4.3.0)
		// This logic is deprecated and after the second upgrade, will not be used
		// by the newer post 4.2.0 clients, so will be safe to remove
		for _, remoteCluster := range f.Tunnel.GetSites() {
			encodedName := kubeutils.EncodeClusterName(remoteCluster.GetName())
			if strings.HasPrefix(req.Host, remoteCluster.GetName()+".") || strings.HasPrefix(req.Host, encodedName+".") {
				f.Debugf("Going to proxy to cluster: %v based on matching host prefix %v.", remoteCluster.GetName(), req.Host)
				targetCluster = remoteCluster
				isRemoteCluster = remoteCluster.GetName() != f.ClusterName
				break
			}
			if f.ClusterOverride != "" && f.ClusterOverride == remoteCluster.GetName() {
				f.Debugf("Going to proxy to cluster: %v based on override %v.", remoteCluster.GetName(), f.ClusterOverride)
				targetCluster = remoteCluster
				isRemoteCluster = remoteCluster.GetName() != f.ClusterName
				f.Debugf("Override isRemoteCluster: %v %v %v", isRemoteCluster, remoteCluster.GetName(), f.ClusterName)
				break
			}
		}
	}
	if targetCluster.GetName() != f.ClusterName && isRemoteUser {
		return nil, trace.AccessDenied("access denied: remote user can not access remote cluster")
	}
	// If this proxy didn't get a kubeconfig at startup, it can only forward
	// requests to remote clusters. Since this is not a remote cluster request,
	// we can't process this request.
	if f.creds == nil && !isRemoteCluster {
		return nil, trace.NotFound("this Teleport proxy is not configured for direct Kubernetes access; you likely need to 'tsh login' into a leaf cluster")
	}

	authCtx := &authContext{
		clientIdleTimeout: roles.AdjustClientIdleTimeout(clusterConfig.GetClientIdleTimeout()),
		sessionTTL:        sessionTTL,
		AuthContext:       ctx,
		kubeGroups:        utils.StringsSet(kubeGroups),
		kubeUsers:         utils.StringsSet(kubeUsers),
		clusterConfig:     clusterConfig,
		cluster: cluster{
			remoteAddr: utils.NetAddr{AddrNetwork: "tcp", Addr: req.RemoteAddr},
			RemoteSite: targetCluster,
			isRemote:   isRemoteCluster,
		},
	}

	disconnectExpiredCert := roles.AdjustDisconnectExpiredCert(clusterConfig.GetDisconnectExpiredCert())
	if !certExpires.IsZero() && disconnectExpiredCert {
		authCtx.disconnectExpiredCert = certExpires
	}

	return authCtx, nil
}

// exec forwards all exec requests to the target server, captures
// all output from the session
func (f *Forwarder) exec(ctx *authContext, w http.ResponseWriter, req *http.Request, p httprouter.Params) (interface{}, error) {
	f.Debugf("Exec %v.", req.URL.String())
	q := req.URL.Query()
	request := remoteCommandRequest{
		podNamespace:       p.ByName("podNamespace"),
		podName:            p.ByName("podName"),
		containerName:      q.Get("container"),
		cmd:                q["command"],
		stdin:              utils.AsBool(q.Get("stdin")),
		stdout:             utils.AsBool(q.Get("stdout")),
		stderr:             utils.AsBool(q.Get("stderr")),
		tty:                utils.AsBool(q.Get("tty")),
		httpRequest:        req,
		httpResponseWriter: w,
		context:            req.Context(),
	}

	var recorder events.SessionRecorder
	sessionID := session.NewID()
	var err error
	if request.tty {
		// create session recorder
		// get the audit log from the server and create a session recorder. this will
		// be a discard audit log if the proxy is in recording mode and a teleport
		// node so we don't create double recordings.
		recorder, err = events.NewForwardRecorder(events.ForwardRecorderConfig{
			DataDir:        filepath.Join(f.DataDir, teleport.LogsDir),
			SessionID:      sessionID,
			Namespace:      f.Namespace,
			RecordSessions: ctx.clusterConfig.GetSessionRecording() != services.RecordOff,
			Component:      teleport.Component(teleport.ComponentSession, teleport.ComponentKube),
			ForwardTo:      f.AuditLog,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer recorder.Close()
		request.onResize = func(resize remotecommand.TerminalSize) {
			params := session.TerminalParams{
				W: int(resize.Width),
				H: int(resize.Height),
			}
			// Build the resize event.
			resizeEvent := events.EventFields{
				events.EventProtocol:  events.EventProtocolKube,
				events.EventType:      events.ResizeEvent,
				events.EventNamespace: f.Namespace,
				events.SessionEventID: sessionID,
				events.EventLogin:     ctx.User.GetName(),
				events.EventUser:      ctx.User.GetName(),
				events.TerminalSize:   params.Serialize(),
			}

			// Report the updated window size to the event log (this is so the sessions
			// can be replayed correctly).
			if err := recorder.GetAuditLog().EmitAuditEvent(events.TerminalResize, resizeEvent); err != nil {
				f.Warnf("Failed to emit terminal resize event: %v", err)
			}
		}
	}

	sess, err := f.getOrCreateClusterSession(*ctx)
	if err != nil {
		// This error goes to kubernetes client and is not visible in the logs
		// of the teleport server if not logged here.
		f.Errorf("Failed to create cluster session: %v.", err)
		return nil, trace.Wrap(err)
	}

	if request.tty {
		// Emit "new session created" event. There are no initial terminal
		// parameters per k8s protocol, so set up with any default
		termParams := session.TerminalParams{
			W: 100,
			H: 100,
		}
		if err := recorder.GetAuditLog().EmitAuditEvent(events.SessionStart, events.EventFields{
			events.EventProtocol:   events.EventProtocolKube,
			events.EventNamespace:  f.Namespace,
			events.SessionEventID:  string(sessionID),
			events.SessionServerID: f.ServerID,
			events.EventLogin:      ctx.User.GetName(),
			events.EventUser:       ctx.User.GetName(),
			events.LocalAddr:       sess.cluster.targetAddr,
			events.RemoteAddr:      req.RemoteAddr,
			events.TerminalSize:    termParams.Serialize(),
		}); err != nil {
			f.Warnf("Failed to emit session start event: %v", err)
		}
	}

	if err := f.setupForwardingHeaders(sess, req); err != nil {
		return nil, trace.Wrap(err)
	}

	proxy, err := createRemoteCommandProxy(request)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxy.Close()

	f.Debugf("Created streams, getting executor.")

	executor, err := f.getExecutor(*ctx, sess, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	streamOptions := proxy.options()

	if request.tty {
		// capture stderr and stdout writes to session recorder
		streamOptions.Stdout = utils.NewBroadcastWriter(streamOptions.Stdout, recorder)
		streamOptions.Stderr = utils.NewBroadcastWriter(streamOptions.Stderr, recorder)
	}

	err = executor.Stream(streamOptions)
	if err := proxy.sendStatus(err); err != nil {
		f.Warningf("Failed to send status: %v. Exec command was aborted by client.", err)
		return nil, trace.Wrap(err)
	}

	if request.tty {
		// send an event indicating that this session has ended
		if err := recorder.GetAuditLog().EmitAuditEvent(events.SessionEnd, events.EventFields{
			events.EventProtocol:  events.EventProtocolKube,
			events.SessionEventID: sessionID,
			events.EventUser:      ctx.User.GetName(),
			events.EventNamespace: f.Namespace,
		}); err != nil {
			f.Warnf("Failed to emit session end event: %v", err)
		}
	} else {
		f.Debugf("No tty, sending exec event.")
		// send an exec event
		fields := events.EventFields{
			events.EventProtocol:    events.EventProtocolKube,
			events.ExecEventCommand: strings.Join(request.cmd, " "),
			events.EventLogin:       ctx.User.GetName(),
			events.EventUser:        ctx.User.GetName(),
			events.LocalAddr:        sess.cluster.targetAddr,
			events.RemoteAddr:       req.RemoteAddr,
			events.EventNamespace:   f.Namespace,
		}
		if err != nil {
			fields[events.ExecEventError] = err.Error()
			if exitErr, ok := err.(utilexec.ExitError); ok && exitErr.Exited() {
				fields[events.ExecEventCode] = fmt.Sprintf("%d", exitErr.ExitStatus())
			}
			if err := f.AuditLog.EmitAuditEvent(events.ExecFailure, fields); err != nil {
				f.Warnf("Failed to emit exec failure event: %v", err)
			}
		} else {
			if err := f.AuditLog.EmitAuditEvent(events.Exec, fields); err != nil {
				f.Warnf("Failed to emit exec failure event: %v", err)
			}
		}
	}

	f.Debugf("Exited successfully.")
	return nil, nil
}

// portForward starts port forwarding to the remote cluster
func (f *Forwarder) portForward(ctx *authContext, w http.ResponseWriter, req *http.Request, p httprouter.Params) (interface{}, error) {
	f.Debugf("Port forward: %v. req headers: %v", req.URL.String(), req.Header)
	sess, err := f.getOrCreateClusterSession(*ctx)
	if err != nil {
		// This error goes to kubernetes client and is not visible in the logs
		// of the teleport server if not logged here.
		f.Errorf("Failed to create cluster session: %v.", err)
		return nil, trace.Wrap(err)
	}

	if err := f.setupForwardingHeaders(sess, req); err != nil {
		f.Debugf("DENIED Port forward: %v.", req.URL.String())
		return nil, trace.Wrap(err)
	}

	dialer, err := f.getDialer(*ctx, sess, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	onPortForward := func(addr string, success bool) {
		event := events.PortForward
		if !success {
			event = events.PortForwardFailure
		}
		if err := f.AuditLog.EmitAuditEvent(event, events.EventFields{
			events.EventProtocol:      events.EventProtocolKube,
			events.PortForwardAddr:    addr,
			events.PortForwardSuccess: success,
			events.EventLogin:         ctx.User.GetName(),
			events.EventUser:          ctx.User.GetName(),
			events.LocalAddr:          sess.cluster.targetAddr,
			events.RemoteAddr:         req.RemoteAddr,
		}); err != nil {
			f.Warnf("Failed to emit port-forward audit event: %v", err)
		}
	}

	q := req.URL.Query()
	request := portForwardRequest{
		podNamespace:       p.ByName("podNamespace"),
		podName:            p.ByName("podName"),
		ports:              q["ports"],
		context:            req.Context(),
		httpRequest:        req,
		httpResponseWriter: w,
		onPortForward:      onPortForward,
		targetDialer:       dialer,
	}
	f.Debugf("Starting %v.", request)
	err = runPortForwarding(request)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	f.Debugf("Done %v.", request)
	return nil, nil
}

const (
	// ImpersonateHeaderPrefix is K8s impersonation prefix for impersonation feature:
	// https://kubernetes.io/docs/reference/access-authn-authz/authentication/#user-impersonation
	ImpersonateHeaderPrefix = "Impersonate-"
	// ImpersonateUserHeader is impersonation header for users
	ImpersonateUserHeader = "Impersonate-User"
	// ImpersonateGroupHeader is K8s impersonation header for user
	ImpersonateGroupHeader = "Impersonate-Group"
	// ImpersonationRequestDeniedMessage is access denied message for impersonation
	ImpersonationRequestDeniedMessage = "impersonation request has been denied"
)

func (f *Forwarder) setupForwardingHeaders(sess *clusterSession, req *http.Request) error {
	if err := setupImpersonationHeaders(f.Entry, sess.authContext, req.Header); err != nil {
		return trace.Wrap(err)
	}

	// Setup scheme, override target URL to the destination address
	req.URL.Scheme = "https"
	req.URL.Host = sess.cluster.targetAddr
	req.RequestURI = req.URL.Path + "?" + req.URL.RawQuery

	// add origin headers so the service consuming the request on the other site
	// is aware of where it came from
	req.Header.Add("X-Forwarded-Proto", "https")
	req.Header.Add("X-Forwarded-Host", req.Host)
	req.Header.Add("X-Forwarded-Path", req.URL.Path)
	req.Header.Add("X-Forwarded-For", req.RemoteAddr)

	return nil
}

// setupImpersonationHeaders sets up Impersonate-User and Impersonate-Group headers
func setupImpersonationHeaders(log log.FieldLogger, ctx authContext, headers http.Header) error {
	var impersonateUser string
	var impersonateGroups []string
	for header, values := range headers {
		if !strings.HasPrefix(header, "Impersonate-") {
			continue
		}
		switch header {
		case ImpersonateUserHeader:
			if impersonateUser != "" {
				return trace.AccessDenied("%v, user already specified to %q", ImpersonationRequestDeniedMessage, impersonateUser)
			}
			if len(values) == 0 || len(values) > 1 {
				return trace.AccessDenied("%v, invalid user header %q", ImpersonationRequestDeniedMessage, values)
			}
			impersonateUser = values[0]
			if _, ok := ctx.kubeUsers[impersonateUser]; !ok {
				return trace.AccessDenied("%v, user header %q is not allowed in roles", ImpersonationRequestDeniedMessage, impersonateUser)
			}
		case ImpersonateGroupHeader:
			for _, group := range values {
				if _, ok := ctx.kubeGroups[group]; !ok {
					return trace.AccessDenied("%v, group header %q value is not allowed in roles", ImpersonationRequestDeniedMessage, group)
				}
				impersonateGroups = append(impersonateGroups, group)
			}
		default:
			return trace.AccessDenied("%v, unsupported impersonation header %q", ImpersonationRequestDeniedMessage, header)
		}
	}

	impersonateGroups = utils.Deduplicate(impersonateGroups)

	// By default, if no kubernetes_users is set (which will be a majority),
	// user will impersonate themselves, which is the backwards-compatible behavior.
	//
	// As long as at least one `kubernetes_users` is set, the forwarder will start
	// limiting the list of users allowed by the client to impersonate.
	//
	// If the users' role set does not include actual user name, it will be rejected,
	// otherwise there will be no way to exclude the user from the list).
	//
	// If the `kubernetes_users` role set includes only one user
	// (quite frequently that's the real intent), teleport will default to it,
	// otherwise it will refuse to select.
	//
	// This will enable the use case when `kubernetes_users` has just one field to
	// link the user identity with the IAM role, for example `IAM#{{external.email}}`
	//
	if impersonateUser == "" {
		switch len(ctx.kubeUsers) {
		// this is currently not possible as kube users have at least one
		// user (user name), but in case if someone breaks it, catch here
		case 0:
			return trace.AccessDenied("assumed at least one user to be present")
		// if there is deterministic choice, make it to improve user experience
		case 1:
			for user := range ctx.kubeUsers {
				impersonateUser = user
				break
			}
		default:
			return trace.AccessDenied(
				"please select a user to impersonate, refusing to select a user due to several kuberenetes_users set up for this user")
		}
	}

	if len(impersonateGroups) == 0 {
		for group := range ctx.kubeGroups {
			impersonateGroups = append(impersonateGroups, group)
		}
	}

	if !ctx.cluster.isRemote {
		headers.Set(ImpersonateUserHeader, impersonateUser)

		// Make sure to overwrite the exiting headers, instead of appending to
		// them.
		headers[ImpersonateGroupHeader] = nil
		for _, group := range impersonateGroups {
			headers.Add(ImpersonateGroupHeader, group)
		}
	}
	return nil
}

// catchAll forwards all HTTP requests to the target k8s API server
func (f *Forwarder) catchAll(ctx *authContext, w http.ResponseWriter, req *http.Request) (interface{}, error) {
	sess, err := f.getOrCreateClusterSession(*ctx)
	if err != nil {
		// This error goes to kubernetes client and is not visible in the logs
		// of the teleport server if not logged here.
		f.Errorf("Failed to create cluster session: %v.", err)
		return nil, trace.Wrap(err)
	}
	if err := f.setupForwardingHeaders(sess, req); err != nil {
		// This error goes to kubernetes client and is not visible in the logs
		// of the teleport server if not logged here.
		f.Errorf("Failed to set up forwarding headers: %v.", err)
		return nil, trace.Wrap(err)
	}
	sess.forwarder.ServeHTTP(w, req)
	return nil, nil
}

func (f *Forwarder) getExecutor(ctx authContext, sess *clusterSession, req *http.Request) (remotecommand.Executor, error) {
	upgradeRoundTripper := NewSpdyRoundTripperWithDialer(roundTripperConfig{
		ctx:             req.Context(),
		authCtx:         ctx,
		dial:            sess.DialWithContext,
		tlsConfig:       sess.tlsConfig,
		followRedirects: true,
	})
	rt, err := f.creds.wrapTransport(upgradeRoundTripper)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return remotecommand.NewSPDYExecutorForTransports(rt, upgradeRoundTripper, req.Method, req.URL)
}

func (f *Forwarder) getDialer(ctx authContext, sess *clusterSession, req *http.Request) (httpstream.Dialer, error) {
	upgradeRoundTripper := NewSpdyRoundTripperWithDialer(roundTripperConfig{
		ctx:             req.Context(),
		authCtx:         ctx,
		dial:            sess.DialWithContext,
		tlsConfig:       sess.tlsConfig,
		followRedirects: true,
	})
	rt, err := f.creds.wrapTransport(upgradeRoundTripper)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := &http.Client{
		Transport: rt,
	}

	return spdy.NewDialer(upgradeRoundTripper, client, req.Method, req.URL), nil
}

// clusterSession contains authenticated user session to the target cluster:
// x509 short lived credentials, forwarding proxies and other data
type clusterSession struct {
	authContext
	parent    *Forwarder
	tlsConfig *tls.Config
	forwarder *forward.Forwarder
}

func (s *clusterSession) monitorConn(conn net.Conn, err error) (net.Conn, error) {
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if s.disconnectExpiredCert.IsZero() && s.clientIdleTimeout == 0 {
		return conn, nil
	}
	ctx, cancel := context.WithCancel(s.parent.ctx)
	tc := &trackingConn{
		Conn:   conn,
		clock:  s.parent.Clock,
		ctx:    ctx,
		cancel: cancel,
	}

	mon, err := srv.NewMonitor(srv.MonitorConfig{
		DisconnectExpiredCert: s.disconnectExpiredCert,
		ClientIdleTimeout:     s.clientIdleTimeout,
		Clock:                 s.parent.Clock,
		Tracker:               tc,
		Conn:                  tc,
		Context:               ctx,
		TeleportUser:          s.User.GetName(),
		ServerID:              s.parent.ServerID,
		Audit:                 s.parent.AuditLog,
		Entry:                 s.parent.Entry,
	})
	if err != nil {
		tc.Close()
		return nil, trace.Wrap(err)
	}
	go mon.Start()
	return tc, nil
}

func (s *clusterSession) Dial(network, addr string) (net.Conn, error) {
	return s.monitorConn(s.cluster.Dial(network, addr))
}

func (s *clusterSession) DialWithContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return s.monitorConn(s.cluster.DialWithContext(ctx, network, addr))
}

type trackingConn struct {
	sync.RWMutex
	net.Conn
	clock      clockwork.Clock
	lastActive time.Time
	ctx        context.Context
	cancel     context.CancelFunc
}

// Read reads data from the connection.
// Read can be made to time out and return an Error with Timeout() == true
// after a fixed time limit; see SetDeadline and SetReadDeadline.
func (t *trackingConn) Read(b []byte) (int, error) {
	n, err := t.Conn.Read(b)
	t.UpdateClientActivity()
	return n, err
}

func (t *trackingConn) Close() error {
	t.cancel()
	return t.Conn.Close()
}

// GetClientLastActive returns time when client was last active
func (t *trackingConn) GetClientLastActive() time.Time {
	t.RLock()
	defer t.RUnlock()
	return t.lastActive
}

// UpdateClientActivity sets last recorded client activity
func (t *trackingConn) UpdateClientActivity() {
	t.Lock()
	defer t.Unlock()
	t.lastActive = t.clock.Now().UTC()
}

func (f *Forwarder) getOrCreateClusterSession(ctx authContext) (*clusterSession, error) {
	client := f.getClusterSession(ctx)
	if client != nil {
		return client, nil
	}
	return f.serializedNewClusterSession(ctx)
}

func (f *Forwarder) getClusterSession(ctx authContext) *clusterSession {
	f.Lock()
	defer f.Unlock()
	creds, ok := f.clusterSessions.Get(ctx.key())
	if !ok {
		return nil
	}
	s := creds.(*clusterSession)
	if s.cluster.isRemote && s.cluster.RemoteSite.IsClosed() {
		f.Debugf("Found an existing clusterSession for remote cluster %q but it has been closed. Discarding it to create a new clusterSession.", ctx.cluster.GetName())
		f.clusterSessions.Remove(ctx.key())
		return nil
	}
	return s
}

func (f *Forwarder) serializedNewClusterSession(authContext authContext) (*clusterSession, error) {
	ctx, cancel := f.getOrCreateRequestContext(authContext.key())
	if cancel != nil {
		f.Debugf("Requesting new creds for %v.", authContext)
		defer cancel()
		return f.newClusterSession(authContext)
	}
	// cancel == nil means that another request is in progress, so simply wait until
	// it finishes or fails
	f.Debugf("Another request is in progress for %v, waiting until it gets completed.", authContext)
	select {
	case <-ctx.Done():
		sess := f.getClusterSession(authContext)
		if sess == nil {
			return nil, trace.BadParameter("failed to request certificate, try again")
		}
		return sess, nil
	case <-f.ctx.Done():
		return nil, trace.BadParameter("forwarder is closing, aborting the request")
	}
}

func (f *Forwarder) newClusterSession(ctx authContext) (*clusterSession, error) {
	var tlsConfig *tls.Config

	// For remote (trusted) clusters, generate a new teleport TLS client
	// certificate for the user via auth server. Effectively, impersonate the
	// user to the remote proxy.
	if ctx.cluster.isRemote {
		var err error
		tlsConfig, err = f.requestCertificate(ctx)
		if err != nil {
			f.Warningf("Failed to get certificate for %v: %v.", ctx, err)
			return nil, trace.AccessDenied("access denied: failed to authenticate with auth server")
		}
	} else {
		if f.creds == nil {
			return nil, trace.NotFound("this Teleport proxy is not configured for direct Kubernetes access; you likely need to 'tsh login' into a leaf cluster")
		}
		tlsConfig = f.creds.tlsConfig
	}

	// remote clusters use special hardcoded URL,
	// and use a special dialer
	if ctx.cluster.isRemote {
		ctx.cluster.targetAddr = reversetunnel.RemoteKubeProxy
	} else {
		ctx.cluster.targetAddr = f.creds.targetAddr
	}

	sess := &clusterSession{
		parent:      f,
		authContext: ctx,
		tlsConfig:   tlsConfig,
	}

	var transport http.RoundTripper = f.newTransport(sess.Dial, tlsConfig)

	// When running inside Kubernetes cluster or using auth/exec providers,
	// kubeconfig provides a transport wrapper that adds a bearer token to
	// requests
	//
	// When forwarding request to a remote cluster, this is not needed
	// as the proxy uses client cert auth to reach out to remote proxy.
	if !ctx.cluster.isRemote {
		var err error
		transport, err = f.creds.wrapTransport(transport)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	fwd, err := forward.New(
		forward.FlushInterval(100*time.Millisecond),
		forward.RoundTripper(transport),
		forward.WebsocketDial(sess.Dial),
		forward.Logger(log.StandardLogger()),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess.forwarder = fwd

	f.Lock()
	defer f.Unlock()

	sessI, ok := f.clusterSessions.Get(ctx.key())
	if ok {
		return sessI.(*clusterSession), nil
	}

	if err = f.clusterSessions.Set(ctx.key(), sess, ctx.sessionTTL); err != nil {
		return nil, trace.Wrap(err)
	}
	f.Debugf("Created new session for %v.", ctx)
	return sess, nil
}

// DialFunc is a network dialer function that returns a network connection
type DialFunc func(string, string) (net.Conn, error)

func (f *Forwarder) newTransport(dial DialFunc, tlsConfig *tls.Config) *http.Transport {
	return &http.Transport{
		Dial:            dial,
		TLSClientConfig: tlsConfig,
		// Increase the size of the connection pool. This substantially improves the
		// performance of Teleport under load as it reduces the number of TLS
		// handshakes performed.
		MaxIdleConns:        defaults.HTTPMaxIdleConns,
		MaxIdleConnsPerHost: defaults.HTTPMaxIdleConnsPerHost,
		// IdleConnTimeout defines the maximum amount of time before idle connections
		// are closed. Leaving this unset will lead to connections open forever and
		// will cause memory leaks in a long running process.
		IdleConnTimeout: defaults.HTTPIdleTimeout,
	}
}

// getOrCreateRequestContext creates a new certificate request for a given context,
// if there is no active CSR request in progress, or returns an existing one.
// if the new context has been created, cancel function is returned as a
// second argument. Caller should call this function to signal that CSR has been
// completed or failed.
func (f *Forwarder) getOrCreateRequestContext(key string) (context.Context, context.CancelFunc) {
	f.Lock()
	defer f.Unlock()
	ctx, ok := f.activeRequests[key]
	if ok {
		return ctx, nil
	}
	ctx, cancel := context.WithCancel(context.TODO())
	f.activeRequests[key] = ctx
	return ctx, func() {
		cancel()
		f.Lock()
		defer f.Unlock()
		delete(f.activeRequests, key)
	}
}

func (f *Forwarder) requestCertificate(ctx authContext) (*tls.Config, error) {
	f.Debugf("Requesting K8s cert for %v.", ctx)
	keyPEM, _, err := f.Keygen.GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	privateKey, err := ssh.ParseRawPrivateKey(keyPEM)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse private key")
	}

	// Note: ctx.Identity can potentially have temporary roles granted via
	// workflow API. Always use the Subject() method to preserve the roles from
	// caller's certificate.
	subject, err := ctx.Identity.Subject()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	csr := &x509.CertificateRequest{
		Subject: subject,
	}
	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, csr, privateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})

	response, err := f.Client.ProcessKubeCSR(auth.KubeCSR{
		Username:    ctx.User.GetName(),
		ClusterName: ctx.cluster.GetName(),
		CSR:         csrPEM,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	f.Debugf("Received valid K8s cert for %v.", ctx)

	cert, err := tls.X509KeyPair(response.Cert, keyPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pool := x509.NewCertPool()
	for _, certAuthority := range response.CertAuthorities {
		ok := pool.AppendCertsFromPEM(certAuthority)
		if !ok {
			return nil, trace.BadParameter("failed to append certificates, check that kubeconfig has correctly encoded certificate authority data")
		}
	}
	tlsConfig := &tls.Config{
		RootCAs:      pool,
		Certificates: []tls.Certificate{cert},
	}
	tlsConfig.BuildNameToCertificate()

	return tlsConfig, nil
}
