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

package proxy

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	gwebsocket "github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/apimachinery/pkg/util/httpstream/wsstream"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"
	kwebsocket "k8s.io/client-go/transport/websocket"
	kubeexec "k8s.io/client-go/util/exec"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/observability/tracing"
	tracehttp "github.com/gravitational/teleport/api/observability/tracing/http"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
	"github.com/gravitational/teleport/lib/kube/proxy/streamproto"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/utils"
)

// KubeServiceType specifies a Teleport service type which can forward Kubernetes requests
type KubeServiceType = string

const (
	// KubeService is a Teleport kubernetes_service. A KubeService always forwards
	// requests directly to a Kubernetes endpoint.
	KubeService = "kube_service"
	// ProxyService is a Teleport proxy_service with kube_listen_addr/
	// kube_public_addr enabled. A ProxyService always forwards requests to a
	// Teleport KubeService or LegacyProxyService.
	ProxyService = "kube_proxy"
	// LegacyProxyService is a Teleport proxy_service with the kubernetes section
	// enabled. A LegacyProxyService can forward requests directly to a Kubernetes
	// endpoint, or to another Teleport LegacyProxyService or KubeService.
	LegacyProxyService = "legacy_proxy"
)

// ForwarderConfig specifies configuration for proxy forwarder
type ForwarderConfig struct {
	// ReverseTunnelSrv is the teleport reverse tunnel server
	ReverseTunnelSrv reversetunnelclient.Server
	// ClusterName is a local cluster name
	ClusterName string
	// Keygen points to a key generator implementation
	Keygen sshca.Authority
	// Authz authenticates user
	Authz authz.Authorizer
	// AuthClient is a auth server client.
	AuthClient auth.ClientI
	// CachingAuthClient is a caching auth server client for read-only access.
	CachingAuthClient auth.ReadKubernetesAccessPoint
	// Emitter is used to emit audit events
	Emitter apievents.Emitter
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
	CheckImpersonationPermissions servicecfg.ImpersonationPermissionsChecker
	// PublicAddr is the address that can be used to reach the kube cluster
	PublicAddr string
	// PROXYSigner is used to sign PROXY headers for securely propagating client IP address
	PROXYSigner multiplexer.PROXYHeaderSigner
	// log is the logger function
	log logrus.FieldLogger
	// TracerProvider is used to create tracers capable
	// of starting spans.
	TracerProvider oteltrace.TracerProvider
	// Tracer is used to start spans.
	tracer oteltrace.Tracer
	// ConnTLSConfig is the TLS client configuration to use when connecting to
	// the upstream Teleport proxy or Kubernetes service when forwarding requests
	// using the forward identity (i.e. proxy impersonating a user) method.
	ConnTLSConfig *tls.Config
	// ClusterFeaturesGetter is a function that returns the Teleport cluster licensed features.
	// It is used to determine if the cluster is licensed for Kubernetes usage.
	ClusterFeatures ClusterFeaturesGetter
}

