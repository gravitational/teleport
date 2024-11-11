package servicemini

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/renameio/v2"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	transportpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/transport/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/agentless"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/accesspoint"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/auth/storage"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/cache"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/joinserver"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/openssh"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/proxy"
	"github.com/gravitational/teleport/lib/proxy/clusterdial"
	"github.com/gravitational/teleport/lib/proxy/peer"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	secretsscannerproxy "github.com/gravitational/teleport/lib/secretsscanner/proxy"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpnproxyauth "github.com/gravitational/teleport/lib/srv/alpnproxy/auth"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/srv/app"
	"github.com/gravitational/teleport/lib/srv/debug"
	"github.com/gravitational/teleport/lib/srv/desktop"
	"github.com/gravitational/teleport/lib/srv/ingress"
	"github.com/gravitational/teleport/lib/srv/regular"
	"github.com/gravitational/teleport/lib/srv/transport/transportv1"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/cert"
	"github.com/gravitational/teleport/lib/utils/hostid"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/quic-go/quic-go"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// TeleportProcess structure holds the state of the Teleport daemon, controlling
// execution and configuration of the teleport services: ssh, auth and proxy.
type TeleportProcess struct {
	Clock clockwork.Clock
	sync.Mutex
	Supervisor
	Config *servicecfg.Config

	// PluginsRegistry handles plugin registrations with Teleport services
	PluginRegistry plugin.Registry

	// localAuth has local auth server listed in case if this process
	// has started with auth server role enabled
	localAuth *auth.Server
	// backend is the process' backend
	backend backend.Backend
	// auditLog is the initialized audit log
	auditLog events.AuditLogSessionStreamer

	// inventorySetupDelay lets us inject a one-time delay in the makeInventoryControlStream
	// method that helps reduce log spam in the event of slow instance cert acquisition.
	inventorySetupDelay sync.Once

	// inventoryHandle is the downstream inventory control handle for this instance.
	inventoryHandle inventory.DownstreamHandle

	// instanceConnector contains the instance-level connector. this is created asynchronously
	// and may not exist for some time if cert migrations are necessary.
	instanceConnector *Connector

	// instanceConnectorReady is closed when the isntance client becomes available.
	instanceConnectorReady chan struct{}

	// instanceConnectorReadyOnce protects instanceConnectorReady from double-close.
	instanceConnectorReadyOnce sync.Once

	// instanceRoles is the collection of enabled service roles (excludes things like "admin"
	// and "instance" which aren't true user-facing services). The values in this mapping are
	// the names of the associated identity events for these roles.
	instanceRoles map[types.SystemRole]string

	// hostedPluginRoles is the collection of dynamically enabled service roles. This element
	// behaves equivalent to instanceRoles except that while instance roles are static assignments
	// set up when the teleport process starts, hosted plugin roles are dynamically assigned by
	// runtime configuration, and may not necessarily be present on the instance cert.
	hostedPluginRoles map[types.SystemRole]string

	// connectors is a list of connected clients and their identities
	connectors map[types.SystemRole]*Connector

	// registeredListeners keeps track of all listeners created by the process
	// used to pass listeners to child processes during live reload
	registeredListeners []registeredListener
	// importedDescriptors is a list of imported file descriptors
	// passed by the parent process
	importedDescriptors []*servicecfg.FileDescriptor
	// listenersClosed is a flag that indicates that the process should not open
	// new listeners (for instance, because we're shutting down and we've already
	// closed all the listeners)
	listenersClosed bool

	// forkedTeleportCount is the count of forked Teleport child processes
	// currently active, as spawned by SIGHUP or SIGUSR2.
	forkedTeleportCount atomic.Int32

	// storage is a server local storage
	storage *storage.ProcessStorage

	// id is a process id - used to identify different processes
	// during in-process reloads.
	id string

	// log is a process-local logrus.Entry.
	// Deprecated: use logger instead.
	log logrus.FieldLogger
	// logger is a process-local slog.Logger.
	logger *slog.Logger

	// reporter is used to report some in memory stats
	reporter *backend.Reporter

	// clusterFeatures contain flags for supported and unsupported features.
	clusterFeatures proto.Features

	// authSubjectiveAddr is the peer address of this process as seen by the auth
	// server during the most recent ping (may be empty).
	authSubjectiveAddr string

	// cloudLabels is a set of labels imported from a cloud provider and shared between
	// services.
	cloudLabels labels.Importer
	// TracingProvider is the provider to be used for exporting traces. In the event
	// that tracing is disabled this will be a no-op provider that drops all spans.
	TracingProvider *tracing.Provider

	// SSHD is used to execute commands to update or validate OpenSSH config.
	SSHD openssh.SSHD

	// resolver is used to identify the reverse tunnel address when connecting via
	// the proxy.
	resolver reversetunnelclient.Resolver
}

// enterpriseServicesEnabled will return true if any enterprise services are enabled.
func (process *TeleportProcess) enterpriseServicesEnabled() bool {
	return modules.GetModules().BuildType() == modules.BuildEnterprise &&
		(process.Config.Okta.Enabled || process.Config.Jamf.Enabled())
}

// enterpriseServicesEnabledWithCommunityBuild will return true if any
// enterprise services are enabled with an OSS teleport build.
func (process *TeleportProcess) enterpriseServicesEnabledWithCommunityBuild() bool {
	return modules.GetModules().IsOSSBuild() &&
		(process.Config.Okta.Enabled || process.Config.Jamf.Enabled())
}

// notifyParent notifies parent process that this process has started
// by writing to in-memory pipe used by communication channel.
func (process *TeleportProcess) notifyParent() {
	signalPipe, err := process.importSignalPipe()
	if err != nil {
		if !trace.IsNotFound(err) {
			process.logger.WarnContext(process.ExitContext(), "Failed to import signal pipe")
		}
		process.logger.DebugContext(process.ExitContext(), "No signal pipe to import, must be first Teleport process.")
		return
	}
	defer signalPipe.Close()

	ctx, cancel := context.WithTimeout(process.ExitContext(), signalPipeTimeout)
	defer cancel()

	if _, err := process.WaitForEvent(ctx, TeleportReadyEvent); err != nil {
		process.logger.ErrorContext(process.ExitContext(), "Timeout waiting for a forked process to start. Initiating self-shutdown.", "error", ctx.Err())
		if err := process.Close(); err != nil {
			process.logger.WarnContext(process.ExitContext(), "Failed to shutdown process.", "error", err)
		}
		return
	}
	process.logger.InfoContext(process.ExitContext(), "New service has started successfully.")

	if err := process.writeToSignalPipe(signalPipe, fmt.Sprintf("Process %v has started.", os.Getpid())); err != nil {
		process.logger.WarnContext(process.ExitContext(), "Failed to write to signal pipe", "error", err)
		// despite the failure, it's ok to proceed,
		// it could mean that the parent process has crashed and the pipe
		// is no longer valid.
	}
}

func (process *TeleportProcess) setLocalAuth(a *auth.Server) {
	process.Lock()
	defer process.Unlock()
	process.localAuth = a
}

func (process *TeleportProcess) getLocalAuth() *auth.Server {
	process.Lock()
	defer process.Unlock()
	return process.localAuth
}

func (process *TeleportProcess) setInstanceConnector(conn *Connector) {
	process.Lock()
	process.instanceConnector = conn
	process.Unlock()
	process.instanceConnectorReadyOnce.Do(func() {
		close(process.instanceConnectorReady)
	})
}

func (process *TeleportProcess) getInstanceConnector() *Connector {
	process.Lock()
	defer process.Unlock()
	return process.instanceConnector
}

// getInstanceClient tries to ge the current instance client without blocking. May return nil if either the
// instance client has yet to be created, or this is an auth-only instance. Auth-only instances cannot use
// the instance client because auth servers need to be able to fully initialize without a valid CA in order
// to support HSMs.
func (process *TeleportProcess) getInstanceClient() *authclient.Client {
	conn := process.getInstanceConnector()
	if conn == nil {
		return nil
	}
	return conn.Client
}

// waitForInstanceConnector waits for the instance connector to become available. returns nil if
// process shutdown is triggered or if this is an auth-only instance. Auth-only instances cannot
// use the instance client because auth servers need to be able to fully initialize without a
// valid CA in order to support HSMs.
func (process *TeleportProcess) waitForInstanceConnector() *Connector {
	select {
	case <-process.instanceConnectorReady:
		return process.getInstanceConnector()
	case <-process.ExitContext().Done():
		return nil
	}
}

// makeInventoryControlStreamWhenReady is the same as makeInventoryControlStream except that it blocks until
// the InstanceReady event is emitted.
func (process *TeleportProcess) makeInventoryControlStreamWhenReady(ctx context.Context) (client.DownstreamInventoryControlStream, error) {
	process.inventorySetupDelay.Do(func() {
		process.WaitForEvent(ctx, InstanceReady)
	})
	return process.makeInventoryControlStream(ctx)
}

func (process *TeleportProcess) makeInventoryControlStream(ctx context.Context) (client.DownstreamInventoryControlStream, error) {
	// if local auth exists, create an in-memory control stream
	if auth := process.getLocalAuth(); auth != nil {
		// we use getAuthSubjectiveAddr to guess our peer address even through we are
		// using an in-memory pipe. this works because heartbeat operations don't start
		// until after their respective services have successfully pinged the auth server.
		return auth.MakeLocalInventoryControlStream(client.ICSPipePeerAddrFn(process.getAuthSubjectiveAddr)), nil
	}

	// fallback to using the instance client
	clt := process.getInstanceClient()
	if clt == nil {
		return nil, trace.Errorf("instance client not yet initialized")
	}
	return clt.InventoryControlStream(ctx)
}

// adminCreds returns admin UID and GID settings based on the OS
func adminCreds() (*int, *int, error) {
	if runtime.GOOS != constants.LinuxOS {
		return nil, nil, nil
	}
	// if the user member of adm linux group,
	// make audit log folder readable by admins
	isAdmin, err := utils.IsGroupMember(teleport.LinuxAdminGID)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if !isAdmin {
		return nil, nil, nil
	}
	uid := os.Getuid()
	gid := teleport.LinuxAdminGID
	return &uid, &gid, nil
}

// Shutdown launches graceful shutdown process and waits
// for it to complete
func (process *TeleportProcess) Shutdown(ctx context.Context) {
	localCtx := process.StartShutdown(ctx)
	// wait until parent context closes
	<-localCtx.Done()
	process.logger.DebugContext(ctx, "Process completed.")
}

