/*
Copyright 2018-2021 Gravitational, Inc.

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
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	mathrand "math/rand"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/gravitational/oxy/forward"
	fwdutils "github.com/gravitational/oxy/utils"
	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/http2"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apiserver/pkg/util/wsstream"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"
	kubeexec "k8s.io/client-go/util/exec"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	tracehttp "github.com/gravitational/teleport/api/observability/tracing/http"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/filesessions"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/kube/proxy/streamproto"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// KubeServiceType specifies a Teleport service type which can forward Kubernetes requests
type KubeServiceType int

const (
	// KubeService is a Teleport kubernetes_service. A KubeService always forwards
	// requests directly to a Kubernetes endpoint.
	KubeService KubeServiceType = iota
	// ProxyService is a Teleport proxy_service with kube_listen_addr/
	// kube_public_addr enabled. A ProxyService always forwards requests to a
	// Teleport KubeService or LegacyProxyService.
	ProxyService
	// LegacyProxyService is a Teleport proxy_service with the kubernetes section
	// enabled. A LegacyProxyService can forward requests directly to a Kubernetes
	// endpoint, or to another Teleport LegacyProxyService or KubeService.
	LegacyProxyService
)

// ForwarderConfig specifies configuration for proxy forwarder
type ForwarderConfig struct {
	// ReverseTunnelSrv is the teleport reverse tunnel server
	ReverseTunnelSrv reversetunnel.Server
	// ClusterName is a local cluster name
	ClusterName string
	// Keygen points to a key generator implementation
	Keygen sshca.Authority
	// Authz authenticates user
	Authz auth.Authorizer
	// AuthClient is a auth server client.
	AuthClient auth.ClientI
	// CachingAuthClient is a caching auth server client for read-only access.
	CachingAuthClient auth.ReadKubernetesAccessPoint
	// StreamEmitter is used to create audit streams
	// and emit audit events
	StreamEmitter events.StreamEmitter
	// DataDir is a data dir to store logs
	DataDir string
	// Namespace is a namespace of the proxy server (not a K8s namespace)
	Namespace string
	// HostID is a unique ID of a proxy server
	HostID string
	// ClusterOverride if set, routes all requests
	// to the cluster name, used in tests
	ClusterOverride string
	// Context passes the optional external context
	// passing global close to all forwarder operations
	Context context.Context
	// KubeconfigPath is a path to kubernetes configuration
	KubeconfigPath string
	// KubeServiceType specifies which Teleport service type this forwarder is for
	KubeServiceType KubeServiceType
	// KubeClusterName is the name of the kubernetes cluster that this
	// forwarder handles.
	KubeClusterName string
	// Clock is a server clock, could be overridden in tests
	Clock clockwork.Clock
	// ConnPingPeriod is a period for sending ping messages on the incoming
	// connection.
	ConnPingPeriod time.Duration
	// Component name to include in log output.
	Component string
	// LockWatcher is a lock watcher.
	LockWatcher *services.LockWatcher
	// CheckImpersonationPermissions is an optional override of the default
	// impersonation permissions check, for use in testing
	CheckImpersonationPermissions ImpersonationPermissionsChecker
	// PublicAddr is the address that can be used to reach the kube cluster
	PublicAddr string
	// log is the logger function
	log logrus.FieldLogger
}

// CheckAndSetDefaults checks and sets default values
func (f *ForwarderConfig) CheckAndSetDefaults() error {
	if f.AuthClient == nil {
		return trace.BadParameter("missing parameter AuthClient")
	}
	if f.CachingAuthClient == nil {
		return trace.BadParameter("missing parameter CachingAuthClient")
	}
	if f.Authz == nil {
		return trace.BadParameter("missing parameter Authz")
	}
	if f.StreamEmitter == nil {
		return trace.BadParameter("missing parameter StreamEmitter")
	}
	if f.ClusterName == "" {
		return trace.BadParameter("missing parameter ClusterName")
	}
	if f.Keygen == nil {
		return trace.BadParameter("missing parameter Keygen")
	}
	if f.DataDir == "" {
		return trace.BadParameter("missing parameter DataDir")
	}
	if f.HostID == "" {
		return trace.BadParameter("missing parameter ServerID")
	}
	if f.Namespace == "" {
		f.Namespace = apidefaults.Namespace
	}
	if f.Context == nil {
		f.Context = context.TODO()
	}
	if f.Clock == nil {
		f.Clock = clockwork.NewRealClock()
	}
	if f.ConnPingPeriod == 0 {
		f.ConnPingPeriod = defaults.HighResPollingPeriod
	}
	if f.Component == "" {
		f.Component = "kube_forwarder"
	}

	if f.CheckImpersonationPermissions == nil {
		f.CheckImpersonationPermissions = checkImpersonationPermissions
	}

	switch f.KubeServiceType {
	case KubeService:
	case ProxyService:
	case LegacyProxyService:
	default:
		return trace.BadParameter("unknown value for KubeServiceType")
	}
	if f.KubeClusterName == "" && f.KubeconfigPath == "" && f.KubeServiceType == LegacyProxyService {
		// Running without a kubeconfig and explicit k8s cluster name. Use
		// teleport cluster name instead, to ask kubeutils.GetKubeConfig to
		// attempt loading the in-cluster credentials.
		f.KubeClusterName = f.ClusterName
	}
	if f.log == nil {
		f.log = logrus.New()
	}
	return nil
}

// NewForwarder returns new instance of Kubernetes request
// forwarding proxy.
func NewForwarder(cfg ForwarderConfig) (*Forwarder, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	clientCredentials, err := ttlmap.New(defaults.ClientCacheSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	closeCtx, close := context.WithCancel(cfg.Context)

	fwd := &Forwarder{
		log:               cfg.log,
		router:            *httprouter.New(),
		cfg:               cfg,
		clientCredentials: clientCredentials,
		activeRequests:    make(map[string]context.Context),
		ctx:               closeCtx,
		close:             close,
		sessions:          make(map[uuid.UUID]*session),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		clusterDetails: make(map[string]*kubeDetails),
	}

	fwd.router.UseRawPath = true

	fwd.router.GET("/version", fwd.withAuth(
		func(ctx *authContext, w http.ResponseWriter, r *http.Request, _ httprouter.Params) (any, error) {
			// Forward version requests to the cluster.
			return fwd.catchAll(ctx, w, r)
		},
		withCustomErrFormatter(fwd.writeResponseErrorToBody),
	))

	fwd.router.POST("/api/:ver/namespaces/:podNamespace/pods/:podName/exec", fwd.withAuth(fwd.exec))
	fwd.router.GET("/api/:ver/namespaces/:podNamespace/pods/:podName/exec", fwd.withAuth(fwd.exec))

	fwd.router.POST("/api/:ver/namespaces/:podNamespace/pods/:podName/attach", fwd.withAuth(fwd.exec))
	fwd.router.GET("/api/:ver/namespaces/:podNamespace/pods/:podName/attach", fwd.withAuth(fwd.exec))

	fwd.router.POST("/api/:ver/namespaces/:podNamespace/pods/:podName/portforward", fwd.withAuth(fwd.portForward))
	fwd.router.GET("/api/:ver/namespaces/:podNamespace/pods/:podName/portforward", fwd.withAuth(fwd.portForward))

	fwd.router.GET("/api/:ver/teleport/join/:session", fwd.withAuthPassthrough(fwd.join))

	fwd.router.NotFound = fwd.withAuthStd(fwd.catchAll)

	if cfg.ClusterOverride != "" {
		fwd.log.Debugf("Cluster override is set, forwarder will send all requests to remote cluster %v.", cfg.ClusterOverride)
	}
	if len(cfg.KubeClusterName) > 0 || len(cfg.KubeconfigPath) > 0 || cfg.KubeServiceType != KubeService {
		fwd.clusterDetails, err = getKubeDetails(cfg.Context, fwd.log, cfg.ClusterName, cfg.KubeClusterName, cfg.KubeconfigPath, cfg.KubeServiceType, cfg.CheckImpersonationPermissions)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return fwd, nil
}

// Forwarder intercepts kubernetes requests, acting as Kubernetes API proxy.
// it blindly forwards most of the requests on HTTPS protocol layer,
// however some requests like exec sessions it intercepts and records.
type Forwarder struct {
	mu     sync.Mutex
	log    logrus.FieldLogger
	router httprouter.Router
	cfg    ForwarderConfig
	// clientCredentials is an expiring cache of ephemeral client credentials.
	// Forwarder requests credentials with client identity, when forwarding to
	// another teleport process (but not when forwarding to k8s API).
	//
	// TODO(klizhentas): flush certs on teleport CA rotation?
	clientCredentials *ttlmap.TTLMap
	// activeRequests is a map used to serialize active CSR requests to the auth server
	activeRequests map[string]context.Context
	// close is a close function
	close context.CancelFunc
	// ctx is a global context signaling exit
	ctx context.Context
	// clusterDetails contain kubernetes credentials for multiple clusters.
	// map key is cluster name.
	clusterDetails map[string]*kubeDetails
	rwMutexDetails sync.RWMutex
	// sessions tracks in-flight sessions
	sessions map[uuid.UUID]*session
	// upgrades connections to websockets
	upgrader websocket.Upgrader
	// getKubernetesServersForKubeCluster is a function that returns a list of
	// kubernetes servers for a given kube cluster but uses different methods
	// depending on the service type.
	// For example, if the service type is KubeService, it will use the
	// local kubernetes clusters. If the service type is Proxy, it will
	// use the heartbeat clusters.
	getKubernetesServersForKubeCluster getKubeServersByNameFunc
}

// getKubeServersByNameFunc is a function that returns a list of
// kubernetes servers for a given kube cluster.
type getKubeServersByNameFunc = func(ctx context.Context, name string) ([]types.KubeServer, error)

// Close signals close to all outstanding or background operations
// to complete
func (f *Forwarder) Close() error {
	f.close()
	return nil
}

func (f *Forwarder) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	f.router.ServeHTTP(rw, r)
}

// authContext is a context of authenticated user,
// contains information about user, target cluster and authenticated groups
type authContext struct {
	auth.Context
	kubeGroups        map[string]struct{}
	kubeUsers         map[string]struct{}
	kubeClusterLabels map[string]string
	kubeCluster       string
	teleportCluster   teleportClusterClient
	recordingConfig   types.SessionRecordingConfig
	// clientIdleTimeout sets information on client idle timeout
	clientIdleTimeout time.Duration
	// disconnectExpiredCert if set, controls the time when the connection
	// should be disconnected because the client cert expires
	disconnectExpiredCert time.Time
	// certExpires is the client certificate expiration timestamp.
	certExpires time.Time
	// sessionTTL specifies the duration of the user's session
	sessionTTL time.Duration
	// kubeServers are the registered agents for the kubernetes cluster the request
	// is targeted to.
	kubeServers []types.KubeServer
}

func (c authContext) String() string {
	return fmt.Sprintf("user: %v, users: %v, groups: %v, teleport cluster: %v, kube cluster: %v", c.User.GetName(), c.kubeUsers, c.kubeGroups, c.teleportCluster.name, c.kubeCluster)
}

func (c *authContext) key() string {
	// it is important that the context key contains user, kubernetes groups and certificate expiry,
	// so that new logins with different parameters will not reuse this context
	return fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v", c.teleportCluster.name, c.User.GetName(), c.kubeUsers, c.kubeGroups, c.kubeCluster, c.certExpires.Unix(), c.Identity.GetIdentity().ActiveRequests)
}

func (c *authContext) eventClusterMeta(req *http.Request) apievents.KubernetesClusterMetadata {
	var kubeUsers, kubeGroups []string

	if impersonateUser, impersonateGroups, err := computeImpersonatedPrincipals(c.kubeUsers, c.kubeGroups, req.Header); err == nil {
		kubeUsers = []string{impersonateUser}
		kubeGroups = impersonateGroups
	} else {
		kubeUsers = utils.StringsSliceFromSet(c.kubeUsers)
		kubeGroups = utils.StringsSliceFromSet(c.kubeGroups)
	}

	return apievents.KubernetesClusterMetadata{
		KubernetesCluster: c.kubeCluster,
		KubernetesUsers:   kubeUsers,
		KubernetesGroups:  kubeGroups,
		KubernetesLabels:  c.kubeClusterLabels,
	}
}

func (c *authContext) eventUserMeta() apievents.UserMetadata {
	name := c.User.GetName()
	meta := c.Identity.GetIdentity().GetUserMetadata()
	meta.User = name
	meta.Login = name
	return meta
}

type dialFunc func(ctx context.Context, network string, endpoint kubeClusterEndpoint) (net.Conn, error)

// teleportClusterClient is a client for either a k8s endpoint in local cluster or a
// proxy endpoint in a remote cluster.
type teleportClusterClient struct {
	remoteAddr     utils.NetAddr
	name           string
	dial           dialFunc
	isRemote       bool
	isRemoteClosed func() bool
}

// dialEndpoint dials a connection to a kube cluster using the given kube cluster endpoint
func (c *teleportClusterClient) dialEndpoint(ctx context.Context, network string, endpoint kubeClusterEndpoint) (net.Conn, error) {
	return c.dial(ctx, network, endpoint)
}

// handlerWithAuthFunc is http handler with passed auth context
type handlerWithAuthFunc func(ctx *authContext, w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error)

// handlerWithAuthFuncStd is http handler with passed auth context
type handlerWithAuthFuncStd func(ctx *authContext, w http.ResponseWriter, r *http.Request) (any, error)

// accessDeniedMsg is a message returned to the client when access is denied.
const accessDeniedMsg = "[00] access denied"

// authenticate function authenticates request
func (f *Forwarder) authenticate(req *http.Request) (*authContext, error) {
	var isRemoteUser bool
	userTypeI := req.Context().Value(auth.ContextUser)
	switch userTypeI.(type) {
	case auth.LocalUser:

	case auth.RemoteUser:
		isRemoteUser = true
	case auth.BuiltinRole:
		f.log.Warningf("Denying proxy access to unauthenticated user of type %T - this can sometimes be caused by inadvertently using an HTTP load balancer instead of a TCP load balancer on the Kubernetes port.", userTypeI)
		return nil, trace.AccessDenied(accessDeniedMsg)
	default:
		f.log.Warningf("Denying proxy access to unsupported user type: %T.", userTypeI)
		return nil, trace.AccessDenied(accessDeniedMsg)
	}

	userContext, err := f.cfg.Authz.Authorize(req.Context())
	if err != nil {
		switch {
		// propagate connection problem error so we can differentiate
		// between connection failed and access denied
		case trace.IsConnectionProblem(err):
			return nil, trace.ConnectionProblem(err, "[07] failed to connect to the database")
		case trace.IsAccessDenied(err):
			// don't print stack trace, just log the warning
			f.log.Warn(err)
		case keys.IsPrivateKeyPolicyError(err):
			// private key policy errors should be returned to the client
			// unaltered so that they know to reauthenticate with a valid key.
			return nil, trace.Unwrap(err)
		default:
			f.log.Warn(trace.DebugReport(err))
		}
		return nil, trace.AccessDenied(accessDeniedMsg)
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
	clientIdentity, err := tlsca.FromSubject(clientCert.Subject, clientCert.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authContext, err := f.setupContext(*userContext, req, isRemoteUser, clientIdentity)
	if err != nil {
		f.log.Warn(err.Error())
		if trace.IsAccessDenied(err) {
			return nil, trace.AccessDenied(accessDeniedMsg)
		}
		return nil, trace.Wrap(err)
	}
	return authContext, nil
}

func (f *Forwarder) withAuthStd(handler handlerWithAuthFuncStd) http.HandlerFunc {
	return httplib.MakeStdHandlerWithErrorWriter(func(w http.ResponseWriter, req *http.Request) (any, error) {
		authContext, err := f.authenticate(req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := f.authorize(req.Context(), authContext); err != nil {
			return nil, trace.Wrap(err)
		}

		return handler(authContext, w, req)
	}, f.formatStatusResponseError)
}

// acquireConnectionLockWithIdentity acquires a connection lock under a given identity.
func (f *Forwarder) acquireConnectionLockWithIdentity(ctx context.Context, identity *authContext) error {
	user := identity.Identity.GetIdentity().Username
	roles, err := getRolesByName(f, identity.Identity.GetIdentity().Groups)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := f.acquireConnectionLock(ctx, user, roles); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// authOption is a functional option for authOptions.
type authOption func(*authOptions)

// authOptions is a set of options for withAuth handler.
type authOptions struct {
	// errFormater is a function that formats the error response.
	errFormater func(http.ResponseWriter, error)
}

// withCustomErrFormatter allows to override the default error formatter.
func withCustomErrFormatter(f func(http.ResponseWriter, error)) authOption {
	return func(o *authOptions) {
		o.errFormater = f
	}
}

func (f *Forwarder) withAuth(handler handlerWithAuthFunc, opts ...authOption) httprouter.Handle {
	authOpts := authOptions{
		errFormater: f.formatStatusResponseError,
	}
	for _, opt := range opts {
		opt(&authOpts)
	}
	return httplib.MakeHandlerWithErrorWriter(func(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
		authContext, err := f.authenticate(req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := f.authorize(req.Context(), authContext); err != nil {
			return nil, trace.Wrap(err)
		}
		err = f.acquireConnectionLockWithIdentity(req.Context(), authContext)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return handler(authContext, w, req, p)
	}, authOpts.errFormater)
}

// withAuthPassthrough authenticates the request and fetches information but doesn't deny if the user
// doesn't have RBAC access to the Kubernetes cluster.
func (f *Forwarder) withAuthPassthrough(handler handlerWithAuthFunc) httprouter.Handle {
	return httplib.MakeHandlerWithErrorWriter(func(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
		authContext, err := f.authenticate(req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		err = f.acquireConnectionLockWithIdentity(req.Context(), authContext)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return handler(authContext, w, req, p)
	}, f.formatStatusResponseError)
}

func (f *Forwarder) formatForwardResponseError(rw http.ResponseWriter, r *http.Request, respErr error) {
	f.formatStatusResponseError(rw, respErr)
}

// writeResponseErrorToBody writes the error response to the body without any formatting.
// It is used for the /version endpoint since Kubernetes doesn't expect a JSON response
// for that endpoint.
func (f *Forwarder) writeResponseErrorToBody(rw http.ResponseWriter, respErr error) {
	http.Error(rw, respErr.Error(), http.StatusInternalServerError)
}

// formatStatusResponseError formats the error response into a kube Status object.
func (f *Forwarder) formatStatusResponseError(rw http.ResponseWriter, respErr error) {
	status := &metav1.Status{
		Status: metav1.StatusFailure,
		// Don't trace.Unwrap the error, in case it was wrapped with a
		// user-friendly message. The underlying root error is likely too
		// low-level to be useful.
		Message: respErr.Error(),
		Code:    int32(trace.ErrorToCode(respErr)),
	}
	data, err := runtime.Encode(statusCodecs.LegacyCodec(), status)
	if err != nil {
		f.log.Warningf("Failed encoding error into kube Status object: %v", err)
		trace.WriteError(rw, respErr)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	// Always write InternalServerError, that's the only code that kubectl will
	// parse the Status object for. The Status object has the real status code
	// embedded.
	rw.WriteHeader(http.StatusInternalServerError)
	if _, err := rw.Write(data); err != nil {
		f.log.Warningf("Failed writing kube error response body: %v", err)
	}
}

func (f *Forwarder) setupContext(authCtx auth.Context, req *http.Request, isRemoteUser bool, clientIdentity *tlsca.Identity) (*authContext, error) {
	roles := authCtx.Checker

	// adjust session ttl to the smaller of two values: the session
	// ttl requested in tsh or the session ttl for the role.
	sessionTTL := roles.AdjustSessionTTL(time.Hour)

	identity := authCtx.Identity.GetIdentity()
	teleportClusterName := identity.RouteToCluster
	if teleportClusterName == "" {
		teleportClusterName = f.cfg.ClusterName
	}

	isRemoteCluster := f.cfg.ClusterName != teleportClusterName

	if isRemoteCluster && isRemoteUser {
		return nil, trace.AccessDenied("access denied: remote user can not access remote cluster")
	}

	kubeCluster := identity.KubernetesCluster
	// Only set a default kube cluster if the user is not accessing a specific cluster.
	// The check for kubeCluster != "" is happens in the next code section.
	if !isRemoteCluster && kubeCluster == "" {
		kc, err := kubeutils.CheckOrSetKubeCluster(req.Context(), f.cfg.CachingAuthClient, identity.KubernetesCluster, teleportClusterName)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			// Fallback for old clusters and old user certs. Assume that the
			// user is trying to access the default cluster name.
			kubeCluster = teleportClusterName
		} else {
			kubeCluster = kc
		}
	}

	var (
		kubeServers []types.KubeServer
		err         error
	)
	// Only check k8s principals for local clusters.
	//
	// For remote clusters, everything will be remapped to new roles on the
	// leaf and checked there.
	if !isRemoteCluster {
		kubeServers, err = f.getKubernetesServersForKubeCluster(req.Context(), kubeCluster)
		if err != nil || len(kubeServers) == 0 {
			return nil, trace.NotFound("cluster %q not found", kubeCluster)
		}
	}

	// Get a dialer for either a k8s endpoint in current cluster or a tunneled
	// endpoint for a leaf teleport cluster.
	var dialFn dialFunc
	var isRemoteClosed func() bool
	if isRemoteCluster {
		// Tunnel is nil for a teleport process with "kubernetes_service" but
		// not "proxy_service".
		if f.cfg.ReverseTunnelSrv == nil {
			return nil, trace.BadParameter("this Teleport process can not dial Kubernetes endpoints in remote Teleport clusters; only proxy_service supports this, make sure a Teleport proxy is first in the request path")
		}

		targetCluster, err := f.cfg.ReverseTunnelSrv.GetSite(teleportClusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		dialFn = func(ctx context.Context, network string, endpoint kubeClusterEndpoint) (net.Conn, error) {
			return targetCluster.DialTCP(reversetunnel.DialParams{
				From:     &utils.NetAddr{AddrNetwork: "tcp", Addr: req.RemoteAddr},
				To:       &utils.NetAddr{AddrNetwork: "tcp", Addr: endpoint.addr},
				ConnType: types.KubeTunnel,
				ServerID: endpoint.serverID,
				ProxyIDs: endpoint.proxyIDs,
			})
		}
		isRemoteClosed = targetCluster.IsClosed
	} else if f.cfg.ReverseTunnelSrv != nil {
		// Not a remote cluster and we have a reverse tunnel server.
		// Use the local reversetunnel.Site which knows how to dial by serverID
		// (for "kubernetes_service" connected over a tunnel) and falls back to
		// direct dial if needed.
		localCluster, err := f.cfg.ReverseTunnelSrv.GetSite(f.cfg.ClusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		dialFn = func(ctx context.Context, network string, endpoint kubeClusterEndpoint) (net.Conn, error) {
			return localCluster.DialTCP(reversetunnel.DialParams{
				From:     &utils.NetAddr{AddrNetwork: "tcp", Addr: req.RemoteAddr},
				To:       &utils.NetAddr{AddrNetwork: "tcp", Addr: endpoint.addr},
				ConnType: types.KubeTunnel,
				ServerID: endpoint.serverID,
				ProxyIDs: endpoint.proxyIDs,
			})
		}
		isRemoteClosed = localCluster.IsClosed
	} else {
		// Don't have a reverse tunnel server, so we can only dial directly.
		dialFn = func(ctx context.Context, network string, endpoint kubeClusterEndpoint) (net.Conn, error) {
			return new(net.Dialer).DialContext(ctx, network, endpoint.addr)
		}
		isRemoteClosed = func() bool { return false }
	}

	netConfig, err := f.cfg.CachingAuthClient.GetClusterNetworkingConfig(f.ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	recordingConfig, err := f.cfg.CachingAuthClient.GetSessionRecordingConfig(f.ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authPref, err := f.cfg.CachingAuthClient.GetAuthPreference(req.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &authContext{
		clientIdleTimeout:     roles.AdjustClientIdleTimeout(netConfig.GetClientIdleTimeout()),
		sessionTTL:            sessionTTL,
		Context:               authCtx,
		recordingConfig:       recordingConfig,
		kubeCluster:           kubeCluster,
		certExpires:           clientIdentity.Expires,
		disconnectExpiredCert: srv.GetDisconnectExpiredCertFromIdentity(roles, authPref, clientIdentity),
		teleportCluster: teleportClusterClient{
			name:           teleportClusterName,
			remoteAddr:     utils.NetAddr{AddrNetwork: "tcp", Addr: req.RemoteAddr},
			dial:           dialFn,
			isRemote:       isRemoteCluster,
			isRemoteClosed: isRemoteClosed,
		},
		kubeServers: kubeServers,
	}, nil
}

// kubeAccessDetails holds the allowed kube groups/users names and the cluster labels for a local kube cluster.
type kubeAccessDetails struct {
	// list of allowed kube users
	kubeUsers []string
	// list of allowed kube groups
	kubeGroups []string
	// kube cluster labels
	clusterLabels map[string]string
}

// getKubeAccessDetails returns the allowed kube groups/users names and the cluster labels for a local kube cluster.
func (f *Forwarder) getKubeAccessDetails(
	kubeServers []types.KubeServer,
	roles services.AccessChecker,
	kubeClusterName string,
	sessionTTL time.Duration,
) (kubeAccessDetails, error) {
	// Find requested kubernetes cluster name and get allowed kube users/groups names.
	for _, s := range kubeServers {
		c := s.GetCluster()
		if c.GetName() != kubeClusterName {
			continue
		}

		// Get list of allowed kube user/groups based on kubernetes service labels.
		labels := types.CombineLabels(c.GetStaticLabels(), types.LabelsToV2(c.GetDynamicLabels()))
		labelsMatcher := services.NewKubernetesClusterLabelMatcher(labels)
		groups, users, err := roles.CheckKubeGroupsAndUsers(sessionTTL, false, labelsMatcher)
		if err != nil {
			return kubeAccessDetails{}, trace.Wrap(err)
		}
		return kubeAccessDetails{
			kubeGroups:    groups,
			kubeUsers:     users,
			clusterLabels: labels,
		}, nil

	}
	// kubeClusterName not found. Empty list of allowed kube users/groups is returned.
	return kubeAccessDetails{
		kubeGroups:    []string{},
		kubeUsers:     []string{},
		clusterLabels: map[string]string{},
	}, nil
}

func (f *Forwarder) authorize(ctx context.Context, actx *authContext) error {
	if actx.teleportCluster.isRemote {
		// Authorization for a remote kube cluster will happen on the remote
		// end (by their proxy), after that cluster has remapped used roles.
		f.log.WithField("auth_context", actx.String()).Debug("Skipping authorization for a remote kubernetes cluster name")
		return nil
	}
	if actx.kubeCluster == "" {
		// This should only happen for remote clusters (filtered above), but
		// check and report anyway.
		f.log.WithField("auth_context", actx.String()).Debug("Skipping authorization due to unknown kubernetes cluster name")
		return nil
	}

	authPref, err := f.cfg.CachingAuthClient.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	mfaParams := actx.MFAParams(authPref.GetRequireMFAType())
	var kubeUsers, kubeGroups []string
	// Only check k8s principals for local clusters.
	//
	// For remote clusters, everything will be remapped to new roles on the
	// leaf and checked there.
	if !actx.teleportCluster.isRemote {
		// check signing TTL and return a list of allowed logins for local cluster based on Kubernetes service labels.
		kubeAccessDetails, err := f.getKubeAccessDetails(actx.kubeServers, actx.Checker, actx.kubeCluster, actx.sessionTTL)
		if err != nil && !trace.IsNotFound(err) {
			// TODO (tigrato): should return another message here.
			return trace.AccessDenied(accessDeniedMsg)
			// roles.CheckKubeGroupsAndUsers returns trace.NotFound if the user does
			// does not have at least one configured kubernetes_users or kubernetes_groups.
		} else if trace.IsNotFound(err) {
			const errMsg = "Your user's Teleport role does not allow Kubernetes access." +
				" Please ask cluster administrator to ensure your role has appropriate kubernetes_groups and kubernetes_users set."
			return trace.NotFound(errMsg)
		}

		kubeUsers = kubeAccessDetails.kubeUsers
		kubeGroups = kubeAccessDetails.kubeGroups
		actx.kubeClusterLabels = kubeAccessDetails.clusterLabels
	}

	// fillDefaultKubePrincipalDetails fills the default details in order to keep
	// the correct behavior when forwarding the request to the Kubernetes API.
	// By default, if no kubernetes_users is set (which will be a majority),
	// user will impersonate themselves, which is the backwards-compatible behavior.
	if len(kubeUsers) == 0 {
		kubeUsers = append(kubeUsers, actx.User.GetName())
	}

	// KubeSystemAuthenticated is a builtin group that allows
	// any user to access common API methods, e.g. discovery methods
	// required for initial client usage, without it, restricted user's
	// kubectl clients will not work
	if !apiutils.SliceContainsStr(kubeGroups, teleport.KubeSystemAuthenticated) {
		kubeGroups = append(kubeGroups, teleport.KubeSystemAuthenticated)
	}

	actx.kubeUsers = utils.StringsSet(kubeUsers)
	actx.kubeGroups = utils.StringsSet(kubeGroups)

	// Check authz against the first match.
	//
	// We assume that users won't register two identically-named clusters with
	// mis-matched labels. If they do, expect weirdness.
	clusterNotFound := trace.AccessDenied("kubernetes cluster %q not found", actx.kubeCluster)
	for _, s := range actx.kubeServers {
		ks := s.GetCluster()
		if ks.GetName() != actx.kubeCluster {
			continue
		}

		if err := actx.Checker.CheckAccess(ks, mfaParams); err != nil {
			return clusterNotFound
		}
		return nil
	}
	if actx.kubeCluster == f.cfg.ClusterName {
		f.log.WithField("auth_context", actx.String()).Debug("Skipping authorization for proxy-based kubernetes cluster,")
		return nil
	}
	return clusterNotFound
}

// newStreamer returns sync or async streamer based on the configuration
// of the server and the session, sync streamer sends the events
// directly to the auth server and blocks if the events can not be received,
// async streamer buffers the events to disk and uploads the events later
func (f *Forwarder) newStreamer(ctx *authContext) (events.Streamer, error) {
	if services.IsRecordSync(ctx.recordingConfig.GetMode()) {
		f.log.Debug("Using sync streamer for session.")
		return f.cfg.AuthClient, nil
	}
	f.log.Debug("Using async streamer for session.")
	dir := filepath.Join(
		f.cfg.DataDir, teleport.LogsDir, teleport.ComponentUpload,
		events.StreamingSessionsDir, apidefaults.Namespace,
	)
	fileStreamer, err := filesessions.NewStreamer(dir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TeeStreamer sends non-print and non disk events
	// to the audit log in async mode, while buffering all
	// events on disk for further upload at the end of the session
	return events.NewTeeStreamer(fileStreamer, f.cfg.StreamEmitter), nil
}

// join joins an existing session over a websocket connection
func (f *Forwarder) join(ctx *authContext, w http.ResponseWriter, req *http.Request, p httprouter.Params) (resp any, err error) {
	f.log.Debugf("Join %v.", req.URL.String())

	sess, err := f.newClusterSession(*ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// sess.Close cancels the connection monitor context to release it sooner.
	// When the server is under heavy load it can take a while to identify that
	// the underlying connection is gone. This change prevents that and releases
	// the resources as soon as we know the session is no longer active.
	defer sess.close()

	if err := f.setupForwardingHeaders(sess, req, false /* withImpersonationHeaders */); err != nil {
		return nil, trace.Wrap(err)
	}

	if !f.isLocalKubeCluster(sess) {
		return f.remoteJoin(ctx, w, req, p, sess)
	}

	sessionIDString := p.ByName("session")
	sessionID, err := uuid.Parse(sessionIDString)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	session := f.getSession(sessionID)
	if session == nil {
		return nil, trace.NotFound("session %v not found", sessionID)
	}

	ws, err := f.upgrader.Upgrade(w, req, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := func() error {
		stream, err := streamproto.NewSessionStream(ws, streamproto.ServerHandshake{MFARequired: session.PresenceEnabled})
		if err != nil {
			return trace.Wrap(err)
		}

		client := &websocketClientStreams{stream}
		party := newParty(*ctx, stream.Mode, client)

		err = session.join(party)
		if err != nil {
			return trace.Wrap(err)
		}
		closeC := make(chan struct{})
		go func() {
			defer close(closeC)
			select {
			case <-stream.Done():
				party.InformClose()
			case <-party.closeC:
				return
			}
		}()
		<-party.closeC
		if _, err := session.leave(party.ID); err != nil {
			f.log.WithError(err).Debugf("Participant %q was unable to leave session %s", party.ID, session.id)
		}
		<-closeC
		return nil
	}(); err != nil {
		writeErr := ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, err.Error()), time.Now().Add(time.Second*10))
		if writeErr != nil {
			f.log.WithError(writeErr).Warn("Failed to send early-exit websocket close message.")
		}
	}

	return nil, nil
}