// ClusterFeaturesGetter is a function that returns the Teleport cluster licensed features.
type ClusterFeaturesGetter func() proto.Features

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
	if f.Emitter == nil {
		return trace.BadParameter("missing parameter Emitter")
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
	if f.ClusterFeatures == nil {
		return trace.BadParameter("missing parameter ClusterFeatures")
	}
	if f.KubeServiceType != KubeService && f.PROXYSigner == nil {
		return trace.BadParameter("missing parameter PROXYSigner")
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

	if f.TracerProvider == nil {
		f.TracerProvider = tracing.DefaultProvider()
	}

	f.tracer = f.TracerProvider.Tracer("kube")

	switch f.KubeServiceType {
	case KubeService:
	case ProxyService, LegacyProxyService:
		if f.ConnTLSConfig == nil {
			return trace.BadParameter("missing parameter TLSConfig")
		}
		// Reset the ServerName to ensure that the proxy does not use the
		// proxy's hostname as the SNI when connecting to the Kubernetes service.
		f.ConnTLSConfig.ServerName = ""
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

	// TODO (tigrato): remove this once we have a better way to handle
	// deleting expired entried clusters and kube_servers entries.
	// In the meantime, we need to make sure that the cache is cleaned
	// from time to time.
	transportClients, err := ttlmap.New(defaults.ClientCacheSize, ttlmap.Clock(cfg.Clock))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	closeCtx, close := context.WithCancel(cfg.Context)
	fwd := &Forwarder{
		log:            cfg.log,
		cfg:            cfg,
		activeRequests: make(map[string]context.Context),
		ctx:            closeCtx,
		close:          close,
		sessions:       make(map[uuid.UUID]*session),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		clusterDetails:  make(map[string]*kubeDetails),
		cachedTransport: transportClients,
	}

	router := httprouter.New()

	router.UseRawPath = true

	router.GET("/version", fwd.withAuth(
		func(ctx *authContext, w http.ResponseWriter, r *http.Request, _ httprouter.Params) (any, error) {
			// Forward version requests to the cluster.
			return fwd.catchAll(ctx, w, r)
		},
		withCustomErrFormatter(fwd.writeResponseErrorToBody),
	))

	router.POST("/api/:ver/namespaces/:podNamespace/pods/:podName/exec", fwd.withAuth(fwd.exec))
	router.GET("/api/:ver/namespaces/:podNamespace/pods/:podName/exec", fwd.withAuth(fwd.exec))

	router.POST("/api/:ver/namespaces/:podNamespace/pods/:podName/attach", fwd.withAuth(fwd.exec))
	router.GET("/api/:ver/namespaces/:podNamespace/pods/:podName/attach", fwd.withAuth(fwd.exec))

	router.POST("/api/:ver/namespaces/:podNamespace/pods/:podName/portforward", fwd.withAuth(fwd.portForward))
	router.GET("/api/:ver/namespaces/:podNamespace/pods/:podName/portforward", fwd.withAuth(fwd.portForward))

	router.POST("/apis/authorization.k8s.io/:ver/selfsubjectaccessreviews", fwd.withAuth(fwd.selfSubjectAccessReviews))

	router.GET("/api/:ver/teleport/join/:session", fwd.withAuthPassthrough(fwd.join))

	router.NotFound = fwd.withAuthStd(fwd.catchAll)

	fwd.router = instrumentHTTPHandler(fwd.cfg.KubeServiceType, router)

	if cfg.ClusterOverride != "" {
		fwd.log.Debugf("Cluster override is set, forwarder will send all requests to remote cluster %v.", cfg.ClusterOverride)
	}
	if len(cfg.KubeClusterName) > 0 || len(cfg.KubeconfigPath) > 0 || cfg.KubeServiceType != KubeService {
		if err := fwd.getKubeDetails(cfg.Context); err != nil {
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
	router http.Handler
	cfg    ForwarderConfig
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

	// cachedTransport is a cache of cachedTransportEntry objects used to
	// connect to Teleport services.
	// TODO(tigrato): Implement a cache eviction policy using watchers.
	cachedTransport *ttlmap.TTLMap
	// cachedTransportMu is a mutex used to protect the cachedTransport.
	cachedTransportMu sync.Mutex
}

// cachedTransportEntry is a cached transport entry used to connect to
// Teleport services. It contains a cached http.RoundTripper and a cached
// tls.Config.
type cachedTransportEntry struct {
	transport http.RoundTripper
	tlsConfig *tls.Config
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
	authz.Context
	kubeGroups        map[string]struct{}
	kubeUsers         map[string]struct{}
	kubeClusterLabels map[string]string
	kubeClusterName   string
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
	// kubeCluster is the Kubernetes cluster the request is targeted to.
	// It's only available after authorization layer.
	kubeCluster types.KubeCluster
	// kubeResource is the kubernetes resource the request is targeted at.
	// Can be nil, if the resource is not a pod or the request is not targeted
	// at a specific pod.
	// If non empty, kubeResource.Kind is populated with type "pod",
	// kubeResource.Namespace is the resource namespace and kubeResource.Name
	// is the resource name.
	kubeResource *types.KubernetesResource
	// requestVerb is the Kubernetes Verb.
	requestVerb string
	// kubeServers are the registered agents for the kubernetes cluster the request
	// is targeted to.
	kubeServers []types.KubeServer
	// apiResource holds the information about the requested API resource.
	apiResource apiResource
}

func (c authContext) String() string {
	return fmt.Sprintf("user: %v, users: %v, groups: %v, teleport cluster: %v, kube cluster: %v", c.User.GetName(), c.kubeUsers, c.kubeGroups, c.teleportCluster.name, c.kubeClusterName)
}

func (c *authContext) key() string {
	// it is important that the context key contains user, kubernetes groups and certificate expiry,
	// so that new logins with different parameters will not reuse this context
	return fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v", c.teleportCluster.name, c.User.GetName(), c.kubeUsers, c.kubeGroups, c.kubeClusterName, c.certExpires.Unix(), c.Identity.GetIdentity().ActiveRequests)
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
		KubernetesCluster: c.kubeClusterName,
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

func (c *authContext) eventUserMetaWithLogin(login string) apievents.UserMetadata {
	meta := c.eventUserMeta()
	meta.Login = login
	return meta
}

// teleportClusterClient is a client for either a k8s endpoint in local cluster or a
// proxy endpoint in a remote cluster.
type teleportClusterClient struct {
	remoteAddr utils.NetAddr
	name       string
	isRemote   bool
}

// handlerWithAuthFunc is http handler with passed auth context
type handlerWithAuthFunc func(ctx *authContext, w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error)

// handlerWithAuthFuncStd is http handler with passed auth context
type handlerWithAuthFuncStd func(ctx *authContext, w http.ResponseWriter, r *http.Request) (any, error)

// accessDeniedMsg is a message returned to the client when access is denied.
const accessDeniedMsg = "[00] access denied"

// authenticate function authenticates request
func (f *Forwarder) authenticate(req *http.Request) (*authContext, error) {
	// If the cluster is not licensed for Kubernetes, return an error to the client.
	if !f.cfg.ClusterFeatures().Kubernetes {
		// If the cluster is not licensed for Kubernetes, return an error to the client.
		return nil, trace.AccessDenied("Teleport cluster is not licensed for Kubernetes")
	}
	ctx, span := f.cfg.tracer.Start(
		req.Context(),
		"kube.Forwarder/authenticate",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
			semconv.RPCSystemKey.String("kube"),
		),
	)
	defer span.End()

	var isRemoteUser bool
	userTypeI, err := authz.UserFromContext(ctx)
	if err != nil {
		f.log.WithError(err).Warn("error getting user from context")
		return nil, trace.AccessDenied(accessDeniedMsg)
	}
	switch userTypeI.(type) {
	case authz.LocalUser:

	case authz.RemoteUser:
		isRemoteUser = true
	case authz.BuiltinRole:
		f.log.Warningf("Denying proxy access to unauthenticated user of type %T - this can sometimes be caused by inadvertently using an HTTP load balancer instead of a TCP load balancer on the Kubernetes port.", userTypeI)
		return nil, trace.AccessDenied(accessDeniedMsg)
	default:
		f.log.Warningf("Denying proxy access to unsupported user type: %T.", userTypeI)
		return nil, trace.AccessDenied(accessDeniedMsg)
	}

	userContext, err := f.cfg.Authz.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authContext, err := f.setupContext(ctx, *userContext, req, isRemoteUser)
	if err != nil {
		f.log.WithError(err).Warn("Unable to setup context.")
		if trace.IsAccessDenied(err) {
			return nil, trace.AccessDenied(accessDeniedMsg)
		}
		return nil, trace.Wrap(err)
	}
	return authContext, nil
}

func (f *Forwarder) withAuthStd(handler handlerWithAuthFuncStd) http.HandlerFunc {
	return httplib.MakeStdHandlerWithErrorWriter(func(w http.ResponseWriter, req *http.Request) (any, error) {
		ctx, span := f.cfg.tracer.Start(
			req.Context(),
			"kube.Forwarder/withAuthStd",
			oteltrace.WithSpanKind(oteltrace.SpanKindServer),
			oteltrace.WithAttributes(
				semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
				semconv.RPCSystemKey.String("kube"),
			),
		)
		req = req.WithContext(ctx)
		defer span.End()

		authContext, err := f.authenticate(req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := f.authorize(ctx, authContext); err != nil {
			return nil, trace.Wrap(err)
		}

		return handler(authContext, w, req)
	}, f.formatStatusResponseError)
}

// acquireConnectionLockWithIdentity acquires a connection lock under a given identity.
func (f *Forwarder) acquireConnectionLockWithIdentity(ctx context.Context, identity *authContext) error {
	ctx, span := f.cfg.tracer.Start(
		ctx,
		"kube.Forwarder/acquireConnectionLockWithIdentity",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
			semconv.RPCSystemKey.String("kube"),
		),
	)
	defer span.End()
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
		ctx, span := f.cfg.tracer.Start(
			req.Context(),
			"kube.Forwarder/withAuth",
			oteltrace.WithSpanKind(oteltrace.SpanKindServer),
			oteltrace.WithAttributes(
				semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
				semconv.RPCSystemKey.String("kube"),
			),
		)
		req = req.WithContext(ctx)
		defer span.End()
		authContext, err := f.authenticate(req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := f.authorize(ctx, authContext); err != nil {
			return nil, trace.Wrap(err)
		}
		err = f.acquireConnectionLockWithIdentity(ctx, authContext)
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
		ctx, span := f.cfg.tracer.Start(
			req.Context(),
			"kube.Forwarder/withAuthPassthrough",
			oteltrace.WithSpanKind(oteltrace.SpanKindServer),
			oteltrace.WithAttributes(
				semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
				semconv.RPCSystemKey.String("kube"),
			),
		)
		req = req.WithContext(ctx)
		defer span.End()

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
	code := trace.ErrorToCode(respErr)
	status := &metav1.Status{
		Status: metav1.StatusFailure,
		// Don't trace.Unwrap the error, in case it was wrapped with a
		// user-friendly message. The underlying root error is likely too
		// low-level to be useful.
		Message: respErr.Error(),
		Code:    int32(code),
		Reason:  errorToKubeStatusReason(respErr, code),
	}
	data, err := runtime.Encode(globalKubeCodecs.LegacyCodec(), status)
	if err != nil {
		f.log.Warningf("Failed encoding error into kube Status object: %v", err)
		trace.WriteError(rw, respErr)
		return
	}
	rw.Header().Set(responsewriters.ContentTypeHeader, "application/json")
	// Always write the correct error code in the response so kubectl can parse
	// it correctly. If response code and status.Code drift, kubectl prints
	// `Error from server (InternalError): an error on the server ("unknown")
	// has prevented the request from succeeding`` instead of the correct reason.
	rw.WriteHeader(trace.ErrorToCode(respErr))
	if _, err := rw.Write(data); err != nil {
		f.log.Warningf("Failed writing kube error response body: %v", err)
	}
}

func (f *Forwarder) setupContext(
	ctx context.Context,
	authCtx authz.Context,
	req *http.Request,
	isRemoteUser bool,
) (*authContext, error) {
	ctx, span := f.cfg.tracer.Start(
		ctx,
		"kube.Forwarder/setupContext",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
			semconv.RPCSystemKey.String("kube"),
		),
	)
	defer span.End()

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

	var (
		kubeServers  []types.KubeServer
		kubeResource *types.KubernetesResource
		apiResource  apiResource
		err          error
	)

	kubeCluster := identity.KubernetesCluster
	// Only check k8s principals for local clusters.
	//
	// For remote clusters, everything will be remapped to new roles on the
	// leaf and checked there.
	if !isRemoteCluster {
		kubeServers, err = f.getKubernetesServersForKubeCluster(ctx, kubeCluster)
		if err != nil || len(kubeServers) == 0 {
			return nil, trace.NotFound("Kubernetes cluster %q not found", kubeCluster)
		}
	}
	if f.isLocalKubeCluster(isRemoteCluster, kubeCluster) {
		kubeResource, apiResource, err = f.parseResourceFromRequest(req, kubeCluster)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	netConfig, err := f.cfg.CachingAuthClient.GetClusterNetworkingConfig(f.ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	recordingConfig, err := f.cfg.CachingAuthClient.GetSessionRecordingConfig(f.ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authPref, err := f.cfg.CachingAuthClient.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &authContext{
		clientIdleTimeout:     roles.AdjustClientIdleTimeout(netConfig.GetClientIdleTimeout()),
		sessionTTL:            sessionTTL,
		Context:               authCtx,
		recordingConfig:       recordingConfig,
		kubeClusterName:       kubeCluster,
		certExpires:           identity.Expires,
		disconnectExpiredCert: srv.GetDisconnectExpiredCertFromIdentity(roles, authPref, &identity),
		teleportCluster: teleportClusterClient{
			name:       teleportClusterName,
			remoteAddr: utils.NetAddr{AddrNetwork: "tcp", Addr: req.RemoteAddr},
			isRemote:   isRemoteCluster,
		},
		kubeServers:  kubeServers,
		requestVerb:  apiResource.getVerb(req),
		apiResource:  apiResource,
		kubeResource: kubeResource,
	}, nil
}

func (f *Forwarder) parseResourceFromRequest(req *http.Request, kubeClusterName string) (*types.KubernetesResource, apiResource, error) {
	switch f.cfg.KubeServiceType {
	case LegacyProxyService:
		if details, err := f.findKubeDetailsByClusterName(kubeClusterName); err == nil {
			resource, apiRes, err := getResourceFromRequest(req, details)
			return resource, apiRes, trace.Wrap(err)
		}
		// When the cluster is not being served by the local service, the LegacyProxy
		// is working as a normal proxy and will forward the request to the remote
		// service. When this happens, proxy won't enforce any Kubernetes RBAC rules
		// and will forward the request as is to the remote service. The remote
		// service will enforce RBAC rules and will return an error if the user is
		// not authorized.
		fallthrough
	case ProxyService:
		// When the service is acting as a proxy (ProxyService or LegacyProxyService
		// if the local cluster wasn't found), the proxy will forward the request
		// to the remote service without enforcing any RBAC rules - we send the
		// details = nil to indicate that we don't want to extract the kube resource
		// from the request.
		resource, apiRes, err := getResourceFromRequest(req, nil /*details*/)
		return resource, apiRes, trace.Wrap(err)
	case KubeService:
		details, err := f.findKubeDetailsByClusterName(kubeClusterName)
		if err != nil {
			return nil, apiResource{}, trace.Wrap(err)
		}
		resource, apiRes, err := getResourceFromRequest(req, details)
		return resource, apiRes, trace.Wrap(err)

	default:
		return nil, apiResource{}, trace.BadParameter("unsupported kube service type: %q", f.cfg.KubeServiceType)
	}
}

// emitAuditEvent emits the audit event for a `kube.request` event if the session
// requires audit events.
func (f *Forwarder) emitAuditEvent(req *http.Request, sess *clusterSession, status int) {
	_, span := f.cfg.tracer.Start(
		req.Context(),
		"kube.Forwarder/emitAuditEvent",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
			semconv.RPCSystemKey.String("kube"),
		),
	)
	defer span.End()

	if sess.noAuditEvents {
		return
	}
	r := sess.apiResource
	if r.skipEvent {
		return
	}
	// Emit audit event.
	event := &apievents.KubeRequest{
		Metadata: apievents.Metadata{
			Type: events.KubeRequestEvent,
			Code: events.KubeRequestCode,
		},
		UserMetadata: sess.eventUserMeta(),
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
		ResponseCode:              int32(status),
		KubernetesClusterMetadata: sess.eventClusterMeta(req),
		SessionMetadata: apievents.SessionMetadata{
			WithMFA: sess.Identity.GetIdentity().MFAVerified,
		},
	}

	r.populateEvent(event)
	if err := f.cfg.AuthClient.EmitAuditEvent(f.ctx, event); err != nil {
		f.log.WithError(err).Warn("Failed to emit event.")
	}
}

// fillDefaultKubePrincipalDetails fills the default details in order to keep
// the correct behavior when forwarding the request to the Kubernetes API.
// By default, if no kubernetes_users are set (which will be a majority), a
// user will impersonate himself, which is the backwards-compatible behavior.
// We also append teleport.KubeSystemAuthenticated to kubernetes_groups, which is
// a builtin group that allows any user to access common API methods,
// e.g. discovery methods required for initial client usage, without it,
// restricted user's kubectl clients will not work.
func fillDefaultKubePrincipalDetails(kubeUsers []string, kubeGroups []string, username string) ([]string, []string) {
	if len(kubeUsers) == 0 {
		kubeUsers = append(kubeUsers, username)
	}

	if !slices.Contains(kubeGroups, teleport.KubeSystemAuthenticated) {
		kubeGroups = append(kubeGroups, teleport.KubeSystemAuthenticated)
	}
	return kubeUsers, kubeGroups
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
	accessChecker services.AccessChecker,
	kubeClusterName string,
	sessionTTL time.Duration,
	kubeResource *types.KubernetesResource,
) (kubeAccessDetails, error) {
	// Find requested kubernetes cluster name and get allowed kube users/groups names.
	for _, s := range kubeServers {
		c := s.GetCluster()
		if c.GetName() != kubeClusterName {
			continue
		}

		// Get list of allowed kube user/groups based on kubernetes service labels.
		labels := types.CombineLabels(c.GetStaticLabels(), types.LabelsToV2(c.GetDynamicLabels()))

		matchers := make([]services.RoleMatcher, 0, 2)
		// Creates a matcher that matches the cluster labels against `kubernetes_labels`
		// defined for each user's role.
		matchers = append(matchers,
			services.NewKubernetesClusterLabelMatcher(labels, accessChecker.Traits()),
		)

		// If the kubeResource is available, append an extra matcher that validates
		// if the kubernetes resource is allowed by the user roles that satisfy the
		// target cluster labels.
		// Each role defines `kubernetes_resources` and when kubeResource is available,
		// KubernetesResourceMatcher will match roles that statisfy the resources at the
		// same time that ClusterLabelMatcher matches the role's "kubernetes_labels".
		// The call to roles.CheckKubeGroupsAndUsers when both matchers are provided
		// results in the intersection of roles that match the "kubernetes_labels" and
		// roles that allow access to the desired "kubernetes_resource".
		// If from the intersection results an empty set, the request is denied.
		if kubeResource != nil {
			matchers = append(
				matchers,
				services.NewKubernetesResourceMatcher(*kubeResource),
			)
		}
		// accessChecker.CheckKubeGroupsAndUsers returns the accumulated kubernetes_groups
		// and kubernetes_users that satisfy te provided matchers.
		// When a KubernetesResourceMatcher, it will gather the Kubernetes principals
		// whose role satisfy the the desired Kubernetes Resource.
		// The users/groups will be forwarded to Kubernetes Cluster as Impersonation
		// headers.
		groups, users, err := accessChecker.CheckKubeGroupsAndUsers(sessionTTL, false /* overrideTTL */, matchers...)
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
	ctx, span := f.cfg.tracer.Start(
		ctx,
		"kube.Forwarder/authorize",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
			semconv.RPCSystemKey.String("kube"),
		),
	)
	defer span.End()

	if actx.teleportCluster.isRemote {
		// Authorization for a remote kube cluster will happen on the remote
		// end (by their proxy), after that cluster has remapped used roles.
		f.log.WithField("auth_context", actx.String()).Debug("Skipping authorization for a remote kubernetes cluster name")
		return nil
	}
	if actx.kubeClusterName == "" {
		// This should only happen for remote clusters (filtered above), but
		// check and report anyway.
		f.log.WithField("auth_context", actx.String()).Debug("Skipping authorization due to unknown kubernetes cluster name")
		return nil
	}

	authPref, err := f.cfg.CachingAuthClient.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	state := actx.GetAccessState(authPref)

	notFoundMessage := fmt.Sprintf("kubernetes cluster %q not found", actx.kubeClusterName)
	var roleMatchers services.RoleMatchers
	if actx.kubeResource != nil {
		notFoundMessage = f.kubeResourceDeniedAccessMsg(
			actx.User.GetName(),
			actx.requestVerb,
			actx.apiResource,
		)
		roleMatchers = services.RoleMatchers{
			// Append a matcher that validates if the Kubernetes resource is allowed
			// by the roles that satisfy the Kubernetes Cluster.
			services.NewKubernetesResourceMatcher(*actx.kubeResource),
		}
	}
	var kubeUsers, kubeGroups []string
	// Only check k8s principals for local clusters.
	//
	// For remote clusters, everything will be remapped to new roles on the
	// leaf and checked there.
	if !actx.teleportCluster.isRemote {
		// check signing TTL and return a list of allowed logins for local cluster based on Kubernetes service labels.
		kubeAccessDetails, err := f.getKubeAccessDetails(actx.kubeServers, actx.Checker, actx.kubeClusterName, actx.sessionTTL, actx.kubeResource)
		if err != nil && !trace.IsNotFound(err) {
			if actx.kubeResource != nil {
				return trace.AccessDenied(notFoundMessage)
			}
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
	kubeUsers, kubeGroups = fillDefaultKubePrincipalDetails(kubeUsers, kubeGroups, actx.User.GetName())
	actx.kubeUsers = utils.StringsSet(kubeUsers)
	actx.kubeGroups = utils.StringsSet(kubeGroups)

	// Check authz against the first match.
	//
	// We assume that users won't register two identically-named clusters with
	// mis-matched labels. If they do, expect weirdness.
	for _, s := range actx.kubeServers {
		ks := s.GetCluster()
		if ks.GetName() != actx.kubeClusterName {
			continue
		}

		switch err := actx.Checker.CheckAccess(ks, state, roleMatchers...); {
		case errors.Is(err, services.ErrTrustedDeviceRequired):
			return trace.Wrap(err)
		case err != nil:
			return trace.AccessDenied(notFoundMessage)
		}

		// If the user has active Access requests we need to validate that they allow
		// the kubeResource.
		// This is required because CheckAccess does not validate the subresource type.
		if actx.kubeResource != nil && len(actx.Checker.GetAllowedResourceIDs()) > 0 {
			// GetKubeResources returns the allowed and denied Kubernetes resources
			// for the user. Since we have active access requests, the allowed
			// resources will be the list of pods that the user requested access to if he
			// requested access to specific pods or the list of pods that his roles
			// allow if the user requested access a kubernetes cluster. If the user
			// did not request access to any Kubernetes resource type, the allowed
			// list will be empty.
			allowed, denied := actx.Checker.GetKubeResources(ks)
			if result, err := matchKubernetesResource(*actx.kubeResource, allowed, denied); err != nil || !result {
				return trace.AccessDenied(notFoundMessage)
			}
		}
		// store a copy of the Kubernetes Cluster.
		actx.kubeCluster = ks
		return nil
	}
	if actx.kubeClusterName == f.cfg.ClusterName {
		f.log.WithField("auth_context", actx.String()).Debug("Skipping authorization for proxy-based kubernetes cluster,")
		return nil
	}
	return trace.AccessDenied(notFoundMessage)
}

// matchKubernetesResource checks if the Kubernetes Resource does not match any
// entry from the deny list and matches at least one entry from the allowed list.
func matchKubernetesResource(resource types.KubernetesResource, allowed, denied []types.KubernetesResource) (bool, error) {
	// utils.KubeResourceMatchesRegex checks if the resource.Kind is strictly equal
	// to each entry and validates if the Name and Namespace fields matches the
	// regex allowed by each entry.
	result, err := utils.KubeResourceMatchesRegex(resource, denied)
	if err != nil {
		return false, trace.Wrap(err)
	} else if result {
		return false, nil
	}

	result, err = utils.KubeResourceMatchesRegex(resource, allowed)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return result, nil
}

// join joins an existing session over a websocket connection
func (f *Forwarder) join(ctx *authContext, w http.ResponseWriter, req *http.Request, p httprouter.Params) (resp any, err error) {
	// Increment the request counter and the in-flight gauge.
	joinSessionsRequestCounter.WithLabelValues(f.cfg.KubeServiceType).Inc()
	joinSessionsInFlightGauge.WithLabelValues(f.cfg.KubeServiceType).Inc()
	defer joinSessionsInFlightGauge.WithLabelValues(f.cfg.KubeServiceType).Dec()

	f.log.Debugf("Join %v.", req.URL.String())

	sess, err := f.newClusterSession(req.Context(), *ctx)
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

	if !f.isLocalKubeCluster(ctx.teleportCluster.isRemote, ctx.kubeClusterName) {
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
	var stream *streamproto.SessionStream
	// Close the stream when we exit to ensure no goroutines are leaked and
	// to ensure the client gets a close message in case of an error.
	defer func() {
		if stream != nil {
			stream.Close()
		}
	}()
	if err := func() error {
		stream, err = streamproto.NewSessionStream(ws, streamproto.ServerHandshake{MFARequired: session.PresenceEnabled})
		if err != nil {
			return trace.Wrap(err)
		}

		client := &websocketClientStreams{stream}
		party := newParty(*ctx, stream.Mode, client)

		err = session.join(party, true /* emitSessionJoinEvent */)
		if err != nil {
			return trace.Wrap(err)
		}
		closeC := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-stream.Done():
				party.InformClose(trace.BadParameter("websocket connection closed"))
			case <-closeC:
				return
			}
		}()

		err = <-party.closeC
		close(closeC)

		if _, err := session.leave(party.ID); err != nil {
			f.log.WithError(err).Debugf("Participant %q was unable to leave session %s", party.ID, session.id)
		}
		wg.Wait()

		return trace.Wrap(err)
	}(); err != nil {
		writeErr := ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, err.Error()), time.Now().Add(time.Second*10))
		if writeErr != nil {
			f.log.WithError(writeErr).Warn("Failed to send early-exit websocket close message.")
		}
	}

	return nil, nil
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
	hostID, err := f.getSessionHostID(req.Context(), ctx, p)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	netDialer := sess.DialWithContext(withTargetHostID(hostID))
	tlsConfig, impersonationHeaders, err := f.getTLSConfig(sess)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dialer := &websocket.Dialer{
		TLSClientConfig: tlsConfig,
		NetDialContext:  netDialer,
	}

	headers := http.Header{}
	if impersonationHeaders {
		if headers, err = auth.IdentityForwardingHeaders(req.Context(), headers); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	url := "wss://" + req.URL.Host
	if req.URL.Port() != "" {
		url = url + ":" + req.URL.Port()
	}
	url = url + req.URL.Path

	wsTarget, respTarget, err := dialer.DialContext(req.Context(), url, headers)
	if err != nil {
		if respTarget == nil {
			return nil, trace.Wrap(err)
		}
		defer respTarget.Body.Close()
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

	wsProxy(f.log, wsSource, wsTarget)

	return nil, nil
}

// getSessionHostID returns the host ID that controls the session being joined.
// If the session is remote, returns an empty string, otherwise returns the host ID
// from the session tracker.
func (f *Forwarder) getSessionHostID(ctx context.Context, authCtx *authContext, p httprouter.Params) (string, error) {
	if authCtx.teleportCluster.isRemote {
		return "", nil
	}
	session := p.ByName("session")
	if session == "" {
		return "", trace.BadParameter("missing session ID")
	}
	sess, err := f.cfg.AuthClient.GetSessionTracker(ctx, session)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return sess.GetHostID(), nil
}

// wsProxy proxies a websocket connection between two clusters transparently to allow for
// remote joins.
func wsProxy(log logrus.FieldLogger, wsSource *websocket.Conn, wsTarget *websocket.Conn) {
	errS := make(chan error, 1)
	errT := make(chan error, 1)
	wg := &sync.WaitGroup{}

	forwardConn := func(dst, src *websocket.Conn, errc chan<- error) {
		defer dst.Close()
		defer src.Close()
		for {
			msgType, msg, err := src.ReadMessage()
			if err != nil {
				m := websocket.FormatCloseMessage(websocket.CloseNormalClosure, err.Error())
				var e *websocket.CloseError
				if errors.As(err, &e) {
					if e.Code != websocket.CloseNoStatusReceived {
						m = websocket.FormatCloseMessage(e.Code, e.Text)
					}
				}
				errc <- err
				dst.WriteMessage(websocket.CloseMessage, m)
				break
			}

			err = dst.WriteMessage(msgType, msg)
			if err != nil {
				errc <- err
				break
			}
		}
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		forwardConn(wsSource, wsTarget, errS)
	}()
	go func() {
		defer wg.Done()
		forwardConn(wsTarget, wsSource, errT)
	}()

	var err error
	var from, to string
	select {
	case err = <-errS:
		from = "client"
		to = "upstream"
	case err = <-errT:
		from = "upstream"
		to = "client"
	}

	var websocketErr *websocket.CloseError
	if errors.As(err, &websocketErr) && websocketErr.Code == websocket.CloseAbnormalClosure {
		log.WithError(err).Debugf("websocket proxy: Error when copying from %s to %s", from, to)
	}
	wg.Wait()
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
func (f *Forwarder) execNonInteractive(ctx *authContext, w http.ResponseWriter, req *http.Request, p httprouter.Params, request remoteCommandRequest, proxy *remoteCommandProxy, sess *clusterSession) error {
	roles, err := getRolesByName(f, ctx.Context.Identity.GetIdentity().Groups)
	if err != nil {
		return trace.Wrap(err)
	}

	var policySets []*types.SessionTrackerPolicySet
	for _, role := range roles {
		policySet := role.GetSessionPolicySet()
		policySets = append(policySets, &policySet)
	}

	authorizer := auth.NewSessionAccessEvaluator(policySets, types.KubernetesSessionKind, ctx.User.GetName())
	canStart, _, err := authorizer.FulfilledFor(nil)
	if err != nil {
		return trace.Wrap(err)
	}
	if !canStart {
		return trace.AccessDenied("insufficient permissions to launch non-interactive session")
	}

	eventPodMeta := request.eventPodMeta(request.context, sess.kubeAPICreds)

	sessionStart := f.cfg.Clock.Now().UTC()

	serverMetadata := apievents.ServerMetadata{
		ServerID:        f.cfg.HostID,
		ServerNamespace: f.cfg.Namespace,
		ServerHostname:  sess.teleportCluster.name,
		ServerAddr:      sess.kubeAddress,
	}

	sessionMetadata := ctx.Identity.GetIdentity().GetSessionMetadata(uuid.NewString())

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

	if err := f.cfg.Emitter.EmitAuditEvent(f.ctx, sessionStartEvent); err != nil {
		f.log.WithError(err).Warn("Failed to emit event.")
		return trace.Wrap(err)
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
		if err := f.cfg.Emitter.EmitAuditEvent(f.ctx, execEvent); err != nil {
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

		if err := f.cfg.Emitter.EmitAuditEvent(f.ctx, sessionEndEvent); err != nil {
			f.log.WithError(err).Warn("Failed to emit session end event.")
		}
	}()

	executor, err := f.getExecutor(sess, req)
	if err != nil {
		execEvent.Code = events.ExecFailureCode
		execEvent.Error, execEvent.ExitCode = exitCode(err)

		f.log.WithError(err).Warning("Failed creating executor.")
		return trace.Wrap(err)
	}

	streamOptions := proxy.options()
	err = executor.StreamWithContext(req.Context(), streamOptions)
	if err != nil {
		execEvent.Code = events.ExecFailureCode
		execEvent.Error, execEvent.ExitCode = exitCode(err)

		f.log.WithError(err).Warning("Executor failed while streaming.")
		return trace.Wrap(err)
	}

	execEvent.Code = events.ExecCode

	return nil
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
		if errMsg == "" {
			errMsg = string(kubeStatusErr.ErrStatus.Reason)
		}
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
func (f *Forwarder) exec(authCtx *authContext, w http.ResponseWriter, req *http.Request, p httprouter.Params) (resp any, err error) {
	// Increment the request counter and the in-flight gauge.
	execSessionsRequestCounter.WithLabelValues(f.cfg.KubeServiceType).Inc()
	execSessionsInFlightGauge.WithLabelValues(f.cfg.KubeServiceType).Inc()
	defer execSessionsInFlightGauge.WithLabelValues(f.cfg.KubeServiceType).Dec()

	ctx, span := f.cfg.tracer.Start(
		req.Context(),
		"kube.Forwarder/exec",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
			semconv.RPCMethodKey.String("Exec"),
			semconv.RPCSystemKey.String("kube"),
		),
	)
	defer span.End()

	f.log.Debugf("Exec %v.", req.URL.String())
	defer func() {
		if err != nil {
			f.log.WithError(err).Debug("Exec request failed")
		}
	}()

	sess, err := f.newClusterSession(ctx, *authCtx)
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
		context:            ctx,
		pingPeriod:         f.cfg.ConnPingPeriod,
		onResize:           func(remotecommand.TerminalSize) {},
	}

	if err := f.setupForwardingHeaders(sess, req, true /* withImpersonationHeaders */); err != nil {
		return nil, trace.Wrap(err)
	}

	return upgradeRequestToRemoteCommandProxy(request,
		func(proxy *remoteCommandProxy) error {
			if sess.noAuditEvents {
				// We're forwarding this to another kubernetes_service instance, let it handle multiplexing.
				return f.remoteExec(authCtx, w, req, p, sess, request, proxy)
			}

			if !request.tty {
				return f.execNonInteractive(authCtx, w, req, p, request, proxy, sess)
			}

			client := newKubeProxyClientStreams(proxy)
			party := newParty(*authCtx, types.SessionPeerMode, client)
			session, err := newSession(*authCtx, f, req, p, party, sess)
			if err != nil {
				return trace.Wrap(err)
			}

			f.setSession(session.id, session)
			// When Teleport attaches the original session creator terminal streams to the
			// session, we don't want to emit session.join event since it won't be required.
			if err = session.join(party, false /* emitSessionJoinEvent */); err != nil {
				return trace.Wrap(err)
			}

			err = <-party.closeC

			if _, errLeave := session.leave(party.ID); errLeave != nil {
				f.log.WithError(errLeave).Debugf("Participant %q was unable to leave session %s", party.ID, session.id)
			}

			return trace.Wrap(err)
		},
	)
}

// remoteExec forwards an exec request to a remote cluster.
func (f *Forwarder) remoteExec(ctx *authContext, w http.ResponseWriter, req *http.Request, p httprouter.Params, sess *clusterSession, request remoteCommandRequest, proxy *remoteCommandProxy) error {
	executor, err := f.getExecutor(sess, req)
	if err != nil {
		f.log.WithError(err).Warning("Failed creating executor.")
		return trace.Wrap(err)
	}
	streamOptions := proxy.options()
	err = executor.StreamWithContext(req.Context(), streamOptions)
	if err != nil {
		f.log.WithError(err).Warning("Executor failed while streaming.")
	}

	return trace.Wrap(err)
}

// portForward starts port forwarding to the remote cluster
func (f *Forwarder) portForward(authCtx *authContext, w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
	// Increment the request counter and the in-flight gauge.
	portforwardRequestCounter.WithLabelValues(f.cfg.KubeServiceType).Inc()
	portforwardSessionsInFlightGauge.WithLabelValues(f.cfg.KubeServiceType).Inc()
	defer portforwardSessionsInFlightGauge.WithLabelValues(f.cfg.KubeServiceType).Dec()

	ctx, span := f.cfg.tracer.Start(
		req.Context(),
		"kube.Forwarder/portForward",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
			semconv.RPCMethodKey.String("portForward"),
			semconv.RPCSystemKey.String("kube"),
		),
	)
	defer span.End()

	f.log.Debugf("Port forward: %v. req headers: %v.", req.URL.String(), req.Header)
	sess, err := f.newClusterSession(ctx, *authCtx)
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

	dialer, err := f.getSPDYDialer(sess, req)
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
			UserMetadata: authCtx.eventUserMeta(),
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
		if err := f.cfg.Emitter.EmitAuditEvent(f.ctx, portForward); err != nil {
			f.log.WithError(err).Warn("Failed to emit event.")
		}
	}

	q := req.URL.Query()
	request := portForwardRequest{
		podNamespace:       p.ByName("podNamespace"),
		podName:            p.ByName("podName"),
		ports:              q["ports"],
		context:            ctx,
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
		if err := setupImpersonationHeaders(f.log, sess, req.Header); err != nil {
			return trace.Wrap(err)
		}
	}
	// Setup scheme, override target URL to the destination address
	req.URL.Scheme = "https"
	req.RequestURI = req.URL.Path + "?" + req.URL.RawQuery

	// We only have a direct host to provide when using local creds.
	// Otherwise, use kube-teleport-proxy-alpn.teleport.cluster.local to pass TLS handshake and leverage TLS Routing.
	req.URL.Host = fmt.Sprintf("%s%s", constants.KubeTeleportProxyALPNPrefix, constants.APIDomain)
	if sess.kubeAPICreds != nil {
		req.URL.Host = sess.kubeAPICreds.getTargetAddr()
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
func setupImpersonationHeaders(log logrus.FieldLogger, sess *clusterSession, headers http.Header) error {
	// If the request is remote or this instance is a proxy,
	// do not set up impersonation headers.
	if sess.teleportCluster.isRemote || sess.kubeAPICreds == nil {
		return nil
	}

	impersonateUser, impersonateGroups, err := computeImpersonatedPrincipals(sess.kubeUsers, sess.kubeGroups, headers)
	if err != nil {
		return trace.Wrap(err)
	}
	return replaceImpersonationHeaders(headers, impersonateUser, impersonateGroups)
}

func replaceImpersonationHeaders(headers http.Header, impersonateUser string, impersonateGroups []string) error {
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
func (f *Forwarder) catchAll(authCtx *authContext, w http.ResponseWriter, req *http.Request) (any, error) {
	ctx, span := f.cfg.tracer.Start(
		req.Context(),
		"kube.Forwarder/catchAll",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
			semconv.RPCMethodKey.String("catchAll"),
			semconv.RPCSystemKey.String("kube"),
		),
	)
	req = req.WithContext(ctx)
	defer span.End()

	sess, err := f.newClusterSession(req.Context(), *authCtx)
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

	isLocalKubeCluster := f.isLocalKubeCluster(sess.teleportCluster.isRemote, sess.kubeClusterName)
	isListRequest := authCtx.requestVerb == types.KubeVerbList
	// Watch requests can be send to a single resource or to a collection of resources.
	// isWatchingCollectionRequest is true when the request is a watch request and
	// the resource is a collection of resources, e.g. /api/v1/pods?watch=true.
	// authCtx.kubeResource is only set when the request targets a single resource.
	isWatchingCollectionRequest := authCtx.requestVerb == types.KubeVerbWatch && authCtx.kubeResource == nil

	switch {
	case isListRequest || isWatchingCollectionRequest:
		return f.listResources(sess, w, req)
	case authCtx.requestVerb == types.KubeVerbDeleteCollection && isLocalKubeCluster:
		return f.deleteResourcesCollection(sess, w, req)
	default:
		rw := httplib.NewResponseStatusRecorder(w)
		sess.forwarder.ServeHTTP(rw, req)

		f.emitAuditEvent(req, sess, rw.Status())

		return nil, nil
	}
}

func (f *Forwarder) getWebsocketExecutor(sess *clusterSession, req *http.Request) (remotecommand.Executor, error) {
	f.log.Debugf("Creating websocket remote executor for request %s %s", req.Method, req.RequestURI)

	tlsConfig, useImpersonation, err := f.getTLSConfig(sess)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	upgradeRoundTripper := NewWebsocketRoundTripperWithDialer(roundTripperConfig{
		ctx:                   req.Context(),
		log:                   f.log,
		sess:                  sess,
		dialWithContext:       sess.DialWithContext(),
		tlsConfig:             tlsConfig,
		originalHeaders:       req.Header,
		useIdentityForwarding: useImpersonation,
		proxier:               sess.getProxier(),
	})
	rt := http.RoundTripper(upgradeRoundTripper)
	if sess.kubeAPICreds != nil {
		var err error
		rt, err = sess.kubeAPICreds.wrapTransport(rt)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	rt = tracehttp.NewTransport(rt)

	cfg := &rest.Config{
		// WrapTransport will replace default roundTripper created for the WebsocketExecutor
		// and on successfully established connection we will set upgrader's websocket connection.
		WrapTransport: func(baseRt http.RoundTripper) http.RoundTripper {
			if wrt, ok := baseRt.(*kwebsocket.RoundTripper); ok {
				upgradeRoundTripper.onConnected = func(wsConn *gwebsocket.Conn) {
					wrt.Conn = wsConn
				}
			}

			return rt
		},
	}

	return remotecommand.NewWebSocketExecutor(cfg, req.Method, req.URL.String())
}

func isRelevantWebsocketError(err error) bool {
	return err != nil && !strings.Contains(err.Error(), "next reader: EOF")
}

func (f *Forwarder) getExecutor(sess *clusterSession, req *http.Request) (remotecommand.Executor, error) {
	if details, ok := f.clusterDetails[sess.kubeClusterName]; ok &&
		kubernetesSupportsExecSubprotocolV5(details.kubeClusterVersion) && f.allServersSupportExecSubprotocolV5(sess) {

		wsExec, err := f.getWebsocketExecutor(sess, req)
		return wsExec, trace.Wrap(err)
	}

	spdyExec, err := f.getSPDYExecutor(sess, req)
	return spdyExec, trace.Wrap(err)
}

func (f *Forwarder) getSPDYExecutor(sess *clusterSession, req *http.Request) (remotecommand.Executor, error) {
	f.log.Debugf("Creating SPDY remote executor for request %s %s", req.Method, req.RequestURI)

	tlsConfig, useImpersonation, err := f.getTLSConfig(sess)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	upgradeRoundTripper := NewSpdyRoundTripperWithDialer(roundTripperConfig{
		ctx:                   req.Context(),
		sess:                  sess,
		dialWithContext:       sess.DialWithContext(),
		tlsConfig:             tlsConfig,
		pingPeriod:            f.cfg.ConnPingPeriod,
		originalHeaders:       req.Header,
		useIdentityForwarding: useImpersonation,
		proxier:               sess.getProxier(),
	})
	rt := http.RoundTripper(upgradeRoundTripper)
	if sess.kubeAPICreds != nil {
		var err error
		rt, err = sess.kubeAPICreds.wrapTransport(rt)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	rt = tracehttp.NewTransport(rt)

	return remotecommand.NewSPDYExecutorForTransports(rt, upgradeRoundTripper, req.Method, req.URL)
}

// getSPDYDialer returns a dialer that can be used to upgrade the connection
// to SPDY protocol.
// SPDY is a deprecated protocol, but it is still used by kubectl to manage data streams.
// The dialer uses an HTTP1.1 connection to upgrade to SPDY.
func (f *Forwarder) getSPDYDialer(sess *clusterSession, req *http.Request) (httpstream.Dialer, error) {
	tlsConfig, useImpersonation, err := f.getTLSConfig(sess)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	upgradeRoundTripper := NewSpdyRoundTripperWithDialer(roundTripperConfig{
		ctx:                   req.Context(),
		sess:                  sess,
		dialWithContext:       sess.DialWithContext(),
		tlsConfig:             tlsConfig,
		pingPeriod:            f.cfg.ConnPingPeriod,
		originalHeaders:       req.Header,
		useIdentityForwarding: useImpersonation,
		proxier:               sess.getProxier(),
	})
	rt := http.RoundTripper(upgradeRoundTripper)
	if sess.kubeAPICreds != nil {
		var err error
		rt, err = sess.kubeAPICreds.wrapTransport(rt)
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
	parent *Forwarder
	// kubeAPICreds are the credentials used to authenticate to the Kubernetes API server.
	// It is non-nil if the kubernetes cluster is served by this teleport service,
	// nil otherwise.
	kubeAPICreds kubeCreds
	forwarder    *reverseproxy.Forwarder
	// noAuditEvents is true if this teleport service should leave audit event
	// logging to another service.
	noAuditEvents bool
	targetAddr    string
	// kubeAddress is the address of this session's active connection (if there is one)
	kubeAddress string
	// upgradeToHTTP2 indicates whether the transport should be configured to use HTTP2.
	// A HTTP2 configured transport does not work with connections that are going to be
	// upgraded to SPDY, like in the cases of exec, port forward...
	upgradeToHTTP2 bool
	// requestContext is the context of the original request.
	requestContext context.Context
	// codecFactory is the codec factory used to create the serializer
	// for unmarshalling the payload.
	codecFactory *serializer.CodecFactory
	// rbacSupportedResources is the list of resources that support RBAC for the
	// current cluster.
	rbacSupportedResources rbacSupportedResources
	// connCtx is the context used to monitor the connection.
	connCtx context.Context
	// connMonitorCancel is the conn monitor connMonitorCancel function.
	connMonitorCancel context.CancelCauseFunc
}

// close cancels the connection monitor context if available.
func (s *clusterSession) close() {
	if s.connMonitorCancel != nil {
		s.connMonitorCancel(io.EOF)
	}
}

func (s *clusterSession) monitorConn(conn net.Conn, err error) (net.Conn, error) {
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tc, err := srv.NewTrackingReadConn(srv.TrackingReadConnConfig{
		Conn:    conn,
		Clock:   s.parent.cfg.Clock,
		Context: s.connCtx,
		Cancel:  s.connMonitorCancel,
	})
	if err != nil {
		s.connMonitorCancel(err)
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
		Context:               s.connCtx,
		TeleportUser:          s.User.GetName(),
		ServerID:              s.parent.cfg.HostID,
		Entry:                 s.parent.log,
		Emitter:               s.parent.cfg.AuthClient,
		EmitterContext:        s.parent.ctx,
	})
	if err != nil {
		tc.CloseWithCause(err)
		return nil, trace.Wrap(err)
	}
	return tc, nil
}

func (s *clusterSession) Dial(network, addr string) (net.Conn, error) {
	return s.monitorConn(s.dial(s.requestContext, network, addr))
}

func (s *clusterSession) DialWithContext(opts ...contextDialerOption) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return s.monitorConn(s.dial(ctx, network, addr, opts...))
	}
}

func (s *clusterSession) dial(ctx context.Context, network, addr string, opts ...contextDialerOption) (net.Conn, error) {
	dialer := s.parent.getContextDialerFunc(s, opts...)

	conn, err := dialer(ctx, network, addr)

	return conn, trace.Wrap(err)
}

// getProxier returns the proxier function to use for this session.
// If the target cluster is not served by this teleport service, the proxier
// must be nil to avoid using it through the reverse tunnel.
// If the target cluster is served by this teleport service, the proxier
// must be set to the default proxy function.
func (s *clusterSession) getProxier() func(req *http.Request) (*url.URL, error) {
	// When the target cluster is not served by this teleport service, the
	// proxier must be nil to avoid using it through the reverse tunnel.
	if s.kubeAPICreds == nil {
		return nil
	}
	return utilnet.NewProxierWithNoProxyCIDR(http.ProxyFromEnvironment)
}

// TODO(awly): unit test this
func (f *Forwarder) newClusterSession(ctx context.Context, authCtx authContext) (*clusterSession, error) {
	ctx, span := f.cfg.tracer.Start(
		ctx,
		"kube.Forwarder/newClusterSession",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
			semconv.RPCMethodKey.String("GlobalRequest"),
			semconv.RPCSystemKey.String("kube"),
		),
	)
	defer span.End()

	if authCtx.teleportCluster.isRemote {
		return f.newClusterSessionRemoteCluster(ctx, authCtx)
	}
	return f.newClusterSessionSameCluster(ctx, authCtx)
}

func (f *Forwarder) newClusterSessionRemoteCluster(ctx context.Context, authCtx authContext) (*clusterSession, error) {
	f.log.Debugf("Forwarding kubernetes session for %v to remote cluster.", authCtx)
	connCtx, cancel := context.WithCancelCause(ctx)
	return &clusterSession{
		parent:      f,
		authContext: authCtx,
		// Proxy uses reverse tunnel dialer to connect to Kubernetes in a leaf cluster
		// and the targetKubernetes cluster endpoint is determined from the identity
		// encoded in the TLS certificate. We're setting the dial endpoint to a hardcoded
		// `kube.teleport.cluster.local` value to indicate this is a Kubernetes proxy request
		targetAddr:        reversetunnelclient.LocalKubernetes,
		requestContext:    ctx,
		connCtx:           connCtx,
		connMonitorCancel: cancel,
	}, nil
}

func (f *Forwarder) newClusterSessionSameCluster(ctx context.Context, authCtx authContext) (*clusterSession, error) {
	// Try local creds first
	sess, localErr := f.newClusterSessionLocal(ctx, authCtx)
	switch {
	case localErr == nil:
		return sess, nil
	case trace.IsConnectionProblem(localErr):
		return nil, trace.Wrap(localErr)
	}

	kubeServers := authCtx.kubeServers
	if len(kubeServers) == 0 && authCtx.kubeClusterName == authCtx.teleportCluster.name {
		return nil, trace.Wrap(localErr)
	}

	if len(kubeServers) == 0 {
		return nil, trace.NotFound("kubernetes cluster %q not found", authCtx.kubeClusterName)
	}

	return f.newClusterSessionDirect(ctx, authCtx)
}

func (f *Forwarder) newClusterSessionLocal(ctx context.Context, authCtx authContext) (*clusterSession, error) {
	details, err := f.findKubeDetailsByClusterName(authCtx.kubeClusterName)
	if err != nil {
		return nil, trace.NotFound("kubernetes cluster %q not found", authCtx.kubeClusterName)
	}

	codecFactory, rbacSupportedResources, err := details.getClusterSupportedResources()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connCtx, cancel := context.WithCancelCause(ctx)
	f.log.Debugf("Handling kubernetes session for %v using local credentials.", authCtx)
	return &clusterSession{
		parent:                 f,
		authContext:            authCtx,
		kubeAPICreds:           details.kubeCreds,
		targetAddr:             details.getTargetAddr(),
		requestContext:         ctx,
		codecFactory:           codecFactory,
		rbacSupportedResources: rbacSupportedResources,
		connCtx:                connCtx,
		connMonitorCancel:      cancel,
	}, nil
}

func (f *Forwarder) newClusterSessionDirect(ctx context.Context, authCtx authContext) (*clusterSession, error) {
	connCtx, cancel := context.WithCancelCause(ctx)
	return &clusterSession{
		parent:      f,
		authContext: authCtx,
		// This session talks to a kubernetes_service, which should handle
		// audit logging. Avoid duplicate logging.
		noAuditEvents:     true,
		requestContext:    ctx,
		connCtx:           connCtx,
		connMonitorCancel: cancel,
	}, nil
}

// makeSessionForwader creates a new forward.Forwarder with a transport that
// is either configured:
// - for HTTP1 in case it's going to be used against streaming andoints like exec and port forward.
// - for HTTP2 in all other cases.
// The reason being is that streaming requests are going to be upgraded to SPDY, which is only
// supported coming from an HTTP1 request.
func (f *Forwarder) makeSessionForwarder(sess *clusterSession) (*reverseproxy.Forwarder, error) {
	transport, err := f.transportForRequest(sess)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	opts := []reverseproxy.Option{
		reverseproxy.WithFlushInterval(100 * time.Millisecond),
		reverseproxy.WithRoundTripper(transport),
		reverseproxy.WithLogger(f.log),
		reverseproxy.WithErrorHandler(f.formatForwardResponseError),
	}
	if f.isLocalKubeCluster(sess.teleportCluster.isRemote, sess.kubeClusterName) {
		// If the target cluster is local, i.e. the cluster that is served by this
		// teleport service, then we set up the forwarder to allow re-writing
		// the response to the client to include user friendly error messages.
		// This is done by adding a response modifier to the forwarder.
		// Right now, the only error that is re-written is the 403 Forbidden error
		// that is returned when the user tries to access a GKE Autopilot cluster
		// with system:masters group impersonation.
		//nolint:bodyclose // the caller closes the response body in httputils.ReverseProxy
		opts = append(opts, reverseproxy.WithResponseModifier(f.rewriteResponseForbidden(sess)))
	}

	forwarder, err := reverseproxy.New(
		opts...,
	)

	return forwarder, trace.Wrap(err)
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

// isLocalKubeCluster checks if the current service must hold the cluster and
// if it's of Type KubeService.
// KubeProxy services or remote clusters are automatically forwarded to
// the final destination.
func (f *Forwarder) isLocalKubeCluster(isRemoteTeleportCluster bool, kubeClusterName string) bool {
	switch f.cfg.KubeServiceType {
	case KubeService:
		// Kubernetes service is always local.
		return true
	case LegacyProxyService:
		// remote clusters are always forwarded to the final destination.
		if isRemoteTeleportCluster {
			return false
		}
		// Legacy proxy service is local only if the kube cluster name matches
		// with clusters served by this agent.
		_, err := f.findKubeDetailsByClusterName(kubeClusterName)
		return err == nil
	default:
		return false
	}
}

// kubeResourceDeniedAccessMsg creates a Kubernetes API like forbidden response.
// Logic from:
// https://github.com/kubernetes/kubernetes/blob/ea0764452222146c47ec826977f49d7001b0ea8c/staging/src/k8s.io/apiserver/pkg/endpoints/handlers/responsewriters/errors.go#L51
func (f *Forwarder) kubeResourceDeniedAccessMsg(user, verb string, resource apiResource) string {
	kind := strings.Split(resource.resourceKind, "/")[0]
	apiGroup := resource.apiGroup
	if apiGroup == "core" {
		apiGroup = ""
	}
	teleportType, ok := defaultRBACResources.getTeleportResourceKindFromAPIResource(resource)
	// If the resource is not in the default resources list, it is a custom resource
	// controlled by a CRD. In this case, we use the namespace to restrict access to.
	if !ok {
		teleportType = types.KindKubeNamespace
	}

	switch {
	case resource.namespace != "" && resource.resourceName != "":
		// <resource> "<pod_name>" is forbidden: User "<user>" cannot create resource "<resource>" in API group "" in the namespace "<namespace>"
		return fmt.Sprintf(
			"%[1]s %[2]q is forbidden: User %[3]q cannot %[4]s resource %[1]q in API group %[5]q in the namespace %[6]q\n"+
				"Ask your Teleport admin to ensure that your Teleport role includes access to the %[7]s in %[8]q field.\n"+
				"Check by running: kubectl auth can-i %[4]s %[1]s/%[2]s --namespace %[6]s ",
			kind,                   // 1
			resource.resourceName,  // 2
			user,                   // 3
			verb,                   // 4
			apiGroup,               // 5
			resource.namespace,     // 6
			teleportType,           // 7
			kubernetesResourcesKey, // 8
		)
	case resource.namespace != "":
		// <resource> is forbidden: User "<user>" cannot create resource "<resource>" in API group "" in the namespace "<namespace>"
		return fmt.Sprintf(
			"%[1]s is forbidden: User %[2]q cannot %[3]s resource %[1]q in API group %[4]q in the namespace %[5]q\n"+
				"Ask your Teleport admin to ensure that your Teleport role includes access to the %[6]s in %[7]q field.\n"+
				"Check by running: kubectl auth can-i %[3]s %[1]s --namespace %[5]s ",
			kind,                   // 1
			user,                   // 2
			verb,                   // 3
			apiGroup,               // 4
			resource.namespace,     // 5
			teleportType,           // 6
			kubernetesResourcesKey, // 7
		)
	case resource.resourceName == "":
		return fmt.Sprintf(
			"%[1]s is forbidden: User %[2]q cannot %[3]s resource %[1]q in API group %[4]q at the cluster scope\n"+
				"Ask your Teleport admin to ensure that your Teleport role includes access to the %[5]s in %[6]q field.\n"+
				"Check by running: kubectl auth can-i %[3]s %[1]s",
			kind,                   // 1
			user,                   // 2
			verb,                   // 3
			apiGroup,               // 4
			teleportType,           // 5
			kubernetesResourcesKey, // 6
		)
	default:
		return fmt.Sprintf(
			"%[1]s %[2]q is forbidden: User %[3]q cannot %[4]s resource %[1]q in API group %[5]q at the cluster scope\n"+
				"Ask your Teleport admin to ensure that your Teleport role includes access to the %[6]s in %[7]q field.\n"+
				"Check by running: kubectl auth can-i %[4]s %[1]s/%[2]s",
			kind,                   // 1
			resource.resourceName,  // 2
			user,                   // 3
			verb,                   // 4
			apiGroup,               // 5
			teleportType,           // 6
			kubernetesResourcesKey, // 7
		)
	}
}

// errorToKubeStatusReason returns an appropriate StatusReason based on the
// provided error type.
func errorToKubeStatusReason(err error, code int) metav1.StatusReason {
	switch {
	case trace.IsAggregate(err):
		return metav1.StatusReasonTimeout
	case trace.IsNotFound(err):
		return metav1.StatusReasonNotFound
	case trace.IsBadParameter(err) || trace.IsOAuth2(err):
		return metav1.StatusReasonBadRequest
	case trace.IsNotImplemented(err):
		return metav1.StatusReasonMethodNotAllowed
	case trace.IsCompareFailed(err):
		return metav1.StatusReasonConflict
	case trace.IsAccessDenied(err):
		return metav1.StatusReasonForbidden
	case trace.IsAlreadyExists(err):
		return metav1.StatusReasonConflict
	case trace.IsLimitExceeded(err):
		return metav1.StatusReasonTooManyRequests
	case trace.IsConnectionProblem(err):
		return metav1.StatusReasonTimeout
	case code == http.StatusInternalServerError:
		return metav1.StatusReasonInternalError
	default:
		return metav1.StatusReasonUnknown
	}
}