// StartShutdown launches non-blocking graceful shutdown process that signals
// completion, returns context that will be closed once the shutdown is done
func (process *TeleportProcess) StartShutdown(ctx context.Context) context.Context {
	// by the time we get here we've already extracted the parent pipe, which is
	// the only potential imported file descriptor that's not a listening
	// socket, so closing every imported FD with a prefix of "" will close all
	// imported listeners that haven't been used so far
	warnOnErr(process.ExitContext(), process.closeImportedDescriptors(""), process.logger)
	warnOnErr(process.ExitContext(), process.stopListeners(), process.logger)

	if process.forkedTeleportCount.Load() == 0 {
		if process.inventoryHandle != nil {
			if err := process.inventoryHandle.SendGoodbye(ctx); err != nil {
				process.logger.WarnContext(process.ExitContext(), "Failed sending inventory goodbye during shutdown", "error", err)
			}
		}
	} else {
		ctx = services.ProcessForkedContext(ctx)
	}

	process.BroadcastEvent(Event{Name: TeleportExitEvent, Payload: ctx})
	localCtx, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()
		if err := process.Supervisor.Wait(); err != nil {
			process.logger.WarnContext(process.ExitContext(), "Error waiting for all services to complete", "error", err)
		}
		process.logger.DebugContext(process.ExitContext(), "All supervisor functions are completed.")

		if localAuth := process.getLocalAuth(); localAuth != nil {
			if err := localAuth.Close(); err != nil {
				process.logger.WarnContext(process.ExitContext(), "Failed closing auth server.", "error", err)
			}
		}

		if process.storage != nil {
			if err := process.storage.Close(); err != nil {
				process.logger.WarnContext(process.ExitContext(), "Failed closing process storage.", "error", err)
			}
		}

		if process.inventoryHandle != nil {
			process.inventoryHandle.Close()
		}
	}()
	go process.printShutdownStatus(localCtx)
	return localCtx
}

// Close broadcasts close signals and exits immediately
func (process *TeleportProcess) Close() error {
	process.BroadcastEvent(Event{Name: TeleportExitEvent})

	var errors []error

	if localAuth := process.getLocalAuth(); localAuth != nil {
		errors = append(errors, localAuth.Close())
	}

	if process.storage != nil {
		errors = append(errors, process.storage.Close())
	}

	if process.inventoryHandle != nil {
		process.inventoryHandle.Close()
	}

	return trace.NewAggregate(errors...)
}

// getAuthSubjectiveAddr accesses the peer address reported by the auth server
// during the most recent ping. May be empty.
func (process *TeleportProcess) getAuthSubjectiveAddr() string {
	process.Lock()
	defer process.Unlock()
	return process.authSubjectiveAddr
}

