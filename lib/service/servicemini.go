package service

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	transportpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/transport/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/agentless"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/keygen"
	"github.com/gravitational/teleport/lib/auth/storage"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/proxy"
	"github.com/gravitational/teleport/lib/proxy/clusterdial"
	"github.com/gravitational/teleport/lib/proxy/peer"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpnproxyauth "github.com/gravitational/teleport/lib/srv/alpnproxy/auth"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/srv/ingress"
	"github.com/gravitational/teleport/lib/srv/regular"
	"github.com/gravitational/teleport/lib/srv/transport/transportv1"
	"github.com/gravitational/teleport/lib/system"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/hostid"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	vc "github.com/gravitational/teleport/lib/versioncontrol"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/quic-go/quic-go"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// NewTeleport takes the daemon configuration, instantiates all required services
// and starts them under a supervisor, returning the supervisor object.
func NewTeleportMini(cfg *servicecfg.Config) (*TeleportProcess, error) {
	var err error

	// Before we do anything reset the SIGINT handler back to the default.
	system.ResetInterruptSignalHandler()

	// Validate the config before accessing it.
	if err := servicecfg.ValidateConfig(cfg); err != nil {
		return nil, trace.Wrap(err, "configuration error")
	}

	processID := fmt.Sprintf("%v", nextProcessID())
	cfg.Log = utils.WrapLogger(cfg.Log.WithFields(logrus.Fields{
		teleport.ComponentKey: teleport.Component(teleport.ComponentProcess, processID),
		"pid":                 fmt.Sprintf("%v.%v", os.Getpid(), processID),
	}))
	cfg.Logger = cfg.Logger.With(
		teleport.ComponentKey, teleport.Component(teleport.ComponentProcess, processID),
		"pid", fmt.Sprintf("%v.%v", os.Getpid(), processID),
	)

	// If FIPS mode was requested make sure binary is build against BoringCrypto.
	if cfg.FIPS {
		if !modules.GetModules().IsBoringBinary() {
			return nil, trace.BadParameter("binary not compiled against BoringCrypto, check " +
				"that Enterprise FIPS release was downloaded from " +
				"a Teleport account https://teleport.sh")
		}
	}

	if cfg.Auth.Preference.GetPrivateKeyPolicy().IsHardwareKeyPolicy() {
		if modules.GetModules().BuildType() != modules.BuildEnterprise {
			return nil, trace.AccessDenied("Hardware Key support is only available with an enterprise license")
		}
	}

	// create the data directory if it's missing
	_, err = os.Stat(cfg.DataDir)
	if os.IsNotExist(err) {
		err := os.MkdirAll(cfg.DataDir, os.ModeDir|0o700)
		if err != nil {
			if errors.Is(err, fs.ErrPermission) {
				cfg.Logger.ErrorContext(context.Background(), "Teleport does not have permission to write to the data directory. Ensure that you are running as a user with appropriate permissions.", "data_dir", cfg.DataDir)
			}
			return nil, trace.ConvertSystemError(err)
		}
	}

	if len(cfg.FileDescriptors) == 0 {
		cfg.FileDescriptors, err = importFileDescriptors(cfg.Log)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	supervisor := NewSupervisor(processID, cfg.Log)
	storage, err := storage.NewProcessStorage(supervisor.ExitContext(), filepath.Join(cfg.DataDir, teleport.ComponentProcess))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = uuid.Parse(cfg.HostUUID)

	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	var cloudLabels labels.Importer

	// if user did not provide auth domain name, use this host's name
	if cfg.Auth.Enabled && cfg.Auth.ClusterName == nil {
		cfg.Auth.ClusterName, err = services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
			ClusterName: cfg.Hostname,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	process := &TeleportProcess{
		PluginRegistry:         cfg.PluginRegistry,
		Clock:                  cfg.Clock,
		Supervisor:             supervisor,
		Config:                 cfg,
		instanceConnectorReady: make(chan struct{}),
		instanceRoles:          make(map[types.SystemRole]string),
		hostedPluginRoles:      make(map[types.SystemRole]string),
		connectors:             make(map[types.SystemRole]*Connector),
		importedDescriptors:    cfg.FileDescriptors,
		storage:                storage,
		id:                     processID,
		log:                    cfg.Log,
		logger:                 cfg.Logger,
		cloudLabels:            cloudLabels,
		TracingProvider:        tracing.NoopProvider(),
	}

	process.registerExpectedServices(cfg)

	// if user started auth and another service (without providing the auth address for
	// that service, the address of the in-process auth will be used
	if process.Config.Auth.Enabled && len(process.Config.AuthServerAddresses()) == 0 {
		process.Config.SetAuthServerAddress(process.Config.Auth.ListenAddr)
	}

	var resolverAddr utils.NetAddr
	if cfg.Version == defaults.TeleportConfigVersionV3 && !cfg.ProxyServer.IsEmpty() {
		resolverAddr = cfg.ProxyServer
	} else {
		resolverAddr = cfg.AuthServerAddresses()[0]
	}

	process.resolver, err = reversetunnelclient.CachingResolver(
		process.ExitContext(),
		reversetunnelclient.WebClientResolver(&webclient.Config{
			Context:   process.ExitContext(),
			ProxyAddr: resolverAddr.String(),
			Insecure:  lib.IsInsecureDevMode(),
			Timeout:   process.Config.Testing.ClientTimeout,
		}),
		process.Clock,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	upgraderKind := os.Getenv(automaticupgrades.EnvUpgrader)
	upgraderVersion := automaticupgrades.GetUpgraderVersion(process.GracefulExitContext())
	if upgraderVersion == "" {
		upgraderKind = ""
	}

	// note: we must create the inventory handle *after* registerExpectedServices because that function determines
	// the list of services (instance roles) to be included in the heartbeat.
	process.inventoryHandle = inventory.NewDownstreamHandle(process.makeInventoryControlStreamWhenReady, proto.UpstreamInventoryHello{
		ServerID:                cfg.HostUUID,
		Version:                 teleport.Version,
		Services:                process.getInstanceRoles(),
		Hostname:                cfg.Hostname,
		ExternalUpgrader:        upgraderKind,
		ExternalUpgraderVersion: vc.Normalize(upgraderVersion),
	})

	process.inventoryHandle.RegisterPingHandler(func(sender inventory.DownstreamSender, ping proto.DownstreamInventoryPing) {
		process.logger.InfoContext(process.ExitContext(), "Handling incoming inventory ping.", "id", ping.ID)
		err := sender.Send(process.ExitContext(), proto.UpstreamInventoryPong{
			ID: ping.ID,
		})
		if err != nil {
			process.logger.WarnContext(process.ExitContext(), "Failed to respond to inventory ping.", "id", ping.ID, "error", err)
		}
	})

	serviceStarted := false

	if !cfg.DiagnosticAddr.IsEmpty() {
		if err := process.initDiagnosticService(); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		warnOnErr(process.ExitContext(), process.closeImportedDescriptors(teleport.ComponentDiagnostic), process.logger)
	}

	if cfg.Tracing.Enabled {
		if err := process.initTracingService(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if cfg.DebugService.Enabled {
		if err := process.initDebugService(); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		warnOnErr(process.ExitContext(), process.closeImportedDescriptors(teleport.ComponentDebug), process.logger)
	}

	// Create a process wide key generator that will be shared. This is so the
	// key generator can pre-generate keys and share these across services.
	if cfg.Keygen == nil {
		cfg.Keygen = keygen.New(process.ExitContext())
	}

	// Produce global TeleportReadyEvent
	// when all components have started
	eventMapping := EventMapping{
		Out: TeleportReadyEvent,
		In:  []string{InstanceReady},
	}

	// Register additional ready events before considering the Teleport instance "ready."
	// Meant for enterprise support.
	if cfg.AdditionalReadyEvents != nil {
		eventMapping.In = append(eventMapping.In, cfg.AdditionalReadyEvents...)
	}

	if cfg.Auth.Enabled {
		eventMapping.In = append(eventMapping.In, AuthTLSReady)
	}
	if cfg.SSH.Enabled {
		eventMapping.In = append(eventMapping.In, NodeSSHReady)
	}
	if cfg.Proxy.Enabled {
		eventMapping.In = append(eventMapping.In, ProxySSHReady)
	}
	if cfg.Kube.Enabled {
		eventMapping.In = append(eventMapping.In, KubernetesReady)
	}
	if cfg.Apps.Enabled {
		eventMapping.In = append(eventMapping.In, AppsReady)
	}
	if cfg.Metrics.Enabled {
		eventMapping.In = append(eventMapping.In, MetricsReady)
	}
	if cfg.WindowsDesktop.Enabled {
		eventMapping.In = append(eventMapping.In, WindowsDesktopReady)
	}
	if cfg.Tracing.Enabled {
		eventMapping.In = append(eventMapping.In, TracingReady)
	}

	process.RegisterEventMapping(eventMapping)

	// initInstance initializes the pseudo-service "Instance" that is active for all teleport
	// instances. All other services inherit their auth client from the "Instance" service, so
	// we initialize it immediately after auth in order to ensure timely client availability.
	if err := process.initInstance(); err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.SSH.Enabled {
		if err := process.initSSH(); err != nil {
			return nil, err
		}
		serviceStarted = true
	} else {
		warnOnErr(process.ExitContext(), process.closeImportedDescriptors(teleport.ComponentNode), process.logger)
	}

	if cfg.Proxy.Enabled {
		if err := process.initProxy(); err != nil {
			return nil, err
		}
		serviceStarted = true
	} else {
		warnOnErr(process.ExitContext(), process.closeImportedDescriptors(teleport.ComponentProxy), process.logger)
	}

	// If this process is proxying applications, start application access server.
	if cfg.Apps.Enabled {
		process.initApps()
		serviceStarted = true
	} else {
		warnOnErr(process.ExitContext(), process.closeImportedDescriptors(teleport.ComponentApp), process.logger)
	}

	if cfg.Metrics.Enabled {
		process.initMetricsService()
		serviceStarted = true
	} else {
		warnOnErr(process.ExitContext(), process.closeImportedDescriptors(teleport.ComponentMetrics), process.logger)
	}

	if cfg.OpenSSH.Enabled {
		process.initOpenSSH()
		serviceStarted = true
	} else {
		process.RegisterFunc("common.rotate", process.periodicSyncRotationState)
	}

	// run one upload completer per-process
	// even in sync recording modes, since the recording mode can be changed
	// at any time with dynamic configuration
	process.RegisterFunc("common.upload.init", process.initUploaderService)

	if !serviceStarted {
		return nil, trace.BadParameter("all services failed to start")
	}

	// create the new pid file only after started successfully
	if cfg.PIDFile != "" {
		if err := createLockedPIDFile(cfg.PIDFile); err != nil {
			return nil, trace.Wrap(err, "creating pidfile")
		}
	}

	// notify parent process that this process has started
	go process.notifyParent()

	return process, nil
}

func (process *TeleportProcess) initMiniProxyEndpoint(conn *Connector) error {
	// clean up unused descriptors passed for proxy, but not used by it
	defer func() {
		if err := process.closeImportedDescriptors(teleport.ComponentProxy); err != nil {
			process.logger.WarnContext(process.ExitContext(), "Failed closing imported file descriptors", "error", err)
		}
	}()
	var err error
	cfg := process.Config
	var tlsConfigWeb *tls.Config

	clusterName := conn.ClusterName()

	proxyLimiter, err := limiter.NewLimiter(cfg.Proxy.Limiter)
	if err != nil {
		return trace.Wrap(err)
	}

	reverseTunnelLimiter, err := limiter.NewLimiter(cfg.Proxy.Limiter)
	if err != nil {
		return trace.Wrap(err)
	}

	// make a caching auth client for the auth server:
	accessPoint, err := process.newLocalCacheForProxy(conn.Client, []string{teleport.ComponentProxy})
	if err != nil {
		return trace.Wrap(err)
	}

	clusterNetworkConfig, err := accessPoint.GetClusterNetworkingConfig(process.ExitContext())
	if err != nil {
		return trace.Wrap(err)
	}

	listeners, err := process.setupProxyListeners(clusterNetworkConfig, accessPoint, clusterName)
	if err != nil {
		return trace.Wrap(err)
	}

	proxySSHAddr := cfg.Proxy.SSHAddr
	// override value of cfg.Proxy.SSHAddr with listener addr in order
	// to support binding to a random port (e.g. `127.0.0.1:0`).
	if listeners.ssh != nil {
		proxySSHAddr.Addr = listeners.ssh.Addr().String()
	}

	logger := process.logger.With(teleport.ComponentKey, teleport.Component(teleport.ComponentReverseTunnelServer, process.id))

	// asyncEmitter makes sure that sessions do not block
	// in case if connections are slow
	asyncEmitter, err := process.NewAsyncEmitter(conn.Client)
	if err != nil {
		return trace.Wrap(err)
	}
	streamEmitter := &events.StreamerAndEmitter{
		Emitter:  asyncEmitter,
		Streamer: conn.Client,
	}

	lockWatcher, err := services.NewLockWatcher(process.ExitContext(), services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Logger:    process.logger.With(teleport.ComponentKey, teleport.ComponentProxy),
			Client:    conn.Client,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	nodeWatcher, err := services.NewNodeWatcher(process.ExitContext(), services.NodeWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:    teleport.ComponentProxy,
			Logger:       process.logger.With(teleport.ComponentKey, teleport.ComponentProxy),
			Client:       accessPoint,
			MaxStaleness: time.Minute,
		},
		NodesGetter: accessPoint,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	caWatcher, err := services.NewCertAuthorityWatcher(process.ExitContext(), services.CertAuthorityWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Logger:    process.logger.With(teleport.ComponentKey, teleport.ComponentProxy),
			Client:    accessPoint,
		},
		AuthorityGetter: accessPoint,
		Types: []types.CertAuthType{
			types.HostCA,
			types.UserCA,
			types.DatabaseCA,
			types.OpenSSHCA,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	serverTLSConfig, err := conn.ServerTLSConfig(cfg.CipherSuites)
	if err != nil {
		return trace.Wrap(err)
	}
	alpnRouter, reverseTunnelALPNRouter := setupALPNRouter(listeners, serverTLSConfig, cfg)
	alpnAddr := ""
	if listeners.alpn != nil {
		alpnAddr = listeners.alpn.Addr().String()
	}
	ingressReporter, err := ingress.NewReporter(alpnAddr)
	if err != nil {
		return trace.Wrap(err)
	}
	proxySigner, err := conn.getPROXYSigner(process.Clock)
	if err != nil {
		return trace.Wrap(err)
	}

	// register SSH reverse tunnel server that accepts connections
	// from remote teleport nodes
	var tsrv reversetunnelclient.Server
	var peerClient *peer.Client
	var peerQUICTransport *quic.Transport
	if !process.Config.Proxy.DisableReverseTunnel {
		if listeners.proxyPeer != nil {
			// TODO(espadolini): allow this when the implementation is merged
			if false && os.Getenv("TELEPORT_UNSTABLE_QUIC_PROXY_PEERING") == "yes" {
				// the stateless reset key is important in case there's a crash
				// so peers can be told to close their side of the connections
				// instead of having to wait for a timeout; for this reason, we
				// store it in the datadir, which should persist just as much as
				// the host ID and the cluster credentials
				resetKey, err := process.readOrInitPeerStatelessResetKey()
				if err != nil {
					return trace.Wrap(err)
				}
				pc, err := process.createPacketConn(string(ListenerProxyPeer), listeners.proxyPeer.Addr().String())
				if err != nil {
					return trace.Wrap(err)
				}
				peerQUICTransport = &quic.Transport{
					Conn: pc,

					StatelessResetKey: resetKey,
				}
			}

			peerClient, err = peer.NewClient(peer.ClientConfig{
				Context:           process.ExitContext(),
				ID:                process.Config.HostUUID,
				AuthClient:        conn.Client,
				AccessPoint:       accessPoint,
				TLSCipherSuites:   cfg.CipherSuites,
				GetTLSCertificate: conn.ClientGetCertificate,
				GetTLSRoots:       conn.ClientGetPool,
				Log:               process.logger,
				Clock:             process.Clock,
				ClusterName:       clusterName,
				QUICTransport:     peerQUICTransport,
			})
			if err != nil {
				return trace.Wrap(err)
			}
		}

		rtListener, err := reverseTunnelLimiter.WrapListener(listeners.reverseTunnel)
		if err != nil {
			return trace.Wrap(err)
		}

		tsrv, err = reversetunnel.NewServer(
			reversetunnel.Config{
				ClientTLSCipherSuites:   process.Config.CipherSuites,
				GetClientTLSCertificate: conn.ClientGetCertificate,

				Context:               process.ExitContext(),
				Component:             teleport.Component(teleport.ComponentProxy, process.id),
				ID:                    process.Config.HostUUID,
				ClusterName:           clusterName,
				Listener:              rtListener,
				GetHostSigners:        conn.ServerGetHostSigners,
				LocalAuthClient:       conn.Client,
				LocalAccessPoint:      accessPoint,
				NewCachingAccessPoint: process.newLocalCacheForRemoteProxy,
				Limiter:               reverseTunnelLimiter,
				KeyGen:                cfg.Keygen,
				Ciphers:               cfg.Ciphers,
				KEXAlgorithms:         cfg.KEXAlgorithms,
				MACAlgorithms:         cfg.MACAlgorithms,
				DataDir:               process.Config.DataDir,
				PollingPeriod:         process.Config.PollingPeriod,
				FIPS:                  cfg.FIPS,
				Emitter:               streamEmitter,
				Log:                   process.log,
				LockWatcher:           lockWatcher,
				PeerClient:            peerClient,
				NodeWatcher:           nodeWatcher,
				CertAuthorityWatcher:  caWatcher,
				CircuitBreakerConfig:  process.Config.CircuitBreakerConfig,
				LocalAuthAddresses:    utils.NetAddrsToStrings(process.Config.AuthServerAddresses()),
				IngressReporter:       ingressReporter,
				PROXYSigner:           proxySigner,
			})
		if err != nil {
			return trace.Wrap(err)
		}
		process.RegisterCriticalFunc("proxy.reversetunnel.server", func() error {
			logger.InfoContext(process.ExitContext(), "Starting reverse tunnel server", "version", teleport.Version, "git_ref", teleport.Gitref, "listen_address", cfg.Proxy.ReverseTunnelListenAddr.Addr, "cache_policy", process.Config.CachePolicy)
			if err := tsrv.Start(); err != nil {
				logger.ErrorContext(process.ExitContext(), "Failed starting reverse tunnel server", "error", err)
				return trace.Wrap(err)
			}

			// notify parties that we've started reverse tunnel server
			process.BroadcastEvent(Event{Name: ProxyReverseTunnelReady, Payload: tsrv})
			tsrv.Wait(process.ExitContext())
			return nil
		})
	}

	if !process.Config.Proxy.DisableTLS {
		tlsConfigWeb, err = process.setupProxyTLSConfig(conn, tsrv, accessPoint, clusterName)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	var proxyRouter *proxy.Router
	if !process.Config.Proxy.DisableReverseTunnel {
		router, err := proxy.NewRouter(proxy.RouterConfig{
			ClusterName:      clusterName,
			Log:              process.log.WithField(teleport.ComponentKey, "router"),
			LocalAccessPoint: accessPoint,
			SiteGetter:       tsrv,
			TracerProvider:   process.TracingProvider,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		proxyRouter = router
	}

	// read the host UUID:
	serverID, err := hostid.ReadOrCreateFile(cfg.DataDir)
	if err != nil {
		return trace.Wrap(err)
	}

	sessionController, err := srv.NewSessionController(srv.SessionControllerConfig{
		Semaphores:     accessPoint,
		AccessPoint:    accessPoint,
		LockEnforcer:   lockWatcher,
		Emitter:        asyncEmitter,
		Component:      teleport.ComponentProxy,
		Logger:         process.log.WithField(teleport.ComponentKey, "sessionctrl"),
		TracerProvider: process.TracingProvider,
		ServerID:       serverID,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Register web proxy server
	alpnHandlerForWeb := &alpnproxy.ConnectionHandlerWrapper{}
	var webServer *web.Server
	var minimalWebServer *web.Server

	logger.InfoContext(process.ExitContext(), "Web UI is disabled.")

	// Register ALPN handler that will be accepting connections for plain
	// TCP applications.
	if alpnRouter != nil {
		alpnRouter.Add(alpnproxy.HandlerDecs{
			MatchFunc: alpnproxy.MatchByProtocol(alpncommon.ProtocolTCP),
			Handler:   webServer.HandleConnection,
		})
	}

	var peerAddrString string
	var peerServer *peer.Server
	var peerQUICServer *peer.QUICServer
	if !process.Config.Proxy.DisableReverseTunnel && listeners.proxyPeer != nil {
		peerAddr, err := process.Config.Proxy.PublicPeerAddr()
		if err != nil {
			return trace.Wrap(err)
		}
		peerAddrString = peerAddr.String()

		peerServer, err = peer.NewServer(peer.ServerConfig{
			Log:           process.logger,
			ClusterDialer: clusterdial.NewClusterDialer(tsrv),
			CipherSuites:  cfg.CipherSuites,
			GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				return conn.serverGetCertificate()
			},
			GetClientCAs: func(chi *tls.ClientHelloInfo) (*x509.CertPool, error) {
				pool, _, err := authclient.ClientCertPool(chi.Context(), accessPoint, clusterName, types.HostCA)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				return pool, nil
			},
		})
		if err != nil {
			return trace.Wrap(err)
		}

		process.RegisterCriticalFunc("proxy.peer", func() error {
			if _, err := process.WaitForEvent(process.ExitContext(), ProxyReverseTunnelReady); err != nil {
				logger.DebugContext(process.ExitContext(), "Process exiting: failed to start peer proxy service waiting for reverse tunnel server.")
				return nil
			}

			logger.InfoContext(process.ExitContext(), "Starting peer proxy service.", "listen_address", logutils.StringerAttr(listeners.proxyPeer.Addr()))
			err := peerServer.Serve(listeners.proxyPeer)
			if err != nil {
				return trace.Wrap(err)
			}

			return nil
		})

		if peerQUICTransport != nil {
			peerQUICServer, err := peer.NewQUICServer(peer.QUICServerConfig{
				Log:           process.logger,
				ClusterDialer: clusterdial.NewClusterDialer(tsrv),
				CipherSuites:  cfg.CipherSuites,
				GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
					return conn.serverGetCertificate()
				},
				GetClientCAs: func(chi *tls.ClientHelloInfo) (*x509.CertPool, error) {
					pool, _, err := authclient.ClientCertPool(chi.Context(), accessPoint, clusterName, types.HostCA)
					if err != nil {
						return nil, trace.Wrap(err)
					}
					return pool, nil
				},
			})
			if err != nil {
				return trace.Wrap(err)
			}

			process.RegisterCriticalFunc("proxy.peer.quic", func() error {
				if _, err := process.WaitForEvent(process.ExitContext(), ProxyReverseTunnelReady); err != nil {
					logger.DebugContext(process.ExitContext(), "Process exiting: failed to start QUIC peer proxy service waiting for reverse tunnel server.")
					return nil
				}

				logger.InfoContext(process.ExitContext(), "Starting QUIC peer proxy service.", "local_addr", logutils.StringerAttr(peerQUICTransport.Conn.LocalAddr()))
				err := peerQUICServer.Serve(peerQUICTransport)
				if err != nil {
					return trace.Wrap(err)
				}

				return nil
			})
		}
	}

	staticLabels := make(map[string]string, 3)
	if cfg.Proxy.ProxyGroupID != "" {
		staticLabels[types.ProxyGroupIDLabel] = cfg.Proxy.ProxyGroupID
	}
	if cfg.Proxy.ProxyGroupGeneration != 0 {
		staticLabels[types.ProxyGroupGenerationLabel] = strconv.FormatUint(cfg.Proxy.ProxyGroupGeneration, 10)
	}
	if len(staticLabels) > 0 {
		logger.InfoContext(process.ExitContext(), "Enabling proxy group labels.", "group_id", cfg.Proxy.ProxyGroupID, "generation", cfg.Proxy.ProxyGroupGeneration)
	}
	if peerQUICTransport != nil {
		staticLabels[types.ProxyPeerQUICLabel] = "x"
		logger.InfoContext(process.ExitContext(), "Advertising proxy peering QUIC support.")
	}

	sshProxy, err := regular.New(
		process.ExitContext(),
		cfg.SSH.Addr,
		cfg.Hostname,
		conn.ServerGetHostSigners,
		accessPoint,
		cfg.DataDir,
		"",
		process.proxyPublicAddr(),
		conn.Client,
		regular.SetLimiter(proxyLimiter),
		regular.SetProxyMode(peerAddrString, tsrv, accessPoint, proxyRouter),
		regular.SetCiphers(cfg.Ciphers),
		regular.SetKEXAlgorithms(cfg.KEXAlgorithms),
		regular.SetMACAlgorithms(cfg.MACAlgorithms),
		regular.SetNamespace(apidefaults.Namespace),
		regular.SetRotationGetter(process.GetRotation),
		regular.SetFIPS(cfg.FIPS),
		regular.SetOnHeartbeat(process.OnHeartbeat(teleport.ComponentProxy)),
		regular.SetEmitter(streamEmitter),
		regular.SetLockWatcher(lockWatcher),
		// Allow Node-wide file copying checks to succeed so they can be
		// accurately checked later when an SCP/SFTP request hits the
		// destination Node.
		regular.SetAllowFileCopying(true),
		regular.SetTracerProvider(process.TracingProvider),
		regular.SetSessionController(sessionController),
		regular.SetIngressReporter(ingress.SSH, ingressReporter),
		regular.SetPROXYSigner(proxySigner),
		regular.SetPublicAddrs(cfg.Proxy.PublicAddrs),
		regular.SetLabels(staticLabels, services.CommandLabels(nil), labels.Importer(nil)),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName:   clusterName,
		AccessPoint:   accessPoint,
		LockWatcher:   lockWatcher,
		Logger:        process.log.WithField(teleport.ComponentKey, teleport.Component(teleport.ComponentReverseTunnelServer, process.id)),
		PermitCaching: process.Config.CachePolicy.Enabled,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// authMiddleware authenticates request assuming TLS client authentication
	// adds authentication information to the context
	// and passes it to the API server
	authMiddleware := &auth.Middleware{
		ClusterName: clusterName,
	}

	sshGRPCTLSConfig := serverTLSConfig.Clone()
	sshGRPCTLSConfig.NextProtos = []string{string(alpncommon.ProtocolHTTP2), string(alpncommon.ProtocolProxySSHGRPC)}
	sshGRPCTLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
	if lib.IsInsecureDevMode() {
		sshGRPCTLSConfig.InsecureSkipVerify = true
		sshGRPCTLSConfig.ClientAuth = tls.RequireAnyClientCert
	}

	// clientTLSConfigGenerator pre-generates specialized per-cluster client TLS config values
	clientTLSConfigGenerator, err := auth.NewClientTLSConfigGenerator(auth.ClientTLSConfigGeneratorConfig{
		TLS:                  sshGRPCTLSConfig,
		ClusterName:          clusterName,
		PermitRemoteClusters: true,
		AccessPoint:          accessPoint,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	sshGRPCTLSConfig.GetConfigForClient = clientTLSConfigGenerator.GetConfigForClient

	sshGRPCCreds, err := auth.NewTransportCredentials(auth.TransportCredentialsConfig{
		TransportCredentials: credentials.NewTLS(sshGRPCTLSConfig),
		UserGetter:           authMiddleware,
		Authorizer:           authorizer,
		GetAuthPreference:    accessPoint.GetAuthPreference,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	sshGRPCServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			interceptors.GRPCServerUnaryErrorInterceptor,
			//nolint:staticcheck // SA1019. There is a data race in the stats.Handler that is replacing
			// the interceptor. See https://github.com/open-telemetry/opentelemetry-go-contrib/issues/4576.
			otelgrpc.UnaryServerInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			interceptors.GRPCServerStreamErrorInterceptor,
			//nolint:staticcheck // SA1019. There is a data race in the stats.Handler that is replacing
			// the interceptor. See https://github.com/open-telemetry/opentelemetry-go-contrib/issues/4576.
			otelgrpc.StreamServerInterceptor(),
		),
		grpc.Creds(sshGRPCCreds),
		grpc.MaxConcurrentStreams(defaults.GRPCMaxConcurrentStreams),
	)

	connMonitor, err := srv.NewConnectionMonitor(srv.ConnectionMonitorConfig{
		AccessPoint:    accessPoint,
		LockWatcher:    lockWatcher,
		Clock:          process.Clock,
		ServerID:       serverID,
		Emitter:        asyncEmitter,
		EmitterContext: process.ExitContext(),
		Logger:         process.log,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	transportService, err := transportv1.NewService(transportv1.ServerConfig{
		FIPS:   cfg.FIPS,
		Logger: process.log.WithField(teleport.ComponentKey, "transport"),
		Dialer: proxyRouter,
		SignerFn: func(authzCtx *authz.Context, clusterName string) agentless.SignerCreator {
			return agentless.SignerFromAuthzContext(authzCtx, accessPoint, clusterName)
		},
		ConnectionMonitor: connMonitor,
		LocalAddr:         listeners.sshGRPC.Addr(),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	transportpb.RegisterTransportServiceServer(sshGRPCServer, transportService)

	process.RegisterCriticalFunc("proxy.ssh", func() error {
		sshListenerAddr := listeners.ssh.Addr().String()
		if cfg.Proxy.SSHAddr.Addr != "" {
			sshListenerAddr = cfg.Proxy.SSHAddr.Addr
		}
		logger.InfoContext(process.ExitContext(), " Starting SSH proxy service", "version", teleport.Version, "git_ref", teleport.Gitref, "listen_address", sshListenerAddr)

		// start ssh server
		go func() {
			listener, err := proxyLimiter.WrapListener(listeners.ssh)
			if err != nil {
				logger.ErrorContext(process.ExitContext(), "Failed to set up SSH proxy server", "error", err)
				return
			}
			if err := sshProxy.Serve(listener); err != nil && !utils.IsOKNetworkError(err) {
				logger.ErrorContext(process.ExitContext(), "SSH proxy server terminated unexpectedly", "error", err)
			}
		}()

		// start grpc server
		go func() {
			listener, err := proxyLimiter.WrapListener(listeners.sshGRPC)
			if err != nil {
				logger.ErrorContext(process.ExitContext(), "Failed to set up SSH proxy server", "error", err)
				return
			}
			if err := sshGRPCServer.Serve(listener); err != nil && !utils.IsOKNetworkError(err) && !errors.Is(err, grpc.ErrServerStopped) {
				logger.ErrorContext(process.ExitContext(), "SSH gRPC server terminated unexpectedly", "error", err)
			}
		}()

		// broadcast that the proxy ssh server has started
		process.BroadcastEvent(Event{Name: ProxySSHReady, Payload: nil})
		return nil
	})

	rcWatchLog := logrus.WithFields(logrus.Fields{
		teleport.ComponentKey: teleport.Component(teleport.ComponentReverseTunnelAgent, process.id),
	})

	// Create and register reverse tunnel AgentPool.
	rcWatcher, err := reversetunnel.NewRemoteClusterTunnelManager(reversetunnel.RemoteClusterTunnelManagerConfig{
		HostUUID:            conn.HostID(),
		AuthClient:          conn.Client,
		AccessPoint:         accessPoint,
		AuthMethods:         conn.ClientAuthMethods(),
		LocalCluster:        clusterName,
		ReverseTunnelServer: tsrv,
		FIPS:                process.Config.FIPS,
		Log:                 rcWatchLog,
		LocalAuthAddresses:  utils.NetAddrsToStrings(process.Config.AuthServerAddresses()),
		PROXYSigner:         proxySigner,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	process.RegisterCriticalFunc("proxy.reversetunnel.watcher", func() error {
		rcWatchLog.Infof("Starting reverse tunnel agent pool.")
		done := make(chan struct{})
		go func() {
			defer close(done)
			rcWatcher.Run(process.ExitContext())
		}()
		process.BroadcastEvent(Event{Name: ProxyAgentPoolReady, Payload: rcWatcher})
		<-done
		return nil
	})

	var (
		grpcServerPublic *grpc.Server
		grpcServerMTLS   *grpc.Server
	)
	if alpnRouter != nil {
		grpcServerPublic, err = process.initPublicGRPCServer(proxyLimiter, conn, listeners.grpcPublic)
		if err != nil {
			return trace.Wrap(err)
		}

		if err != nil {
			return trace.Wrap(err)
		}
	}

	var alpnServer *alpnproxy.Proxy
	var reverseTunnelALPNServer *alpnproxy.Proxy
	if !cfg.Proxy.DisableTLS && !cfg.Proxy.DisableALPNSNIListener && listeners.web != nil {
		authDialerService := alpnproxyauth.NewAuthProxyDialerService(
			tsrv,
			clusterName,
			utils.NetAddrsToStrings(process.Config.AuthServerAddresses()),
			proxySigner,
			process.log,
			process.TracingProvider.Tracer(teleport.ComponentProxy))

		alpnRouter.Add(alpnproxy.HandlerDecs{
			MatchFunc:           alpnproxy.MatchByALPNPrefix(string(alpncommon.ProtocolAuth)),
			HandlerWithConnInfo: authDialerService.HandleConnection,
			ForwardTLS:          true,
		})
		alpnServer, err = alpnproxy.New(alpnproxy.ProxyConfig{
			WebTLSConfig:      tlsConfigWeb.Clone(),
			IdentityTLSConfig: serverTLSConfig,
			Router:            alpnRouter,
			Listener:          listeners.alpn,
			ClusterName:       clusterName,
			AccessPoint:       accessPoint,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		alpnTLSConfigForWeb, err := process.setupALPNTLSConfigForWeb(serverTLSConfig, accessPoint, clusterName)
		if err != nil {
			return trace.Wrap(err)
		}
		alpnHandlerForWeb.Set(alpnServer.MakeConnectionHandler(alpnTLSConfigForWeb))

		process.RegisterCriticalFunc("proxy.tls.alpn.sni.proxy", func() error {
			logger.InfoContext(process.ExitContext(), "Starting TLS ALPN SNI proxy server on.", "listen_address", logutils.StringerAttr(listeners.alpn.Addr()))
			if err := alpnServer.Serve(process.ExitContext()); err != nil {
				logger.WarnContext(process.ExitContext(), "TLS ALPN SNI proxy proxy server exited with error.", "error", err)
			}
			return nil
		})

		if reverseTunnelALPNRouter != nil {
			reverseTunnelALPNServer, err = alpnproxy.New(alpnproxy.ProxyConfig{
				WebTLSConfig:      tlsConfigWeb.Clone(),
				IdentityTLSConfig: serverTLSConfig,
				Router:            reverseTunnelALPNRouter,
				Listener:          listeners.reverseTunnelALPN,
				ClusterName:       clusterName,
				AccessPoint:       accessPoint,
			})
			if err != nil {
				return trace.Wrap(err)
			}

			process.RegisterCriticalFunc("proxy.tls.alpn.sni.proxy.reverseTunnel", func() error {
				logger.InfoContext(process.ExitContext(), "Starting TLS ALPN SNI reverse tunnel proxy server.", "listen_address", listeners.reverseTunnelALPN.Addr())
				if err := reverseTunnelALPNServer.Serve(process.ExitContext()); err != nil {
					logger.WarnContext(process.ExitContext(), "TLS ALPN SNI proxy proxy on reverse tunnel server exited with error.", "error", err)
				}
				return nil
			})
		}
	}

	// execute this when process is asked to exit:
	process.OnExit("proxy.shutdown", func(payload interface{}) {
		// Close the listeners at the beginning of shutdown, because we are not
		// really guaranteed to be capable to serve new requests if we're
		// halfway through a shutdown, and double closing a listener is fine.
		listeners.Close()
		if payload == nil {
			logger.InfoContext(process.ExitContext(), "Shutting down immediately.")
			if tsrv != nil {
				warnOnErr(process.ExitContext(), tsrv.Close(), logger)
			}
			warnOnErr(process.ExitContext(), rcWatcher.Close(), logger)
			if peerServer != nil {
				warnOnErr(process.ExitContext(), peerServer.Close(), logger)
			}
			if peerQUICServer != nil {
				warnOnErr(process.ExitContext(), peerQUICServer.Close(), logger)
			}
			if webServer != nil {
				warnOnErr(process.ExitContext(), webServer.Close(), logger)
			}
			if minimalWebServer != nil {
				warnOnErr(process.ExitContext(), minimalWebServer.Close(), logger)
			}
			if peerClient != nil {
				warnOnErr(process.ExitContext(), peerClient.Stop(), logger)
			}
			warnOnErr(process.ExitContext(), sshProxy.Close(), logger)
			sshGRPCServer.Stop()
			if grpcServerPublic != nil {
				grpcServerPublic.Stop()
			}
			if grpcServerMTLS != nil {
				grpcServerMTLS.Stop()
			}
			if alpnServer != nil {
				warnOnErr(process.ExitContext(), alpnServer.Close(), logger)
			}
			if reverseTunnelALPNServer != nil {
				warnOnErr(process.ExitContext(), reverseTunnelALPNServer.Close(), logger)
			}

			if clientTLSConfigGenerator != nil {
				clientTLSConfigGenerator.Close()
			}
		} else {
			logger.InfoContext(process.ExitContext(), "Shutting down gracefully.")
			ctx := payloadContext(payload)
			if tsrv != nil {
				warnOnErr(ctx, tsrv.DrainConnections(ctx), logger)
			}
			warnOnErr(ctx, sshProxy.Shutdown(ctx), logger)
			sshGRPCServer.GracefulStop()
			if webServer != nil {
				warnOnErr(ctx, webServer.Shutdown(ctx), logger)
			}
			if minimalWebServer != nil {
				warnOnErr(ctx, minimalWebServer.Shutdown(ctx), logger)
			}
			if tsrv != nil {
				warnOnErr(ctx, tsrv.Shutdown(ctx), logger)
			}
			warnOnErr(ctx, rcWatcher.Close(), logger)
			if peerServer != nil {
				warnOnErr(ctx, peerServer.Shutdown(), logger)
			}
			if peerQUICServer != nil {
				warnOnErr(ctx, peerQUICServer.Shutdown(ctx), logger)
			}
			if peerClient != nil {
				peerClient.Shutdown(ctx)
			}
			if grpcServerPublic != nil {
				grpcServerPublic.GracefulStop()
			}
			if grpcServerMTLS != nil {
				grpcServerMTLS.GracefulStop()
			}
			if alpnServer != nil {
				warnOnErr(ctx, alpnServer.Close(), logger)
			}
			if reverseTunnelALPNServer != nil {
				warnOnErr(ctx, reverseTunnelALPNServer.Close(), logger)
			}

			// Explicitly deleting proxy heartbeats helps the behavior of
			// reverse tunnel agents during rollouts, as otherwise they'll keep
			// trying to reach proxies until the heartbeats expire.
			if services.ShouldDeleteServerHeartbeatsOnShutdown(ctx) {
				if err := conn.Client.DeleteProxy(ctx, process.Config.HostUUID); err != nil {
					if !trace.IsNotFound(err) {
						logger.WarnContext(ctx, "Failed to delete heartbeat.", "error", err)
					} else {
						logger.DebugContext(ctx, "Failed to delete heartbeat.", "error", err)
					}
				}
			}

			if clientTLSConfigGenerator != nil {
				clientTLSConfigGenerator.Close()
			}
		}
		if peerQUICTransport != nil {
			_ = peerQUICTransport.Close()
			_ = peerQUICTransport.Conn.Close()
		}
		warnOnErr(process.ExitContext(), asyncEmitter.Close(), logger)
		warnOnErr(process.ExitContext(), conn.Close(), logger)
		logger.InfoContext(process.ExitContext(), "Exited.")
	})

	return nil
}
