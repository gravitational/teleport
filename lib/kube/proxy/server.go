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
	"log/slog"
	"maps"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
	"golang.org/x/net/http2"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/ingress"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

// TLSServerConfig is a configuration for TLS server
type TLSServerConfig struct {
	// ForwarderConfig is a config of a forwarder
	ForwarderConfig
	// TLS is a base TLS configuration
	TLS *tls.Config
	// LimiterConfig is limiter config
	LimiterConfig limiter.Config
	// AccessPoint is caching access point
	AccessPoint authclient.ReadKubernetesAccessPoint
	// OnHeartbeat is a callback for kubernetes_service heartbeats.
	OnHeartbeat func(error)
	// GetRotation returns the certificate rotation state.
	GetRotation services.RotationGetter
	// ConnectedProxyGetter gets the proxies teleport is connected to.
	ConnectedProxyGetter *reversetunnel.ConnectedProxyGetter
	// Log is the logger.
	Log *slog.Logger
	// Selectors is a list of resource monitor selectors.
	ResourceMatchers []services.ResourceMatcher
	// OnReconcile is called after each kube_cluster resource reconciliation.
	OnReconcile func(types.KubeClusters)
	// CloudClients is a set of cloud clients that Teleport supports.
	CloudClients cloud.Clients
	awsClients   *awsClientsGetter
	// StaticLabels is a map of static labels associated with this service.
	// Each cluster advertised by this kubernetes_service will include these static labels.
	// If the service and a cluster define labels with the same key,
	// service labels take precedence over cluster labels.
	// Used for RBAC.
	StaticLabels map[string]string
	// DynamicLabels define the dynamic labels associated with this service.
	// Each cluster advertised by this kubernetes_service will include these dynamic labels.
	// If the service and a cluster define labels with the same key,
	// service labels take precedence over cluster labels.
	// Used for RBAC.
	DynamicLabels *labels.Dynamic
	// CloudLabels is a map of static labels imported from a cloud provider associated with this
	// service. Used for RBAC.
	// If StaticLabels and CloudLabels define labels with the same key,
	// StaticLabels take precedence over CloudLabels.
	CloudLabels labels.Importer
	// IngressReporter reports new and active connections.
	IngressReporter *ingress.Reporter
	// KubernetesServersWatcher is used by the kube proxy to watch for changes in the
	// kubernetes servers of a cluster. Proxy requires it to update the kubeServersMap
	// which holds the list of kubernetes_services connected to the proxy for a given
	// kubernetes cluster name. Proxy uses this map to route requests to the correct
	// kubernetes_service. The servers are kept in memory to avoid making unnecessary
	// unmarshal calls followed by filtering and to improve memory usage.
	KubernetesServersWatcher *services.GenericWatcher[types.KubeServer, readonly.KubeServer]
	// PROXYProtocolMode controls behavior related to unsigned PROXY protocol headers.
	PROXYProtocolMode multiplexer.PROXYProtocolMode
	// InventoryHandle is used to send kube server heartbeats via the inventory control stream.
	InventoryHandle inventory.DownstreamHandle
}

type awsClientsGetter struct{}

func (f *awsClientsGetter) GetConfig(ctx context.Context, region string, optFns ...awsconfig.OptionsFn) (aws.Config, error) {
	return awsconfig.GetConfig(ctx, region, optFns...)
}

func (f *awsClientsGetter) GetAWSEKSClient(cfg aws.Config) EKSClient {
	return eks.NewFromConfig(cfg)
}

func (f *awsClientsGetter) GetAWSSTSPresignClient(cfg aws.Config) STSPresignClient {
	stsClient := stsutils.NewFromConfig(cfg)
	return sts.NewPresignClient(stsClient)
}