// isLocalKubeCluster checks if the current service must hold the cluster and
// if it's of Type KubeService.
// KubeProxy services or remote clusters are automatically forwarded to
// the final destination.
func (f *Forwarder) isLocalKubeCluster(sess *clusterSession) bool {
	return !sess.authContext.teleportCluster.isRemote && f.cfg.KubeServiceType == KubeService
}

// getSession retrieves the session from in-memory database.
// If the session was not found, returns nil.
// This method locks f.mu.
func (f *Forwarder) getSession(id uuid.UUID) *session {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sessions[id]
}

// setSession sets the session into in-memory database.
// If the session was not found, returns nil.
// This method locks f.mu.
func (f *Forwarder) setSession(id uuid.UUID, sess *session) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions[id] = sess
}

// deleteSession removes a session.
// This method locks f.mu.
func (f *Forwarder) deleteSession(id uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.sessions, id)
}

// remoteJoin forwards a join request to a remote cluster.
func (f *Forwarder) remoteJoin(ctx *authContext, w http.ResponseWriter, req *http.Request, p httprouter.Params, sess *clusterSession) (resp any, err error) {
	dialer := &websocket.Dialer{
		TLSClientConfig: sess.tlsConfig,
		NetDialContext:  sess.DialWithContext,
	}

	url := "wss://" + req.URL.Host
	if req.URL.Port() != "" {
		url = url + ":" + req.URL.Port()
	}
	url = url + req.URL.Path

	wsTarget, respTarget, err := dialer.Dial(url, nil)
	if err != nil {
		msg, err := io.ReadAll(respTarget.Body)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		var obj map[string]any
		if err := json.Unmarshal(msg, &obj); err != nil {
			return nil, trace.Wrap(err)
		}

		return obj, trace.Wrap(err)
	}
	defer wsTarget.Close()
	defer respTarget.Body.Close()

	wsSource, err := f.upgrader.Upgrade(w, req, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer wsSource.Close()

	err = wsProxy(wsSource, wsTarget)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return nil, nil
}

// wsProxy proxies a websocket connection between two clusters transparently to allow for
// remote joins.
func wsProxy(wsSource *websocket.Conn, wsTarget *websocket.Conn) error {
	closeM := make(chan struct{})
	errS := make(chan error)
	errT := make(chan error)

	go func() {
		for {
			ty, data, err := wsSource.ReadMessage()
			if err != nil {
				wsSource.Close()
				errS <- trace.Wrap(err)
				return
			}

			wsTarget.WriteMessage(ty, data)

			if ty == websocket.CloseMessage {
				closeM <- struct{}{}
				return
			}
		}
	}()

	go func() {
		for {
			ty, data, err := wsTarget.ReadMessage()
			if err != nil {
				wsTarget.Close()
				errT <- trace.Wrap(err)
				return
			}

			wsSource.WriteMessage(ty, data)

			if ty == websocket.CloseMessage {
				closeM <- struct{}{}
				return
			}
		}
	}()

	var err error
	select {
	case err = <-errS:
		wsTarget.WriteMessage(websocket.CloseMessage, []byte{})
	case err = <-errT:
		wsSource.WriteMessage(websocket.CloseMessage, []byte{})
	case <-closeM:
	}

	return trace.Wrap(err)
}

// acquireConnectionLock acquires a semaphore used to limit connections to the Kubernetes agent.
// The semaphore is releasted when the request is returned/connection is closed.
// Returns an error if a semaphore could not be acquired.
func (f *Forwarder) acquireConnectionLock(ctx context.Context, user string, roles services.RoleSet) error {
	maxConnections := roles.MaxKubernetesConnections()
	if maxConnections == 0 {
		return nil
	}

	_, err := services.AcquireSemaphoreLock(ctx, services.SemaphoreLockConfig{
		Service: f.cfg.AuthClient,
		Expiry:  sessionMaxLifetime,
		Params: types.AcquireSemaphoreRequest{
			SemaphoreKind: types.SemaphoreKindKubernetesConnection,
			SemaphoreName: user,
			MaxLeases:     maxConnections,
			Holder:        user,
		},
	})
	if err != nil {
		if strings.Contains(err.Error(), teleport.MaxLeases) {
			err = trace.AccessDenied("too many concurrent kubernetes connections for user %q (max=%d)",
				user,
				maxConnections,
			)
		}

		return trace.Wrap(err)
	}

	return nil
}

// execNonInteractive handles all exec sessions without a TTY.
func (f *Forwarder) execNonInteractive(ctx *authContext, w http.ResponseWriter, req *http.Request, p httprouter.Params, request remoteCommandRequest, proxy *remoteCommandProxy, sess *clusterSession) (resp any, err error) {
	defer proxy.Close()

	roles, err := getRolesByName(f, ctx.Context.Identity.GetIdentity().Groups)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var policySets []*types.SessionTrackerPolicySet
	for _, role := range roles {
		policySet := role.GetSessionPolicySet()
		policySets = append(policySets, &policySet)
	}

	authorizer := auth.NewSessionAccessEvaluator(policySets, types.KubernetesSessionKind, ctx.User.GetName())
	canStart, _, err := authorizer.FulfilledFor(nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !canStart {
		return nil, trace.AccessDenied("insufficient permissions to launch non-interactive session")
	}

	eventPodMeta := request.eventPodMeta(request.context, sess.creds)

	sessionStart := f.cfg.Clock.Now().UTC()

	serverMetadata := apievents.ServerMetadata{
		ServerID:        f.cfg.HostID,
		ServerNamespace: f.cfg.Namespace,
		ServerHostname:  sess.teleportCluster.name,
		ServerAddr:      sess.kubeAddress,
	}

	sessionMetadata := apievents.SessionMetadata{
		SessionID: uuid.NewString(),
		WithMFA:   ctx.Identity.GetIdentity().MFAVerified,
	}

	connectionMetdata := apievents.ConnectionMetadata{
		RemoteAddr: req.RemoteAddr,
		LocalAddr:  sess.kubeAddress,
		Protocol:   events.EventProtocolKube,
	}

	sessionStartEvent := &apievents.SessionStart{
		Metadata: apievents.Metadata{
			Type:        events.SessionStartEvent,
			Code:        events.SessionStartCode,
			ClusterName: f.cfg.ClusterName,
		},
		ServerMetadata:            serverMetadata,
		SessionMetadata:           sessionMetadata,
		UserMetadata:              ctx.eventUserMeta(),
		ConnectionMetadata:        connectionMetdata,
		KubernetesClusterMetadata: ctx.eventClusterMeta(req),
		KubernetesPodMetadata:     eventPodMeta,

		InitialCommand:   request.cmd,
		SessionRecording: ctx.recordingConfig.GetMode(),
	}

	if err := f.cfg.StreamEmitter.EmitAuditEvent(f.ctx, sessionStartEvent); err != nil {
		f.log.WithError(err).Warn("Failed to emit event.")
	}

	execEvent := &apievents.Exec{
		Metadata: apievents.Metadata{
			Type:        events.ExecEvent,
			ClusterName: f.cfg.ClusterName,
		},
		ServerMetadata:     serverMetadata,
		SessionMetadata:    sessionMetadata,
		UserMetadata:       ctx.eventUserMeta(),
		ConnectionMetadata: connectionMetdata,
		CommandMetadata: apievents.CommandMetadata{
			Command: strings.Join(request.cmd, " "),
		},
		KubernetesClusterMetadata: ctx.eventClusterMeta(req),
		KubernetesPodMetadata:     eventPodMeta,
	}

	defer func() {
		if err := f.cfg.StreamEmitter.EmitAuditEvent(f.ctx, execEvent); err != nil {
			f.log.WithError(err).Warn("Failed to emit exec event.")
		}

		sessionEndEvent := &apievents.SessionEnd{
			Metadata: apievents.Metadata{
				Type:        events.SessionEndEvent,
				Code:        events.SessionEndCode,
				ClusterName: f.cfg.ClusterName,
			},
			ServerMetadata:            serverMetadata,
			SessionMetadata:           sessionMetadata,
			UserMetadata:              ctx.eventUserMeta(),
			ConnectionMetadata:        connectionMetdata,
			Interactive:               false,
			StartTime:                 sessionStart,
			EndTime:                   f.cfg.Clock.Now().UTC(),
			KubernetesClusterMetadata: ctx.eventClusterMeta(req),
			KubernetesPodMetadata:     eventPodMeta,
			InitialCommand:            request.cmd,
			SessionRecording:          ctx.recordingConfig.GetMode(),
		}

		if err := f.cfg.StreamEmitter.EmitAuditEvent(f.ctx, sessionEndEvent); err != nil {
			f.log.WithError(err).Warn("Failed to emit session end event.")
		}
	}()

	executor, err := f.getExecutor(*ctx, sess, req)
	if err != nil {
		execEvent.Code = events.ExecFailureCode
		execEvent.Error, execEvent.ExitCode = exitCode(err)

		f.log.WithError(err).Warning("Failed creating executor.")
		return nil, trace.Wrap(err)
	}

	streamOptions := proxy.options()
	err = executor.StreamWithContext(req.Context(), streamOptions)
	// send the status back to the client when forwarding mode is enabled
	// sendStatus sends a payload even if the error is nil to make sure the client
	// receives the status and can close the connection.
	if sendErr := proxy.sendStatus(err); sendErr != nil {
		f.log.WithError(sendErr).Warning("Failed to send status. Exec command was aborted by client.")
	}
	if err != nil {
		execEvent.Code = events.ExecFailureCode
		execEvent.Error, execEvent.ExitCode = exitCode(err)

		f.log.WithError(err).Warning("Executor failed while streaming.")
		// do not return the error otherwise the fwd.withAuth interceptor will try to write it into a hijacked connection
		return nil, nil
	}

	execEvent.Code = events.ExecCode

	return nil, nil
}

func exitCode(err error) (errMsg, code string) {
	var (
		kubeStatusErr = &kubeerrors.StatusError{}
		kubeExecErr   = kubeexec.CodeExitError{}
	)

	if errors.As(err, &kubeStatusErr) {
		if kubeStatusErr.ErrStatus.Status == metav1.StatusSuccess {
			return
		}
		errMsg = kubeStatusErr.ErrStatus.Message
		code = strconv.Itoa(int(kubeStatusErr.ErrStatus.Code))
	} else if errors.As(err, &kubeExecErr) {
		if kubeExecErr.Err != nil {
			errMsg = kubeExecErr.Err.Error()
		}
		code = strconv.Itoa(kubeExecErr.Code)
	} else if err != nil {
		errMsg = err.Error()
	}

	return
}

// exec forwards all exec requests to the target server, captures
// all output from the session
func (f *Forwarder) exec(ctx *authContext, w http.ResponseWriter, req *http.Request, p httprouter.Params) (resp any, err error) {
	f.log.Debugf("Exec %v.", req.URL.String())
	defer func() {
		if err != nil {
			f.log.WithError(err).Debug("Exec request failed")
		}
	}()

	sess, err := f.newClusterSession(*ctx)
	if err != nil {
		// This error goes to kubernetes client and is not visible in the logs
		// of the teleport server if not logged here.
		f.log.Errorf("Failed to create cluster session: %v.", err)
		return nil, trace.Wrap(err)
	}
	// sess.Close cancels the connection monitor context to release it sooner.
	// When the server is under heavy load it can take a while to identify that
	// the underlying connection is gone. This change prevents that and releases
	// the resources as soon as we know the session is no longer active.
	defer sess.close()

	sess.forwarder, err = f.makeSessionForwarder(sess)
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
		pingPeriod:         f.cfg.ConnPingPeriod,
		onResize:           func(remotecommand.TerminalSize) {},
	}

	if err := f.setupForwardingHeaders(sess, req, true /* withImpersonationHeaders */); err != nil {
		return nil, trace.Wrap(err)
	}

	proxy, err := createRemoteCommandProxy(request)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if sess.noAuditEvents {
		// We're forwarding this to another kubernetes_service instance, let it handle multiplexing.
		return f.remoteExec(ctx, w, req, p, sess, request, proxy)
	}

	if !request.tty {
		resp, err = f.execNonInteractive(ctx, w, req, p, request, proxy, sess)
		return
	}

	client := newKubeProxyClientStreams(proxy)
	party := newParty(*ctx, types.SessionPeerMode, client)
	session, err := newSession(*ctx, f, req, p, party, sess)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	f.setSession(session.id, session)
	err = session.join(party)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	<-party.closeC

	if _, err := session.leave(party.ID); err != nil {
		f.log.WithError(err).Debugf("Participant %q was unable to leave session %s", party.ID, session.id)
	}

	return nil, nil
}

// remoteExec forwards an exec request to a remote cluster.
func (f *Forwarder) remoteExec(ctx *authContext, w http.ResponseWriter, req *http.Request, p httprouter.Params, sess *clusterSession, request remoteCommandRequest, proxy *remoteCommandProxy) (resp any, err error) {
	defer proxy.Close()

	executor, err := f.getExecutor(*ctx, sess, req)
	if err != nil {
		f.log.WithError(err).Warning("Failed creating executor.")
		return nil, trace.Wrap(err)
	}
	streamOptions := proxy.options()
	err = executor.StreamWithContext(req.Context(), streamOptions)
	// send the status back to the client when forwarding mode is enabled
	// sendStatus sends a payload even if the error is nil to make sure the client
	// receives the status and can close the connection.
	if sendErr := proxy.sendStatus(err); sendErr != nil {
		f.log.WithError(sendErr).Warning("Failed to send status. Exec command was aborted by client.")
	}
	if err != nil {
		f.log.WithError(err).Warning("Executor failed while streaming.")
		// do not return the error otherwise the fwd.withAuth interceptor will try to write it into a hijacked connection
		return nil, nil
	}

	return nil, nil
}

// portForward starts port forwarding to the remote cluster
func (f *Forwarder) portForward(ctx *authContext, w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
	f.log.Debugf("Port forward: %v. req headers: %v.", req.URL.String(), req.Header)
	sess, err := f.newClusterSession(*ctx)
	if err != nil {
		// This error goes to kubernetes client and is not visible in the logs
		// of the teleport server if not logged here.
		f.log.Errorf("Failed to create cluster session: %v.", err)
		return nil, trace.Wrap(err)
	}
	// sess.Close cancels the connection monitor context to release it sooner.
	// When the server is under heavy load it can take a while to identify that
	// the underlying connection is gone. This change prevents that and releases
	// the resources as soon as we know the session is no longer active.
	defer sess.close()

	sess.forwarder, err = f.makeSessionForwarder(sess)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := f.setupForwardingHeaders(sess, req, true /* withImpersonationHeaders */); err != nil {
		f.log.Debugf("DENIED Port forward: %v.", req.URL.String())
		return nil, trace.Wrap(err)
	}

	dialer, err := f.getDialer(*ctx, sess, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	onPortForward := func(addr string, success bool) {
		if sess.noAuditEvents {
			return
		}
		portForward := &apievents.PortForward{
			Metadata: apievents.Metadata{
				Type: events.PortForwardEvent,
				Code: events.PortForwardCode,
			},
			UserMetadata: ctx.eventUserMeta(),
			ConnectionMetadata: apievents.ConnectionMetadata{
				LocalAddr:  sess.kubeAddress,
				RemoteAddr: req.RemoteAddr,
				Protocol:   events.EventProtocolKube,
			},
			Addr: addr,
			Status: apievents.Status{
				Success: success,
			},
		}
		if !success {
			portForward.Code = events.PortForwardFailureCode
		}
		if err := f.cfg.StreamEmitter.EmitAuditEvent(f.ctx, portForward); err != nil {
			f.log.WithError(err).Warn("Failed to emit event.")
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
		pingPeriod:         f.cfg.ConnPingPeriod,
	}
	f.log.Debugf("Starting %v.", request)
	err = runPortForwarding(request)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	f.log.Debugf("Done %v.", request)
	return nil, nil
}

// runPortForwarding checks if the request contains WebSocket upgrade headers and
// decides which protocol the client expects.
// Go client uses SPDY while other clients still require WebSockets.
// This function will run until the end of the execution of the request.
func runPortForwarding(req portForwardRequest) error {
	if wsstream.IsWebSocketRequest(req.httpRequest) {
		return trace.Wrap(runPortForwardingWebSocket(req))
	}
	return trace.Wrap(runPortForwardingHTTPStreams(req))
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

func (f *Forwarder) setupForwardingHeaders(sess *clusterSession, req *http.Request, withImpersonationHeaders bool) error {
	if withImpersonationHeaders {
		if err := setupImpersonationHeaders(f.log, sess.authContext, req.Header); err != nil {
			return trace.Wrap(err)
		}
	}
	// Setup scheme, override target URL to the destination address
	req.URL.Scheme = "https"
	req.RequestURI = req.URL.Path + "?" + req.URL.RawQuery

	// We only have a direct host to provide when using local creds.
	// Otherwise, use kube-teleport-proxy-alpn.teleport.cluster.local to pass TLS handshake and leverage TLS Routing.
	// TODO(smallinsky) UPDATE IN 11.0. use KubeTeleportProxyALPNPrefix instead.
	req.URL.Host = fmt.Sprintf("%s%s", constants.KubeSNIPrefix, constants.APIDomain)
	if sess.creds != nil {
		req.URL.Host = sess.creds.getTargetAddr()
	}

	// add origin headers so the service consuming the request on the other site
	// is aware of where it came from
	req.Header.Add("X-Forwarded-Proto", "https")
	req.Header.Add("X-Forwarded-Host", req.Host)
	req.Header.Add("X-Forwarded-Path", req.URL.Path)
	req.Header.Add("X-Forwarded-For", req.RemoteAddr)

	return nil
}

// setupImpersonationHeaders sets up Impersonate-User and Impersonate-Group headers
func setupImpersonationHeaders(log logrus.FieldLogger, ctx authContext, headers http.Header) error {
	if ctx.teleportCluster.isRemote {
		return nil
	}

	impersonateUser, impersonateGroups, err := computeImpersonatedPrincipals(ctx.kubeUsers, ctx.kubeGroups, headers)
	if err != nil {
		return trace.Wrap(err)
	}

	headers.Set(ImpersonateUserHeader, impersonateUser)

	// Make sure to overwrite the exiting headers, instead of appending to
	// them.
	headers.Del(ImpersonateGroupHeader)
	for _, group := range impersonateGroups {
		headers.Add(ImpersonateGroupHeader, group)
	}

	return nil
}

// copyImpersonationHeaders copies the impersonation headers from the source
// request to the destination request.
func copyImpersonationHeaders(dst, src http.Header) {
	dst.Del(ImpersonateUserHeader)
	dst.Del(ImpersonateGroupHeader)

	for _, v := range src.Values(ImpersonateUserHeader) {
		dst.Add(ImpersonateUserHeader, v)
	}

	for _, v := range src.Values(ImpersonateGroupHeader) {
		dst.Add(ImpersonateGroupHeader, v)
	}
}

// computeImpersonatedPrincipals computes the intersection between the information
// received in the `Impersonate-User` and `Impersonate-Groups` headers and the
// allowed values. If the user didn't specify any user and groups to impersonate,
// Teleport will use every group the user is allowed to impersonate.
func computeImpersonatedPrincipals(kubeUsers, kubeGroups map[string]struct{}, headers http.Header) (string, []string, error) {
	var impersonateUser string
	var impersonateGroups []string
	for header, values := range headers {
		if !strings.HasPrefix(header, "Impersonate-") {
			continue
		}
		switch header {
		case ImpersonateUserHeader:
			if impersonateUser != "" {
				return "", nil, trace.AccessDenied("%v, user already specified to %q", ImpersonationRequestDeniedMessage, impersonateUser)
			}
			if len(values) == 0 || len(values) > 1 {
				return "", nil, trace.AccessDenied("%v, invalid user header %q", ImpersonationRequestDeniedMessage, values)
			}
			// when Kubernetes go-client sends impersonated groups it also sends the impersonated user.
			// The issue arrises when the impersonated user was not defined and the user want to just impersonate
			// a subset of his groups. In that case the request would fail because empty user is not on
			// ctx.kubeUsers. If Teleport receives an empty impersonated user it will ignore it and later will fill it
			// with the Teleport username.
			if len(values[0]) == 0 {
				continue
			}
			impersonateUser = values[0]
			if _, ok := kubeUsers[impersonateUser]; !ok {
				return "", nil, trace.AccessDenied("%v, user header %q is not allowed in roles", ImpersonationRequestDeniedMessage, impersonateUser)
			}
		case ImpersonateGroupHeader:
			for _, group := range values {
				if _, ok := kubeGroups[group]; !ok {
					return "", nil, trace.AccessDenied("%v, group header %q value is not allowed in roles", ImpersonationRequestDeniedMessage, group)
				}
				impersonateGroups = append(impersonateGroups, group)
			}
		default:
			return "", nil, trace.AccessDenied("%v, unsupported impersonation header %q", ImpersonationRequestDeniedMessage, header)
		}
	}
	impersonateGroups = apiutils.Deduplicate(impersonateGroups)
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
		switch len(kubeUsers) {
		// this is currently not possible as kube users have at least one
		// user (user name), but in case if someone breaks it, catch here
		case 0:
			return "", nil, trace.AccessDenied("assumed at least one user to be present")
		// if there is deterministic choice, make it to improve user experience
		case 1:
			for user := range kubeUsers {
				impersonateUser = user
				break
			}
		default:
			return "", nil, trace.AccessDenied(
				"please select a user to impersonate, refusing to select a user due to several kubernetes_users set up for this user")
		}
	}
	if len(impersonateGroups) == 0 {
		for group := range kubeGroups {
			impersonateGroups = append(impersonateGroups, group)
		}
	}
	return impersonateUser, impersonateGroups, nil
}

// catchAll forwards all HTTP requests to the target k8s API server
func (f *Forwarder) catchAll(ctx *authContext, w http.ResponseWriter, req *http.Request) (any, error) {
	sess, err := f.newClusterSession(*ctx)
	if err != nil {
		// This error goes to kubernetes client and is not visible in the logs
		// of the teleport server if not logged here.
		f.log.Errorf("Failed to create cluster session: %v.", err)
		return nil, trace.Wrap(err)
	}
	// sess.Close cancels the connection monitor context to release it sooner.
	// When the server is under heavy load it can take a while to identify that
	// the underlying connection is gone. This change prevents that and releases
	// the resources as soon as we know the session is no longer active.
	defer sess.close()

	sess.upgradeToHTTP2 = true
	sess.forwarder, err = f.makeSessionForwarder(sess)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := f.setupForwardingHeaders(sess, req, true /* withImpersonationHeaders */); err != nil {
		// This error goes to kubernetes client and is not visible in the logs
		// of the teleport server if not logged here.
		f.log.Errorf("Failed to set up forwarding headers: %v.", err)
		return nil, trace.Wrap(err)
	}
	rw := httplib.NewResponseStatusRecorder(w)
	sess.forwarder.ServeHTTP(rw, req)

	if sess.noAuditEvents {
		return nil, nil
	}
	// Emit audit event.
	event := &apievents.KubeRequest{
		Metadata: apievents.Metadata{
			Type: events.KubeRequestEvent,
			Code: events.KubeRequestCode,
		},
		UserMetadata: ctx.eventUserMeta(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: req.RemoteAddr,
			LocalAddr:  sess.kubeAddress,
			Protocol:   events.EventProtocolKube,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerID:        f.cfg.HostID,
			ServerNamespace: f.cfg.Namespace,
		},
		RequestPath:               req.URL.Path,
		Verb:                      req.Method,
		ResponseCode:              int32(rw.Status()),
		KubernetesClusterMetadata: ctx.eventClusterMeta(req),
	}
	r := parseResourcePath(req.URL.Path)
	if r.skipEvent {
		return nil, nil
	}
	r.populateEvent(event)
	if err := f.cfg.AuthClient.EmitAuditEvent(f.ctx, event); err != nil {
		f.log.WithError(err).Warn("Failed to emit event.")
	}

	return nil, nil
}

func (f *Forwarder) getExecutor(ctx authContext, sess *clusterSession, req *http.Request) (remotecommand.Executor, error) {
	upgradeRoundTripper := NewSpdyRoundTripperWithDialer(roundTripperConfig{
		ctx:             req.Context(),
		authCtx:         ctx,
		dial:            sess.DialWithContext,
		tlsConfig:       sess.tlsConfig,
		pingPeriod:      f.cfg.ConnPingPeriod,
		originalHeaders: req.Header,
	})
	rt := http.RoundTripper(upgradeRoundTripper)
	if sess.creds != nil {
		var err error
		rt, err = sess.creds.wrapTransport(rt)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	rt = tracehttp.NewTransport(rt)

	return remotecommand.NewSPDYExecutorForTransports(rt, upgradeRoundTripper, req.Method, req.URL)
}

func (f *Forwarder) getDialer(ctx authContext, sess *clusterSession, req *http.Request) (httpstream.Dialer, error) {
	upgradeRoundTripper := NewSpdyRoundTripperWithDialer(roundTripperConfig{
		ctx:             req.Context(),
		authCtx:         ctx,
		dial:            sess.DialWithContext,
		tlsConfig:       sess.tlsConfig,
		pingPeriod:      f.cfg.ConnPingPeriod,
		originalHeaders: req.Header,
	})
	rt := http.RoundTripper(upgradeRoundTripper)
	if sess.creds != nil {
		var err error
		rt, err = sess.creds.wrapTransport(rt)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	client := &http.Client{
		Transport: tracehttp.NewTransport(rt),
	}

	return spdy.NewDialer(upgradeRoundTripper, client, req.Method, req.URL), nil
}

// clusterSession contains authenticated user session to the target cluster:
// x509 short lived credentials, forwarding proxies and other data
type clusterSession struct {
	authContext
	parent    *Forwarder
	creds     kubeCreds
	tlsConfig *tls.Config
	forwarder *forward.Forwarder
	// noAuditEvents is true if this teleport service should leave audit event
	// logging to another service.
	noAuditEvents        bool
	kubeClusterEndpoints []kubeClusterEndpoint
	// kubeAddress is the address of this session's active connection (if there is one)
	kubeAddress string
	// upgradeToHTTP2 indicates whether the transport should be configured to use HTTP2.
	// A HTTP2 configured transport does not work with connections that are going to be
	// upgraded to SPDY, like in the cases of exec, port forward...
	upgradeToHTTP2 bool
	// monitorCancel is the conn monitor monitorCancel function.
	monitorCancel context.CancelFunc
}

// close cancels the connection monitor context if available.
func (s *clusterSession) close() {
	if s.monitorCancel != nil {
		s.monitorCancel()
	}
}

// kubeClusterEndpoint can be used to connect to a kube cluster
type kubeClusterEndpoint struct {
	// addr is a direct network address.
	addr string
	// serverID is the server:cluster ID of the endpoint,
	// which is used to find its corresponding reverse tunnel.
	serverID string
	// proxyIDs is the list of proxy ids that the cluster is
	// connected to.
	proxyIDs []string
}

func (s *clusterSession) monitorConn(conn net.Conn, err error) (net.Conn, error) {
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(s.parent.ctx)
	s.monitorCancel = cancel
	tc, err := srv.NewTrackingReadConn(srv.TrackingReadConnConfig{
		Conn:    conn,
		Clock:   s.parent.cfg.Clock,
		Context: ctx,
		Cancel:  cancel,
	})
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	err = srv.StartMonitor(srv.MonitorConfig{
		LockWatcher:           s.parent.cfg.LockWatcher,
		LockTargets:           s.LockTargets(),
		DisconnectExpiredCert: s.disconnectExpiredCert,
		ClientIdleTimeout:     s.clientIdleTimeout,
		Clock:                 s.parent.cfg.Clock,
		Tracker:               tc,
		Conn:                  tc,
		Context:               ctx,
		TeleportUser:          s.User.GetName(),
		ServerID:              s.parent.cfg.HostID,
		Entry:                 s.parent.log,
		Emitter:               s.parent.cfg.AuthClient,
	})
	if err != nil {
		tc.Close()
		cancel()
		return nil, trace.Wrap(err)
	}
	return tc, nil
}

func (s *clusterSession) Dial(network, addr string) (net.Conn, error) {
	return s.monitorConn(s.dial(context.Background(), network))
}

func (s *clusterSession) DialWithContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return s.monitorConn(s.dial(ctx, network))
}

func (s *clusterSession) dial(ctx context.Context, network string) (net.Conn, error) {
	if len(s.kubeClusterEndpoints) == 0 {
		return nil, trace.BadParameter("no kube services to dial")
	}

	// Shuffle endpoints to balance load
	shuffledEndpoints := make([]kubeClusterEndpoint, len(s.kubeClusterEndpoints))
	copy(shuffledEndpoints, s.kubeClusterEndpoints)
	mathrand.Shuffle(len(shuffledEndpoints), func(i, j int) {
		shuffledEndpoints[i], shuffledEndpoints[j] = shuffledEndpoints[j], shuffledEndpoints[i]
	})

	errs := []error{}
	for _, endpoint := range shuffledEndpoints {
		conn, err := s.teleportCluster.dialEndpoint(ctx, network, endpoint)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		s.kubeAddress = endpoint.addr
		return conn, nil
	}
	return nil, trace.NewAggregate(errs...)
}

// TODO(awly): unit test this
func (f *Forwarder) newClusterSession(ctx authContext) (*clusterSession, error) {
	if ctx.teleportCluster.isRemote {
		return f.newClusterSessionRemoteCluster(ctx)
	}
	return f.newClusterSessionSameCluster(ctx)
}

func (f *Forwarder) newClusterSessionRemoteCluster(ctx authContext) (*clusterSession, error) {
	tlsConfig, err := f.getOrRequestClientCreds(ctx)
	if err != nil {
		f.log.Warningf("Failed to get certificate for %v: %v.", ctx, err)
		return nil, trace.AccessDenied("access denied: failed to authenticate with auth server")
	}

	f.log.Debugf("Forwarding kubernetes session for %v to remote cluster.", ctx)
	return &clusterSession{
		parent:      f,
		authContext: ctx,
		// Proxy uses reverse tunnel dialer to connect to Kubernetes in a leaf cluster
		// and the targetKubernetes cluster endpoint is determined from the identity
		// encoded in the TLS certificate. We're setting the dial endpoint to a hardcoded
		// `kube.teleport.cluster.local` value to indicate this is a Kubernetes proxy request
		kubeClusterEndpoints: []kubeClusterEndpoint{{addr: reversetunnel.LocalKubernetes}},
		tlsConfig:            tlsConfig.Clone(),
	}, nil
}

func (f *Forwarder) newClusterSessionSameCluster(ctx authContext) (*clusterSession, error) {
	// Try local creds first
	sess, localErr := f.newClusterSessionLocal(ctx)
	if localErr == nil {
		return sess, nil
	}

	kubeServers := ctx.kubeServers
	if len(kubeServers) == 0 && ctx.kubeCluster == ctx.teleportCluster.name {
		return nil, trace.Wrap(localErr)
	}

	// Validate that the requested kube cluster is registered.
	var endpoints []kubeClusterEndpoint
	for _, s := range kubeServers {
		kubeCluster := s.GetCluster()
		if kubeCluster.GetName() != ctx.kubeCluster {
			continue
		}

		// TODO(awly): check RBAC
		endpoints = append(endpoints, kubeClusterEndpoint{
			serverID: fmt.Sprintf("%s.%s", s.GetHostID(), ctx.teleportCluster.name),
			addr:     s.GetHostname(),
			proxyIDs: s.GetProxyIDs(),
		})
		continue

	}
	if len(endpoints) == 0 {
		return nil, trace.NotFound("kubernetes cluster %q is not found in teleport cluster %q", ctx.kubeCluster, ctx.teleportCluster.name)
	}
	return f.newClusterSessionDirect(ctx, endpoints)
}

func (f *Forwarder) newClusterSessionLocal(ctx authContext) (*clusterSession, error) {
	details, err := f.findKubeDetailsByClusterName(ctx.kubeCluster)
	if err != nil {
		return nil, trace.NotFound("kubernetes cluster %q not found", ctx.kubeCluster)
	}

	f.log.Debugf("Handling kubernetes session for %v using local credentials.", ctx)
	return &clusterSession{
		parent:               f,
		authContext:          ctx,
		creds:                details.kubeCreds,
		kubeClusterEndpoints: []kubeClusterEndpoint{{addr: details.getTargetAddr()}},
		tlsConfig:            details.getTLSConfig().Clone(),
	}, nil
}

func (f *Forwarder) newClusterSessionDirect(ctx authContext, endpoints []kubeClusterEndpoint) (*clusterSession, error) {
	if len(endpoints) == 0 {
		return nil, trace.BadParameter("no kube cluster endpoints provided")
	}

	tlsConfig, err := f.getOrRequestClientCreds(ctx)
	if err != nil {
		f.log.Warningf("Failed to get certificate for %v: %v.", ctx, err)
		return nil, trace.AccessDenied("access denied: failed to authenticate with auth server")
	}

	f.log.WithField("kube_service.endpoints", endpoints).Debugf("Kubernetes session for %v forwarded to remote kubernetes_service instance.", ctx)
	return &clusterSession{
		parent:               f,
		authContext:          ctx,
		kubeClusterEndpoints: endpoints,
		tlsConfig:            tlsConfig.Clone(),
		// This session talks to a kubernetes_service, which should handle
		// audit logging. Avoid duplicate logging.
		noAuditEvents: true,
	}, nil
}

// makeSessionForwader creates a new forward.Forwarder with a transport that
// is either configured:
// - for HTTP1 in case it's going to be used against streaming andoints like exec and port forward.
// - for HTTP2 in all other cases.
// The reason being is that streaming requests are going to be upgraded to SPDY, which is only
// supported coming from an HTTP1 request.
func (f *Forwarder) makeSessionForwarder(sess *clusterSession) (*forward.Forwarder, error) {
	var err error
	transport := f.newTransport(sess.Dial, sess.tlsConfig)

	if sess.upgradeToHTTP2 {
		// Upgrade transport to h2 where HTTP_PROXY and HTTPS_PROXY
		// envs are not take into account purposely.
		if err := http2.ConfigureTransport(transport); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	rt := http.RoundTripper(transport)
	if sess.creds != nil {
		rt, err = sess.creds.wrapTransport(rt)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	rt = tracehttp.NewTransport(rt)

	forwarder, err := forward.New(
		forward.FlushInterval(100*time.Millisecond),
		forward.RoundTripper(rt),
		forward.WebsocketDial(sess.Dial),
		forward.Logger(f.log),
		forward.ErrorHandler(fwdutils.ErrorHandlerFunc(f.formatForwardResponseError)),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return forwarder, nil
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
	f.mu.Lock()
	defer f.mu.Unlock()
	ctx, ok := f.activeRequests[key]
	if ok {
		return ctx, nil
	}
	ctx, cancel := context.WithCancel(f.ctx)
	f.activeRequests[key] = ctx
	return ctx, func() {
		cancel()
		f.mu.Lock()
		defer f.mu.Unlock()
		delete(f.activeRequests, key)
	}
}

func (f *Forwarder) getOrRequestClientCreds(ctx authContext) (*tls.Config, error) {
	c := f.getClientCreds(ctx)
	if c == nil {
		return f.serializedRequestClientCreds(ctx)
	}
	return c, nil
}

func (f *Forwarder) getClientCreds(ctx authContext) *tls.Config {
	f.mu.Lock()
	defer f.mu.Unlock()
	creds, ok := f.clientCredentials.Get(ctx.key())
	if !ok {
		return nil
	}
	c := creds.(*tls.Config)
	if !validClientCreds(f.cfg.Clock, c) {
		return nil
	}
	return c
}

func (f *Forwarder) saveClientCreds(ctx authContext, c *tls.Config) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.clientCredentials.Set(ctx.key(), c, ctx.sessionTTL)
}

func validClientCreds(clock clockwork.Clock, c *tls.Config) bool {
	if len(c.Certificates) == 0 || len(c.Certificates[0].Certificate) == 0 {
		return false
	}
	crt, err := x509.ParseCertificate(c.Certificates[0].Certificate[0])
	if err != nil {
		return false
	}
	// Make sure that the returned cert will be valid for at least 1 more
	// minute.
	return clock.Now().Add(time.Minute).Before(crt.NotAfter)
}

func (f *Forwarder) serializedRequestClientCreds(authContext authContext) (*tls.Config, error) {
	ctx, cancel := f.getOrCreateRequestContext(authContext.key())
	if cancel != nil {
		f.log.Debugf("Requesting new ephemeral user certificate for %v.", authContext)
		defer cancel()
		c, err := f.requestCertificate(authContext)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return c, f.saveClientCreds(authContext, c)
	}
	// cancel == nil means that another request is in progress, so simply wait until
	// it finishes or fails
	f.log.Debugf("Another request is in progress for %v, waiting until it gets completed.", authContext)
	select {
	case <-ctx.Done():
		c := f.getClientCreds(authContext)
		if c == nil {
			return nil, trace.BadParameter("failed to request ephemeral certificate, try again")
		}
		return c, nil
	case <-f.ctx.Done():
		return nil, trace.BadParameter("forwarder is closing, aborting the request")
	}
}

func (f *Forwarder) requestCertificate(ctx authContext) (*tls.Config, error) {
	f.log.Debugf("Requesting K8s cert for %v.", ctx)
	keyPEM, _, err := native.GenerateKeyPair()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	privateKey, err := ssh.ParseRawPrivateKey(keyPEM)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse private key")
	}

	// Note: ctx.UnmappedIdentity can potentially have temporary roles granted via
	// workflow API. Always use the Subject() method to preserve the roles from
	// caller's certificate.
	//
	// Also note: we need to send the UnmappedIdentity which could be a remote
	// user identity. If we used the local mapped identity instead, the
	// receiver of this certificate will think this is a local user and fail to
	// find it in the backend.
	callerIdentity := ctx.UnmappedIdentity.GetIdentity()
	subject, err := callerIdentity.Subject()
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

	response, err := f.cfg.AuthClient.ProcessKubeCSR(auth.KubeCSR{
		Username:    ctx.User.GetName(),
		ClusterName: ctx.teleportCluster.name,
		CSR:         csrPEM,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	f.log.Debugf("Received valid K8s cert for %v.", ctx)

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
	//nolint:staticcheck // Keep BuildNameToCertificate to avoid changes in legacy behavior.
	tlsConfig.BuildNameToCertificate()

	return tlsConfig, nil
}

// kubeClusters returns the list of available clusters
func (f *Forwarder) kubeClusters() types.KubeClusters {
	f.rwMutexDetails.RLock()
	defer f.rwMutexDetails.RUnlock()
	res := make(types.KubeClusters, 0, len(f.clusterDetails))
	for _, cred := range f.clusterDetails {
		cluster := cred.kubeCluster.Copy()
		res = append(res,
			cluster,
		)
	}
	return res
}

// findKubeDetailsByClusterName searches for the cluster details otherwise returns a trace.NotFound error.
func (f *Forwarder) findKubeDetailsByClusterName(name string) (*kubeDetails, error) {
	f.rwMutexDetails.RLock()
	defer f.rwMutexDetails.RUnlock()

	if creds, ok := f.clusterDetails[name]; ok {
		return creds, nil
	}

	return nil, trace.NotFound("cluster %s not found", name)
}

// upsertKubeDetails updates the details in f.ClusterDetails for key if they exist,
// otherwise inserts them.
func (f *Forwarder) upsertKubeDetails(key string, clusterDetails *kubeDetails) {
	f.rwMutexDetails.Lock()
	defer f.rwMutexDetails.Unlock()

	if oldDetails, ok := f.clusterDetails[key]; ok {
		oldDetails.Close()
	}
	// replace existing details in map
	f.clusterDetails[key] = clusterDetails
}

// removeKubeDetails removes the kubeDetails from map.
func (f *Forwarder) removeKubeDetails(name string) {
	f.rwMutexDetails.Lock()
	defer f.rwMutexDetails.Unlock()

	if oldDetails, ok := f.clusterDetails[name]; ok {
		oldDetails.Close()
	}
	delete(f.clusterDetails, name)
}
