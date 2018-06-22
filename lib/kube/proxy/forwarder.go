package proxy

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
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
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/oxy/forward"
	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
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
	// TargetAddr is a target address
	TargetAddr string
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
	if f.TargetAddr == "" {
		f.TargetAddr = teleport.KubeServiceAddr
	}
	if f.Namespace == "" {
		f.Namespace = defaults.Namespace
	}
	return nil
}

// NewForwarder returns new instance of Kubernetes request
// forwarding proxy.
func NewForwarder(cfg ForwarderConfig) (*Forwarder, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	clusterSessions, err := ttlmap.New(defaults.ClientCacheSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fwd := &Forwarder{
		Entry: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.Component(teleport.ComponentKube),
		}),
		Router:          *httprouter.New(),
		ForwarderConfig: cfg,
		clusterSessions: clusterSessions,
	}

	fwd.POST("/api/:ver/namespaces/:podNamespace/pods/:podName/exec", fwd.withAuth(fwd.exec))
	fwd.GET("/api/:ver/namespaces/:podNamespace/pods/:podName/exec", fwd.withAuth(fwd.exec))

	fwd.POST("/api/:ver/namespaces/:podNamespace/pods/:podName/attach", fwd.withAuth(fwd.exec))
	fwd.GET("/api/:ver/namespaces/:podNamespace/pods/:podName/attach", fwd.withAuth(fwd.exec))

	fwd.POST("/api/:ver/namespaces/:podNamespace/pods/:podName/portforward", fwd.withAuth(fwd.portForward))
	fwd.GET("/api/:ver/namespaces/:podNamespace/pods/:podName/portforward", fwd.withAuth(fwd.portForward))

	fwd.NotFound = fwd.withAuthStd(fwd.catchAll)

	fwd.Debugf("Forwarder started, going to forward kubernetes requests to https://%v", cfg.TargetAddr)
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
	*logrus.Entry
	httprouter.Router
	ForwarderConfig
	// clusterSessions is an expiring cache associated with authenticated
	// user connected to a remote cluster, session is invalidated
	// if user changes kubernetes groups via RBAC or cache has expired
	// TODO(klizhentas): flush certs on teleport CA rotation?
	clusterSessions *ttlmap.TTLMap
}

// authContext is a context of authenticated user,
// contains information about user, target cluster and authenticated groups
type authContext struct {
	// sessionTTL specifies the duration of the user's session
	sessionTTL time.Duration
	auth.AuthContext
	kubeGroups []string
	cluster    cluster
}

func (c authContext) String() string {
	return fmt.Sprintf("user: %v, groups: %v, cluster: %v", c.User.GetName(), c.kubeGroups, c.cluster.GetName())
}

func (c *authContext) key() string {
	return fmt.Sprintf("%v:%v:%v", c.cluster.GetName(), c.User.GetName(), c.kubeGroups)
}

// cluster represents cluster information, name of the cluster
// target address and custom dialer
type cluster struct {
	remoteAddr utils.NetAddr
	reversetunnel.RemoteSite
	targetAddr string
}

func (c *cluster) Dial(_, _ string) (net.Conn, error) {
	return c.RemoteSite.Dial(
		&c.remoteAddr,
		&utils.NetAddr{AddrNetwork: "tcp", Addr: c.targetAddr},
		nil)
}