// CheckAndSetDefaults checks and sets default values
func (c *TLSServerConfig) CheckAndSetDefaults() error {
	if err := c.ForwarderConfig.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if c.TLS == nil {
		return trace.BadParameter("missing parameter TLS")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("missing parameter AccessPoint")
	}
	if c.InventoryHandle == nil {
		return trace.BadParameter("missing parameter InventoryHandle")
	}

	if err := c.validateLabelKeys(); err != nil {
		return trace.Wrap(err)
	}

	switch c.KubeServiceType {
	case ProxyService, LegacyProxyService:
		if c.KubernetesServersWatcher == nil {
			return trace.BadParameter("missing parameter KubernetesServersWatcher")
		}
	}

	if c.Log == nil {
		c.Log = slog.Default()
	}
	if c.CloudClients == nil {
		cloudClients, err := cloud.NewClients()
		if err != nil {
			return trace.Wrap(err)
		}
		c.CloudClients = cloudClients
	}
	if c.awsClients == nil {
		c.awsClients = &awsClientsGetter{}
	}
	if c.ConnectedProxyGetter == nil {
		c.ConnectedProxyGetter = reversetunnel.NewConnectedProxyGetter()
	}
	return nil
}

// validateLabelKeys checks that all labels keys are valid.
// Dynamic labels are validated in labels.NewDynamicLabels.
func (c *TLSServerConfig) validateLabelKeys() error {
	for name := range c.StaticLabels {
		if !types.IsValidLabelKey(name) {
			return trace.BadParameter("invalid label key: %q", name)
		}
	}
	return nil
}

// TLSServer is TLS auth server
type TLSServer struct {
	*http.Server
	// TLSServerConfig is TLS server configuration used for auth server
	TLSServerConfig
	fwd          *Forwarder
	mu           sync.Mutex
	listener     net.Listener
	heartbeats   map[string]*srv.HeartbeatV2
	closeContext context.Context
	closeFunc    context.CancelFunc
	// kubeClusterWatcher monitors changes to kube cluster resources.
	kubeClusterWatcher *services.GenericWatcher[types.KubeCluster, readonly.KubeCluster]
	// reconciler reconciles proxied kube clusters with kube_clusters resources.
	reconciler *services.Reconciler[types.KubeCluster]
	// monitoredKubeClusters contains all kube clusters the proxied kube_clusters are
	// reconciled against.
	monitoredKubeClusters monitoredKubeClusters
	// reconcileCh triggers reconciliation of proxied kube_clusters.
	reconcileCh chan struct{}
	log         *slog.Logger
}