func (process *TeleportProcess) setupProxyTLSConfig(conn *Connector, tsrv reversetunnelclient.Server, accessPoint authclient.ReadProxyAccessPoint, clusterName string) (*tls.Config, error) {
	cfg := process.Config
	var tlsConfig *tls.Config
	acmeCfg := process.Config.Proxy.ACME
	if acmeCfg.Enabled {
		process.Config.Logger.InfoContext(process.ExitContext(), "Managing certs using ACME https://datatracker.ietf.org/doc/rfc8555/.")

		acmePath := filepath.Join(process.Config.DataDir, teleport.ComponentACME)
		if err := os.MkdirAll(acmePath, teleport.PrivateDirMode); err != nil {
			return nil, trace.ConvertSystemError(err)
		}
		hostChecker, err := newHostPolicyChecker(hostPolicyCheckerConfig{
			publicAddrs: process.Config.Proxy.PublicAddrs,
			clt:         conn.Client,
			tun:         tsrv,
			clusterName: conn.ClusterName(),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		m := &autocert.Manager{
			Cache:      autocert.DirCache(acmePath),
			Prompt:     autocert.AcceptTOS,
			HostPolicy: hostChecker.checkHost,
			Email:      acmeCfg.Email,
		}
		if acmeCfg.URI != "" {
			m.Client = &acme.Client{DirectoryURL: acmeCfg.URI}
		}
		// We have to duplicate the behavior of `m.TLSConfig()` here because
		// http/1.1 needs to take precedence over h2 due to
		// https://bugs.chromium.org/p/chromium/issues/detail?id=1379017#c5 in Chrome.
		tlsConfig = &tls.Config{
			GetCertificate: m.GetCertificate,
			NextProtos: []string{
				string(alpncommon.ProtocolHTTP), string(alpncommon.ProtocolHTTP2), // enable HTTP/2
				acme.ALPNProto, // enable tls-alpn ACME challenges
			},
		}
		utils.SetupTLSConfig(tlsConfig, cfg.CipherSuites)
	} else {
		certReloader := NewCertReloader(CertReloaderConfig{
			KeyPairs:               process.Config.Proxy.KeyPairs,
			KeyPairsReloadInterval: process.Config.Proxy.KeyPairsReloadInterval,
		})
		if err := certReloader.Run(process.ExitContext()); err != nil {
			return nil, trace.Wrap(err)
		}

		tlsConfig = utils.TLSConfig(cfg.CipherSuites)
		tlsConfig.GetCertificate = certReloader.GetCertificate
	}

	setupTLSConfigALPNProtocols(tlsConfig)
	if err := process.setupTLSConfigClientCAGeneratorForCluster(tlsConfig, accessPoint, clusterName); err != nil {
		return nil, trace.Wrap(err)
	}
	return tlsConfig, nil
}

func setupTLSConfigALPNProtocols(tlsConfig *tls.Config) {
	// Go 1.17 introduced strict ALPN https://golang.org/doc/go1.17#ALPN If a client protocol is not recognized
	// the TLS handshake will fail.
	tlsConfig.NextProtos = apiutils.Deduplicate(append(tlsConfig.NextProtos, alpncommon.ProtocolsToString(alpncommon.SupportedProtocols)...))
}

func (process *TeleportProcess) setupTLSConfigClientCAGeneratorForCluster(tlsConfig *tls.Config, accessPoint authclient.ReadProxyAccessPoint, clusterName string) error {
	// create a local copy of the TLS config so we can change some settings that are only
	// relevant to the config returned by GetConfigForClient.
	tlsClone := tlsConfig.Clone()

	// Set client auth to "verify client cert if given" to support
	// app access CLI flow.
	//
	// Clients (like curl) connecting to the web proxy endpoint will
	// present a client certificate signed by the cluster's user CA.
	//
	// Browser connections to web UI and other clients (like database
	// access) connecting to web proxy won't be affected since they
	// don't present a certificate.
	tlsClone.ClientAuth = tls.VerifyClientCertIfGiven

	// Set up the client CA generator containing for the local cluster's CAs in
	// order to be able to validate certificates provided by app access CLI clients.
	generator, err := auth.NewClientTLSConfigGenerator(auth.ClientTLSConfigGeneratorConfig{
		TLS:                  tlsClone,
		ClusterName:          clusterName,
		PermitRemoteClusters: false,
		AccessPoint:          accessPoint,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	process.OnExit("closer", func(payload interface{}) {
		generator.Close()
	})

	// set getter on the original TLS config.
	tlsConfig.GetConfigForClient = generator.GetConfigForClient

	// note: generator will be closed via the passed in context, rather than an explicit call to Close.
	return nil
}

func (process *TeleportProcess) setupALPNTLSConfigForWeb(tlsConfig *tls.Config, accessPoint authclient.ReadProxyAccessPoint, clusterName string) (*tls.Config, error) {
	tlsConfig = tlsConfig.Clone()
	setupTLSConfigALPNProtocols(tlsConfig)
	if err := process.setupTLSConfigClientCAGeneratorForCluster(tlsConfig, accessPoint, clusterName); err != nil {
		return nil, trace.Wrap(err)
	}

	return tlsConfig, nil
}

func setupALPNRouter(listeners *proxyListeners, serverTLSConfig *tls.Config, cfg *servicecfg.Config) (router, rtRouter *alpnproxy.Router) {
	if listeners.web == nil || cfg.Proxy.DisableTLS || cfg.Proxy.DisableALPNSNIListener {
		return nil, nil
	}
	// ALPN proxy service will use web listener where listener.web will be overwritten by alpn wrapper
	// that allows to dispatch the http/1.1 and h2 traffic to webService.
	listeners.alpn = listeners.web
	router = alpnproxy.NewRouter()

	if listeners.minimalWeb != nil {
		listeners.reverseTunnelALPN = listeners.minimalWeb
		rtRouter = alpnproxy.NewRouter()
	}

	if !cfg.Proxy.DisableReverseTunnel {
		reverseTunnel := alpnproxy.NewMuxListenerWrapper(listeners.reverseTunnel, listeners.web)
		router.Add(alpnproxy.HandlerDecs{
			MatchFunc: alpnproxy.MatchByProtocol(alpncommon.ProtocolReverseTunnel),
			Handler:   reverseTunnel.HandleConnection,
		})
		listeners.reverseTunnel = reverseTunnel

		if rtRouter != nil {
			minimalWeb := alpnproxy.NewMuxListenerWrapper(nil, listeners.reverseTunnelALPN)
			rtRouter.Add(alpnproxy.HandlerDecs{
				MatchFunc: alpnproxy.MatchByProtocol(
					alpncommon.ProtocolHTTP,
					alpncommon.ProtocolHTTP2,
					alpncommon.ProtocolDefault,
				),
				Handler:    minimalWeb.HandleConnection,
				ForwardTLS: true,
			})
			listeners.minimalWeb = minimalWeb
		}

	}

	if !cfg.Proxy.DisableWebService {
		webWrapper := alpnproxy.NewMuxListenerWrapper(nil, listeners.web)
		router.Add(alpnproxy.HandlerDecs{
			MatchFunc: alpnproxy.MatchByProtocol(
				alpncommon.ProtocolHTTP,
				alpncommon.ProtocolHTTP2,
				acme.ALPNProto,
			),
			Handler:    webWrapper.HandleConnection,
			ForwardTLS: false,
		})
		listeners.web = webWrapper
	}
	// grpcPublicListener is a listener that does not enforce mTLS authentication.
	// It must not be used for any services that require authentication and currently
	// it is only used by the join service which nodes rely on to join the cluster.
	grpcPublicListener := alpnproxy.NewMuxListenerWrapper(nil /* serviceListener */, listeners.web)
	grpcPublicListener = alpnproxy.NewMuxListenerWrapper(grpcPublicListener, listeners.reverseTunnel)
	router.Add(alpnproxy.HandlerDecs{
		MatchFunc: alpnproxy.MatchByProtocol(alpncommon.ProtocolProxyGRPCInsecure),
		Handler:   grpcPublicListener.HandleConnection,
	})
	if rtRouter != nil {
		rtRouter.Add(alpnproxy.HandlerDecs{
			MatchFunc: alpnproxy.MatchByProtocol(alpncommon.ProtocolProxyGRPCInsecure),
			Handler:   grpcPublicListener.HandleConnection,
		})
	}
	listeners.grpcPublic = grpcPublicListener

	// grpcSecureListener is a listener that is used by a gRPC server that enforces
	// mTLS authentication. It must be used for any gRPC services that require authentication.
	grpcSecureListener := alpnproxy.NewMuxListenerWrapper(nil /* serviceListener */, listeners.web)
	router.Add(alpnproxy.HandlerDecs{
		MatchFunc: alpnproxy.MatchByProtocol(alpncommon.ProtocolProxyGRPCSecure),
		Handler:   grpcSecureListener.HandleConnection,
		// Forward the TLS configuration to the gRPC server so that it can handle mTLS authentication.
		ForwardTLS: true,
	})
	listeners.grpcMTLS = grpcSecureListener

	sshProxyListener := alpnproxy.NewMuxListenerWrapper(listeners.ssh, listeners.web)

	proxySSHTLSConfig := serverTLSConfig.Clone()
	proxySSHTLSConfig.NextProtos = []string{string(alpncommon.ProtocolProxySSH)}
	router.Add(alpnproxy.HandlerDecs{
		MatchFunc: alpnproxy.MatchByProtocol(alpncommon.ProtocolProxySSH),
		Handler:   sshProxyListener.HandleConnection,
		TLSConfig: proxySSHTLSConfig,
	})
	listeners.ssh = sshProxyListener

	sshGRPCListener := alpnproxy.NewMuxListenerWrapper(listeners.sshGRPC, listeners.web)
	// TLS forwarding is used instead of providing the TLSConfig so that the
	// authentication information makes it into the gRPC credentials.
	router.Add(alpnproxy.HandlerDecs{
		MatchFunc:  alpnproxy.MatchByProtocol(alpncommon.ProtocolProxySSHGRPC),
		Handler:    sshGRPCListener.HandleConnection,
		ForwardTLS: true,
	})
	listeners.sshGRPC = sshGRPCListener

	return router, rtRouter
}

// OnExit allows individual services to register a callback function which will be
// called when Teleport Process is asked to exit. Usually services terminate themselves
// when the callback is called
func (process *TeleportProcess) OnExit(serviceName string, callback func(interface{})) {
	process.RegisterFunc(serviceName, func() error {
		event, _ := process.WaitForEvent(context.TODO(), TeleportExitEvent)
		callback(event.Payload)
		return nil
	})
}

// initPublicGRPCServer creates and registers a gRPC server that does not use client
// certificates for authentication. This is used by the join service, which nodes
// use to receive a signed certificate from the auth server.
func (process *TeleportProcess) initPublicGRPCServer(
	limiter *limiter.Limiter,
	conn *Connector,
	listener net.Listener,
) (*grpc.Server, error) {
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			interceptors.GRPCServerUnaryErrorInterceptor,
			limiter.UnaryServerInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			interceptors.GRPCServerStreamErrorInterceptor,
			limiter.StreamServerInterceptor,
		),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			// Using an aggressive idle timeout here since this gRPC server
			// currently only hosts the join service, which has no need for
			// long-lived idle connections.
			//
			// The reason for introducing this is that teleport clients
			// before #17685 is fixed will hold connections open
			// indefinitely if they encounter an error during the joining
			// process, and this seems like the best way for the server to
			// forcibly close those connections.
			//
			// If another gRPC service is added here in the future, it
			// should be alright to increase or remove this idle timeout as
			// necessary once the client fix has been released and widely
			// available for some time.
			MaxConnectionIdle: 10 * time.Second,
		}),
		grpc.MaxConcurrentStreams(defaults.GRPCMaxConcurrentStreams),
	)
	joinServiceServer := joinserver.NewJoinServiceGRPCServer(conn.Client)
	proto.RegisterJoinServiceServer(server, joinServiceServer)

	accessGraphProxySvc, err := secretsscannerproxy.New(
		secretsscannerproxy.ServiceConfig{
			AuthClient: conn.Client,
			Log:        process.logger,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessgraphsecretsv1pb.RegisterSecretsScannerServiceServer(server, accessGraphProxySvc)

	process.RegisterCriticalFunc("proxy.grpc.public", func() error {
		process.logger.InfoContext(process.ExitContext(), "Starting proxy gRPC server.", "listen_address", listener.Addr())
		return trace.Wrap(server.Serve(listener))
	})
	return server, nil
}

// getIdentity returns the current identity (credentials to the auth server) for
// a given system role.
func (process *TeleportProcess) getIdentity(role types.SystemRole) (i *state.Identity, err error) {
	process.Lock()
	defer process.Unlock()

	i, err = process.storage.ReadIdentity(state.IdentityCurrent, role)

	if err == nil {
		return i, nil
	}
	if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	id := state.IdentityID{
		Role:     role,
		HostUUID: process.Config.HostUUID,
		NodeName: process.Config.Hostname,
	}
	if role == types.RoleAdmin {
		// for admin identity use local auth server
		// because admin identity is requested by auth server
		// itself
		principals, dnsNames, err := process.getAdditionalPrincipals(role)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		i, err = auth.GenerateIdentity(process.localAuth, id, principals, dnsNames)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return i, nil
	}

	// try to locate static identity provided in the file
	i, err = process.findStaticIdentity(id)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	process.logger.InfoContext(process.ExitContext(), "Found static identity in the config file, writing to disk.", "identity", logutils.StringerAttr(&id))
	if err = process.storage.WriteIdentity(state.IdentityCurrent, *i); err != nil {
		return nil, trace.Wrap(err)
	}

	return i, nil
}

// addConnector adds connector to registered connectors list,
// it will overwrite the connector for the same role
func (process *TeleportProcess) addConnector(connector *Connector) {
	process.Lock()
	defer process.Unlock()

	process.connectors[connector.Role()] = connector
}

// getInstanceRoles returns the list of enabled service roles.  this differs from simply
// checking the roles of the existing connectors  in two key ways.  First, pseudo-services
// like "admin" or "instance" are not included. Secondly, instance roles are recorded synchronously
// at the time the associated component's init function runs, as opposed to connectors which are
// initialized asynchronously in the background.
func (process *TeleportProcess) getInstanceRoles() []types.SystemRole {
	process.Lock()
	defer process.Unlock()

	out := make([]types.SystemRole, 0, len(process.instanceRoles))
	for role := range process.instanceRoles {
		out = append(out, role)
	}
	return out
}

// getInstanceRoleEventMapping returns the same instance roles as getInstanceRoles, but as a mapping
// of the form `role => event_name`. This can be used to determine what identity event should be
// awaited in order to get a connector for a given role. Used in assertion-based migration to
// iteratively create a system role assertion through each client.
func (process *TeleportProcess) getInstanceRoleEventMapping() map[types.SystemRole]string {
	process.Lock()
	defer process.Unlock()
	out := make(map[types.SystemRole]string, len(process.instanceRoles))
	for role, event := range process.instanceRoles {
		out[role] = event
	}
	return out
}

// OnHeartbeat generates the default OnHeartbeat callback for the specified component.
func (process *TeleportProcess) OnHeartbeat(component string) func(err error) {
	return func(err error) {
		if err != nil {
			process.BroadcastEvent(Event{Name: TeleportDegradedEvent, Payload: component})
		} else {
			process.BroadcastEvent(Event{Name: TeleportOKEvent, Payload: component})
		}
	}
}

func (process *TeleportProcess) findStaticIdentity(id state.IdentityID) (*state.Identity, error) {
	for i := range process.Config.Identities {
		identity := process.Config.Identities[i]
		if identity.ID.Equals(id) {
			return identity, nil
		}
	}
	return nil, trace.NotFound("identity %v not found", &id)
}

// getAdditionalPrincipals returns a list of additional principals to add
// to role's service certificates.
func (process *TeleportProcess) getAdditionalPrincipals(role types.SystemRole) ([]string, []string, error) {
	var principals []string
	var dnsNames []string
	if process.Config.Hostname != "" {
		principals = append(principals, process.Config.Hostname)
		if lh := utils.ToLowerCaseASCII(process.Config.Hostname); lh != process.Config.Hostname {
			// openssh expects all hostnames to be lowercase
			principals = append(principals, lh)
		}
	}
	var addrs []utils.NetAddr

	// Add default DNSNames to the dnsNames list.
	// For identities generated by teleport <= v6.1.6 the teleport.cluster.local DNS is not present
	dnsNames = append(dnsNames, auth.DefaultDNSNamesForRole(role)...)

	switch role {
	case types.RoleProxy:
		addrs = append(process.Config.Proxy.PublicAddrs,
			process.Config.Proxy.WebAddr,
			process.Config.Proxy.SSHAddr,
			process.Config.Proxy.ReverseTunnelListenAddr,
			process.Config.Proxy.MySQLAddr,
			process.Config.Proxy.PeerAddress,
			utils.NetAddr{Addr: string(teleport.PrincipalLocalhost)},
			utils.NetAddr{Addr: string(teleport.PrincipalLoopbackV4)},
			utils.NetAddr{Addr: string(teleport.PrincipalLoopbackV6)},
			utils.NetAddr{Addr: reversetunnelclient.LocalKubernetes},
		)
		addrs = append(addrs, process.Config.Proxy.SSHPublicAddrs...)
		addrs = append(addrs, process.Config.Proxy.TunnelPublicAddrs...)
		addrs = append(addrs, process.Config.Proxy.PostgresPublicAddrs...)
		addrs = append(addrs, process.Config.Proxy.MySQLPublicAddrs...)
		addrs = append(addrs, process.Config.Proxy.Kube.PublicAddrs...)
		addrs = append(addrs, process.Config.Proxy.PeerPublicAddr)
		// Automatically add wildcards for every proxy public address for k8s SNI routing
		if process.Config.Proxy.Kube.Enabled {
			for _, publicAddr := range utils.JoinAddrSlices(process.Config.Proxy.PublicAddrs, process.Config.Proxy.Kube.PublicAddrs) {
				host, err := utils.Host(publicAddr.Addr)
				if err != nil {
					return nil, nil, trace.Wrap(err)
				}
				if ip := net.ParseIP(host); ip == nil {
					dnsNames = append(dnsNames, "*."+host)
				}
			}
		}
	case types.RoleAuth, types.RoleAdmin:
		addrs = process.Config.Auth.PublicAddrs
	case types.RoleNode:
		// DELETE IN 5.0: We are manually adding HostUUID here in order
		// to allow UUID based routing to function with older Auth Servers
		// which don't automatically add UUID to the principal list.
		principals = append(principals, process.Config.HostUUID)
		addrs = process.Config.SSH.PublicAddrs
		// If advertise IP is set, add it to the list of principals. Otherwise
		// add in the default (0.0.0.0) which will be replaced by the Auth Server
		// when a host certificate is issued.
		if process.Config.AdvertiseIP != "" {
			advertiseIP, err := utils.ParseAddr(process.Config.AdvertiseIP)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			addrs = append(addrs, *advertiseIP)
		} else {
			addrs = append(addrs, process.Config.SSH.Addr)
		}
	case types.RoleKube:
		addrs = append(addrs,
			utils.NetAddr{Addr: string(teleport.PrincipalLocalhost)},
			utils.NetAddr{Addr: string(teleport.PrincipalLoopbackV4)},
			utils.NetAddr{Addr: string(teleport.PrincipalLoopbackV6)},
			utils.NetAddr{Addr: reversetunnelclient.LocalKubernetes},
		)
		addrs = append(addrs, process.Config.Kube.PublicAddrs...)
	case types.RoleApp, types.RoleOkta:
		principals = append(principals, process.Config.HostUUID)
	case types.RoleWindowsDesktop:
		addrs = append(addrs,
			utils.NetAddr{Addr: string(teleport.PrincipalLocalhost)},
			utils.NetAddr{Addr: string(teleport.PrincipalLoopbackV4)},
			utils.NetAddr{Addr: string(teleport.PrincipalLoopbackV6)},
			utils.NetAddr{Addr: reversetunnelclient.LocalWindowsDesktop},
			utils.NetAddr{Addr: desktop.WildcardServiceDNS},
		)
		addrs = append(addrs, process.Config.WindowsDesktop.PublicAddrs...)
	}

	if process.Config.OpenSSH.Enabled {
		for _, a := range process.Config.OpenSSH.AdditionalPrincipals {
			addr, err := utils.ParseAddr(a)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			addrs = append(addrs, *addr)
		}
	}

	for _, addr := range addrs {
		if addr.IsEmpty() {
			continue
		}
		host := addr.Host()
		if host == "" {
			host = defaults.BindIP
		}
		principals = append(principals, host)
	}
	return principals, dnsNames, nil
}

// initProxy gets called if teleport runs with 'proxy' role enabled.
// this means it will do several things:
//  1. serve a web UI
//  2. proxy SSH connections to nodes running with 'node' role
//  3. take care of reverse tunnels
//  4. optionally proxy kubernetes connections
func (process *TeleportProcess) initProxy() error {
	// If no TLS key was provided for the web listener, generate a self-signed cert
	if len(process.Config.Proxy.KeyPairs) == 0 &&
		!process.Config.Proxy.DisableTLS &&
		!process.Config.Proxy.ACME.Enabled {
		err := initSelfSignedHTTPSCert(process.Config)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	process.RegisterWithAuthServer(types.RoleProxy, ProxyIdentityEvent)
	process.RegisterCriticalFunc("proxy.init", func() error {
		conn, err := process.WaitForConnector(ProxyIdentityEvent, process.logger)
		if conn == nil {
			return trace.Wrap(err)
		}

		if err := process.initProxyEndpoint(conn); err != nil {
			warnOnErr(process.ExitContext(), conn.Close(), process.logger)
			return trace.Wrap(err)
		}

		return nil
	})
	return nil
}

// initSelfSignedHTTPSCert generates and self-signs a TLS key+cert pair for https connection
// to the proxy server.
func initSelfSignedHTTPSCert(cfg *servicecfg.Config) (err error) {
	ctx := context.Background()
	cfg.Logger.WarnContext(ctx, "No TLS Keys provided, using self-signed certificate.")

	keyPath := filepath.Join(cfg.DataDir, defaults.SelfSignedKeyPath)
	certPath := filepath.Join(cfg.DataDir, defaults.SelfSignedCertPath)

	cfg.Proxy.KeyPairs = append(cfg.Proxy.KeyPairs, servicecfg.KeyPairPath{
		PrivateKey:  keyPath,
		Certificate: certPath,
	})

	// return the existing pair if they have already been generated:
	_, err = tls.LoadX509KeyPair(certPath, keyPath)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return trace.Wrap(err, "unrecognized error reading certs")
	}
	cfg.Logger.WarnContext(ctx, "Generating self-signed key and cert.", "key_path", keyPath, "cert_path", certPath)

	hosts := []string{cfg.Hostname, "localhost"}
	var ips []string

	// add web public address hosts to self-signed cert
	for _, addr := range cfg.Proxy.PublicAddrs {
		proxyHost, _, err := net.SplitHostPort(addr.String())
		if err != nil {
			// log and skip error since this is a nice to have
			cfg.Logger.WarnContext(ctx, "Error parsing proxy.public_address, skipping adding to self-signed cert", "public_address", addr.String(), "error", err)
			continue
		}
		// If the address is a IP have it added as IP SAN
		if ip := net.ParseIP(proxyHost); ip != nil {
			ips = append(ips, proxyHost)
		} else {
			hosts = append(hosts, proxyHost)
		}
	}

	creds, err := cert.GenerateSelfSignedCert(hosts, ips)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := os.WriteFile(keyPath, creds.PrivateKey, 0o600); err != nil {
		return trace.Wrap(err, "error writing key PEM")
	}
	if err := os.WriteFile(certPath, creds.Cert, 0o600); err != nil {
		return trace.Wrap(err, "error writing key PEM")
	}
	return nil
}

// RegisterWithAuthServer uses one time provisioning token obtained earlier
// from the server to get a pair of SSH keys signed by Auth server host
// certificate authority
func (process *TeleportProcess) RegisterWithAuthServer(role types.SystemRole, eventName string) {
	serviceName := strings.ToLower(role.String())

	process.RegisterCriticalFunc(fmt.Sprintf("register.%v", serviceName), func() error {
		if role.IsLocalService() && !(process.instanceRoleExpected(role) || process.hostedPluginRoleExpected(role)) {
			// if you hit this error, your probably forgot to call SetExpectedInstanceRole inside of
			// the registerExpectedServices function, or forgot to call SetExpectedHostedPluginRole during
			// the hosted plugin init process.
			process.logger.ErrorContext(process.ExitContext(), "Register called for unexpected instance role (this is a bug).", "role", role)
		}

		connector, err := process.reconnectToAuthService(role)
		if err != nil {
			return trace.Wrap(err)
		}

		process.BroadcastEvent(Event{Name: eventName, Payload: connector})
		return nil
	})
}

// waitForInstanceConnector waits for the instance connector to be ready,
// logging a warning if this is taking longer than expected.
func waitForInstanceConnector(process *TeleportProcess, log *slog.Logger) (*Connector, error) {
	type r struct {
		c   *Connector
		err error
	}
	ch := make(chan r, 1)
	go func() {
		conn, err := process.WaitForConnector(InstanceIdentityEvent, log)
		ch <- r{conn, err}
	}()

	t := time.NewTicker(30 * time.Second)
	defer t.Stop()

	for {
		select {
		case result := <-ch:
			if result.c == nil {
				return nil, trace.Wrap(result.err, "waiting for instance connector")
			}
			return result.c, nil
		case <-t.C:
			log.WarnContext(process.ExitContext(), "The Instance connector is still not available, process-wide services such as session uploading will not function")
		}
	}
}

// WaitForConnector is a utility function to wait for an identity event and cast
// the resulting payload as a *Connector. Returns (nil, nil) when the
// ExitContext is done, so error checking should happen on the connector rather
// than the error:
//
//	conn, err := process.WaitForConnector("FooIdentity", log)
//	if conn == nil {
//		return trace.Wrap(err)
//	}
func (process *TeleportProcess) WaitForConnector(identityEvent string, log *slog.Logger) (*Connector, error) {
	event, err := process.WaitForEvent(process.ExitContext(), identityEvent)
	if err != nil {
		if log != nil {
			log.DebugContext(process.ExitContext(), "Process is exiting.")
		}
		return nil, nil
	}
	if log != nil {
		log.DebugContext(process.ExitContext(), "Received event.", "event", event.Name)
	}

	conn, ok := (event.Payload).(*Connector)
	if !ok {
		return nil, trace.BadParameter("unsupported connector type: %T", event.Payload)
	}

	return conn, nil
}

// SetExpectedInstanceRole marks a given instance role as active, storing the name of its associated
// identity event.
func (process *TeleportProcess) SetExpectedInstanceRole(role types.SystemRole, eventName string) {
	process.Lock()
	defer process.Unlock()
	process.instanceRoles[role] = eventName
}

// SetExpectedHostedPluginRole marks a given hosted plugin role as active, storing the name of its associated
// identity event.
func (process *TeleportProcess) SetExpectedHostedPluginRole(role types.SystemRole, eventName string) {
	process.Lock()
	defer process.Unlock()
	process.hostedPluginRoles[role] = eventName
}

func (process *TeleportProcess) instanceRoleExpected(role types.SystemRole) bool {
	process.Lock()
	defer process.Unlock()
	_, ok := process.instanceRoles[role]
	return ok
}

func (process *TeleportProcess) hostedPluginRoleExpected(role types.SystemRole) bool {
	process.Lock()
	defer process.Unlock()
	_, ok := process.hostedPluginRoles[role]
	return ok
}

func (process *TeleportProcess) initProxyEndpoint(conn *Connector) error {
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

// getConnectors returns a copy of the identities registered for auth server
func (process *TeleportProcess) getConnectors() []*Connector {
	process.Lock()
	defer process.Unlock()

	out := make([]*Connector, 0, len(process.connectors))
	for role := range process.connectors {
		out = append(out, process.connectors[role])
	}
	return out
}

func (process *TeleportProcess) setClusterFeatures(features *proto.Features) {
	process.Lock()
	defer process.Unlock()

	if features != nil {
		process.clusterFeatures = *features
	}
}

// setAuthSubjectiveAddr records the peer address that the auth server observed
// for this process during the most recent ping.
func (process *TeleportProcess) setAuthSubjectiveAddr(ip string) {
	process.Lock()
	defer process.Unlock()
	if ip != "" {
		process.authSubjectiveAddr = ip
	}
}

// setupProxyListeners sets up web proxy listeners based on the configuration
func (process *TeleportProcess) setupProxyListeners(networkingConfig types.ClusterNetworkingConfig, accessPoint authclient.ProxyAccessPoint, clusterName string) (*proxyListeners, error) {
	cfg := process.Config
	process.logger.DebugContext(process.ExitContext(), "Setting up Proxy listeners", "web_address", cfg.Proxy.WebAddr.Addr, "tunnel_address", cfg.Proxy.ReverseTunnelListenAddr.Addr)
	var err error
	var listeners proxyListeners

	muxCAGetter := func(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
		return accessPoint.GetCertAuthority(ctx, id, loadKeys)
	}

	if !cfg.Proxy.SSHAddr.IsEmpty() {
		l, err := process.importOrCreateListener(ListenerProxySSH, cfg.Proxy.SSHAddr.Addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		mux, err := multiplexer.New(multiplexer.Config{
			Listener:            l,
			PROXYProtocolMode:   cfg.Proxy.PROXYProtocolMode,
			ID:                  teleport.Component(teleport.ComponentProxy, "ssh"),
			CertAuthorityGetter: muxCAGetter,
			LocalClusterName:    clusterName,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		listeners.sshMux = mux
		listeners.ssh = mux.SSH()
		listeners.sshGRPC = mux.TLS()
		go func() {
			if err := mux.Serve(); err != nil && !utils.IsOKNetworkError(err) {
				mux.Entry.WithError(err).Error("Mux encountered err serving")
			}
		}()
	}

	tunnelStrategy, err := networkingConfig.GetTunnelStrategyType()
	if err != nil {
		process.logger.WarnContext(process.ExitContext(), "Failed to get tunnel strategy. Falling back to agent mesh strategy.", "error", err)
		tunnelStrategy = types.AgentMesh
	}

	if tunnelStrategy == types.ProxyPeering &&
		modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil, trace.AccessDenied("proxy peering is an enterprise-only feature")
	}

	if !cfg.Proxy.DisableReverseTunnel && tunnelStrategy == types.ProxyPeering {
		addr, err := process.Config.Proxy.PeerAddr()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		listener, err := process.importOrCreateListener(ListenerProxyPeer, addr.String())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		listeners.proxyPeer = listener
	}

	switch {
	case cfg.Proxy.DisableWebService && cfg.Proxy.DisableReverseTunnel:
		process.logger.DebugContext(process.ExitContext(), "Setup Proxy: Reverse tunnel proxy and web proxy are disabled.")
		return &listeners, nil
	case cfg.Proxy.ReverseTunnelListenAddr == cfg.Proxy.WebAddr && !cfg.Proxy.DisableTLS:
		process.logger.DebugContext(process.ExitContext(), "Setup Proxy: Reverse tunnel proxy and web proxy listen on the same port, multiplexing is on.")
		listener, err := process.importOrCreateListener(ListenerProxyTunnelAndWeb, cfg.Proxy.WebAddr.Addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		listeners.mux, err = multiplexer.New(multiplexer.Config{
			PROXYProtocolMode:   cfg.Proxy.PROXYProtocolMode,
			Listener:            listener,
			ID:                  teleport.Component(teleport.ComponentProxy, "tunnel", "web", process.id),
			CertAuthorityGetter: muxCAGetter,
			LocalClusterName:    clusterName,
		})
		if err != nil {
			listener.Close()
			return nil, trace.Wrap(err)
		}
		if !cfg.Proxy.DisableWebService {
			listeners.web = listeners.mux.TLS()
		}
		go func() {
			if err := listeners.mux.Serve(); err != nil && !utils.IsOKNetworkError(err) {
				listeners.mux.Entry.WithError(err).Error("Mux encountered err serving")
			}
		}()
		return &listeners, nil
	case cfg.Proxy.PROXYProtocolMode != multiplexer.PROXYProtocolOff && !cfg.Proxy.DisableWebService && !cfg.Proxy.DisableTLS:
		process.logger.DebugContext(process.ExitContext(), "Setup Proxy: PROXY protocol is enabled for web service, multiplexing is on.")
		listener, err := process.importOrCreateListener(ListenerProxyWeb, cfg.Proxy.WebAddr.Addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		listeners.mux, err = multiplexer.New(multiplexer.Config{
			PROXYProtocolMode:   cfg.Proxy.PROXYProtocolMode,
			Listener:            listener,
			ID:                  teleport.Component(teleport.ComponentProxy, "web", process.id),
			CertAuthorityGetter: muxCAGetter,
			LocalClusterName:    clusterName,
		})
		if err != nil {
			listener.Close()
			return nil, trace.Wrap(err)
		}
		listeners.web = listeners.mux.TLS()
		go func() {
			if err := listeners.mux.Serve(); err != nil && !utils.IsOKNetworkError(err) {
				listeners.mux.Entry.WithError(err).Error("Mux encountered err serving")
			}
		}()
		return &listeners, nil
	default:
		process.logger.DebugContext(process.ExitContext(), "Setup Proxy: Proxy and reverse tunnel are listening on separate ports.")
		if !cfg.Proxy.DisableReverseTunnel && !cfg.Proxy.ReverseTunnelListenAddr.IsEmpty() {
			if cfg.Proxy.DisableWebService {
				listeners.reverseTunnel, err = process.importOrCreateListener(ListenerProxyTunnel, cfg.Proxy.ReverseTunnelListenAddr.Addr)
				if err != nil {
					listeners.Close()
					return nil, trace.Wrap(err)
				}
			} else {
				if err := process.initMinimalReverseTunnelListener(cfg, &listeners); err != nil {
					listeners.Close()
					return nil, trace.Wrap(err)
				}
			}
		}
		if !cfg.Proxy.DisableWebService && !cfg.Proxy.WebAddr.IsEmpty() {
			listener, err := process.importOrCreateListener(ListenerProxyWeb, cfg.Proxy.WebAddr.Addr)
			if err != nil {
				listeners.Close()
				return nil, trace.Wrap(err)
			}
			// Unless database proxy is explicitly disabled (which is currently
			// only done by tests and not exposed via file config), the web
			// listener is multiplexing both web and db client connections.
			if !cfg.Proxy.DisableDatabaseProxy && !cfg.Proxy.DisableTLS {
				process.logger.DebugContext(process.ExitContext(), "Setup Proxy: Multiplexing web and database proxy on the same port.")
				listeners.mux, err = multiplexer.New(multiplexer.Config{
					PROXYProtocolMode:   cfg.Proxy.PROXYProtocolMode,
					Listener:            listener,
					ID:                  teleport.Component(teleport.ComponentProxy, "web", process.id),
					CertAuthorityGetter: muxCAGetter,
					LocalClusterName:    clusterName,
				})
				if err != nil {
					listener.Close()
					listeners.Close()
					return nil, trace.Wrap(err)
				}
				listeners.web = listeners.mux.TLS()
				go func() {
					if err := listeners.mux.Serve(); err != nil && !utils.IsOKNetworkError(err) {
						listeners.mux.Entry.WithError(err).Error("Mux encountered err serving")
					}
				}()
			} else {
				process.logger.DebugContext(process.ExitContext(), "Setup Proxy: TLS is disabled, multiplexing is off.")
				listeners.web = listener
			}
		}

		// Even if web service API was disabled create a web listener used for ALPN/SNI service as the master port
		if cfg.Proxy.DisableWebService && !cfg.Proxy.DisableTLS && listeners.web == nil {
			listeners.web, err = process.importOrCreateListener(ListenerProxyWeb, cfg.Proxy.WebAddr.Addr)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return &listeners, nil
	}
}

// GetRotation returns the process rotation.
func (process *TeleportProcess) GetRotation(role types.SystemRole) (*types.Rotation, error) {
	state, err := process.storage.GetState(context.TODO(), role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &state.Spec.Rotation, nil
}

func (process *TeleportProcess) proxyPublicAddr() utils.NetAddr {
	if len(process.Config.Proxy.PublicAddrs) == 0 {
		return utils.NetAddr{}
	}
	return process.Config.Proxy.PublicAddrs[0]
}

// newLocalCacheForProxy returns new instance of access point configured for a local proxy.
func (process *TeleportProcess) newLocalCacheForProxy(clt authclient.ClientI, cacheName []string) (authclient.ProxyAccessPoint, error) {
	// if caching is disabled, return access point
	if !process.Config.CachePolicy.Enabled {
		return clt, nil
	}

	cache, err := process.NewLocalCache(clt, cache.ForProxy, cacheName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return authclient.NewProxyWrapper(clt, cache), nil
}

// NewLocalCache returns new instance of access point
func (process *TeleportProcess) NewLocalCache(clt authclient.ClientI, setupConfig cache.SetupConfigFn, cacheName []string) (*cache.Cache, error) {
	return process.newAccessCacheForClient(accesspoint.Config{
		Setup:     setupConfig,
		CacheName: cacheName,
	}, clt)
}

func (process *TeleportProcess) newAccessCacheForClient(cfg accesspoint.Config, client authclient.ClientI) (*cache.Cache, error) {
	cfg.Context = process.ExitContext()
	cfg.ProcessID = process.id
	cfg.TracingProvider = process.TracingProvider
	cfg.MaxRetryPeriod = process.Config.CachePolicy.MaxRetryPeriod

	cfg.Access = client
	cfg.AccessLists = client.AccessListClient()
	cfg.AccessMonitoringRules = client.AccessMonitoringRuleClient()
	cfg.AppSession = client
	cfg.Apps = client
	cfg.ClusterConfig = client
	cfg.CrownJewels = client.CrownJewelServiceClient()
	cfg.DatabaseObjects = client.DatabaseObjectsClient()
	cfg.DatabaseServices = client
	cfg.Databases = client
	cfg.DiscoveryConfigs = client.DiscoveryConfigClient()
	cfg.DynamicAccess = client
	cfg.Events = client
	cfg.Integrations = client
	cfg.UserTasks = client.UserTasksServiceClient()
	cfg.KubeWaitingContainers = client
	cfg.Kubernetes = client
	cfg.Notifications = client
	cfg.Okta = client.OktaClient()
	cfg.Presence = client
	cfg.Provisioner = client
	cfg.Restrictions = client
	cfg.SAMLIdPServiceProviders = client
	cfg.SAMLIdPSession = client
	cfg.SecReports = client.SecReportsClient()
	cfg.SnowflakeSession = client
	cfg.StaticHostUsers = client.StaticHostUserClient()
	cfg.Trust = client
	cfg.UserGroups = client
	cfg.UserLoginStates = client.UserLoginStateClient()
	cfg.Users = client
	cfg.WebSession = client.WebSessions()
	cfg.WebToken = client.WebTokens()
	cfg.WindowsDesktops = client
	cfg.DynamicWindowsDesktops = client.DynamicDesktopClient()
	cfg.AutoUpdateService = client

	return accesspoint.NewCache(cfg)
}

// NewAsyncEmitter wraps client and returns emitter that never blocks, logs some events and checks values.
// It is caller's responsibility to call Close on the emitter once done.
func (process *TeleportProcess) NewAsyncEmitter(clt apievents.Emitter) (*events.AsyncEmitter, error) {
	emitter, err := events.NewCheckingEmitter(events.CheckingEmitterConfig{
		Inner: events.NewMultiEmitter(events.NewLoggingEmitter(process.GetClusterFeatures().Cloud), clt),
		Clock: process.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// asyncEmitter makes sure that sessions do not block
	// in case if connections are slow
	return events.NewAsyncEmitter(events.AsyncEmitterConfig{
		Inner: emitter,
	})
}

func payloadContext(payload any) context.Context {
	if ctx, ok := payload.(context.Context); ok {
		return ctx
	}

	return context.TODO()
}

func (process *TeleportProcess) readOrInitPeerStatelessResetKey() (*quic.StatelessResetKey, error) {
	resetKeyPath := filepath.Join(process.Config.DataDir, "peer_stateless_reset_key")
	k := new(quic.StatelessResetKey)
	stored, err := os.ReadFile(resetKeyPath)
	if err == nil && len(stored) == len(k) {
		copy(k[:], stored)
		return k, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		process.logger.WarnContext(process.ExitContext(), "Stateless reset key file unreadable or invalid.", "error", err)
	}
	if _, err := rand.Read(k[:]); err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	if err := renameio.WriteFile(resetKeyPath, k[:], 0o600); err != nil {
		process.logger.WarnContext(process.ExitContext(), "Failed to persist stateless reset key.", "error", err)
	}
	return k, nil
}

// newLocalCacheForRemoteProxy returns new instance of access point configured for a remote proxy.
func (process *TeleportProcess) newLocalCacheForRemoteProxy(clt authclient.ClientI, cacheName []string) (authclient.RemoteProxyAccessPoint, error) {
	// if caching is disabled, return access point
	if !process.Config.CachePolicy.Enabled {
		return clt, nil
	}

	cache, err := process.NewLocalCache(clt, cache.ForRemoteProxy, cacheName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return authclient.NewRemoteProxyWrapper(clt, cache), nil
}

// initMinimalReverseTunnelListener starts a listener over a reverse tunnel that multiplexes a minimal subset of the
// web API.
func (process *TeleportProcess) initMinimalReverseTunnelListener(cfg *servicecfg.Config, listeners *proxyListeners) error {
	listener, err := process.importOrCreateListener(ListenerProxyTunnel, cfg.Proxy.ReverseTunnelListenAddr.Addr)
	if err != nil {
		return trace.Wrap(err)
	}
	listeners.reverseTunnelMux, err = multiplexer.New(multiplexer.Config{
		PROXYProtocolMode: cfg.Proxy.PROXYProtocolMode,
		Listener:          listener,
		ID:                teleport.Component(teleport.ComponentProxy, "tunnel", "web", process.id),
	})
	if err != nil {
		listener.Close()
		return trace.Wrap(err)
	}
	listeners.reverseTunnel = listeners.reverseTunnelMux.SSH()
	go func() {
		if err := listeners.reverseTunnelMux.Serve(); err != nil {
			process.logger.DebugContext(process.ExitContext(), "Minimal reverse tunnel mux exited with error", "error", err)
		}
	}()
	listeners.minimalWeb = listeners.reverseTunnelMux.TLS()
	return nil
}

// GetClusterFeatures returns the cluster features.
func (process *TeleportProcess) GetClusterFeatures() proto.Features {
	process.Lock()
	defer process.Unlock()

	return process.clusterFeatures
}

// initMetricsService starts the metrics service currently serving metrics for
// prometheus consumption
func (process *TeleportProcess) initMetricsService() error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	logger := process.logger.With(teleport.ComponentKey, teleport.Component(teleport.ComponentMetrics, process.id))

	listener, err := process.importOrCreateListener(ListenerMetrics, process.Config.Metrics.ListenAddr.Addr)
	if err != nil {
		return trace.Wrap(err)
	}
	warnOnErr(process.ExitContext(), process.closeImportedDescriptors(teleport.ComponentMetrics), logger)

	tlsConfig := &tls.Config{}
	if process.Config.Metrics.MTLS {
		for _, pair := range process.Config.Metrics.KeyPairs {
			certificate, err := tls.LoadX509KeyPair(pair.Certificate, pair.PrivateKey)
			if err != nil {
				return trace.Wrap(err, "failed to read keypair: %+v", err)
			}
			tlsConfig.Certificates = append(tlsConfig.Certificates, certificate)
		}

		if len(tlsConfig.Certificates) == 0 {
			return trace.BadParameter("no keypairs were provided for the metrics service with mtls enabled")
		}

		addedCerts := false
		pool := x509.NewCertPool()
		for _, caCertPath := range process.Config.Metrics.CACerts {
			caCert, err := os.ReadFile(caCertPath)
			if err != nil {
				return trace.Wrap(err, "failed to read prometheus CA certificate %+v", caCertPath)
			}

			if !pool.AppendCertsFromPEM(caCert) {
				return trace.BadParameter("failed to parse prometheus CA certificate: %+v", caCertPath)
			}
			addedCerts = true
		}

		if !addedCerts {
			return trace.BadParameter("no prometheus ca certs were provided for the metrics service with mtls enabled")
		}

		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		tlsConfig.ClientCAs = pool
		//nolint:staticcheck // Keep BuildNameToCertificate to avoid changes in legacy behavior.
		tlsConfig.BuildNameToCertificate()

		listener = tls.NewListener(listener, tlsConfig)
	}

	server := &http.Server{
		Handler:           mux,
		ReadTimeout:       apidefaults.DefaultIOTimeout,
		ReadHeaderTimeout: defaults.ReadHeadersTimeout,
		WriteTimeout:      apidefaults.DefaultIOTimeout,
		IdleTimeout:       apidefaults.DefaultIdleTimeout,
		TLSConfig:         tlsConfig,
	}

	logger.InfoContext(process.ExitContext(), "Starting metrics service.", "listen_address", process.Config.Metrics.ListenAddr.Addr)

	process.RegisterFunc("metrics.service", func() error {
		err := server.Serve(listener)
		if err != nil && err != http.ErrServerClosed {
			logger.WarnContext(process.ExitContext(), "Metrics server exited with error.", "error", err)
		}
		return nil
	})

	process.OnExit("metrics.shutdown", func(payload interface{}) {
		if payload == nil {
			logger.InfoContext(process.ExitContext(), "Shutting down immediately.")
			warnOnErr(process.ExitContext(), server.Close(), logger)
		} else {
			logger.InfoContext(process.ExitContext(), "Shutting down gracefully.")
			ctx := payloadContext(payload)
			warnOnErr(process.ExitContext(), server.Shutdown(ctx), logger)
		}
		logger.InfoContext(process.ExitContext(), "Exited.")
	})

	process.BroadcastEvent(Event{Name: MetricsReady, Payload: nil})
	return nil
}

func (process *TeleportProcess) initApps() {
	// If no applications are specified, exit early. This is due to the strange
	// behavior in reading file configuration. If the user does not specify an
	// "app_service" section, that is considered enabling "app_service".
	if len(process.Config.Apps.Apps) == 0 &&
		!process.Config.Apps.DebugApp &&
		len(process.Config.Apps.ResourceMatchers) == 0 {
		return
	}

	// Connect to the Auth Server, a client connected to the Auth Server will
	// be returned. For this to be successful, credentials to connect to the
	// Auth Server need to exist on disk or a registration token should be
	// provided.
	process.RegisterWithAuthServer(types.RoleApp, AppsIdentityEvent)

	// Define logger to prefix log lines with the name of the component and PID.
	component := teleport.Component(teleport.ComponentApp, process.id)
	logger := process.logger.With(teleport.ComponentKey, component)

	process.RegisterCriticalFunc("apps.start", func() error {
		conn, err := process.WaitForConnector(AppsIdentityEvent, logger)
		if conn == nil {
			return trace.Wrap(err)
		}

		shouldSkipCleanup := false
		defer func() {
			if !shouldSkipCleanup {
				warnOnErr(process.ExitContext(), conn.Close(), logger)
			}
		}()

		// Create a caching client to the Auth Server. It is to reduce load on
		// the Auth Server.
		accessPoint, err := process.newLocalCacheForApps(conn.Client, []string{component})
		if err != nil {
			return trace.Wrap(err)
		}
		resp, err := accessPoint.GetClusterNetworkingConfig(process.ExitContext())
		if err != nil {
			return trace.Wrap(err)
		}

		// If this process connected through the web proxy, it will discover the
		// reverse tunnel address correctly and store it in the connector.
		//
		// If it was not, it is running in single process mode which is used for
		// development and demos. In that case, wait until all dependencies (like
		// auth and reverse tunnel server) are ready before starting.
		tunnelAddrResolver := conn.TunnelProxyResolver()
		if tunnelAddrResolver == nil {
			tunnelAddrResolver = process.SingleProcessModeResolver(resp.GetProxyListenerMode())

			// run the resolver. this will check configuration for errors.
			_, _, err := tunnelAddrResolver(process.ExitContext())
			if err != nil {
				return trace.Wrap(err)
			}

			// Block and wait for all dependencies to start before starting.
			logger.DebugContext(process.ExitContext(), "Waiting for application service dependencies to start.")
			process.waitForAppDepend()
			logger.DebugContext(process.ExitContext(), "Application service dependencies have started, continuing.")
		}

		clusterName := conn.ClusterName()

		// Start header dumping debugging application if requested.
		if process.Config.Apps.DebugApp {
			process.initDebugApp()

			// Block until the header dumper application is ready, and once it is,
			// figure out where it's running and add it to the list of applications.
			event, err := process.WaitForEvent(process.ExitContext(), DebugAppReady)
			if err != nil {
				return trace.Wrap(err)
			}
			server, ok := event.Payload.(*httptest.Server)
			if !ok {
				return trace.BadParameter("unexpected payload %T", event.Payload)
			}
			process.Config.Apps.Apps = append(process.Config.Apps.Apps, servicecfg.App{
				Name: "dumper",
				URI:  server.URL,
			})
		}

		// Loop over each application and create a server.
		var applications types.Apps
		for _, app := range process.Config.Apps.Apps {
			publicAddr, err := getPublicAddr(accessPoint, app)
			if err != nil {
				return trace.Wrap(err)
			}

			var rewrite *types.Rewrite
			if app.Rewrite != nil {
				rewrite = &types.Rewrite{
					Redirect:  app.Rewrite.Redirect,
					JWTClaims: app.Rewrite.JWTClaims,
				}
				for _, header := range app.Rewrite.Headers {
					rewrite.Headers = append(rewrite.Headers,
						&types.Header{
							Name:  header.Name,
							Value: header.Value,
						})
				}
			}

			var aws *types.AppAWS
			if app.AWS != nil {
				aws = &types.AppAWS{
					ExternalID: app.AWS.ExternalID,
				}
			}

			a, err := types.NewAppV3(types.Metadata{
				Name:        app.Name,
				Description: app.Description,
				Labels:      app.StaticLabels,
			}, types.AppSpecV3{
				URI:                app.URI,
				PublicAddr:         publicAddr,
				DynamicLabels:      types.LabelsToV2(app.DynamicLabels),
				InsecureSkipVerify: app.InsecureSkipVerify,
				Rewrite:            rewrite,
				AWS:                aws,
				Cloud:              app.Cloud,
				RequiredAppNames:   app.RequiredAppNames,
				CORS:               makeApplicationCORS(app.CORS),
			})
			if err != nil {
				return trace.Wrap(err)
			}

			applications = append(applications, a)
		}

		lockWatcher, err := services.NewLockWatcher(process.ExitContext(), services.LockWatcherConfig{
			ResourceWatcherConfig: services.ResourceWatcherConfig{
				Component: teleport.ComponentApp,
				Logger:    process.logger.With(teleport.ComponentKey, component),
				Client:    conn.Client,
			},
		})
		if err != nil {
			return trace.Wrap(err)
		}
		authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
			ClusterName: clusterName,
			AccessPoint: accessPoint,
			LockWatcher: lockWatcher,
			Logger:      process.log.WithField(teleport.ComponentKey, component),
			DeviceAuthorization: authz.DeviceAuthorizationOpts{
				// Ignore the global device_trust.mode toggle, but allow role-based
				// settings to be applied.
				DisableGlobalMode: true,
			},
			PermitCaching: process.Config.CachePolicy.Enabled,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		tlsConfig, err := conn.ServerTLSConfig(process.Config.CipherSuites)
		if err != nil {
			return trace.Wrap(err)
		}

		asyncEmitter, err := process.NewAsyncEmitter(conn.Client)
		if err != nil {
			return trace.Wrap(err)
		}
		defer func() {
			if !shouldSkipCleanup {
				warnOnErr(process.ExitContext(), asyncEmitter.Close(), logger)
			}
		}()

		proxyGetter := reversetunnel.NewConnectedProxyGetter()

		connMonitor, err := srv.NewConnectionMonitor(srv.ConnectionMonitorConfig{
			AccessPoint:         accessPoint,
			LockWatcher:         lockWatcher,
			Clock:               process.Config.Clock,
			ServerID:            process.Config.HostUUID,
			Emitter:             asyncEmitter,
			EmitterContext:      process.ExitContext(),
			Logger:              process.log,
			MonitorCloseChannel: process.Config.Apps.MonitorCloseChannel,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		connectionsHandler, err := app.NewConnectionsHandler(process.ExitContext(), &app.ConnectionsHandlerConfig{
			Clock:             process.Config.Clock,
			DataDir:           process.Config.DataDir,
			AuthClient:        conn.Client,
			AccessPoint:       accessPoint,
			Authorizer:        authorizer,
			TLSConfig:         tlsConfig,
			CipherSuites:      process.Config.CipherSuites,
			HostID:            process.Config.HostUUID,
			Emitter:           asyncEmitter,
			ConnectionMonitor: connMonitor,
			ServiceComponent:  teleport.ComponentApp,
			Logger:            logger,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		appServer, err := app.New(process.ExitContext(), &app.Config{
			Clock:                process.Config.Clock,
			AuthClient:           conn.Client,
			AccessPoint:          accessPoint,
			HostID:               process.Config.HostUUID,
			Hostname:             process.Config.Hostname,
			GetRotation:          process.GetRotation,
			Apps:                 applications,
			CloudLabels:          process.cloudLabels,
			ResourceMatchers:     process.Config.Apps.ResourceMatchers,
			OnHeartbeat:          process.OnHeartbeat(teleport.ComponentApp),
			ConnectedProxyGetter: proxyGetter,
			ConnectionsHandler:   connectionsHandler,
			InventoryHandle:      process.inventoryHandle,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		defer func() {
			if !shouldSkipCleanup {
				warnOnErr(process.ExitContext(), appServer.Close(), logger)
			}
		}()

		// Start the apps server. This starts the server, heartbeat (services.App),
		// and (dynamic) label update.
		if err := appServer.Start(process.ExitContext()); err != nil {
			return trace.Wrap(err)
		}

		// Create and start an agent pool.
		agentPool, err := reversetunnel.NewAgentPool(
			process.ExitContext(),
			reversetunnel.AgentPoolConfig{
				Component:            teleport.ComponentApp,
				HostUUID:             conn.HostID(),
				Resolver:             tunnelAddrResolver,
				Client:               conn.Client,
				Server:               appServer,
				AccessPoint:          accessPoint,
				AuthMethods:          conn.ClientAuthMethods(),
				Cluster:              clusterName,
				FIPS:                 process.Config.FIPS,
				ConnectedProxyGetter: proxyGetter,
			})
		if err != nil {
			return trace.Wrap(err)
		}
		err = agentPool.Start()
		if err != nil {
			return trace.Wrap(err)
		}

		process.BroadcastEvent(Event{Name: AppsReady, Payload: nil})
		logger.InfoContext(process.ExitContext(), "All applications successfully started.")

		// Cancel deferred cleanup actions, because we're going
		// to register an OnExit handler to take care of it
		shouldSkipCleanup = true

		// Execute this when process is asked to exit.
		process.OnExit("apps.stop", func(payload interface{}) {
			if payload == nil {
				logger.InfoContext(process.ExitContext(), "Shutting down immediately.")
				warnOnErr(process.ExitContext(), appServer.Close(), logger)
			} else {
				logger.InfoContext(process.ExitContext(), "Shutting down gracefully.")
				warnOnErr(process.ExitContext(), appServer.Shutdown(payloadContext(payload)), logger)
			}
			if asyncEmitter != nil {
				warnOnErr(process.ExitContext(), asyncEmitter.Close(), logger)
			}
			agentPool.Stop()
			warnOnErr(process.ExitContext(), asyncEmitter.Close(), logger)
			warnOnErr(process.ExitContext(), conn.Close(), logger)
			logger.InfoContext(process.ExitContext(), "Exited.")
		})

		// Block and wait while the server and agent pool are running.
		if err := appServer.Wait(); err != nil && !errors.Is(err, context.Canceled) {
			return trace.Wrap(err)
		}
		agentPool.Wait()
		return nil
	})
}

// newLocalCacheForApps returns new instance of access point configured for a remote proxy.
func (process *TeleportProcess) newLocalCacheForApps(clt authclient.ClientI, cacheName []string) (authclient.AppsAccessPoint, error) {
	// if caching is disabled, return access point
	if !process.Config.CachePolicy.Enabled {
		return clt, nil
	}

	cache, err := process.NewLocalCache(clt, cache.ForApps, cacheName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return authclient.NewAppsWrapper(clt, cache), nil
}

// makeApplicationCORS converts a servicecfg.CORS to a types.CORS.
func makeApplicationCORS(c *servicecfg.CORS) *types.CORSPolicy {
	if c == nil {
		return nil
	}

	return &types.CORSPolicy{
		AllowedOrigins:   c.AllowedOrigins,
		AllowedMethods:   c.AllowedMethods,
		AllowedHeaders:   c.AllowedHeaders,
		AllowCredentials: c.AllowCredentials,
		MaxAge:           uint32(c.MaxAge),
	}
}

// getPublicAddr waits for a proxy to be registered with Teleport.
func getPublicAddr(authClient authclient.ReadAppsAccessPoint, a servicecfg.App) (string, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case <-ticker.C:
			publicAddr, err := app.FindPublicAddr(authClient, a.PublicAddr, a.Name)

			if err == nil {
				return publicAddr, nil
			}
		case <-timeout.C:
			return "", trace.BadParameter("timed out waiting for proxy with public address")
		}
	}
}

// appDependEvents is a list of events that the application service depends on.
var appDependEvents = []string{
	AuthTLSReady,
	AuthIdentityEvent,
	ProxySSHReady,
	ProxyWebServerReady,
	ProxyReverseTunnelReady,
}

// waitForAppDepend waits until all dependencies for an application service
// are ready.
func (process *TeleportProcess) waitForAppDepend() {
	for _, event := range appDependEvents {
		_, err := process.WaitForEvent(process.ExitContext(), event)
		if err != nil {
			process.logger.DebugContext(process.ExitContext(), "Process is exiting.")
			break
		}
	}
}

// initDebugApp starts a debug server that dumpers request headers.
func (process *TeleportProcess) initDebugApp() {
	process.RegisterFunc("debug.app.service", func() error {
		server := httptest.NewServer(http.HandlerFunc(dumperHandler))
		process.BroadcastEvent(Event{Name: DebugAppReady, Payload: server})

		process.OnExit("debug.app.shutdown", func(payload interface{}) {
			server.Close()
			process.logger.InfoContext(process.ExitContext(), "Exited.")
		})
		return nil
	})
}

// SingleProcessModeResolver returns the reversetunnel.Resolver that should be used when running all components needed
// within the same process. It's used for development and demo purposes.
func (process *TeleportProcess) SingleProcessModeResolver(mode types.ProxyListenerMode) reversetunnelclient.Resolver {
	return func(context.Context) (*utils.NetAddr, types.ProxyListenerMode, error) {
		addr, ok := process.singleProcessMode(mode)
		if !ok {
			return nil, mode, trace.BadParameter(`failed to find reverse tunnel address, if running in single process mode, make sure "auth_service", "proxy_service", and "app_service" are all enabled`)
		}
		return addr, mode, nil
	}
}

// dumperHandler is an Application Access debugging application that will
// dump the headers of a request.
func dumperHandler(w http.ResponseWriter, r *http.Request) {
	requestDump, err := httputil.DumpRequest(r, true)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	randomBytes := make([]byte, 8)
	rand.Reader.Read(randomBytes)
	cookieValue := hex.EncodeToString(randomBytes)

	http.SetCookie(w, &http.Cookie{
		Name:     "dumper.session.cookie",
		Value:    cookieValue,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, string(requestDump))
}

// singleProcessMode returns true when running all components needed within
// the same process. It's used for development and demo purposes.
func (process *TeleportProcess) singleProcessMode(mode types.ProxyListenerMode) (*utils.NetAddr, bool) {
	if !process.Config.Proxy.Enabled || !process.Config.Auth.Enabled {
		return nil, false
	}
	if process.Config.Proxy.DisableReverseTunnel {
		return nil, false
	}

	if !process.Config.Proxy.DisableTLS && !process.Config.Proxy.DisableALPNSNIListener && mode == types.ProxyListenerMode_Multiplex {
		var addr utils.NetAddr
		switch {
		// Use the public address if available.
		case len(process.Config.Proxy.PublicAddrs) != 0:
			addr = process.Config.Proxy.PublicAddrs[0]

		// If WebAddress is unspecified "0.0.0.0" replace 0.0.0.0 with localhost since 0.0.0.0 is never a valid
		// principal (auth server explicitly removes it when issuing host certs) and when WebPort is used
		// in the single process mode to establish SSH reverse tunnel connection the host is validated against
		// the valid principal list.
		default:
			addr = process.Config.Proxy.WebAddr
			addr.Addr = utils.ReplaceUnspecifiedHost(&addr, defaults.HTTPListenPort)
		}

		// In case the address has "https" scheme for TLS Routing, make sure
		// "tcp" is used when dialing reverse tunnel.
		if addr.AddrNetwork == "https" {
			addr.AddrNetwork = "tcp"
		}
		return &addr, true
	}

	if len(process.Config.Proxy.TunnelPublicAddrs) == 0 {
		addr, err := utils.ParseHostPortAddr(string(teleport.PrincipalLocalhost), defaults.SSHProxyTunnelListenPort)
		if err != nil {
			return nil, false
		}
		return addr, true
	}
	return &process.Config.Proxy.TunnelPublicAddrs[0], true
}

// initDiagnosticService starts diagnostic service currently serving healthz
// and prometheus endpoints
func (process *TeleportProcess) initDiagnosticService() error {
	mux := http.NewServeMux()

	// support legacy metrics collection in the diagnostic service.
	// metrics will otherwise be served by the metrics service if it's enabled
	// in the config.
	if !process.Config.Metrics.Enabled {
		mux.Handle("/metrics", promhttp.Handler())
	}

	if process.Config.Debug {
		process.logger.InfoContext(process.ExitContext(), "Adding diagnostic debugging handlers. To connect with profiler, use `go tool pprof <listen_address>`.", "listen_address", process.Config.DiagnosticAddr.Addr)

		noWriteTimeout := func(h http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				rc := http.NewResponseController(w) //nolint:bodyclose // bodyclose gets really confused about NewResponseController
				if err := rc.SetWriteDeadline(time.Time{}); err == nil {
					// don't let the pprof handlers know about the WriteTimeout
					r = r.WithContext(context.WithValue(r.Context(), http.ServerContextKey, nil))
				}
				h(w, r)
			}
		}

		mux.HandleFunc("/debug/pprof/", noWriteTimeout(pprof.Index))
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", noWriteTimeout(pprof.Profile))
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", noWriteTimeout(pprof.Trace))
	}

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		roundtrip.ReplyJSON(w, http.StatusOK, map[string]interface{}{"status": "ok"})
	})

	logger := process.logger.With(teleport.ComponentKey, teleport.Component(teleport.ComponentDiagnostic, process.id))

	// Create a state machine that will process and update the internal state of
	// Teleport based off Events. Use this state machine to return return the
	// status from the /readyz endpoint.
	ps, err := newProcessState(process)
	if err != nil {
		return trace.Wrap(err)
	}

	process.RegisterFunc("readyz.monitor", func() error {
		// Start loop to monitor for events that are used to update Teleport state.
		ctx, cancel := context.WithCancel(process.GracefulExitContext())
		defer cancel()

		eventCh := make(chan Event, 1024)
		process.ListenForEvents(ctx, TeleportDegradedEvent, eventCh)
		process.ListenForEvents(ctx, TeleportOKEvent, eventCh)

		for {
			select {
			case e := <-eventCh:
				ps.update(e)
			case <-ctx.Done():
				logger.DebugContext(process.ExitContext(), "Teleport is exiting, returning.")
				return nil
			}
		}
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		switch ps.getState() {
		// 503
		case stateDegraded:
			roundtrip.ReplyJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
				"status": "teleport is in a degraded state, check logs for details",
			})
		// 400
		case stateRecovering:
			roundtrip.ReplyJSON(w, http.StatusBadRequest, map[string]interface{}{
				"status": "teleport is recovering from a degraded state, check logs for details",
			})
		case stateStarting:
			roundtrip.ReplyJSON(w, http.StatusBadRequest, map[string]interface{}{
				"status": "teleport is starting and hasn't joined the cluster yet",
			})
		// 200
		case stateOK:
			roundtrip.ReplyJSON(w, http.StatusOK, map[string]interface{}{
				"status": "ok",
			})
		}
	})

	listener, err := process.importOrCreateListener(ListenerDiagnostic, process.Config.DiagnosticAddr.Addr)
	if err != nil {
		return trace.Wrap(err)
	}
	warnOnErr(process.ExitContext(), process.closeImportedDescriptors(teleport.ComponentDiagnostic), logger)

	server := &http.Server{
		Handler:           mux,
		ReadTimeout:       apidefaults.DefaultIOTimeout,
		ReadHeaderTimeout: defaults.ReadHeadersTimeout,
		WriteTimeout:      apidefaults.DefaultIOTimeout,
		IdleTimeout:       apidefaults.DefaultIdleTimeout,
	}

	logger.InfoContext(process.ExitContext(), "Starting diagnostic service.", "listen_address", process.Config.DiagnosticAddr.Addr)

	muxListener, err := multiplexer.New(multiplexer.Config{
		Context:                        process.ExitContext(),
		Listener:                       listener,
		PROXYProtocolMode:              multiplexer.PROXYProtocolUnspecified,
		SuppressUnexpectedPROXYWarning: true,
		ID:                             teleport.Component(teleport.ComponentDiagnostic),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	process.RegisterFunc("diagnostic.service", func() error {
		listenerHTTP := muxListener.HTTP()
		go func() {
			if err := muxListener.Serve(); err != nil && !utils.IsOKNetworkError(err) {
				muxListener.Entry.WithError(err).Error("Mux encountered err serving")
			}
		}()

		if err := server.Serve(listenerHTTP); !errors.Is(err, http.ErrServerClosed) {
			logger.WarnContext(process.ExitContext(), "Diagnostic server exited with error.", "error", err)
		}
		return nil
	})

	process.OnExit("diagnostic.shutdown", func(payload interface{}) {
		warnOnErr(process.ExitContext(), muxListener.Close(), logger)
		if payload == nil {
			logger.InfoContext(process.ExitContext(), "Shutting down immediately.")
			warnOnErr(process.ExitContext(), server.Close(), logger)
		} else {
			logger.InfoContext(process.ExitContext(), "Shutting down gracefully.")
			ctx := payloadContext(payload)
			warnOnErr(process.ExitContext(), server.Shutdown(ctx), logger)
		}
		logger.InfoContext(process.ExitContext(), "Exited.")
	})

	return nil
}

func (process *TeleportProcess) initTracingService() error {
	logger := process.logger.With(teleport.ComponentKey, teleport.Component(teleport.ComponentTracing, process.id))
	logger.InfoContext(process.ExitContext(), "Initializing tracing provider and exporter.")

	attrs := []attribute.KeyValue{
		attribute.String(tracing.ProcessIDKey, process.id),
		attribute.String(tracing.HostnameKey, process.Config.Hostname),
		attribute.String(tracing.HostIDKey, process.Config.HostUUID),
	}

	traceConf, err := process.Config.Tracing.Config(attrs...)
	if err != nil {
		return trace.Wrap(err)
	}
	traceConf.Logger = process.logger.With(teleport.ComponentKey, teleport.Component(teleport.ComponentTracing, process.id))

	provider, err := tracing.NewTraceProvider(process.ExitContext(), *traceConf)
	if err != nil {
		return trace.Wrap(err)
	}
	process.TracingProvider = provider

	process.OnExit("tracing.shutdown", func(payload interface{}) {
		if payload == nil {
			logger.InfoContext(process.ExitContext(), "Shutting down immediately.")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			warnOnErr(process.ExitContext(), provider.Shutdown(ctx), logger)
		} else {
			logger.InfoContext(process.ExitContext(), "Shutting down gracefully.")
			ctx := payloadContext(payload)
			warnOnErr(process.ExitContext(), provider.Shutdown(ctx), logger)
		}
		process.logger.InfoContext(process.ExitContext(), "Exited.")
	})

	process.BroadcastEvent(Event{Name: TracingReady, Payload: nil})
	return nil
}

// initDebugService starts debug service serving endpoints used for
// troubleshooting the instance.
func (process *TeleportProcess) initDebugService() error {
	logger := process.logger.With(teleport.ComponentKey, teleport.Component(teleport.ComponentDebug, process.id))

	listener, err := process.importOrCreateListener(ListenerDebug, filepath.Join(process.Config.DataDir, teleport.DebugServiceSocketName))
	if err != nil {
		return trace.Wrap(err)
	}

	server := &http.Server{
		Handler:           debug.NewServeMux(logger, process.Config),
		ReadTimeout:       apidefaults.DefaultIOTimeout,
		ReadHeaderTimeout: defaults.ReadHeadersTimeout,
		// pprof endpoints support delta profiles and cpu and trace profiling
		// over time, both of which can be effectively unbounded in time; care
		// should be taken when adding more endpoints to this server, however,
		// and if necessary, a timeout can be either added to some more
		// sensitive endpoint, or the timeout can be removed from the more lax
		// ones
		WriteTimeout: 0,
		IdleTimeout:  apidefaults.DefaultIdleTimeout,
	}

	process.RegisterFunc("debug.service", func() error {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.WarnContext(process.ExitContext(), "Debug server exited with error.", "error", err)
		}
		return nil
	})
	warnOnErr(process.ExitContext(), process.closeImportedDescriptors(teleport.ComponentDebug), logger)

	process.OnExit("debug.shutdown", func(payload interface{}) {
		if payload == nil {
			logger.InfoContext(process.ExitContext(), "Shutting down immediately.")
			warnOnErr(process.ExitContext(), server.Close(), logger)
		} else {
			logger.InfoContext(process.ExitContext(), "Shutting down gracefully.")
			ctx := payloadContext(payload)
			warnOnErr(process.ExitContext(), server.Shutdown(ctx), logger)
		}
		logger.InfoContext(process.ExitContext(), "Exited.")
	})

	return nil
}

// registerExpectedServices sets up the instance role -> identity event mapping.
func (process *TeleportProcess) registerExpectedServices(cfg *servicecfg.Config) {
	// Register additional expected services for this Teleport instance.
	// Meant for enterprise support.
	for _, r := range cfg.AdditionalExpectedRoles {
		process.SetExpectedInstanceRole(r.Role, r.IdentityEvent)
	}

	if cfg.Auth.Enabled {
		process.SetExpectedInstanceRole(types.RoleAuth, AuthIdentityEvent)
	}

	if cfg.SSH.Enabled || cfg.OpenSSH.Enabled {
		process.SetExpectedInstanceRole(types.RoleNode, SSHIdentityEvent)
	}

	if cfg.Proxy.Enabled {
		process.SetExpectedInstanceRole(types.RoleProxy, ProxyIdentityEvent)
	}

	if cfg.Apps.Enabled {
		process.SetExpectedInstanceRole(types.RoleApp, AppsIdentityEvent)
	}
	if cfg.Discovery.Enabled {
		process.SetExpectedInstanceRole(types.RoleDiscovery, DiscoveryIdentityEvent)
	}
}

// WaitWithContext waits until all internal services stop.
func (process *TeleportProcess) WaitWithContext(ctx context.Context) {
	local, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()
		if err := process.Supervisor.Wait(); err != nil {
			process.logger.WarnContext(process.ExitContext(), "Error waiting for all services to complete", "error", err)
		}
	}()

	<-local.Done()
}

// GetID returns the process ID.
func (process *TeleportProcess) GetID() string {
	return process.id
}

// GetAuthServer returns the process' auth server
func (process *TeleportProcess) GetAuthServer() *auth.Server {
	return process.localAuth
}