func (c *cluster) DialWithContext(ctx context.Context, _, _ string) (net.Conn, error) {
	return c.RemoteSite.Dial(
		&c.remoteAddr,
		&utils.NetAddr{AddrNetwork: "tcp", Addr: c.targetAddr},
		nil)
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
	authContext, err := f.setupContext(*userContext, req, isRemoteUser)
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

func (f *Forwarder) setupContext(ctx auth.AuthContext, req *http.Request, isRemoteUser bool) (*authContext, error) {
	roles := ctx.Checker

	// adjust session ttl to the smaller of two values: the session
	// ttl requested in tsh or the session ttl for the role.
	sessionTTL := roles.AdjustSessionTTL(time.Hour)

	// check signing TTL and return a list of allowed logins
	kubeGroups, err := roles.CheckKubeGroups(sessionTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var isRemoteCluster bool
	targetCluster, err := f.Tunnel.GetSite(f.ClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, remoteCluster := range f.Tunnel.GetSites() {
		if strings.HasSuffix(req.Host, remoteCluster.GetName()+".") {
			f.Debugf("Going to proxy to cluster: %v based on matching host suffix %v.", remoteCluster.GetName(), req.Host)
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
	if targetCluster.GetName() != f.ClusterName && isRemoteUser {
		return nil, trace.AccessDenied("access denied: remote user can not access remote cluster")
	}
	authCtx := &authContext{
		sessionTTL:  sessionTTL,
		AuthContext: ctx,
		kubeGroups:  kubeGroups,
		cluster: cluster{
			remoteAddr: utils.NetAddr{AddrNetwork: "tcp", Addr: req.RemoteAddr},
			RemoteSite: targetCluster,
			targetAddr: f.TargetAddr,
		},
	}
	if isRemoteCluster {
		authCtx.cluster.targetAddr = reversetunnel.RemoteKubeProxy
	}
	return authCtx, nil
}

// exec forwards all exec requests to the target server, captures
// all output from the session
func (f *Forwarder) exec(ctx *authContext, w http.ResponseWriter, req *http.Request, p httprouter.Params) (interface{}, error) {
	f.Debugf("Exec %v.", req.URL.String())
	clusterConfig, err := f.AccessPoint.GetClusterConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

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

	var recorder *events.SessionRecorder
	sessionID := session.NewID()
	if request.tty {
		// create session recorder
		// get the audit log from the server and create a session recorder. this will
		// be a discard audit log if the proxy is in recording mode and a teleport
		// node so we don't create double recordings.
		recorder, err = events.NewSessionRecorder(events.SessionRecorderConfig{
			DataDir:        filepath.Join(f.DataDir, teleport.LogsDir),
			SessionID:      sessionID,
			Namespace:      f.Namespace,
			RecordSessions: clusterConfig.GetSessionRecording() != services.RecordOff,
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
			recorder.AuditLog.EmitAuditEvent(events.ResizeEvent, resizeEvent)
		}
	}

	sess, err := f.getOrCreateClusterSession(*ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if request.tty {
		// Emit "new session created" event. There are no initial terminal
		// parameters per k8s protocol, so set up with any default
		termParams := session.TerminalParams{
			W: 100,
			H: 100,
		}
		recorder.AuditLog.EmitAuditEvent(events.SessionStartEvent, events.EventFields{
			events.EventProtocol:   events.EventProtocolKube,
			events.EventNamespace:  f.Namespace,
			events.SessionEventID:  string(sessionID),
			events.SessionServerID: f.ServerID,
			events.EventLogin:      ctx.User.GetName(),
			events.EventUser:       ctx.User.GetName(),
			events.LocalAddr:       sess.targetAddr,
			events.RemoteAddr:      req.RemoteAddr,
			events.TerminalSize:    termParams.Serialize(),
		})
	}

	setupForwardingHeaders(sess, req)

	proxy, err := createRemoteCommandProxy(request)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxy.Close()

	f.Debugf("Created streams, getting executor.")

	executor, err := f.getExecutor(sess, req)
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
		recorder.AuditLog.EmitAuditEvent(events.SessionEndEvent, events.EventFields{
			events.EventProtocol:  events.EventProtocolKube,
			events.SessionEventID: sessionID,
			events.EventUser:      ctx.User.GetName(),
			events.EventNamespace: f.Namespace,
		})
	} else {
		f.Debugf("No tty, sending exec event.")
		// send an exec event
		fields := events.EventFields{
			events.EventProtocol:    events.EventProtocolKube,
			events.ExecEventCommand: strings.Join(request.cmd, " "),
			events.EventLogin:       ctx.User.GetName(),
			events.EventUser:        ctx.User.GetName(),
			events.LocalAddr:        sess.targetAddr,
			events.RemoteAddr:       req.RemoteAddr,
			events.EventNamespace:   f.Namespace,
		}
		if err != nil {
			fields[events.ExecEventError] = err.Error()
			if exitErr, ok := err.(utilexec.ExitError); ok && exitErr.Exited() {
				fields[events.ExecEventCode] = fmt.Sprintf("%d", exitErr.ExitStatus())
			}
		}
		f.AuditLog.EmitAuditEvent(events.ExecEvent, fields)
	}

	f.Debugf("Exited successfully.")
	return nil, nil
}

// portForward starts port forwarding to the remote cluster
func (f *Forwarder) portForward(ctx *authContext, w http.ResponseWriter, req *http.Request, p httprouter.Params) (interface{}, error) {
	f.Debugf("Port forward: %v.", req.URL.String())
	sess, err := f.getOrCreateClusterSession(*ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	setupForwardingHeaders(sess, req)

	dialer, err := f.getDialer(sess, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	onPortForward := func(addr string, success bool) {
		f.AuditLog.EmitAuditEvent(events.PortForwardEvent, events.EventFields{
			events.EventProtocol:      events.EventProtocolKube,
			events.PortForwardAddr:    addr,
			events.PortForwardSuccess: success,
			events.EventLogin:         ctx.User.GetName(),
			events.EventUser:          ctx.User.GetName(),
			events.LocalAddr:          sess.targetAddr,
			events.RemoteAddr:         req.RemoteAddr,
		})
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

func setupForwardingHeaders(sess *clusterSession, req *http.Request) {
	// Setup scheme, override target URL to the destination address
	req.URL.Scheme = "https"
	req.URL.Host = sess.targetAddr
	req.RequestURI = req.URL.Path + "?" + req.URL.RawQuery

	// add origin headers so the service consuming the request on the other site
	// is aware of where it came from
	req.Header.Add("X-Forwarded-Proto", "https")
	req.Header.Add("X-Forwarded-Host", req.Host)
	req.Header.Add("X-Forwarded-Path", req.URL.Path)
}

// catchAll forwards all HTTP requests to the target k8s API server
func (f *Forwarder) catchAll(ctx *authContext, w http.ResponseWriter, req *http.Request) (interface{}, error) {
	sess, err := f.getOrCreateClusterSession(*ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	setupForwardingHeaders(sess, req)
	sess.forwarder.ServeHTTP(w, req)
	return nil, nil
}

func (f *Forwarder) getExecutor(sess *clusterSession, req *http.Request) (remotecommand.Executor, error) {
	upgradeRoundTripper := NewSpdyRoundTripperWithDialer(req.Context(), sess.cluster.DialWithContext, sess.tlsConfig, true)
	return remotecommand.NewSPDYExecutorForTransports(upgradeRoundTripper, upgradeRoundTripper, req.Method, req.URL)
}

func (f *Forwarder) getDialer(sess *clusterSession, req *http.Request) (httpstream.Dialer, error) {
	upgradeRoundTripper := NewSpdyRoundTripperWithDialer(req.Context(), sess.cluster.DialWithContext, sess.tlsConfig, true)

	client := &http.Client{
		Transport: upgradeRoundTripper,
	}

	return spdy.NewDialer(upgradeRoundTripper, client, req.Method, req.URL), nil
}

// clusterSession contains authenticated user session to the target cluster:
// x509 short lived credentials, forwarding proxies and other data
type clusterSession struct {
	cluster
	tlsConfig *tls.Config
	forwarder *forward.Forwarder
}

func (f *Forwarder) getOrCreateClusterSession(ctx authContext) (*clusterSession, error) {
	client := f.getClusterSession(ctx)
	if client != nil {
		f.Debugf("Returning existing creds for %v.", ctx)
		return client, nil
	}
	return f.newClusterSession(ctx)
}

func (f *Forwarder) getClusterSession(ctx authContext) *clusterSession {
	f.Lock()
	defer f.Unlock()
	creds, ok := f.clusterSessions.Get(ctx.key())
	if ok {
		return creds.(*clusterSession)
	}
	return nil
}

func (f *Forwarder) newClusterSession(ctx authContext) (*clusterSession, error) {
	response, err := f.requestCertificate(ctx)
	if err != nil {
		f.Warningf("Failed to get certificate for %v: %v.", ctx, err)
		return nil, trace.AccessDenied("access denied: failed to authenticate with kubernetes server")
	}

	cert, err := tls.X509KeyPair(response.cert, response.key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pool := x509.NewCertPool()
	for _, certAuthority := range response.certAuthorities {
		ok := pool.AppendCertsFromPEM(certAuthority)
		if !ok {
			return nil, trace.BadParameter("failed to append certs from PEM")
		}
	}

	tlsConfig := &tls.Config{
		RootCAs:      pool,
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	tlsConfig.BuildNameToCertificate()

	fwd, err := forward.New(
		forward.RoundTripper(f.newTransport(ctx.cluster.Dial, tlsConfig)),
		forward.WebsocketDial(ctx.cluster.Dial),
		forward.Logger(logrus.StandardLogger()),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	f.Lock()
	defer f.Unlock()

	sessI, ok := f.clusterSessions.Get(ctx.key())
	if ok {
		return sessI.(*clusterSession), nil
	}
	sess := &clusterSession{
		cluster:   ctx.cluster,
		tlsConfig: tlsConfig,
		forwarder: fwd,
	}
	f.clusterSessions.Set(ctx.key(), sess, ctx.sessionTTL)
	f.Debugf("Created new session for %v.", ctx)
	return sess, nil
}

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

type bundle struct {
	cert            []byte
	key             []byte
	certAuthorities [][]byte
}

func (f *Forwarder) requestCertificate(ctx authContext) (*bundle, error) {
	f.Debugf("Requesting K8s cert for %v.", ctx)
	keyPEM, _, err := f.Keygen.GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	privateKey, err := ssh.ParseRawPrivateKey(keyPEM)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse private key")
	}

	csr := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   ctx.User.GetName(),
			Organization: ctx.kubeGroups,
		},
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
	return &bundle{
		cert:            response.Cert,
		certAuthorities: response.CertAuthorities,
		key:             keyPEM,
	}, nil
}