// NewTLSServer returns new unstarted TLS server
func NewTLSServer(cfg TLSServerConfig) (*TLSServer, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	log := cfg.Log.With(teleport.ComponentKey, cfg.Component)
	// limiter limits requests by frequency and amount of simultaneous
	// connections per client
	limiter, err := limiter.NewLimiter(cfg.LimiterConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.ForwarderConfig.log = log
	fwd, err := NewForwarder(cfg.ForwarderConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	} else if len(fwd.kubeClusters()) == 0 && cfg.KubeServiceType == KubeService &&
		len(cfg.ResourceMatchers) == 0 {
		// if fwd has no clusters and the service type is KubeService but no resource watcher is configured
		// then the kube_service does not need to start since it will not serve any static or dynamic cluster.
		return nil, trace.BadParameter("kube_service won't start because it has neither static clusters nor a resource watcher configured.")
	}

	clustername, err := cfg.AccessPoint.GetClusterName(cfg.Context)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// authMiddleware authenticates request assuming TLS client authentication
	// adds authentication information to the context
	// and passes it to the API server
	authMiddleware := &auth.Middleware{
		ClusterName:   clustername.GetClusterName(),
		AcceptedUsage: []string{teleport.UsageKubeOnly},
		// EnableCredentialsForwarding is set to true to allow the proxy to forward
		// the client identity to the target service using headers instead of TLS
		// certificates. This is required for the kube service and leaf cluster proxy
		// to be able to replace the client identity with the header payload when
		// the request is forwarded from a Teleport Proxy.
		EnableCredentialsForwarding: true,
	}
	authMiddleware.Wrap(fwd)
	// Wrap sets the next middleware in chain to the authMiddleware
	limiter.WrapHandle(authMiddleware)
	// force client auth if given
	cfg.TLS.ClientAuth = tls.VerifyClientCertIfGiven

	server := &TLSServer{
		fwd:             fwd,
		TLSServerConfig: cfg,
		Server: &http.Server{
			Handler:           httplib.MakeTracingHandler(limiter, teleport.ComponentKube),
			ReadHeaderTimeout: apidefaults.DefaultIOTimeout * 2,
			// Setting ReadTimeout and WriteTimeout will cause the server to
			// terminate long running requests. This will cause issues with
			// long running watch streams. The server will close the connection
			// and the client will receive incomplete data and will fail to
			// parse it.
			IdleTimeout: apidefaults.DefaultIdleTimeout,
			TLSConfig:   cfg.TLS,
			ConnState:   ingress.HTTPConnStateReporter(ingress.Kube, cfg.IngressReporter),
			ConnContext: func(ctx context.Context, c net.Conn) context.Context {
				return authz.ContextWithClientAddrs(ctx, c.RemoteAddr(), c.LocalAddr())
			},
		},
		heartbeats: make(map[string]*srv.HeartbeatV2),
		monitoredKubeClusters: monitoredKubeClusters{
			static: fwd.kubeClusters(),
		},
		reconcileCh: make(chan struct{}),
		log:         log,
	}
	server.TLS.GetConfigForClient = server.GetConfigForClient
	server.closeContext, server.closeFunc = context.WithCancel(cfg.Context)
	// register into the forwarder the method to get kubernetes servers for a kube cluster.
	server.fwd.getKubernetesServersForKubeCluster, err = server.getKubernetesServersForKubeClusterFunc()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return server, nil
}

// ServeOption is a functional option for the multiplexer.
type ServeOption func(*multiplexer.Config)

// WithMultiplexerIgnoreSelfConnections is used for tests, it makes multiplexer ignore the fact that it's self
// connection (coming from same IP as the listening address) when deciding if it should drop connection with
// missing required PROXY header. This is needed since all connections in tests are self connections.
func WithMultiplexerIgnoreSelfConnections() ServeOption {
	return func(cfg *multiplexer.Config) {
		cfg.IgnoreSelfConnections = true
	}
}

// Serve takes TCP listener, upgrades to TLS using config and starts serving
func (t *TLSServer) Serve(listener net.Listener, options ...ServeOption) error {
	caGetter := func(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
		return t.CachingAuthClient.GetCertAuthority(ctx, id, loadKeys)
	}
	muxConfig := multiplexer.Config{
		Context:             t.Context,
		Listener:            listener,
		Clock:               t.Clock,
		PROXYProtocolMode:   t.PROXYProtocolMode,
		ID:                  t.Component,
		CertAuthorityGetter: caGetter,
		LocalClusterName:    t.ClusterName,
		// Increases deadline until the agent receives the first byte to 10s.
		// It's required to accommodate setups with high latency and where the time
		// between the TCP being accepted and the time for the first byte is longer
		// than the default value -  1s.
		DetectTimeout: 10 * time.Second,
	}
	for _, opt := range options {
		opt(&muxConfig)
	}

	// Wrap listener with a multiplexer to get PROXY Protocol support.
	mux, err := multiplexer.New(muxConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	go mux.Serve()
	defer mux.Close()

	t.mu.Lock()
	select {
	// If the server is closed before the listener is started, return early
	// to avoid deadlock.
	case <-t.closeContext.Done():
		t.mu.Unlock()
		return nil
	default:
	}
	t.listener = mux.TLS()
	err = http2.ConfigureServer(t.Server, &http2.Server{})
	t.mu.Unlock()
	if err != nil {
		return trace.Wrap(err)
	}

	// startStaticClusterHeartbeats starts the heartbeat process for static clusters.
	// static clusters can be specified via kubeconfig or clusterName for Teleport agent
	// running in Kubernetes.
	if err := t.startStaticClustersHeartbeat(); err != nil {
		return trace.Wrap(err)
	}

	// Start reconciler that will be reconciling proxied clusters with
	// kube_cluster resources.
	if err := t.startReconciler(t.closeContext); err != nil {
		return trace.Wrap(err)
	}

	// Initialize watcher that will be dynamically (un-)registering
	// proxied clusters based on the kube_cluster resources.
	// This watcher is only started for the kube_service if a resource watcher
	// is configured.
	kubeClusterWatcher, err := t.startKubeClusterResourceWatcher(t.closeContext)
	if err != nil {
		return trace.Wrap(err)
	}
	t.mu.Lock()
	t.kubeClusterWatcher = kubeClusterWatcher
	t.mu.Unlock()

	// kubeServerWatcher is used by the kube proxy to watch for changes in the
	// kubernetes servers of a cluster. Proxy requires it to update the kubeServersMap
	// which holds the list of kubernetes_services connected to the proxy for a given
	// kubernetes cluster name. Proxy uses this map to route requests to the correct
	// kubernetes_service. The servers are kept in memory to avoid making unnecessary
	// unmarshal calls followed by filtering to improve memory usage.
	if t.KubernetesServersWatcher != nil {
		// Wait for the watcher to initialize before starting the server so that the
		// proxy can start routing requests to the kubernetes_service instead of
		// returning an error because the cache is not initialized.
		if err := t.KubernetesServersWatcher.WaitInitialization(); err != nil {
			return trace.Wrap(err)
		}
	}

	return t.Server.Serve(tls.NewListener(mux.TLS(), t.TLS))
}

// Close closes the server and cleans up all resources.
func (t *TLSServer) Close() error {
	return trace.Wrap(t.close(t.closeContext))
}

// Shutdown closes the server and cleans up all resources.
func (t *TLSServer) Shutdown(ctx context.Context) error {
	// TODO(tigrato): handle connections gracefully and wait for them to finish.
	// This might be problematic because exec and port forwarding connections
	// are long lived connections and if we wait for them to finish, we might
	// end up waiting forever.
	return trace.Wrap(t.close(ctx))
}

// close closes the server and cleans up all resources.
func (t *TLSServer) close(ctx context.Context) error {
	var errs []error
	for _, kubeCluster := range t.fwd.kubeClusters() {
		errs = append(errs, t.unregisterKubeCluster(ctx, kubeCluster.GetName()))
	}
	errs = append(errs, t.fwd.Close(), t.Server.Close())

	t.closeFunc()

	t.mu.Lock()
	kubeClusterWatcher := t.kubeClusterWatcher
	t.mu.Unlock()
	// Stop the kube_cluster resource watcher.
	if kubeClusterWatcher != nil {
		kubeClusterWatcher.Close()
	}

	// Stop the kube_server resource watcher.
	if t.KubernetesServersWatcher != nil {
		t.KubernetesServersWatcher.Close()
	}

	var listClose error
	t.mu.Lock()
	if t.listener != nil {
		listClose = t.listener.Close()
	}
	t.mu.Unlock()
	return trace.NewAggregate(append(errs, listClose)...)
}

// GetConfigForClient is getting called on every connection
// and server's GetConfigForClient reloads the list of trusted
// local and remote certificate authorities
func (t *TLSServer) GetConfigForClient(info *tls.ClientHelloInfo) (*tls.Config, error) {
	return authclient.WithClusterCAs(t.TLS, t.AccessPoint, t.ClusterName, t.log)(info)
}

// GetServerInfo returns a services.Server object for heartbeats (aka
// presence).
func (t *TLSServer) getServerInfo(name string) (*types.KubernetesServerV3, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	var addr string
	if t.TLSServerConfig.ForwarderConfig.PublicAddr != "" {
		addr = t.TLSServerConfig.ForwarderConfig.PublicAddr
	} else if t.listener != nil {
		addr = t.listener.Addr().String()
	}

	cluster, err := t.getKubeClusterWithServiceLabels(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Both proxy and kubernetes services can run in the same instance (same
	// cluster names). Add a name suffix to make them distinct.
	//
	// Note: we *don't* want to add suffix for kubernetes_service!
	// This breaks reverse tunnel routing, which uses server.Name.
	if t.KubeServiceType != KubeService {
		name += teleport.KubeLegacyProxySuffix
	}

	srv, err := types.NewKubernetesServerV3(
		types.Metadata{
			Name:      name,
			Namespace: t.Namespace,
		},
		types.KubernetesServerSpecV3{
			Version:  teleport.Version,
			Hostname: addr,
			HostID:   t.TLSServerConfig.HostID,
			Rotation: t.getRotationState(),
			Cluster:  cluster,
			ProxyIDs: t.ConnectedProxyGetter.GetProxyIDs(),
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	srv.SetExpiry(t.Clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL))
	return srv, nil
}

// getKubeClusterWithServiceLabels finds the kube cluster by name, strips the credentials,
// replaces the cluster dynamic labels with their latest value available and updates
// the cluster with the service dynamic and static labels.
// We strip the Azure, AWS and Kubeconfig credentials so they are not leaked when
// heartbeating the cluster.
func (t *TLSServer) getKubeClusterWithServiceLabels(name string) (*types.KubernetesClusterV3, error) {
	// it is safe do read from details since the structure is never updated.
	// we replace the whole structure each time an update happens to a dynamic cluster.
	details, err := t.fwd.findKubeDetailsByClusterName(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// NewKubernetesClusterV3WithoutSecrets creates a copy of details.kubeCluster without
	// any credentials or cloud access details.
	clusterWithoutCreds, err := types.NewKubernetesClusterV3WithoutSecrets(details.kubeCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if details.dynamicLabels != nil {
		clusterWithoutCreds.SetDynamicLabels(details.dynamicLabels.Get())
	}

	t.setServiceLabels(clusterWithoutCreds)

	return clusterWithoutCreds, nil
}

// startHeartbeat starts the registration heartbeat to the auth server.
func (t *TLSServer) startHeartbeat(name string) error {
	heartbeat, err := srv.NewKubernetesServerHeartbeat(srv.HeartbeatV2Config[*types.KubernetesServerV3]{
		InventoryHandle: t.InventoryHandle,
		Announcer:       t.TLSServerConfig.AuthClient,
		GetResource:     func(context.Context) (*types.KubernetesServerV3, error) { return t.getServerInfo(name) },
		OnHeartbeat:     t.TLSServerConfig.OnHeartbeat,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go heartbeat.Run()
	t.mu.Lock()
	defer t.mu.Unlock()
	t.heartbeats[name] = heartbeat
	return nil
}

// getRotationState is a helper to return this server's CA rotation state.
func (t *TLSServer) getRotationState() types.Rotation {
	rotation, err := t.TLSServerConfig.GetRotation(types.RoleKube)
	if err != nil && !trace.IsNotFound(err) {
		t.log.WarnContext(t.closeContext, "Failed to get rotation state", "error", err)
	}
	if rotation != nil {
		return *rotation
	}
	return types.Rotation{}
}

func (t *TLSServer) startStaticClustersHeartbeat() error {
	// Start the heartbeat to announce kubernetes_service presence.
	//
	// Only announce when running in an actual kube_server, or when
	// running in proxy_service with local kube credentials. This means that
	// proxy_service will pretend to also be kube_server.
	if t.KubeServiceType == KubeService ||
		t.KubeServiceType == LegacyProxyService {
		t.log.DebugContext(t.closeContext, "Starting kubernetes_service heartbeats")
		for _, cluster := range t.fwd.kubeClusters() {
			if err := t.startHeartbeat(cluster.GetName()); err != nil {
				return trace.Wrap(err)
			}
		}
	} else {
		t.log.DebugContext(t.closeContext, "No local kube credentials on proxy, will not start kubernetes_service heartbeats")
	}

	return nil
}

// stopHeartbeat stops the registration heartbeat to the auth server.
func (t *TLSServer) stopHeartbeat(name string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	heartbeat, ok := t.heartbeats[name]
	if !ok {
		return nil
	}
	delete(t.heartbeats, name)
	return trace.Wrap(heartbeat.Close())
}

// getServiceStaticLabels gets the labels that the server should present as static,
// which includes Cloud labels if available.
func (t *TLSServer) getServiceStaticLabels() map[string]string {
	if t.CloudLabels == nil {
		return t.StaticLabels
	}
	labels := maps.Clone(t.CloudLabels.Get())
	// Let static labels override ec2 labels.
	maps.Copy(labels, t.StaticLabels)
	return labels
}

// setServiceLabels updates the cluster labels with the kubernetes_service labels.
// If the cluster and the service define overlapping labels the service labels take precedence.
// This function manipulates the original cluster.
func (t *TLSServer) setServiceLabels(cluster types.KubeCluster) {
	serviceStaticLabels := t.getServiceStaticLabels()
	if len(serviceStaticLabels) > 0 {
		staticLabels := cluster.GetStaticLabels()
		if staticLabels == nil {
			staticLabels = make(map[string]string)
		}
		// if cluster and service define the same static label key, service labels have precedence.
		maps.Copy(staticLabels, serviceStaticLabels)
		cluster.SetStaticLabels(staticLabels)
	}

	if t.DynamicLabels != nil {
		dstDynLabels := cluster.GetDynamicLabels()
		if dstDynLabels == nil {
			dstDynLabels = map[string]types.CommandLabel{}
		}
		// get service level dynamic labels.
		serviceDynLabels := t.DynamicLabels.Get()
		// if cluster and service define the same dynamic label key, service labels have precedence.
		maps.Copy(dstDynLabels, serviceDynLabels)
		cluster.SetDynamicLabels(dstDynLabels)
	}
}

// getKubernetesServersForKubeClusterFunc returns a function that returns the kubernetes servers
// for a given kube cluster depending on the type of service.
func (t *TLSServer) getKubernetesServersForKubeClusterFunc() (getKubeServersByNameFunc, error) {
	switch t.KubeServiceType {
	case KubeService:
		return func(_ context.Context, name string) ([]types.KubeServer, error) {
			// If this is a kube_service, we can just return the local kube servers.
			kube, err := t.getKubeClusterWithServiceLabels(name)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			srv, err := types.NewKubernetesServerV3FromCluster(kube, "", t.HostID)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return []types.KubeServer{srv}, nil
		}, nil
	case ProxyService:
		return func(ctx context.Context, name string) ([]types.KubeServer, error) {
			servers, err := t.KubernetesServersWatcher.CurrentResourcesWithFilter(ctx, func(ks readonly.KubeServer) bool {
				return ks.GetCluster().GetName() == name
			})
			return servers, trace.Wrap(err)
		}, nil
	case LegacyProxyService:
		return func(ctx context.Context, name string) ([]types.KubeServer, error) {
			// If this is a legacy kube proxy, then we need to return the local kube servers if
			// the local server is proxying the target cluster, otherwise act like a proxy_service.
			// and forward the request to the next proxy.
			kube, err := t.getKubeClusterWithServiceLabels(name)
			if err != nil {
				servers, err := t.KubernetesServersWatcher.CurrentResourcesWithFilter(ctx, func(ks readonly.KubeServer) bool {
					return ks.GetCluster().GetName() == name
				})
				return servers, trace.Wrap(err)
			}
			srv, err := types.NewKubernetesServerV3FromCluster(kube, "", t.HostID)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return []types.KubeServer{srv}, nil
		}, nil
	default:
		return nil, trace.BadParameter("unknown kubernetes service type %q", t.KubeServiceType)
	}
}
