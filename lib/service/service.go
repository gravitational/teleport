/*
Copyright 2015-2021 Gravitational, Inc.

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

// Package service implements teleport running service, takes care
// of initialization, cleanup and shutdown procedures
package service

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
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

	awscredentials "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/google/uuid"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	kubeproto "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	transportpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/transport/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/agentless"
	"github.com/gravitational/teleport/lib/ai"
	"github.com/gravitational/teleport/lib/ai/embedding"
	"github.com/gravitational/teleport/lib/auditd"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/accesspoint"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/keygen"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/auth/storage"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/dynamo"
	"github.com/gravitational/teleport/lib/backend/etcdbk"
	"github.com/gravitational/teleport/lib/backend/firestore"
	"github.com/gravitational/teleport/lib/backend/kubernetes"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/backend/pgbk"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/cache"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/athena"
	"github.com/gravitational/teleport/lib/events/azsessions"
	"github.com/gravitational/teleport/lib/events/dynamoevents"
	"github.com/gravitational/teleport/lib/events/filesessions"
	"github.com/gravitational/teleport/lib/events/firestoreevents"
	"github.com/gravitational/teleport/lib/events/gcssessions"
	"github.com/gravitational/teleport/lib/events/pgevents"
	"github.com/gravitational/teleport/lib/events/s3sessions"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/integrations/externalauditstorage"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/joinserver"
	kubegrpc "github.com/gravitational/teleport/lib/kube/grpc"
	kubeproxy "github.com/gravitational/teleport/lib/kube/proxy"
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
	restricted "github.com/gravitational/teleport/lib/restrictedsession"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpnproxyauth "github.com/gravitational/teleport/lib/srv/alpnproxy/auth"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/srv/app"
	"github.com/gravitational/teleport/lib/srv/db"
	"github.com/gravitational/teleport/lib/srv/desktop"
	"github.com/gravitational/teleport/lib/srv/ingress"
	"github.com/gravitational/teleport/lib/srv/regular"
	"github.com/gravitational/teleport/lib/srv/transport/transportv1"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/system"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/cert"
	vc "github.com/gravitational/teleport/lib/versioncontrol"
	"github.com/gravitational/teleport/lib/versioncontrol/endpoint"
	uw "github.com/gravitational/teleport/lib/versioncontrol/upgradewindow"
	"github.com/gravitational/teleport/lib/web"
)

const (
	// AuthIdentityEvent is generated when the Auth Servers identity has been
	// initialized in the backend.
	AuthIdentityEvent = "AuthIdentity"

	// InstanceIdentityEvent is generated by the supervisor when the instance-level
	// identity has been registered with the Auth server.
	InstanceIdentityEvent = "InstanceIdentity"

	// ProxyIdentityEvent is generated by the supervisor when the proxy's
	// identity has been registered with the Auth Server.
	ProxyIdentityEvent = "ProxyIdentity"

	// SSHIdentityEvent is generated when node's identity has been registered
	// with the Auth Server.
	SSHIdentityEvent = "SSHIdentity"

	// KubeIdentityEvent is generated by the supervisor when the kubernetes
	// service's identity has been registered with the Auth Server.
	KubeIdentityEvent = "KubeIdentity"

	// AppsIdentityEvent is generated when the identity of the application proxy
	// service has been registered with the Auth Server.
	AppsIdentityEvent = "AppsIdentity"

	// DatabasesIdentityEvent is generated when the identity of the database
	// proxy service has been registered with the auth server.
	DatabasesIdentityEvent = "DatabasesIdentity"

	// WindowsDesktopIdentityEvent is generated by the supervisor when the
	// windows desktop service's identity has been registered with the Auth
	// Server.
	WindowsDesktopIdentityEvent = "WindowsDesktopIdentity"

	// DiscoveryIdentityEvent is generated when the identity of the
	DiscoveryIdentityEvent = "DiscoveryIdentityEvent"

	// AuthTLSReady is generated when the Auth Server has initialized the
	// TLS Mutual Auth endpoint and is ready to start accepting connections.
	AuthTLSReady = "AuthTLSReady"

	// ProxyWebServerReady is generated when the proxy has initialized the web
	// server and is ready to start accepting connections.
	ProxyWebServerReady = "ProxyWebServerReady"

	// ProxyReverseTunnelReady is generated when the proxy has initialized the
	// reverse tunnel server and is ready to start accepting connections.
	ProxyReverseTunnelReady = "ProxyReverseTunnelReady"

	// DebugAppReady is generated when the debugging application has been started
	// and is ready to serve requests.
	DebugAppReady = "DebugAppReady"

	// ProxyAgentPoolReady is generated when the proxy has initialized the
	// remote cluster watcher (to spawn reverse tunnels) and is ready to start
	// accepting connections.
	ProxyAgentPoolReady = "ProxyAgentPoolReady"

	// ProxySSHReady is generated when the proxy has initialized a SSH server
	// and is ready to start accepting connections.
	ProxySSHReady = "ProxySSHReady"

	// NodeSSHReady is generated when the Teleport node has initialized a SSH server
	// and is ready to start accepting SSH connections.
	NodeSSHReady = "NodeReady"

	// KubernetesReady is generated when the kubernetes service has been initialized.
	KubernetesReady = "KubernetesReady"

	// AppsReady is generated when the Teleport app proxy service is ready to
	// start accepting connections.
	AppsReady = "AppsReady"

	// DatabasesReady is generated when the Teleport database proxy service
	// is ready to start accepting connections.
	DatabasesReady = "DatabasesReady"

	// MetricsReady is generated when the Teleport metrics service is ready to
	// start accepting connections.
	MetricsReady = "MetricsReady"

	// WindowsDesktopReady is generated when the Teleport windows desktop
	// service is ready to start accepting connections.
	WindowsDesktopReady = "WindowsDesktopReady"

	// TracingReady is generated when the Teleport tracing service is ready to
	// start exporting spans.
	TracingReady = "TracingReady"

	// InstanceReady is generated when the teleport instance control handle has
	// been set up.
	InstanceReady = "InstanceReady"

	// DiscoveryReady is generated when the Teleport discovery service
	// is ready to start accepting connections.
	DiscoveryReady = "DiscoveryReady"

	// TeleportExitEvent is generated when the Teleport process begins closing
	// all listening sockets and exiting.
	TeleportExitEvent = "TeleportExit"

	// TeleportReloadEvent is generated to trigger in-process teleport
	// service reload - all servers and clients will be re-created
	// in a graceful way.
	TeleportReloadEvent = "TeleportReload"

	// TeleportPhaseChangeEvent is generated to indidate that teleport
	// CA rotation phase has been updated, used in tests
	TeleportPhaseChangeEvent = "TeleportPhaseChange"

	// TeleportReadyEvent is generated to signal that all teleport
	// internal components have started successfully.
	TeleportReadyEvent = "TeleportReady"

	// ServiceExitedWithErrorEvent is emitted whenever a service
	// has exited with an error, the payload includes the error
	ServiceExitedWithErrorEvent = "ServiceExitedWithError"

	// TeleportDegradedEvent is emitted whenever a service is operating in a
	// degraded manner.
	TeleportDegradedEvent = "TeleportDegraded"

	// TeleportOKEvent is emitted whenever a service is operating normally.
	TeleportOKEvent = "TeleportOKEvent"
)

const (
	// embeddingInitialDelay is the time to wait before the first embedding
	// routine is started.
	embeddingInitialDelay = 10 * time.Second
	// embeddingPeriod is the time between two embedding routines.
	// A seventh jitter is applied on the period.
	embeddingPeriod = 20 * time.Minute
)

// Connector has all resources process needs to connect to other parts of the
// cluster: client and identity.
type Connector struct {
	// ClientIdentity is the identity to be used in internal cluster
	// clients to the auth service.
	ClientIdentity *state.Identity

	// ServerIdentity is the identity to be used in servers - serving SSH
	// and x509 certificates to clients.
	ServerIdentity *state.Identity

	// Client is authenticated client with credentials from ClientIdentity.
	Client *authclient.Client

	// ReusedClient, if true, indicates that the client reference is owned by
	// a different connector and should not be closed.
	ReusedClient bool
}

// TunnelProxyResolver if non-nil, indicates that the client is connected to the Auth Server
// through the reverse SSH tunnel proxy
func (c *Connector) TunnelProxyResolver() reversetunnelclient.Resolver {
	if c.Client == nil || c.Client.Dialer() == nil {
		return nil
	}

	switch dialer := c.Client.Dialer().(type) {
	case *reversetunnelclient.TunnelAuthDialer:
		return dialer.Resolver
	default:
		return nil
	}
}

// UseTunnel indicates if the client is connected directly to the Auth Server
// (false) or through the proxy (true).
func (c *Connector) UseTunnel() bool {
	return c.TunnelProxyResolver() != nil
}

// Close closes resources associated with connector
func (c *Connector) Close() error {
	if c.Client != nil && !c.ReusedClient {
		return c.Client.Close()
	}
	return nil
}

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

	// identities of this process (credentials to auth sever, basically)
	Identities map[types.SystemRole]*state.Identity

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

	// forkedPIDs is a collection of a teleport processes forked
	// during restart used to collect their status in case if the
	// child process crashed.
	forkedPIDs []int

	// storage is a server local storage
	storage *storage.ProcessStorage

	// id is a process id - used to identify different processes
	// during in-process reloads.
	id string

	// log is a process-local log entry.
	log logrus.FieldLogger

	// keyPairs holds private/public key pairs used
	// to get signed host certificates from auth server
	keyPairs map[keyPairKey]KeyPair
	// keyMutex is a mutex to serialize key generation
	keyMutex sync.Mutex

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

type keyPairKey struct {
	role   types.SystemRole
	reason string
}

// processIndex is an internal process index
// to help differentiate between two different teleport processes
// during in-process reload.
var processID int32

func nextProcessID() int32 {
	return atomic.AddInt32(&processID, 1)
}

// GetAuthServer returns the process' auth server
func (process *TeleportProcess) GetAuthServer() *auth.Server {
	return process.localAuth
}

// GetAuditLog returns the process' audit log
func (process *TeleportProcess) GetAuditLog() events.AuditLogSessionStreamer {
	return process.auditLog
}

// GetBackend returns the process' backend
func (process *TeleportProcess) GetBackend() backend.Backend {
	return process.backend
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

// addConnector adds connector to registered connectors list,
// it will overwrite the connector for the same role
func (process *TeleportProcess) addConnector(connector *Connector) {
	process.Lock()
	defer process.Unlock()

	process.connectors[connector.ClientIdentity.ID.Role] = connector
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
func (process *TeleportProcess) WaitForConnector(identityEvent string, log logrus.FieldLogger) (*Connector, error) {
	event, err := process.WaitForEvent(process.ExitContext(), identityEvent)
	if err != nil {
		if log != nil {
			log.Debugf("Process is exiting.")
		}
		return nil, nil
	}
	if log != nil {
		log.Debugf("Received event %q.", event.Name)
	}

	conn, ok := (event.Payload).(*Connector)
	if !ok {
		return nil, trace.BadParameter("unsupported connector type: %T", event.Payload)
	}

	return conn, nil
}

// GetID returns the process ID.
func (process *TeleportProcess) GetID() string {
	return process.id
}

func (process *TeleportProcess) setClusterFeatures(features *proto.Features) {
	process.Lock()
	defer process.Unlock()

	if features != nil {
		process.clusterFeatures = *features
	}
}

// GetClusterFeatures returns the cluster features.
func (process *TeleportProcess) GetClusterFeatures() proto.Features {
	process.Lock()
	defer process.Unlock()

	return process.clusterFeatures
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

// getAuthSubjectiveAddr accesses the peer address reported by the auth server
// during the most recent ping. May be empty.
func (process *TeleportProcess) getAuthSubjectiveAddr() string {
	process.Lock()
	defer process.Unlock()
	return process.authSubjectiveAddr
}

// GetIdentity returns the process identity (credentials to the auth server) for a given
// teleport Role. A teleport process can have any combination of 3 roles: auth, node, proxy
// and they have their own identities
func (process *TeleportProcess) GetIdentity(role types.SystemRole) (i *state.Identity, err error) {
	var found bool

	process.Lock()
	defer process.Unlock()

	i, found = process.Identities[role]
	if found {
		return i, nil
	}
	i, err = process.storage.ReadIdentity(state.IdentityCurrent, role)
	id := state.IdentityID{
		Role:     role,
		HostUUID: process.Config.HostUUID,
		NodeName: process.Config.Hostname,
	}
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
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
		} else {
			// try to locate static identity provided in the file
			i, err = process.findStaticIdentity(id)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			process.log.Infof("Found static identity %v in the config file, writing to disk.", &id)
			if err = process.storage.WriteIdentity(state.IdentityCurrent, *i); err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}
	process.Identities[role] = i
	return i, nil
}

// Process is a interface for processes
type Process interface {
	// Closer closes all resources used by the process
	io.Closer
	// Start starts the process in a non-blocking way
	Start() error
	// WaitForSignals waits for and handles system process signals.
	WaitForSignals(context.Context) error
	// ExportFileDescriptors exports service listeners
	// file descriptors used by the process.
	ExportFileDescriptors() ([]*servicecfg.FileDescriptor, error)
	// Shutdown starts graceful shutdown of the process,
	// blocks until all resources are freed and go-routines are
	// shut down.
	Shutdown(context.Context)
	// WaitForEvent waits for one event with the specified name (returns the
	// latest such event if at least one has been broadcasted already, ignoring
	// the context). Returns an error if the context is canceled before an event
	// is received.
	WaitForEvent(ctx context.Context, name string) (Event, error)
	// WaitWithContext waits for the service to stop. This is a blocking
	// function.
	WaitWithContext(ctx context.Context)
}

// NewProcess is a function that creates new teleport from config
type NewProcess func(cfg *servicecfg.Config) (Process, error)

func newTeleportProcess(cfg *servicecfg.Config) (Process, error) {
	return NewTeleport(cfg)
}

// Run starts teleport processes, waits for signals
// and handles internal process reloads.
func Run(ctx context.Context, cfg servicecfg.Config, newTeleport NewProcess) error {
	if newTeleport == nil {
		newTeleport = newTeleportProcess
	}
	copyCfg := cfg
	srv, err := newTeleport(&copyCfg)
	if err != nil {
		return trace.Wrap(err, "initialization failed")
	}
	if srv == nil {
		return trace.BadParameter("process has returned nil server")
	}
	if err := srv.Start(); err != nil {
		return trace.Wrap(err, "startup failed")
	}
	// Wait and reload until called exit.
	for {
		srv, err = waitAndReload(ctx, cfg, srv, newTeleport)
		if err != nil {
			// This error means that was a clean shutdown
			// and no reload is necessary.
			if err == ErrTeleportExited {
				return nil
			}
			return trace.Wrap(err)
		}
	}
}

func waitAndReload(ctx context.Context, cfg servicecfg.Config, srv Process, newTeleport NewProcess) (Process, error) {
	err := srv.WaitForSignals(ctx)
	if err == nil {
		return nil, ErrTeleportExited
	}
	if err != ErrTeleportReloading {
		return nil, trace.Wrap(err)
	}
	cfg.Log.Infof("Started in-process service reload.")
	fileDescriptors, err := srv.ExportFileDescriptors()
	if err != nil {
		warnOnErr(srv.Close(), cfg.Log)
		return nil, trace.Wrap(err)
	}
	newCfg := cfg
	newCfg.FileDescriptors = fileDescriptors
	newSrv, err := newTeleport(&newCfg)
	if err != nil {
		warnOnErr(srv.Close(), cfg.Log)
		return nil, trace.Wrap(err, "failed to create a new service")
	}
	cfg.Log.Infof("Created new process.")
	if err := newSrv.Start(); err != nil {
		warnOnErr(srv.Close(), cfg.Log)
		return nil, trace.Wrap(err, "failed to start a new service")
	}
	// Wait for the new server to report that it has started
	// before shutting down the old one.
	startTimeoutCtx, startCancel := context.WithTimeout(ctx, signalPipeTimeout)
	defer startCancel()
	if _, err := newSrv.WaitForEvent(startTimeoutCtx, TeleportReadyEvent); err != nil {
		warnOnErr(newSrv.Close(), cfg.Log)
		warnOnErr(srv.Close(), cfg.Log)
		return nil, trace.BadParameter("the new service has failed to start")
	}
	cfg.Log.Infof("New service has started successfully.")
	shutdownTimeout := cfg.Testing.ShutdownTimeout
	if shutdownTimeout == 0 {
		// The default shutdown timeout is very generous to avoid disrupting
		// longer running connections.
		shutdownTimeout = defaults.DefaultGracefulShutdownTimeout
	}
	cfg.Log.Infof("Shutting down the old service with timeout %v.", shutdownTimeout)
	// After the new process has started, initiate the graceful shutdown of the old process
	// new process could have generated connections to the new process's server
	// so not all connections can be kept forever.
	timeoutCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()
	srv.Shutdown(services.ProcessReloadContext(timeoutCtx))
	if timeoutCtx.Err() == context.DeadlineExceeded {
		// The new service can start initiating connections to the old service
		// keeping it from shutting down gracefully, or some external
		// connections can keep hanging the old auth service and prevent
		// the services from shutting down, so abort the graceful way
		// after some time to keep going.
		cfg.Log.Infof("Some connections to the old service were aborted after timeout of %v.", shutdownTimeout)
		// Make sure that all parts of the service have exited, this function
		// can not allow execution to continue if the shutdown is not complete,
		// otherwise subsequent Run executions will hold system resources in case
		// if old versions of the service are not exiting completely.
		timeoutCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
		defer cancel()
		srv.WaitWithContext(timeoutCtx)
		if timeoutCtx.Err() == context.DeadlineExceeded {
			return nil, trace.BadParameter("the old service has failed to exit.")
		}
	} else {
		cfg.Log.Infof("The old service was successfully shut down gracefully.")
	}
	return newSrv, nil
}

// NewTeleport takes the daemon configuration, instantiates all required services
// and starts them under a supervisor, returning the supervisor object.
func NewTeleport(cfg *servicecfg.Config) (*TeleportProcess, error) {
	var err error

	// auth and proxy benefit from precomputing keys since they can experience spikes in key
	// generation due to web session creation and recorded session creation respectively.
	// for all other agents precomputing keys consumes excess resources.
	if cfg.Auth.Enabled || cfg.Proxy.Enabled {
		native.PrecomputeKeys()
	}

	// Before we do anything reset the SIGINT handler back to the default.
	system.ResetInterruptSignalHandler()

	// Validate the config before accessing it.
	if err := servicecfg.ValidateConfig(cfg); err != nil {
		return nil, trace.Wrap(err, "configuration error")
	}

	processID := fmt.Sprintf("%v", nextProcessID())
	cfg.Log = utils.WrapLogger(cfg.Log.WithFields(logrus.Fields{
		trace.Component: teleport.Component(teleport.ComponentProcess, processID),
		"pid":           fmt.Sprintf("%v.%v", os.Getpid(), processID),
	}))

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
				cfg.Log.Errorf("Teleport does not have permission to write to: %v. Ensure that you are running as a user with appropriate permissions.", cfg.DataDir)
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

	var kubeBackend kubernetesBackend
	// If running in a Kubernetes Pod we must init the backend storage for `host_uuid` storage/retrieval.
	if kubernetes.InKubeCluster() {
		kubeBackend, err = kubernetes.New()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Load `host_uuid` from different storages. If this process is running in a Kubernetes Cluster,
	// readOrGenerateHostID will try to read the `host_uuid` from the Kubernetes Secret. If the
	// key is empty or if not running in a Kubernetes Cluster, it will read the
	// `host_uuid` from local data directory.
	// If no host id is available, it will generate a new host id and persist it to available storages.
	if err := readOrGenerateHostID(supervisor.ExitContext(), cfg, kubeBackend); err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = uuid.Parse(cfg.HostUUID)
	if err != nil && !aws.IsEC2NodeID(cfg.HostUUID) {
		cfg.Log.Warnf("Host UUID %q is not a true UUID (not eligible for UUID-based proxying)", cfg.HostUUID)
	}

	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	if cfg.PluginRegistry == nil {
		cfg.PluginRegistry = plugin.NewRegistry()
	}

	var cloudLabels labels.Importer

	// Check if we're on a cloud instance, and if we should override the node's hostname.
	imClient := cfg.InstanceMetadataClient
	if imClient == nil {
		imClient, err = cloud.DiscoverInstanceMetadata(supervisor.ExitContext())
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}

	if imClient != nil && imClient.GetType() != types.InstanceMetadataTypeDisabled {
		cloudHostname, err := imClient.GetHostname(supervisor.ExitContext())
		if err == nil {
			cloudHostname = strings.ReplaceAll(cloudHostname, " ", "_")
			if utils.IsValidHostname(cloudHostname) {
				cfg.Log.Infof("Found %q tag in cloud instance. Using %q as hostname.", types.CloudHostnameTag, cloudHostname)
				cfg.Hostname = cloudHostname

				// cloudHostname exists but is not a valid hostname.
			} else if cloudHostname != "" {
				cfg.Log.Infof("Found %q tag in cloud instance, but %q is not a valid hostname.", types.CloudHostnameTag, cloudHostname)
			}
		} else if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}

		cloudLabels, err = labels.NewCloudImporter(supervisor.ExitContext(), &labels.CloudConfig{
			Client: imClient,
			Clock:  cfg.Clock,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if cloudLabels != nil {
		cloudLabels.Start(supervisor.ExitContext())
	}

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
		Identities:             make(map[types.SystemRole]*state.Identity),
		connectors:             make(map[types.SystemRole]*Connector),
		importedDescriptors:    cfg.FileDescriptors,
		storage:                storage,
		id:                     processID,
		log:                    cfg.Log,
		keyPairs:               make(map[keyPairKey]KeyPair),
		cloudLabels:            cloudLabels,
		TracingProvider:        tracing.NoopProvider(),
	}

	process.registerExpectedServices(cfg)

	process.log = cfg.Log.WithFields(logrus.Fields{
		trace.Component: teleport.Component(teleport.ComponentProcess, process.id),
	})

	// if user started auth and another service (without providing the auth address for
	// that service, the address of the in-process auth will be used
	if process.Config.Auth.Enabled && len(process.Config.AuthServerAddresses()) == 0 {
		process.Config.SetAuthServerAddress(process.Config.Auth.ListenAddr)
	}

	if len(process.Config.AuthServerAddresses()) != 0 && process.Config.AuthServerAddresses()[0].Port(0) == 0 {
		// port appears undefined, attempt early listener creation so that we can get the real port
		listener, err := process.importOrCreateListener(ListenerAuth, process.Config.Auth.ListenAddr.Addr)
		if err == nil {
			process.Config.SetAuthServerAddress(utils.FromAddr(listener.Addr()))
		}
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

	// Instances deployed using the AWS OIDC integration are automatically updated
	// by the proxy. The instance heartbeat should properly reflect that.
	externalUpgrader := upgraderKind
	if externalUpgrader == "" && os.Getenv(types.InstallMethodAWSOIDCDeployServiceEnvVar) == "true" {
		externalUpgrader = types.OriginIntegrationAWSOIDC
	}

	// note: we must create the inventory handle *after* registerExpectedServices because that function determines
	// the list of services (instance roles) to be included in the heartbeat.
	process.inventoryHandle = inventory.NewDownstreamHandle(process.makeInventoryControlStreamWhenReady, proto.UpstreamInventoryHello{
		ServerID:                cfg.HostUUID,
		Version:                 teleport.Version,
		Services:                process.getInstanceRoles(),
		Hostname:                cfg.Hostname,
		ExternalUpgrader:        externalUpgrader,
		ExternalUpgraderVersion: vc.Normalize(upgraderVersion),
	})

	process.inventoryHandle.RegisterPingHandler(func(sender inventory.DownstreamSender, ping proto.DownstreamInventoryPing) {
		process.log.Infof("Handling incoming inventory ping (id=%d).", ping.ID)
		err := sender.Send(process.ExitContext(), proto.UpstreamInventoryPong{
			ID: ping.ID,
		})
		if err != nil {
			process.log.Warnf("Failed to respond to inventory ping (id=%d): %v", ping.ID, err)
		}
	})

	// if an external upgrader is defined, we need to set up an appropriate upgrade window exporter.
	if upgraderKind != "" {
		if process.Config.Auth.Enabled || process.Config.Proxy.Enabled {
			process.log.Warnf("Use of external upgraders on control-plane instances is not recommended.")
		}

		if upgraderKind == "unit" {
			process.RegisterFunc("autoupdates.endpoint.export", func() error {
				conn, err := waitForInstanceConnector(process, process.logger)
				if err != nil {
					return trace.Wrap(err)
				}
				if conn == nil {
					return trace.BadParameter("process exiting and Instance connector never became available")
				}

				resp, err := conn.Client.Ping(process.ExitContext())
				if err != nil {
					return trace.Wrap(err)
				}
				if !resp.GetServerFeatures().GetCloud() {
					return nil
				}

				if err := endpoint.Export(process.ExitContext(), resolverAddr.String()); err != nil {
					process.logger.WarnContext(process.ExitContext(),
						"Failed to export and validate autoupdates endpoint.",
						"addr", resolverAddr.String(),
						"error", err)
					return trace.Wrap(err)
				}
				process.logger.InfoContext(process.ExitContext(), "Exported autoupdates endpoint.", "addr", resolverAddr.String())
				return nil
			})
		}

		driver, err := uw.NewDriver(upgraderKind)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		exporter, err := uw.NewExporter(uw.ExporterConfig[inventory.DownstreamSender]{
			Driver:                   driver,
			ExportFunc:               process.exportUpgradeWindows,
			AuthConnectivitySentinel: process.inventoryHandle.Sender(),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		process.RegisterCriticalFunc("upgradeewindow.export", exporter.Run)
		process.OnExit("upgradewindow.export.stop", func(_ interface{}) {
			exporter.Close()
		})

		process.log.Infof("Configured upgrade window exporter for external upgrader. kind=%s", upgraderKind)
	}

	if process.Config.Proxy.Enabled {
		process.RegisterFunc("update.aws-oidc.deploy.service", process.initAWSOIDCDeployServiceUpdater)
	}

	serviceStarted := false

	if !cfg.DiagnosticAddr.IsEmpty() {
		if err := process.initDiagnosticService(); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		warnOnErr(process.closeImportedDescriptors(teleport.ComponentDiagnostic), process.log)
	}

	if cfg.Tracing.Enabled {
		if err := process.initTracingService(); err != nil {
			return nil, trace.Wrap(err)
		}
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
	if process.shouldInitDatabases() {
		eventMapping.In = append(eventMapping.In, DatabasesReady)
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
	if process.shouldInitDiscovery() {
		eventMapping.In = append(eventMapping.In, DiscoveryReady)
	}

	process.RegisterEventMapping(eventMapping)

	if cfg.Auth.Enabled {
		if err := process.initAuthService(); err != nil {
			return nil, trace.Wrap(err)
		}
		serviceStarted = true
	} else {
		warnOnErr(process.closeImportedDescriptors(teleport.ComponentAuth), process.log)
	}

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
		warnOnErr(process.closeImportedDescriptors(teleport.ComponentNode), process.log)
	}

	if cfg.Proxy.Enabled {
		if err := process.initProxy(); err != nil {
			return nil, err
		}
		serviceStarted = true
	} else {
		warnOnErr(process.closeImportedDescriptors(teleport.ComponentProxy), process.log)
	}

	if cfg.Kube.Enabled {
		process.initKubernetes()
		serviceStarted = true
	} else {
		warnOnErr(process.closeImportedDescriptors(teleport.ComponentKube), process.log)
	}

	// If this process is proxying applications, start application access server.
	if cfg.Apps.Enabled {
		process.initApps()
		serviceStarted = true
	} else {
		warnOnErr(process.closeImportedDescriptors(teleport.ComponentApp), process.log)
	}

	if process.shouldInitDatabases() {
		process.initDatabases()
		serviceStarted = true
	} else {
		if process.Config.Databases.Enabled {
			process.log.Warn("Database service is enabled with empty configuration, skipping initialization")
		}
		warnOnErr(process.closeImportedDescriptors(teleport.ComponentDatabase), process.log)
	}

	if cfg.Metrics.Enabled {
		process.initMetricsService()
		serviceStarted = true
	} else {
		warnOnErr(process.closeImportedDescriptors(teleport.ComponentMetrics), process.log)
	}

	if cfg.WindowsDesktop.Enabled {
		process.initWindowsDesktopService()
		serviceStarted = true
	} else {
		warnOnErr(process.closeImportedDescriptors(teleport.ComponentWindowsDesktop), process.log)
	}

	if process.shouldInitDiscovery() {
		process.initDiscovery()
		serviceStarted = true
	} else {
		if process.Config.Discovery.Enabled {
			process.log.Warn("Discovery service is enabled with empty configuration, skipping initialization")
		}
		warnOnErr(process.closeImportedDescriptors(teleport.ComponentDiscovery), process.log)
	}

	if process.enterpriseServicesEnabledWithCommunityBuild() {
		var services []string
		if process.Config.Okta.Enabled {
			services = append(services, "okta")
		}
		if process.Config.Jamf.Enabled() {
			services = append(services, "jamf")
		}
		return nil, trace.BadParameter("Attempting to use enterprise only services %v, with a community teleport build", services)
	}

	// Enterprise services will be handled by the enterprise binary. We'll let these set serviceStarted
	// to true and let the enterprise binary error if need be.
	if process.enterpriseServicesEnabled() {
		serviceStarted = true
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
		f, err := os.OpenFile(cfg.PIDFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o666)
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}
		_, err = fmt.Fprintf(f, "%v", os.Getpid())
		if err = trace.NewAggregate(err, f.Close()); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// notify parent process that this process has started
	go process.notifyParent()

	return process, nil
}

// enterpriseServicesEnabled will return true if any enterprise services are enabled.
func (process *TeleportProcess) enterpriseServicesEnabled() bool {
	return modules.GetModules().BuildType() == modules.BuildEnterprise &&
		(process.Config.Okta.Enabled || process.Config.Jamf.Enabled())
}

// enterpriseServicesEnabledWithCommunityBuild will return true if any
// enterprise services are enabled with an OSS teleport build.
func (process *TeleportProcess) enterpriseServicesEnabledWithCommunityBuild() bool {
	return modules.GetModules().BuildType() == modules.BuildOSS &&
		(process.Config.Okta.Enabled || process.Config.Jamf.Enabled())
}

// notifyParent notifies parent process that this process has started
// by writing to in-memory pipe used by communication channel.
func (process *TeleportProcess) notifyParent() {
	signalPipe, err := process.importSignalPipe()
	if err != nil {
		if !trace.IsNotFound(err) {
			process.log.Warningf("Failed to import signal pipe")
		}
		process.log.Debugf("No signal pipe to import, must be first Teleport process.")
		return
	}
	defer signalPipe.Close()

	ctx, cancel := context.WithTimeout(process.ExitContext(), signalPipeTimeout)
	defer cancel()

	if _, err := process.WaitForEvent(ctx, TeleportReadyEvent); err != nil {
		process.log.Errorf("Timeout waiting for a forked process to start: %v. Initiating self-shutdown.", ctx.Err())
		if err := process.Close(); err != nil {
			process.log.Warningf("Failed to shutdown process: %v.", err)
		}
		return
	}
	process.log.Infof("New service has started successfully.")

	if err := process.writeToSignalPipe(signalPipe, fmt.Sprintf("Process %v has started.", os.Getpid())); err != nil {
		process.log.Warningf("Failed to write to signal pipe: %v", err)
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

// exportUpgradeWindow is a helper for calling ExportUpgradeWindows either on the local in-memory auth server, or via the instance client, depending on
// which is available.
func (process *TeleportProcess) exportUpgradeWindows(ctx context.Context, req proto.ExportUpgradeWindowsRequest) (proto.ExportUpgradeWindowsResponse, error) {
	if auth := process.getLocalAuth(); auth != nil {
		return auth.ExportUpgradeWindows(ctx, req)
	}

	clt := process.getInstanceClient()
	if clt == nil {
		return proto.ExportUpgradeWindowsResponse{}, trace.Errorf("instance client not yet initialized")
	}
	return clt.ExportUpgradeWindows(ctx, req)
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

// initAuthUploadHandler initializes the auth server's upload handler based upon the configuration.
// When configured to store session recordings in external storage, this will be an API client for
// cloud-provider storage. Otherwise a local file-based handler is used which stores the recordings
// on disk.
func initAuthUploadHandler(ctx context.Context, auditConfig types.ClusterAuditConfig, dataDir string, externalAuditStorage *externalauditstorage.Configurator) (events.MultipartHandler, error) {
	uriString := auditConfig.AuditSessionsURI()
	if externalAuditStorage.IsUsed() {
		uriString = externalAuditStorage.GetSpec().SessionRecordingsURI
	}
	if uriString == "" {
		recordsDir := filepath.Join(dataDir, events.RecordsDir)
		if err := os.MkdirAll(recordsDir, teleport.SharedDirMode); err != nil {
			return nil, trace.ConvertSystemError(err)
		}
		handler, err := filesessions.NewHandler(filesessions.Config{
			Directory: recordsDir,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return handler, nil
	}
	uri, err := apiutils.ParseSessionsURI(uriString)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch uri.Scheme {
	case teleport.SchemeGCS:
		config := gcssessions.Config{}
		if err := config.SetFromURL(uri); err != nil {
			return nil, trace.Wrap(err)
		}
		handler, err := gcssessions.DefaultNewHandler(ctx, config)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return handler, nil
	case teleport.SchemeS3:
		config := s3sessions.Config{
			UseFIPSEndpoint: auditConfig.GetUseFIPSEndpoint(),
		}
		if externalAuditStorage.IsUsed() {
			config.Credentials = awscredentials.NewCredentials(externalAuditStorage.CredentialsProviderSDKV1())
		}
		if err := config.SetFromURL(uri, auditConfig.Region()); err != nil {
			return nil, trace.Wrap(err)
		}

		var handler events.MultipartHandler
		handler, err = s3sessions.NewHandler(ctx, config)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if externalAuditStorage.IsUsed() {
			handler = externalAuditStorage.ErrorCounter.WrapSessionHandler(handler)
		}
		return handler, nil
	case teleport.SchemeAZBlob, teleport.SchemeAZBlobHTTP:
		var config azsessions.Config
		if err := config.SetFromURL(uri); err != nil {
			return nil, trace.Wrap(err)
		}
		handler, err := azsessions.NewHandler(ctx, config)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return handler, nil
	case teleport.SchemeFile:
		if err := os.MkdirAll(uri.Path, teleport.SharedDirMode); err != nil {
			return nil, trace.ConvertSystemError(err)
		}
		handler, err := filesessions.NewHandler(filesessions.Config{
			Directory: uri.Path,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return handler, nil
	default:
		return nil, trace.BadParameter(
			"unsupported scheme for audit_sessions_uri: %q, currently supported schemes are: %v",
			uri.Scheme, strings.Join([]string{
				teleport.SchemeS3, teleport.SchemeGCS, teleport.SchemeAZBlob, teleport.SchemeFile,
			}, ", "))
	}
}

var externalAuditMissingAthenaError = trace.BadParameter("athena audit_events_uri must be configured when External Audit Storage is enabled")

// initAuthExternalAuditLog initializes the auth server's audit log.
func (process *TeleportProcess) initAuthExternalAuditLog(auditConfig types.ClusterAuditConfig, externalAuditStorage *externalauditstorage.Configurator) (events.AuditLogger, error) {
	ctx := process.ExitContext()
	var hasNonFileLog bool
	var loggers []events.AuditLogger
	for _, eventsURI := range auditConfig.AuditEventsURIs() {
		uri, err := apiutils.ParseSessionsURI(eventsURI)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if externalAuditStorage.IsUsed() && (len(loggers) > 0 || uri.Scheme != teleport.ComponentAthena) {
			process.log.Infof("Skipping events URI %s because External Audit Storage is enabled", eventsURI)
			continue
		}
		switch uri.Scheme {
		case pgevents.Schema, pgevents.AltSchema:
			hasNonFileLog = true
			var cfg pgevents.Config
			if err := cfg.SetFromURL(uri); err != nil {
				return nil, trace.Wrap(err)
			}
			logger, err := pgevents.New(ctx, cfg)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			loggers = append(loggers, logger)
		case firestore.GetName():
			hasNonFileLog = true
			cfg := firestoreevents.EventsConfig{}
			err = cfg.SetFromURL(uri)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			logger, err := firestoreevents.New(cfg)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			loggers = append(loggers, logger)
		case dynamo.GetName():
			hasNonFileLog = true

			cfg := dynamoevents.Config{
				Tablename:               uri.Host,
				Region:                  auditConfig.Region(),
				EnableContinuousBackups: auditConfig.EnableContinuousBackups(),
				EnableAutoScaling:       auditConfig.EnableAutoScaling(),
				ReadMinCapacity:         auditConfig.ReadMinCapacity(),
				ReadMaxCapacity:         auditConfig.ReadMaxCapacity(),
				ReadTargetValue:         auditConfig.ReadTargetValue(),
				WriteMinCapacity:        auditConfig.WriteMinCapacity(),
				WriteMaxCapacity:        auditConfig.WriteMaxCapacity(),
				WriteTargetValue:        auditConfig.WriteTargetValue(),
				RetentionPeriod:         auditConfig.RetentionPeriod(),
				UseFIPSEndpoint:         auditConfig.GetUseFIPSEndpoint(),
			}

			err = cfg.SetFromURL(uri)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			logger, err := dynamoevents.New(ctx, cfg)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			loggers = append(loggers, logger)
		case teleport.ComponentAthena:
			hasNonFileLog = true
			cfg := athena.Config{
				Region:  auditConfig.Region(),
				Backend: process.backend,
			}
			if process.TracingProvider != nil {
				cfg.Tracer = process.TracingProvider.Tracer(teleport.ComponentAthena)
			}
			err = cfg.SetFromURL(uri)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if externalAuditStorage.IsUsed() {
				// External Audit Storage uses the topicArn, largeEventsS3, and
				// queueURL from the athena audit_events_uri passed by cloud,
				// and overwrites the remaining fields.
				if err := cfg.UpdateForExternalAuditStorage(ctx, externalAuditStorage); err != nil {
					return nil, trace.Wrap(err)
				}
			}
			var logger events.AuditLogger
			logger, err = athena.New(ctx, cfg)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if externalAuditStorage.IsUsed() {
				logger = externalAuditStorage.ErrorCounter.WrapAuditLogger(logger)
			}
			if cfg.LimiterBurst > 0 {
				// Wrap athena logger with rate limiter on search events.
				logger, err = events.NewSearchEventLimiter(events.SearchEventsLimiterConfig{
					RefillTime:   cfg.LimiterRefillTime,
					RefillAmount: cfg.LimiterRefillAmount,
					Burst:        cfg.LimiterBurst,
					AuditLogger:  logger,
				})
				if err != nil {
					return nil, trace.Wrap(err)
				}
			}
			loggers = append(loggers, logger)
		case teleport.SchemeFile:
			if uri.Path == "" {
				return nil, trace.BadParameter("unsupported audit uri: %q (missing path component)", uri)
			}
			if uri.Host != "" && uri.Host != "localhost" {
				return nil, trace.BadParameter("unsupported audit uri: %q (nonlocal host component: %q)", uri, uri.Host)
			}
			if err := os.MkdirAll(uri.Path, teleport.SharedDirMode); err != nil {
				return nil, trace.ConvertSystemError(err)
			}
			logger, err := events.NewFileLog(events.FileLogConfig{
				Dir: uri.Path,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			loggers = append(loggers, logger)
		case teleport.SchemeStdout:
			logger := events.NewWriterEmitter(utils.NopWriteCloser(os.Stdout))
			loggers = append(loggers, logger)
		default:
			return nil, trace.BadParameter(
				"unsupported scheme for audit_events_uri: %q, currently supported schemes are: %v",
				uri.Scheme, strings.Join([]string{
					teleport.SchemeFile, dynamo.GetName(), firestore.GetName(),
					pgevents.Schema, teleport.ComponentAthena, teleport.SchemeStdout,
				}, ", "))
		}
	}

	if len(loggers) < 1 {
		if externalAuditStorage.IsUsed() {
			return nil, externalAuditMissingAthenaError

		}
		return nil, nil
	}

	if !auditConfig.ShouldUploadSessions() && hasNonFileLog {
		// if audit events are being exported, session recordings should
		// be exported as well.
		return nil, trace.BadParameter("please specify audit_sessions_uri when using external audit backends")
	}

	if len(loggers) > 1 {
		return events.NewMultiLog(loggers...)
	}

	return loggers[0], nil
}

// initAuthService can be called to initialize auth server service
func (process *TeleportProcess) initAuthService() error {
	var err error
	cfg := process.Config

	// Initialize the storage back-ends for keys, events and records
	b, err := process.initAuthStorage()
	if err != nil {
		return trace.Wrap(err)
	}
	process.backend = b

	var emitter apievents.Emitter
	var streamer events.Streamer
	var uploadHandler events.MultipartHandler
	var externalAuditStorage *externalauditstorage.Configurator

	// create the audit log, which will be consuming (and recording) all events
	// and recording all sessions.
	if cfg.Auth.NoAudit {
		// this is for teleconsole
		process.auditLog = events.NewDiscardAuditLog()

		warningMessage := "Warning: Teleport audit and session recording have been " +
			"turned off. This is dangerous, you will not be able to view audit events " +
			"or save and playback recorded sessions."
		process.log.Warn(warningMessage)
		emitter, streamer = events.NewDiscardEmitter(), events.NewDiscardStreamer()
	} else {
		// check if session recording has been disabled. note, we will continue
		// logging audit events, we just won't record sessions.
		if cfg.Auth.SessionRecordingConfig.GetMode() == types.RecordOff {
			warningMessage := "Warning: Teleport session recording have been turned off. " +
				"This is dangerous, you will not be able to save and playback sessions."
			process.log.Warn(warningMessage)
		}

		if cfg.FIPS {
			cfg.Auth.AuditConfig.SetUseFIPSEndpoint(types.ClusterAuditConfigSpecV2_FIPS_ENABLED)
		}

		externalAuditStorage, err = process.newExternalAuditStorageConfigurator()
		if err != nil {
			return trace.Wrap(err)
		}

		uploadHandler, err = initAuthUploadHandler(
			process.ExitContext(), cfg.Auth.AuditConfig, filepath.Join(cfg.DataDir, teleport.LogsDir), externalAuditStorage)
		if err != nil {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}
		streamer, err = events.NewProtoStreamer(events.ProtoStreamerConfig{
			Uploader: uploadHandler,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		// initialize external loggers.  may return (nil, nil) if no
		// external loggers have been defined.
		externalLog, err := process.initAuthExternalAuditLog(cfg.Auth.AuditConfig, externalAuditStorage)
		if err != nil {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}

		auditServiceConfig := events.AuditLogConfig{
			Context:       process.ExitContext(),
			DataDir:       filepath.Join(cfg.DataDir, teleport.LogsDir),
			ServerID:      cfg.HostUUID,
			UploadHandler: uploadHandler,
			ExternalLog:   externalLog,
		}
		auditServiceConfig.UID, auditServiceConfig.GID, err = adminCreds()
		if err != nil {
			return trace.Wrap(err)
		}
		localLog, err := events.NewAuditLog(auditServiceConfig)
		if err != nil {
			return trace.Wrap(err)
		}
		process.auditLog = localLog
		if externalLog != nil {
			externalEmitter, ok := externalLog.(apievents.Emitter)
			if !ok {
				return trace.BadParameter("expected emitter, but %T does not emit", externalLog)
			}
			emitter = externalEmitter
		} else {
			emitter = localLog
		}
	}

	clusterName := cfg.Auth.ClusterName.GetClusterName()
	ident, err := process.storage.ReadIdentity(state.IdentityCurrent, types.RoleAdmin)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if ident != nil {
		clusterName = ident.ClusterName
	}

	checkingEmitter, err := events.NewCheckingEmitter(events.CheckingEmitterConfig{
		Inner:       events.NewMultiEmitter(events.NewLoggingEmitter(process.GetClusterFeatures().Cloud), emitter),
		Clock:       process.Clock,
		ClusterName: clusterName,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	traceClt := tracing.NewNoopClient()
	if cfg.Tracing.Enabled {
		traceConf, err := process.Config.Tracing.Config()
		if err != nil {
			return trace.Wrap(err)
		}
		traceConf.Logger = process.log.WithField(trace.Component, teleport.ComponentTracing)

		clt, err := tracing.NewStartedClient(process.ExitContext(), *traceConf)
		if err != nil {
			return trace.Wrap(err)
		}

		traceClt = clt
	}

	var embedderClient embedding.Embedder
	if cfg.Auth.AssistAPIKey != "" {
		// cfg.Testing.OpenAIConfig is set in tests to change the OpenAI API endpoint
		// Like for proxy, if a custom OpenAIConfig is passed, the token from
		// cfg.Auth.AssistAPIKey is ignored and the one from the config is used.
		if cfg.Testing.OpenAIConfig != nil {
			embedderClient = ai.NewClientFromConfig(*cfg.Testing.OpenAIConfig)
		} else {
			embedderClient = ai.NewClient(cfg.Auth.AssistAPIKey)
		}
	}

	embeddingsRetriever := ai.NewSimpleRetriever()
	cn, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: clusterName,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	keystoreConfig := keystore.Config{
		PKCS11: keystore.PKCS11Config{
			Path:       cfg.Auth.KeyStore.PKCS11.Path,
			SlotNumber: cfg.Auth.KeyStore.PKCS11.SlotNumber,
			TokenLabel: cfg.Auth.KeyStore.PKCS11.TokenLabel,
			Pin:        cfg.Auth.KeyStore.PKCS11.Pin,
			HostUUID:   cfg.Auth.KeyStore.PKCS11.HostUUID,
		},
		GCPKMS: keystore.GCPKMSConfig{
			KeyRing:         cfg.Auth.KeyStore.GCPKMS.KeyRing,
			ProtectionLevel: cfg.Auth.KeyStore.GCPKMS.ProtectionLevel,
			HostUUID:        cfg.Auth.KeyStore.GCPKMS.HostUUID,
		},
		Logger: process.log,
	}

	// first, create the AuthServer
	authServer, err := auth.Init(
		process.ExitContext(),
		auth.InitConfig{
			Backend:                 b,
			VersionStorage:          process.storage,
			Authority:               cfg.Keygen,
			ClusterConfiguration:    cfg.ClusterConfiguration,
			ClusterAuditConfig:      cfg.Auth.AuditConfig,
			ClusterNetworkingConfig: cfg.Auth.NetworkingConfig,
			SessionRecordingConfig:  cfg.Auth.SessionRecordingConfig,
			ClusterName:             cn,
			AuthServiceName:         cfg.Hostname,
			DataDir:                 cfg.DataDir,
			HostUUID:                cfg.HostUUID,
			NodeName:                cfg.Hostname,
			Authorities:             cfg.Auth.Authorities,
			ApplyOnStartupResources: cfg.Auth.ApplyOnStartupResources,
			BootstrapResources:      cfg.Auth.BootstrapResources,
			ReverseTunnels:          cfg.ReverseTunnels,
			Trust:                   cfg.Trust,
			Presence:                cfg.Presence,
			Events:                  cfg.Events,
			Provisioner:             cfg.Provisioner,
			Identity:                cfg.Identity,
			Access:                  cfg.Access,
			UsageReporter:           cfg.UsageReporter,
			StaticTokens:            cfg.Auth.StaticTokens,
			Roles:                   cfg.Auth.Roles,
			AuthPreference:          cfg.Auth.Preference,
			OIDCConnectors:          cfg.OIDCConnectors,
			AuditLog:                process.auditLog,
			CipherSuites:            cfg.CipherSuites,
			KeyStoreConfig:          keystoreConfig,
			Emitter:                 checkingEmitter,
			Streamer:                events.NewReportingStreamer(streamer, process.Config.Testing.UploadEventsC),
			TraceClient:             traceClt,
			FIPS:                    cfg.FIPS,
			LoadAllCAs:              cfg.Auth.LoadAllCAs,
			AccessMonitoringEnabled: cfg.Auth.IsAccessMonitoringEnabled(),
			Clock:                   cfg.Clock,
			HTTPClientForAWSSTS:     cfg.Auth.HTTPClientForAWSSTS,
			EmbeddingRetriever:      embeddingsRetriever,
			EmbeddingClient:         embedderClient,
			Tracer:                  process.TracingProvider.Tracer(teleport.ComponentAuth),
		}, func(as *auth.Server) error {
			if !process.Config.CachePolicy.Enabled {
				return nil
			}

			cache, err := process.newAccessCache(accesspoint.AccessCacheConfig{
				Services:  as.Services,
				Setup:     cache.ForAuth,
				CacheName: []string{teleport.ComponentAuth},
				Events:    true,
				Unstarted: true,
			})
			if err != nil {
				return trace.Wrap(err)
			}
			as.Cache = cache

			return nil
		})
	if err != nil {
		return trace.Wrap(err)
	}

	log := process.log.WithFields(logrus.Fields{
		trace.Component: teleport.Component(teleport.ComponentAuth, process.id),
	})

	lockWatcher, err := services.NewLockWatcher(process.ExitContext(), services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentAuth,
			Log:       log,
			Client:    authServer.Services,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	authServer.SetLockWatcher(lockWatcher)

	if externalAuditStorage.IsUsed() {
		externalAuditStorage.SetGenerateOIDCTokenFn(authServer.GenerateExternalAuditStorageOIDCToken)
	}

	unifiedResourcesCache, err := services.NewUnifiedResourceCache(process.ExitContext(), services.UnifiedResourceCacheConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			QueueSize:    defaults.UnifiedResourcesQueueSize,
			Component:    teleport.ComponentUnifiedResource,
			Log:          process.log.WithField(trace.Component, teleport.ComponentUnifiedResource),
			Client:       authServer,
			MaxStaleness: time.Minute,
		},
		ResourceGetter: authServer,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	authServer.SetUnifiedResourcesCache(unifiedResourcesCache)

	accessRequestCache, err := services.NewAccessRequestCache(services.AccessRequestCacheConfig{
		Events: authServer.Services,
		Getter: authServer.Services,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	authServer.SetAccessRequestCache(accessRequestCache)

	if embedderClient != nil {
		log.Debugf("Starting embedding watcher")
		embeddingProcessor := ai.NewEmbeddingProcessor(&ai.EmbeddingProcessorConfig{
			AIClient:            embedderClient,
			EmbeddingsRetriever: embeddingsRetriever,
			EmbeddingSrv:        authServer,
			NodeSrv:             authServer.UnifiedResourceCache,
			Log:                 log,
			Jitter:              retryutils.NewFullJitter(),
		})

		process.RegisterFunc("ai.embedding-processor", func() error {
			// We check the Assist feature flag here rather than on creation of TeleportProcess,
			// as when running Enterprise and the feature source is Cloud,
			// features may be loaded at two different times:
			// 1. When Cloud is reachable, features will be fetched from Cloud
			//    before constructing TeleportProcess
			// 2. When Cloud is not reachable, we will attempt to load cached features
			//    from the Teleport backend.
			// In the second case, we don't know the final value of Features().Assist
			// when constructing the process.
			// Services in the supervisor will only start after either 1 or 2 has succeeded,
			// so we can make the decision here.
			//
			// Ref: e/tool/teleport/process/process.go
			if !modules.GetModules().Features().Assist {
				log.Debug("Skipping start of embedding processor: Assist feature not enabled for license")
				return nil
			}
			log.Debugf("Starting embedding processor")
			return embeddingProcessor.Run(process.GracefulExitContext(), embeddingInitialDelay, embeddingPeriod)
		})
	}

	headlessAuthenticationWatcher, err := local.NewHeadlessAuthenticationWatcher(process.ExitContext(), local.HeadlessAuthenticationWatcherConfig{
		Backend: b,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	authServer.SetHeadlessAuthenticationWatcher(headlessAuthenticationWatcher)

	process.setLocalAuth(authServer)

	// The auth server runs its own upload completer, which is necessary in sync recording modes where
	// a node can abandon an upload before it is competed.
	// (In async recording modes, auth only ever sees completed uploads, as the node's upload completer
	// packages up the parts into a single upload before sending to auth)
	if uploadHandler != nil {
		err = events.StartNewUploadCompleter(process.ExitContext(), events.UploadCompleterConfig{
			Uploader:       uploadHandler,
			Component:      teleport.ComponentAuth,
			ClusterName:    clusterName,
			AuditLog:       process.auditLog,
			SessionTracker: authServer.Services,
			Semaphores:     authServer.Services,
			ServerID:       cfg.HostUUID,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	connector, err := process.connectToAuthService(types.RoleAdmin)
	if err != nil {
		return trace.Wrap(err)
	}

	// second, create the API Server: it's actually a collection of API servers,
	// each serving requests for a "role" which is assigned to every connected
	// client based on their certificate (user, server, admin, etc)
	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: clusterName,
		AccessPoint: authServer,
		LockWatcher: lockWatcher,
		Logger:      log,
		// Auth Server does explicit device authorization.
		// Various Auth APIs must allow access to unauthorized devices, otherwise it
		// is not possible to acquire device-aware certificates in the first place.
		DisableDeviceAuthorization: true,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	var accessGraphCAData []byte
	if cfg.AccessGraph.Enabled && cfg.AccessGraph.CA != "" {
		accessGraphCAData, err = os.ReadFile(cfg.AccessGraph.CA)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	apiConf := &auth.APIConfig{
		AuthServer:     authServer,
		Authorizer:     authorizer,
		AuditLog:       process.auditLog,
		PluginRegistry: process.PluginRegistry,
		Emitter:        authServer,
		MetadataGetter: uploadHandler,
		AccessGraph: auth.AccessGraphConfig{
			Enabled:  cfg.AccessGraph.Enabled,
			Address:  cfg.AccessGraph.Addr,
			CA:       accessGraphCAData,
			Insecure: cfg.AccessGraph.Insecure,
		},
	}

	// Auth initialization is done (including creation/updating of all singleton
	// configuration resources) so now we can start the cache.
	if c, ok := authServer.Cache.(*cache.Cache); ok {
		if err := c.Start(); err != nil {
			return trace.Wrap(err)
		}
	}

	// Register TLS endpoint of the auth service
	tlsConfig, err := connector.ServerIdentity.TLSConfig(cfg.CipherSuites)
	if err != nil {
		return trace.Wrap(err)
	}
	listener, err := process.importOrCreateListener(ListenerAuth, cfg.Auth.ListenAddr.Addr)
	if err != nil {
		log.Errorf("PID: %v Failed to bind to address %v: %v, exiting.", os.Getpid(), cfg.Auth.ListenAddr.Addr, err)
		return trace.Wrap(err)
	}

	// use listener addr instead of cfg.Auth.ListenAddr in order to support
	// binding to a random port (e.g. `127.0.0.1:0`).
	authAddr := listener.Addr().String()

	// clean up unused descriptors passed for proxy, but not used by it
	warnOnErr(process.closeImportedDescriptors(teleport.ComponentAuth), log)

	if cfg.Auth.PROXYProtocolMode == multiplexer.PROXYProtocolOn {
		log.Info("Starting Auth service with external PROXY protocol support.")
	}
	if cfg.Auth.PROXYProtocolMode == multiplexer.PROXYProtocolUnspecified {
		log.Warn("'proxy_protocol' unspecified. " +
			"Starting Auth service with external PROXY protocol support, " +
			"but IP pinned connection affected by PROXY headers will not be allowed. " +
			"Set 'proxy_protocol: on' in 'auth_service' config if Auth service runs behind L4 load balancer with enabled " +
			"PROXY protocol, or set 'proxy_protocol: off' otherwise")
	}

	muxCAGetter := func(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
		return authServer.GetCertAuthority(ctx, id, loadKeys)
	}
	// use multiplexer to leverage support for proxy protocol.
	mux, err := multiplexer.New(multiplexer.Config{
		PROXYProtocolMode:   cfg.Auth.PROXYProtocolMode,
		Listener:            listener,
		ID:                  teleport.Component(process.id),
		CertAuthorityGetter: muxCAGetter,
		LocalClusterName:    connector.ServerIdentity.ClusterName,
	})
	if err != nil {
		listener.Close()
		return trace.Wrap(err)
	}
	go mux.Serve()
	authMetrics := &auth.Metrics{GRPCServerLatency: cfg.Metrics.GRPCServerLatency}

	tlsServer, err := auth.NewTLSServer(process.ExitContext(), auth.TLSServerConfig{
		TLS:           tlsConfig,
		APIConfig:     *apiConf,
		LimiterConfig: cfg.Auth.Limiter,
		AccessPoint:   authServer.Cache,
		Component:     teleport.Component(teleport.ComponentAuth, process.id),
		ID:            process.id,
		Listener:      mux.TLS(),
		Metrics:       authMetrics,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	process.RegisterCriticalFunc("auth.tls", func() error {
		log.Infof("Auth service %s:%s is starting on %v.", teleport.Version, teleport.Gitref, authAddr)

		// since tlsServer.Serve is a blocking call, we emit this even right before
		// the service has started
		process.BroadcastEvent(Event{Name: AuthTLSReady, Payload: nil})
		err := tlsServer.Serve()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Warningf("TLS server exited with error: %v.", err)
		}
		return nil
	})
	process.RegisterFunc("auth.heartbeat.broadcast", func() error {
		// Heart beat auth server presence, this is not the best place for this
		// logic, consolidate it into auth package later
		connector, err := process.connectToAuthService(types.RoleAdmin)
		if err != nil {
			return trace.Wrap(err)
		}
		// External integrations rely on this event:
		process.BroadcastEvent(Event{Name: AuthIdentityEvent, Payload: connector})
		process.OnExit("auth.broadcast", func(payload interface{}) {
			connector.Close()
		})
		return nil
	})

	host, port, err := net.SplitHostPort(authAddr)
	if err != nil {
		return trace.Wrap(err)
	}
	// advertise-ip is explicitly set:
	if process.Config.AdvertiseIP != "" {
		ahost, aport, err := utils.ParseAdvertiseAddr(process.Config.AdvertiseIP)
		if err != nil {
			return trace.Wrap(err)
		}
		// if port is not set in the advertise addr, use the default one
		if aport == "" {
			aport = port
		}
		authAddr = net.JoinHostPort(ahost, aport)
	} else {
		// advertise-ip is not set, while the CA is listening on 0.0.0.0? lets try
		// to guess the 'advertise ip' then:
		if net.ParseIP(host).IsUnspecified() {
			ip, err := utils.GuessHostIP()
			if err != nil {
				log.Warn(err)
			} else {
				authAddr = net.JoinHostPort(ip.String(), port)
			}
		}
		log.Warnf("Configuration setting auth_service/advertise_ip is not set. guessing %v.", authAddr)
	}

	heartbeat, err := srv.NewHeartbeat(srv.HeartbeatConfig{
		Mode:      srv.HeartbeatModeAuth,
		Context:   process.GracefulExitContext(),
		Component: teleport.ComponentAuth,
		Announcer: authServer,
		GetServerInfo: func() (types.Resource, error) {
			srv := types.ServerV2{
				Kind:    types.KindAuthServer,
				Version: types.V2,
				Metadata: types.Metadata{
					Namespace: apidefaults.Namespace,
					Name:      process.Config.HostUUID,
				},
				Spec: types.ServerSpecV2{
					Addr:     authAddr,
					Hostname: process.Config.Hostname,
					Version:  teleport.Version,
				},
			}
			state, err := process.storage.GetState(process.GracefulExitContext(), types.RoleAdmin)
			if err != nil {
				if !trace.IsNotFound(err) {
					log.Warningf("Failed to get rotation state: %v.", err)
					return nil, trace.Wrap(err)
				}
			} else {
				srv.Spec.Rotation = state.Spec.Rotation
			}
			srv.SetExpiry(process.Clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL))
			return &srv, nil
		},
		KeepAlivePeriod: apidefaults.ServerKeepAliveTTL(),
		AnnouncePeriod:  apidefaults.ServerAnnounceTTL/2 + utils.RandomDuration(apidefaults.ServerAnnounceTTL/10),
		CheckPeriod:     defaults.HeartbeatCheckPeriod,
		ServerTTL:       apidefaults.ServerAnnounceTTL,
		OnHeartbeat:     process.OnHeartbeat(teleport.ComponentAuth),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	process.RegisterFunc("auth.heartbeat", heartbeat.Run)

	process.RegisterFunc("auth.server_info", func() error {
		return trace.Wrap(authServer.ReconcileServerInfos(process.GracefulExitContext()))
	})
	// execute this when process is asked to exit:
	process.OnExit("auth.shutdown", func(payload any) {
		// The listeners have to be closed here, because if shutdown
		// was called before the start of the http server,
		// the http server would have not started tracking the listeners
		// and http.Shutdown will do nothing.
		if mux != nil {
			warnOnErr(mux.Close(), log)
		}
		if listener != nil {
			warnOnErr(listener.Close(), log)
		}
		if payload == nil {
			log.Info("Shutting down immediately.")
			warnOnErr(tlsServer.Close(), log)
		} else {
			ctx := payloadContext(payload, log)
			log.Info("Shutting down immediately (auth service does not currently support graceful shutdown).")
			// NOTE: Graceful shutdown of auth.TLSServer is disabled right now, because we don't
			// have a good model for performing it.  In particular, watchers and other gRPC streams
			// are a problem.  Even if we distinguish between user-created and server-created streams
			// (as is done with ssh connections), we don't have a way to distinguish "service accounts"
			// such as access workflow plugins from normal users.  Without this, a graceful shutdown
			// of the auth server basically never exits.
			warnOnErr(tlsServer.Close(), log)

			if g, ok := authServer.Services.UsageReporter.(usagereporter.GracefulStopper); ok {
				if err := g.GracefulStop(ctx); err != nil {
					log.WithError(err).Warn("Error while gracefully stopping usage reporter.")
				}
			}
		}
		log.Info("Exited.")
	})
	return nil
}

func payloadContext(payload interface{}, log logrus.FieldLogger) context.Context {
	ctx, ok := payload.(context.Context)
	if ok {
		return ctx
	}
	log.Errorf("Expected context, got %T.", payload)
	return context.TODO()
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

// newAccessCache returns new local cache access point
func (process *TeleportProcess) newAccessCache(cfg accesspoint.AccessCacheConfig) (*cache.Cache, error) {
	cfg.Context = process.ExitContext()
	cfg.ProcessID = process.id
	cfg.TracingProvider = process.TracingProvider
	cfg.MaxRetryPeriod = process.Config.CachePolicy.MaxRetryPeriod

	return accesspoint.NewAccessCache(cfg)
}

// newLocalCacheForNode returns new instance of access point configured for a local proxy.
func (process *TeleportProcess) newLocalCacheForNode(clt authclient.ClientI, cacheName []string) (authclient.NodeAccessPoint, error) {
	// if caching is disabled, return access point
	if !process.Config.CachePolicy.Enabled {
		return clt, nil
	}

	cache, err := process.NewLocalCache(clt, cache.ForNode, cacheName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return authclient.NewNodeWrapper(clt, cache), nil
}

// newLocalCacheForKubernetes returns new instance of access point configured for a kubernetes service.
func (process *TeleportProcess) newLocalCacheForKubernetes(clt authclient.ClientI, cacheName []string) (authclient.KubernetesAccessPoint, error) {
	// if caching is disabled, return access point
	if !process.Config.CachePolicy.Enabled {
		return clt, nil
	}

	cache, err := process.NewLocalCache(clt, cache.ForKubernetes, cacheName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return authclient.NewKubernetesWrapper(clt, cache), nil
}

// newLocalCacheForDatabase returns new instance of access point configured for a database service.
func (process *TeleportProcess) newLocalCacheForDatabase(clt authclient.ClientI, cacheName []string) (authclient.DatabaseAccessPoint, error) {
	// if caching is disabled, return access point
	if !process.Config.CachePolicy.Enabled {
		return clt, nil
	}

	cache, err := process.NewLocalCache(clt, cache.ForDatabases, cacheName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return authclient.NewDatabaseWrapper(clt, cache), nil
}

type discoveryConfigClient interface {
	UpdateDiscoveryConfigStatus(ctx context.Context, name string, status discoveryconfig.Status) (*discoveryconfig.DiscoveryConfig, error)
	services.DiscoveryConfigsGetter
}

// combinedDiscoveryClient is an auth.Client client with other, specific, services added to it.
type combinedDiscoveryClient struct {
	authclient.ClientI
	discoveryConfigClient
}

// newLocalCacheForDiscovery returns a new instance of access point for a discovery service.
func (process *TeleportProcess) newLocalCacheForDiscovery(clt authclient.ClientI, cacheName []string) (authclient.DiscoveryAccessPoint, error) {
	client := combinedDiscoveryClient{
		ClientI:               clt,
		discoveryConfigClient: clt.DiscoveryConfigClient(),
	}

	// if caching is disabled, return access point
	if !process.Config.CachePolicy.Enabled {
		return client, nil
	}
	cache, err := process.NewLocalCache(clt, cache.ForDiscovery, cacheName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return authclient.NewDiscoveryWrapper(client, cache), nil
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

// DELETE IN: 8.0.0
//
// newLocalCacheForOldRemoteProxy returns new instance of access point
// configured for an old remote proxy.
func (process *TeleportProcess) newLocalCacheForOldRemoteProxy(clt authclient.ClientI, cacheName []string) (authclient.RemoteProxyAccessPoint, error) {
	// if caching is disabled, return access point
	if !process.Config.CachePolicy.Enabled {
		return clt, nil
	}

	cache, err := process.NewLocalCache(clt, cache.ForOldRemoteProxy, cacheName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return authclient.NewRemoteProxyWrapper(clt, cache), nil
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

// newLocalCacheForWindowsDesktop returns new instance of access point configured for a windows desktop service.
func (process *TeleportProcess) newLocalCacheForWindowsDesktop(clt authclient.ClientI, cacheName []string) (authclient.WindowsDesktopAccessPoint, error) {
	// if caching is disabled, return access point
	if !process.Config.CachePolicy.Enabled {
		return clt, nil
	}

	cache, err := process.NewLocalCache(clt, cache.ForWindowsDesktop, cacheName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return authclient.NewWindowsDesktopWrapper(clt, cache), nil
}

// accessPointWrapper is a wrapper around auth.ClientI that reduces the surface area of the
// auth.ClientI.DiscoveryConfigClient interface to services.DiscoveryConfigs.
// Cache doesn't implement the full auth.ClientI interface, so we need to wrap auth.ClientI to
// to make it compatible with the services.DiscoveryConfigs interface.
type accessPointWrapper struct {
	authclient.ClientI
}

func (a accessPointWrapper) DiscoveryConfigClient() services.DiscoveryConfigs {
	return a.ClientI.DiscoveryConfigClient()
}

// NewLocalCache returns new instance of access point
func (process *TeleportProcess) NewLocalCache(clt authclient.ClientI, setupConfig cache.SetupConfigFn, cacheName []string) (*cache.Cache, error) {
	return process.newAccessCache(accesspoint.AccessCacheConfig{
		Services:  &accessPointWrapper{ClientI: clt},
		Setup:     setupConfig,
		CacheName: cacheName,
	})
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

// initInstance initializes the pseudo-service "Instance" that is active on all teleport instances.
func (process *TeleportProcess) initInstance() error {
	var hasNonAuthRole bool
	for _, role := range process.getInstanceRoles() {
		if role != types.RoleAuth {
			hasNonAuthRole = true
			break
		}
	}

	if process.Config.Auth.Enabled && !hasNonAuthRole {
		// if we have a local auth server and no other services, we cannot create an instance client without breaking HSM rotation.
		// instance control stream will be created via in-memory pipe, but until this limitation is resolved
		// or a fully in-memory instance client is implemented, we cannot rely on the instance client existing
		// for purposes other than the control stream.
		// TODO(fspmarshall): implement one of the two potential solutions listed above.
		process.setInstanceConnector(nil)
		process.BroadcastEvent(Event{Name: InstanceReady, Payload: nil})
		return nil
	}
	process.RegisterWithAuthServer(types.RoleInstance, InstanceIdentityEvent)

	log := process.log.WithFields(logrus.Fields{
		trace.Component: teleport.Component(teleport.ComponentInstance, process.id),
	})

	process.RegisterCriticalFunc("instance.init", func() error {
		conn, err := process.WaitForConnector(InstanceIdentityEvent, log)
		if conn == nil {
			return trace.Wrap(err)
		}

		process.setInstanceConnector(conn)
		log.Infof("Successfully registered instance client.")
		process.BroadcastEvent(Event{Name: InstanceReady, Payload: nil})
		return nil
	})

	return nil
}

// initSSH initializes the "node" role, i.e. a simple SSH server connected to the auth server.
func (process *TeleportProcess) initSSH() error {
	process.RegisterWithAuthServer(types.RoleNode, SSHIdentityEvent)

	log := process.log.WithFields(logrus.Fields{
		trace.Component: teleport.Component(teleport.ComponentNode, process.id),
	})

	proxyGetter := reversetunnel.NewConnectedProxyGetter()

	process.RegisterCriticalFunc("ssh.node", func() error {
		// restartingOnGracefulShutdown will be set to true before the function
		// exits if the function is exiting because Teleport is gracefully
		// shutting down as a consequence of internally-triggered reloading or
		// being signaled to restart.
		var restartingOnGracefulShutdown bool

		conn, err := process.WaitForConnector(SSHIdentityEvent, log)
		if conn == nil {
			return trace.Wrap(err)
		}

		defer func() { warnOnErr(conn.Close(), log) }()

		cfg := process.Config

		limiter, err := limiter.NewLimiter(cfg.SSH.Limiter)
		if err != nil {
			return trace.Wrap(err)
		}

		authClient, err := process.newLocalCacheForNode(conn.Client, []string{teleport.ComponentNode})
		if err != nil {
			return trace.Wrap(err)
		}

		// If session recording is disabled at the cluster level and the node is
		// attempting to enabled enhanced session recording, show an error.
		recConfig, err := authClient.GetSessionRecordingConfig(process.ExitContext())
		if err != nil {
			return trace.Wrap(err)
		}
		if recConfig.GetMode() == types.RecordOff && cfg.SSH.BPF.Enabled {
			return trace.BadParameter("session recording is disabled at the cluster " +
				"level. To enable enhanced session recording, enable session recording at " +
				"the cluster level, then restart Teleport.")
		}

		// Restricted session requires BPF (enhanced recording)
		if cfg.SSH.RestrictedSession.Enabled && !cfg.SSH.BPF.Enabled {
			return trace.BadParameter("restricted_session requires enhanced_recording " +
				"to be enabled")
		}

		// If BPF is enabled in file configuration, but the operating system does
		// not support enhanced session recording (like macOS), exit right away.
		if cfg.SSH.BPF.Enabled && !bpf.SystemHasBPF() {
			return trace.BadParameter("operating system does not support enhanced " +
				"session recording, check Teleport documentation for more details on " +
				"supported operating systems, kernels, and configuration")
		}

		// Start BPF programs. This is blocking and if the BPF programs fail to
		// load, the node will not start. If BPF is not enabled, this will simply
		// return a NOP struct that can be used to discard BPF data.
		ebpf, err := bpf.New(cfg.SSH.BPF, cfg.SSH.RestrictedSession)
		if err != nil {
			return trace.Wrap(err)
		}
		defer func() { warnOnErr(ebpf.Close(restartingOnGracefulShutdown), log) }()

		// Start access control programs. This is blocking and if the BPF programs fail to
		// load, the node will not start. If access control is not enabled, this will simply
		// return a NOP struct.
		rm, err := restricted.New(cfg.SSH.RestrictedSession, conn.Client)
		if err != nil {
			return trace.Wrap(err)
		}
		// TODO: are we missing rm.Close()

		// make sure the default namespace is used
		if ns := cfg.SSH.Namespace; ns != "" && ns != apidefaults.Namespace {
			return trace.BadParameter("cannot start with custom namespace %q, custom namespaces are deprecated. "+
				"use builtin namespace %q, or omit the 'namespace' config option.", ns, apidefaults.Namespace)
		}
		namespace := types.ProcessNamespace(cfg.SSH.Namespace)
		_, err = authClient.GetNamespace(namespace)
		if err != nil {
			if trace.IsNotFound(err) {
				return trace.NotFound(
					"namespace %v is not found, ask your system administrator to create this namespace so you can register nodes there.", namespace)
			}
			return trace.Wrap(err)
		}

		if auditd.IsLoginUIDSet() {
			log.Warnf("Login UID is set, but it shouldn't be. Incorrect login UID breaks session ID when using auditd. " +
				"Please make sure that Teleport runs as a daemon and any parent process doesn't set the login UID.")
		}

		// Provide helpful log message if listen_addr or public_addr are not being
		// used (tunnel is used to connect to cluster).
		//
		// If a tunnel is not being used, set the default here (could not be done in
		// file configuration because at that time it's not known if server is
		// joining cluster directly or through a tunnel).
		if conn.UseTunnel() {
			if !cfg.SSH.Addr.IsEmpty() {
				log.Info("Connected to cluster over tunnel connection, ignoring listen_addr setting.")
			}
			if len(cfg.SSH.PublicAddrs) > 0 {
				log.Info("Connected to cluster over tunnel connection, ignoring public_addr setting.")
			}
		}
		if !conn.UseTunnel() && cfg.SSH.Addr.IsEmpty() {
			cfg.SSH.Addr = *defaults.SSHServerListenAddr()
		}

		// asyncEmitter makes sure that sessions do not block
		// in case if connections are slow
		asyncEmitter, err := process.NewAsyncEmitter(conn.Client)
		if err != nil {
			return trace.Wrap(err)
		}
		defer func() { warnOnErr(asyncEmitter.Close(), log) }()

		lockWatcher, err := services.NewLockWatcher(process.ExitContext(), services.LockWatcherConfig{
			ResourceWatcherConfig: services.ResourceWatcherConfig{
				Component: teleport.ComponentNode,
				Log:       log,
				Client:    conn.Client,
			},
		})
		if err != nil {
			return trace.Wrap(err)
		}

		storagePresence := local.NewPresenceService(process.storage.BackendStorage)

		// read the host UUID:
		serverID, err := utils.ReadOrMakeHostUUID(cfg.DataDir)
		if err != nil {
			return trace.Wrap(err)
		}

		sessionController, err := srv.NewSessionController(srv.SessionControllerConfig{
			Semaphores:     authClient,
			AccessPoint:    authClient,
			LockEnforcer:   lockWatcher,
			Emitter:        &events.StreamerAndEmitter{Emitter: asyncEmitter, Streamer: conn.Client},
			Component:      teleport.ComponentNode,
			Logger:         process.log.WithField(trace.Component, "sessionctrl"),
			TracerProvider: process.TracingProvider,
			ServerID:       serverID,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		s, err := regular.New(
			process.ExitContext(),
			cfg.SSH.Addr,
			cfg.Hostname,
			[]ssh.Signer{conn.ServerIdentity.KeySigner},
			authClient,
			cfg.DataDir,
			cfg.AdvertiseIP,
			process.proxyPublicAddr(),
			conn.Client,
			regular.SetLimiter(limiter),
			regular.SetEmitter(&events.StreamerAndEmitter{Emitter: asyncEmitter, Streamer: conn.Client}),
			regular.SetLabels(cfg.SSH.Labels, cfg.SSH.CmdLabels, process.cloudLabels),
			regular.SetNamespace(namespace),
			regular.SetPermitUserEnvironment(cfg.SSH.PermitUserEnvironment),
			regular.SetCiphers(cfg.Ciphers),
			regular.SetKEXAlgorithms(cfg.KEXAlgorithms),
			regular.SetMACAlgorithms(cfg.MACAlgorithms),
			regular.SetPAMConfig(cfg.SSH.PAM),
			regular.SetRotationGetter(process.GetRotation),
			regular.SetUseTunnel(conn.UseTunnel()),
			regular.SetFIPS(cfg.FIPS),
			regular.SetBPF(ebpf),
			regular.SetRestrictedSessionManager(rm),
			regular.SetOnHeartbeat(process.OnHeartbeat(teleport.ComponentNode)),
			regular.SetAllowTCPForwarding(cfg.SSH.AllowTCPForwarding),
			regular.SetLockWatcher(lockWatcher),
			regular.SetX11ForwardingConfig(cfg.SSH.X11),
			regular.SetAllowFileCopying(cfg.SSH.AllowFileCopying),
			regular.SetConnectedProxyGetter(proxyGetter),
			regular.SetCreateHostUser(!cfg.SSH.DisableCreateHostUser),
			regular.SetStoragePresenceService(storagePresence),
			regular.SetInventoryControlHandle(process.inventoryHandle),
			regular.SetTracerProvider(process.TracingProvider),
			regular.SetSessionController(sessionController),
			regular.SetPublicAddrs(cfg.SSH.PublicAddrs),
		)
		if err != nil {
			return trace.Wrap(err)
		}
		defer func() { warnOnErr(s.Close(), log) }()

		var agentPool *reversetunnel.AgentPool
		if !conn.UseTunnel() {
			listener, err := process.importOrCreateListener(ListenerNodeSSH, cfg.SSH.Addr.Addr)
			if err != nil {
				return trace.Wrap(err)
			}
			// clean up unused descriptors passed for proxy, but not used by it
			warnOnErr(process.closeImportedDescriptors(teleport.ComponentNode), log)

			log.Infof("Service %s:%s is starting on %v %v.", teleport.Version, teleport.Gitref, cfg.SSH.Addr.Addr, process.Config.CachePolicy)

			// Use multiplexer to leverage support for signed PROXY protocol headers.
			mux, err := multiplexer.New(multiplexer.Config{
				Context:             process.ExitContext(),
				PROXYProtocolMode:   multiplexer.PROXYProtocolOff,
				Listener:            listener,
				ID:                  teleport.Component(teleport.ComponentNode, process.id),
				CertAuthorityGetter: authClient.GetCertAuthority,
				LocalClusterName:    conn.ServerIdentity.ClusterName,
				FixedHeader:         sshutils.SSHVersionPrefix + "\r\n",
			})
			if err != nil {
				return trace.Wrap(err)
			}

			go func() {
				if err := mux.Serve(); err != nil && !utils.IsOKNetworkError(err) {
					mux.Entry.WithError(err).Error("node ssh multiplexer terminated unexpectedly")
				}
			}()
			defer mux.Close()

			listener, err = limiter.WrapListener(mux.SSH())
			if err != nil {
				return trace.Wrap(err)
			}

			go s.Serve(listener)
		} else {
			// Start the SSH server. This kicks off updating labels and starting the
			// heartbeat.
			if err := s.Start(); err != nil {
				return trace.Wrap(err)
			}

			// Create and start an agent pool.
			agentPool, err = reversetunnel.NewAgentPool(
				process.ExitContext(),
				reversetunnel.AgentPoolConfig{
					Component:            teleport.ComponentNode,
					HostUUID:             conn.ServerIdentity.ID.HostUUID,
					Resolver:             conn.TunnelProxyResolver(),
					Client:               conn.Client,
					AccessPoint:          authClient,
					HostSigner:           conn.ServerIdentity.KeySigner,
					Cluster:              conn.ServerIdentity.Cert.Extensions[utils.CertExtensionAuthority],
					Server:               s,
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
			log.Infof("Service is starting in tunnel mode.")
		}

		// Broadcast that the node has started.
		process.BroadcastEvent(Event{Name: NodeSSHReady, Payload: nil})

		// Block and wait while the node is running.
		event, err := process.WaitForEvent(process.ExitContext(), TeleportExitEvent)
		if err != nil {
			return trace.Wrap(err)
		}

		if event.Payload == nil {
			log.Infof("Shutting down immediately.")
			warnOnErr(s.Close(), log)
		} else {
			log.Infof("Shutting down gracefully.")
			ctx := payloadContext(event.Payload, log)
			restartingOnGracefulShutdown = services.IsProcessReloading(ctx) || services.HasProcessForked(ctx)
			warnOnErr(s.Shutdown(ctx), log)
		}

		s.Wait()
		agentPool.Stop()
		agentPool.Wait()

		log.Infof("Exited.")
		return nil
	})

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
			process.log.Errorf("Register called for unexpected instance role %q (this is a bug).", role)
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
func waitForInstanceConnector(process *TeleportProcess, log *logrus.Entry) (*Connector, error) {
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
			log.Warn("The Instance connector is still not available, process-wide services " +
				"such as session uploading will not function")
		}
	}
}

// initUploaderService starts a file-based uploader that scans the local streaming logs directory
// (data/log/upload/streaming/default/)
func (process *TeleportProcess) initUploaderService() error {
	component := teleport.Component(teleport.ComponentUpload, process.id)
	log := process.log.WithFields(logrus.Fields{
		trace.Component: component,
	})

	var clusterName string

	type procUploader interface {
		events.Streamer
		events.AuditLogSessionStreamer
		services.SessionTrackerService
	}

	// use the local auth server for uploads if auth happens to be
	// running in this process, otherwise wait for the instance client
	var uploaderClient procUploader
	if la := process.getLocalAuth(); la != nil {
		// The auth service's upload completer is initialized separately,
		// so as a special case we can stop early if auth happens to be
		// the only service running in this process.
		if srs := process.getInstanceRoles(); len(srs) == 1 && srs[0] == types.RoleAuth {
			log.Debug("this process only runs the auth service, no separate upload completer will run")
			return nil
		}

		uploaderClient = la
		cn, err := la.GetClusterName()
		if err != nil {
			return trace.Wrap(err, "cannot get cluster name")
		}
		clusterName = cn.GetClusterName()
	} else {
		log.Debug("auth is not running in-process, waiting for instance connector")
		conn, err := waitForInstanceConnector(process, log)
		if err != nil {
			return trace.Wrap(err)
		}
		if conn == nil {
			return trace.BadParameter("process exiting and Instance connector never became available")
		}
		uploaderClient = conn.Client
		clusterName = conn.ServerIdentity.ClusterName
	}

	log.Info("starting upload completer service")

	// create folder for uploads
	uid, gid, err := adminCreds()
	if err != nil {
		return trace.Wrap(err)
	}

	// prepare directories for uploader
	paths := [][]string{
		{process.Config.DataDir, teleport.LogsDir, teleport.ComponentUpload, events.StreamingSessionsDir, apidefaults.Namespace},
		{process.Config.DataDir, teleport.LogsDir, teleport.ComponentUpload, events.CorruptedSessionsDir, apidefaults.Namespace},
	}
	for _, path := range paths {
		for i := 1; i < len(path); i++ {
			dir := filepath.Join(path[:i+1]...)
			log.Infof("Creating directory %v.", dir)
			err := os.Mkdir(dir, 0o755)
			err = trace.ConvertSystemError(err)
			if err != nil && !trace.IsAlreadyExists(err) {
				return trace.Wrap(err)
			}
			if uid != nil && gid != nil {
				log.Infof("Setting directory %v owner to %v:%v.", dir, *uid, *gid)
				err := os.Lchown(dir, *uid, *gid)
				if err != nil {
					return trace.ConvertSystemError(err)
				}
			}
		}
	}

	uploadsDir := filepath.Join(paths[0]...)
	corruptedDir := filepath.Join(paths[1]...)

	fileUploader, err := filesessions.NewUploader(filesessions.UploaderConfig{
		Streamer:         uploaderClient,
		ScanDir:          uploadsDir,
		CorruptedDir:     corruptedDir,
		EventsC:          process.Config.Testing.UploadEventsC,
		InitialScanDelay: 15 * time.Second,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	process.RegisterFunc("fileuploader.service", func() error {
		err := fileUploader.Serve(process.ExitContext())
		if err != nil {
			log.WithError(err).Errorf("File uploader server exited with error.")
		}

		return nil
	})

	process.OnExit("fileuploader.shutdown", func(payload interface{}) {
		log.Infof("File uploader is shutting down.")
		fileUploader.Close()
		log.Infof("File uploader has shut down.")
	})

	// upload completer scans for uploads that have been initiated, but not completed
	// by the client (aborted or crashed) and completes them. It will be closed once
	// the uploader context is closed.
	handler, err := filesessions.NewHandler(filesessions.Config{Directory: uploadsDir})
	if err != nil {
		return trace.Wrap(err)
	}

	uploadCompleter, err := events.NewUploadCompleter(events.UploadCompleterConfig{
		Component:      component,
		Uploader:       handler,
		AuditLog:       uploaderClient,
		SessionTracker: uploaderClient,
		ClusterName:    clusterName,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	process.RegisterFunc("fileuploadcompleter.service", func() error {
		if err := uploadCompleter.Serve(process.ExitContext()); err != nil {
			log.WithError(err).Errorf("File uploader server exited with error.")
		}
		return nil
	})

	process.OnExit("fileuploadcompleter.shutdown", func(payload interface{}) {
		log.Infof("File upload completer is shutting down.")
		uploadCompleter.Close()
		log.Infof("File upload completer has shut down.")
	})

	return nil
}

// initMetricsService starts the metrics service currently serving metrics for
// prometheus consumption
func (process *TeleportProcess) initMetricsService() error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	log := process.log.WithFields(logrus.Fields{
		trace.Component: teleport.Component(teleport.ComponentMetrics, process.id),
	})

	listener, err := process.importOrCreateListener(ListenerMetrics, process.Config.Metrics.ListenAddr.Addr)
	if err != nil {
		return trace.Wrap(err)
	}
	warnOnErr(process.closeImportedDescriptors(teleport.ComponentMetrics), log)

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
		ErrorLog:          utils.NewStdlogger(log.Error, teleport.ComponentMetrics),
		TLSConfig:         tlsConfig,
	}

	log.Infof("Starting metrics service on %v.", process.Config.Metrics.ListenAddr.Addr)

	process.RegisterFunc("metrics.service", func() error {
		err := server.Serve(listener)
		if err != nil && err != http.ErrServerClosed {
			log.Warningf("Metrics server exited with error: %v.", err)
		}
		return nil
	})

	process.OnExit("metrics.shutdown", func(payload interface{}) {
		if payload == nil {
			log.Infof("Shutting down immediately.")
			warnOnErr(server.Close(), log)
		} else {
			log.Infof("Shutting down gracefully.")
			ctx := payloadContext(payload, log)
			warnOnErr(server.Shutdown(ctx), log)
		}
		log.Infof("Exited.")
	})

	process.BroadcastEvent(Event{Name: MetricsReady, Payload: nil})
	return nil
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
		process.log.Infof("Adding diagnostic debugging handlers. To connect with profiler, use `go tool pprof %v`.", process.Config.DiagnosticAddr.Addr)

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

	log := process.log.WithFields(logrus.Fields{
		trace.Component: teleport.Component(teleport.ComponentDiagnostic, process.id),
	})

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
				log.Debugf("Teleport is exiting, returning.")
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
	warnOnErr(process.closeImportedDescriptors(teleport.ComponentDiagnostic), log)

	server := &http.Server{
		Handler:           mux,
		ReadTimeout:       apidefaults.DefaultIOTimeout,
		ReadHeaderTimeout: defaults.ReadHeadersTimeout,
		WriteTimeout:      apidefaults.DefaultIOTimeout,
		IdleTimeout:       apidefaults.DefaultIdleTimeout,
		ErrorLog:          utils.NewStdlogger(log.Error, teleport.ComponentDiagnostic),
	}

	log.Infof("Starting diagnostic service on %v.", process.Config.DiagnosticAddr.Addr)

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
			log.Warningf("Diagnostic server exited with error: %v.", err)
		}
		return nil
	})

	process.OnExit("diagnostic.shutdown", func(payload interface{}) {
		warnOnErr(muxListener.Close(), log)
		if payload == nil {
			log.Infof("Shutting down immediately.")
			warnOnErr(server.Close(), log)
		} else {
			log.Infof("Shutting down gracefully.")
			ctx := payloadContext(payload, log)
			warnOnErr(server.Shutdown(ctx), log)
		}
		log.Infof("Exited.")
	})

	return nil
}

func (process *TeleportProcess) initTracingService() error {
	log := process.log.WithField(trace.Component, teleport.Component(teleport.ComponentTracing, process.id))
	log.Info("Initializing tracing provider and exporter.")

	attrs := []attribute.KeyValue{
		attribute.String(tracing.ProcessIDKey, process.id),
		attribute.String(tracing.HostnameKey, process.Config.Hostname),
		attribute.String(tracing.HostIDKey, process.Config.HostUUID),
	}

	traceConf, err := process.Config.Tracing.Config(attrs...)
	if err != nil {
		return trace.Wrap(err)
	}
	traceConf.Logger = log

	provider, err := tracing.NewTraceProvider(process.ExitContext(), *traceConf)
	if err != nil {
		return trace.Wrap(err)
	}
	process.TracingProvider = provider

	process.OnExit("tracing.shutdown", func(payload interface{}) {
		if payload == nil {
			log.Info("Shutting down immediately.")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			warnOnErr(provider.Shutdown(ctx), log)
		} else {
			log.Infof("Shutting down gracefully.")
			ctx := payloadContext(payload, log)
			warnOnErr(provider.Shutdown(ctx), log)
		}
		process.log.Info("Exited.")
	})

	process.BroadcastEvent(Event{Name: TracingReady, Payload: nil})
	return nil
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
// this means it will do four things:
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
		conn, err := process.WaitForConnector(ProxyIdentityEvent, process.log)
		if conn == nil {
			return trace.Wrap(err)
		}

		if err := process.initProxyEndpoint(conn); err != nil {
			warnOnErr(conn.Close(), process.log)
			return trace.Wrap(err)
		}

		return nil
	})
	return nil
}

type proxyListeners struct {
	mux    *multiplexer.Mux
	sshMux *multiplexer.Mux
	tls    *multiplexer.WebListener
	// ssh receives SSH traffic that is multiplexed on the Proxy SSH Port. When TLS routing
	// is enabled only traffic with the TLS ALPN protocol common.ProtocolProxySSH is received.
	ssh net.Listener
	// sshGRPC receives gRPC traffic that is multiplexed on the Proxy SSH Port. When TLS routing
	// is enabled only traffic with the TLS ALPN protocol common.ProtocolProxySSHGRPC is received.
	sshGRPC       net.Listener
	web           net.Listener
	reverseTunnel net.Listener
	kube          net.Listener
	db            dbListeners
	alpn          net.Listener
	// reverseTunnelALPN handles ALPN traffic on the reverse tunnel port when TLS routing
	// is not enabled. It's used to redirect traffic on that port to the gRPC
	// listener.
	reverseTunnelALPN net.Listener
	proxyPeer         net.Listener
	// grpcPublic receives gRPC traffic that has the TLS ALPN protocol common.ProtocolProxyGRPCInsecure. This
	// listener does not enforce mTLS authentication since it's used to handle cluster join requests.
	grpcPublic net.Listener
	// grpcMTLS receives gRPC traffic that has the TLS ALPN protocol common.ProtocolProxyGRPCSecure. This
	// listener is only enabled when TLS routing is enabled and the gRPC server will enforce mTLS authentication.
	grpcMTLS         net.Listener
	reverseTunnelMux *multiplexer.Mux
	// minimalWeb handles traffic on the reverse tunnel port when TLS routing
	// is not enabled. It serves only the subset of web traffic required for
	// agents to join the cluster.
	minimalWeb net.Listener
	minimalTLS *multiplexer.WebListener
}

// Close closes all proxy listeners.
func (l *proxyListeners) Close() {
	if l.mux != nil {
		l.mux.Close()
	}
	if l.sshMux != nil {
		l.sshMux.Close()
	}
	if l.tls != nil {
		l.tls.Close()
	}
	if l.ssh != nil {
		l.ssh.Close()
	}
	if l.sshGRPC != nil {
		l.sshGRPC.Close()
	}
	if l.web != nil {
		l.web.Close()
	}
	if l.reverseTunnel != nil {
		l.reverseTunnel.Close()
	}
	if l.kube != nil {
		l.kube.Close()
	}
	l.db.Close()
	if l.alpn != nil {
		l.alpn.Close()
	}
	if l.reverseTunnelALPN != nil {
		l.reverseTunnelALPN.Close()
	}
	if l.proxyPeer != nil {
		l.proxyPeer.Close()
	}
	if l.grpcPublic != nil {
		l.grpcPublic.Close()
	}
	if l.grpcMTLS != nil {
		l.grpcMTLS.Close()
	}
	if l.reverseTunnelMux != nil {
		l.reverseTunnelMux.Close()
	}
	if l.minimalWeb != nil {
		l.minimalWeb.Close()
	}
	if l.minimalTLS != nil {
		l.minimalTLS.Close()
	}
}

// dbListeners groups database access listeners.
type dbListeners struct {
	// postgres serves Postgres clients.
	postgres net.Listener
	// mysql serves MySQL clients.
	mysql net.Listener
	// mongo serves Mongo clients.
	mongo net.Listener
	// tls serves database clients that use plain TLS handshake.
	tls net.Listener
}

// Empty returns true if no database access listeners are initialized.
func (l *dbListeners) Empty() bool {
	return l.postgres == nil && l.mysql == nil && l.tls == nil && l.mongo == nil
}

// Close closes all database access listeners.
func (l *dbListeners) Close() {
	if l.postgres != nil {
		l.postgres.Close()
	}
	if l.mysql != nil {
		l.mysql.Close()
	}
	if l.tls != nil {
		l.tls.Close()
	}
	if l.mongo != nil {
		l.mongo.Close()
	}
}

// setupProxyListeners sets up web proxy listeners based on the configuration
func (process *TeleportProcess) setupProxyListeners(networkingConfig types.ClusterNetworkingConfig, accessPoint authclient.ProxyAccessPoint, clusterName string) (*proxyListeners, error) {
	cfg := process.Config
	process.log.Debugf("Setup Proxy: Web Proxy Address: %v, Reverse Tunnel Proxy Address: %v", cfg.Proxy.WebAddr.Addr, cfg.Proxy.ReverseTunnelListenAddr.Addr)
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

	if cfg.Proxy.Kube.Enabled && !cfg.Proxy.Kube.ListenAddr.IsEmpty() {
		process.log.Debugf("Setup Proxy: turning on Kubernetes proxy.")
		listener, err := process.importOrCreateListener(ListenerProxyKube, cfg.Proxy.Kube.ListenAddr.Addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		listeners.kube = listener
	}

	if !cfg.Proxy.DisableDatabaseProxy {
		if !cfg.Proxy.MySQLAddr.IsEmpty() {
			process.log.Debugf("Setup Proxy: MySQL proxy address: %v.", cfg.Proxy.MySQLAddr.Addr)
			listener, err := process.importOrCreateListener(ListenerProxyMySQL, cfg.Proxy.MySQLAddr.Addr)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			listeners.db.mysql = listener
		}

		if !cfg.Proxy.MongoAddr.IsEmpty() {
			process.log.Debugf("Setup Proxy: Mongo proxy address: %v.", cfg.Proxy.MongoAddr.Addr)
			listener, err := process.importOrCreateListener(ListenerProxyMongo, cfg.Proxy.MongoAddr.Addr)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			listeners.db.mongo = listener
		}

		if !cfg.Proxy.PostgresAddr.IsEmpty() {
			process.log.Debugf("Setup Proxy: Postgres proxy address: %v.", cfg.Proxy.PostgresAddr.Addr)
			listener, err := process.importOrCreateListener(ListenerProxyPostgres, cfg.Proxy.PostgresAddr.Addr)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			listeners.db.postgres = listener
		}

	}

	tunnelStrategy, err := networkingConfig.GetTunnelStrategyType()
	if err != nil {
		process.log.WithError(err).Warn("Failed to get tunnel strategy. Falling back to agent mesh strategy.")
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
		process.log.Debugf("Setup Proxy: Reverse tunnel proxy and web proxy are disabled.")
		return &listeners, nil
	case cfg.Proxy.ReverseTunnelListenAddr == cfg.Proxy.WebAddr && !cfg.Proxy.DisableTLS:
		process.log.Debugf("Setup Proxy: Reverse tunnel proxy and web proxy listen on the same port, multiplexing is on.")
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
		process.muxPostgresOnWebPort(cfg, &listeners)
		if !cfg.Proxy.DisableReverseTunnel {
			listeners.reverseTunnel = listeners.mux.SSH()
		}
		go func() {
			if err := listeners.mux.Serve(); err != nil && !utils.IsOKNetworkError(err) {
				listeners.mux.Entry.WithError(err).Error("Mux encountered err serving")
			}
		}()
		return &listeners, nil
	case cfg.Proxy.PROXYProtocolMode != multiplexer.PROXYProtocolOff && !cfg.Proxy.DisableWebService && !cfg.Proxy.DisableTLS:
		process.log.Debug("Setup Proxy: PROXY protocol is enabled for web service, multiplexing is on.")
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
		process.muxPostgresOnWebPort(cfg, &listeners)
		if !cfg.Proxy.ReverseTunnelListenAddr.IsEmpty() {
			if err := process.initMinimalReverseTunnelListener(cfg, &listeners); err != nil {
				listener.Close()
				listeners.Close()
				return nil, trace.Wrap(err)
			}
		}
		go func() {
			if err := listeners.mux.Serve(); err != nil && !utils.IsOKNetworkError(err) {
				listeners.mux.Entry.WithError(err).Error("Mux encountered err serving")
			}
		}()
		return &listeners, nil
	default:
		process.log.Debug("Setup Proxy: Proxy and reverse tunnel are listening on separate ports.")
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
				process.log.Debug("Setup Proxy: Multiplexing web and database proxy on the same port.")
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
				process.muxPostgresOnWebPort(cfg, &listeners)
				go func() {
					if err := listeners.mux.Serve(); err != nil && !utils.IsOKNetworkError(err) {
						listeners.mux.Entry.WithError(err).Error("Mux encountered err serving")
					}
				}()
			} else {
				process.log.Debug("Setup Proxy: TLS is disabled, multiplexing is off.")
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
			process.log.WithError(err).Debug("Minimal reverse tunnel mux exited with error")
		}
	}()
	listeners.minimalWeb = listeners.reverseTunnelMux.TLS()
	return nil
}

// muxPostgresOnWebPort starts Postgres proxy listener multiplexed on Teleport Proxy web port,
// unless postgres_listen_addr was specified.
func (process *TeleportProcess) muxPostgresOnWebPort(cfg *servicecfg.Config, listeners *proxyListeners) {
	if !cfg.Proxy.DisableDatabaseProxy && cfg.Proxy.PostgresAddr.IsEmpty() {
		listeners.db.postgres = listeners.mux.DB()
	}
}

func (process *TeleportProcess) initProxyEndpoint(conn *Connector) error {
	// clean up unused descriptors passed for proxy, but not used by it
	defer func() {
		if err := process.closeImportedDescriptors(teleport.ComponentProxy); err != nil {
			process.log.Warnf("Failed closing imported file descriptors: %v", err)
		}
	}()
	var err error
	cfg := process.Config
	var tlsConfigWeb *tls.Config

	clusterName := conn.ServerIdentity.ClusterName

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

	clientTLSConfig, err := conn.ClientIdentity.TLSConfig(cfg.CipherSuites)
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

	log := process.log.WithFields(logrus.Fields{
		trace.Component: teleport.Component(teleport.ComponentReverseTunnelServer, process.id),
	})

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
			Log:       process.log.WithField(trace.Component, teleport.ComponentProxy),
			Client:    conn.Client,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	nodeWatcher, err := services.NewNodeWatcher(process.ExitContext(), services.NodeWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:    teleport.ComponentProxy,
			Log:          process.log.WithField(trace.Component, teleport.ComponentProxy),
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
			Log:       process.log.WithField(trace.Component, teleport.ComponentProxy),
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

	serverTLSConfig, err := conn.ServerIdentity.TLSConfig(cfg.CipherSuites)
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
	proxySigner, err := process.getPROXYSigner(conn.ServerIdentity)
	if err != nil {
		return trace.Wrap(err)
	}

	// register SSH reverse tunnel server that accepts connections
	// from remote teleport nodes
	var tsrv reversetunnelclient.Server
	var peerClient *peer.Client

	if !process.Config.Proxy.DisableReverseTunnel {
		if listeners.proxyPeer != nil {
			peerClient, err = peer.NewClient(peer.ClientConfig{
				Context:     process.ExitContext(),
				ID:          process.Config.HostUUID,
				AuthClient:  conn.Client,
				AccessPoint: accessPoint,
				TLSConfig:   clientTLSConfig,
				Log:         process.log,
				Clock:       process.Clock,
				ClusterName: clusterName,
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
				Context:                       process.ExitContext(),
				Component:                     teleport.Component(teleport.ComponentProxy, process.id),
				ID:                            process.Config.HostUUID,
				ClusterName:                   clusterName,
				ClientTLS:                     clientTLSConfig,
				Listener:                      rtListener,
				HostSigners:                   []ssh.Signer{conn.ServerIdentity.KeySigner},
				LocalAuthClient:               conn.Client,
				LocalAccessPoint:              accessPoint,
				NewCachingAccessPoint:         process.newLocalCacheForRemoteProxy,
				NewCachingAccessPointOldProxy: process.newLocalCacheForOldRemoteProxy,
				Limiter:                       reverseTunnelLimiter,
				KeyGen:                        cfg.Keygen,
				Ciphers:                       cfg.Ciphers,
				KEXAlgorithms:                 cfg.KEXAlgorithms,
				MACAlgorithms:                 cfg.MACAlgorithms,
				DataDir:                       process.Config.DataDir,
				PollingPeriod:                 process.Config.PollingPeriod,
				FIPS:                          cfg.FIPS,
				Emitter:                       streamEmitter,
				Log:                           process.log,
				LockWatcher:                   lockWatcher,
				PeerClient:                    peerClient,
				NodeWatcher:                   nodeWatcher,
				CertAuthorityWatcher:          caWatcher,
				CircuitBreakerConfig:          process.Config.CircuitBreakerConfig,
				LocalAuthAddresses:            utils.NetAddrsToStrings(process.Config.AuthServerAddresses()),
				IngressReporter:               ingressReporter,
				PROXYSigner:                   proxySigner,
			})
		if err != nil {
			return trace.Wrap(err)
		}
		process.RegisterCriticalFunc("proxy.reversetunnel.server", func() error {
			log.Infof("Starting %s:%s on %v using %v", teleport.Version, teleport.Gitref, cfg.Proxy.ReverseTunnelListenAddr.Addr, process.Config.CachePolicy)
			if err := tsrv.Start(); err != nil {
				log.Error(err)
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
			ClusterName:         clusterName,
			Log:                 process.log.WithField(trace.Component, "router"),
			RemoteClusterGetter: accessPoint,
			SiteGetter:          tsrv,
			TracerProvider:      process.TracingProvider,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		proxyRouter = router
	}

	// read the host UUID:
	serverID, err := utils.ReadOrMakeHostUUID(cfg.DataDir)
	if err != nil {
		return trace.Wrap(err)
	}

	sessionController, err := srv.NewSessionController(srv.SessionControllerConfig{
		Semaphores:     accessPoint,
		AccessPoint:    accessPoint,
		LockEnforcer:   lockWatcher,
		Emitter:        asyncEmitter,
		Component:      teleport.ComponentProxy,
		Logger:         process.log.WithField(trace.Component, "sessionctrl"),
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

	if !process.Config.Proxy.DisableWebService {
		var fs http.FileSystem
		if !process.Config.Proxy.DisableWebInterface {
			fs, err = newHTTPFileSystem()
			if err != nil {
				return trace.Wrap(err)
			}
		}

		proxySettings := &proxySettings{
			cfg:          cfg,
			proxySSHAddr: proxySSHAddr,
			accessPoint:  accessPoint,
		}

		proxyKubeAddr := cfg.Proxy.Kube.ListenAddr
		if len(cfg.Proxy.Kube.PublicAddrs) > 0 {
			proxyKubeAddr = cfg.Proxy.Kube.PublicAddrs[0]
		}

		traceClt := tracing.NewNoopClient()
		if cfg.Tracing.Enabled {
			traceConf, err := process.Config.Tracing.Config()
			if err != nil {
				return trace.Wrap(err)
			}
			traceConf.Logger = process.log.WithField(trace.Component, teleport.ComponentTracing)

			clt, err := tracing.NewStartedClient(process.ExitContext(), *traceConf)
			if err != nil {
				return trace.Wrap(err)
			}

			traceClt = clt
		}

		var accessGraphAddr utils.NetAddr
		if cfg.AccessGraph.Enabled {
			addr, err := utils.ParseAddr(cfg.AccessGraph.Addr)
			if err != nil {
				return trace.Wrap(err)
			}
			accessGraphAddr = *addr
		}

		webConfig := web.Config{
			Proxy:            tsrv,
			AuthServers:      cfg.AuthServerAddresses()[0],
			DomainName:       cfg.Hostname,
			ProxyClient:      conn.Client,
			ProxySSHAddr:     proxySSHAddr,
			ProxyWebAddr:     cfg.Proxy.WebAddr,
			ProxyPublicAddrs: cfg.Proxy.PublicAddrs,
			CipherSuites:     cfg.CipherSuites,
			FIPS:             cfg.FIPS,
			AccessPoint:      accessPoint,
			Emitter:          asyncEmitter,
			PluginRegistry:   process.PluginRegistry,
			HostUUID:         process.Config.HostUUID,
			Context:          process.GracefulExitContext(),
			StaticFS:         fs,
			ClusterFeatures:  process.GetClusterFeatures(),
			GetProxyIdentity: func() (*state.Identity, error) {
				return process.GetIdentity(types.RoleProxy)
			},
			UI:              cfg.Proxy.UI,
			ProxySettings:   proxySettings,
			PublicProxyAddr: process.proxyPublicAddr().Addr,
			ALPNHandler:     alpnHandlerForWeb.HandleConnection,
			ProxyKubeAddr:   proxyKubeAddr,
			TraceClient:     traceClt,
			Router:          proxyRouter,
			SessionControl: web.SessionControllerFunc(func(ctx context.Context, sctx *web.SessionContext, login, localAddr, remoteAddr string) (context.Context, error) {
				controller := srv.WebSessionController(sessionController)
				ctx, err := controller(ctx, sctx, login, localAddr, remoteAddr)
				return ctx, trace.Wrap(err)
			}),
			PROXYSigner:               proxySigner,
			OpenAIConfig:              cfg.Testing.OpenAIConfig,
			NodeWatcher:               nodeWatcher,
			AccessGraphAddr:           accessGraphAddr,
			TracerProvider:            process.TracingProvider,
			AutomaticUpgradesChannels: cfg.Proxy.AutomaticUpgradesChannels,
		}
		webHandler, err := web.NewHandler(webConfig)
		if err != nil {
			return trace.Wrap(err)
		}
		if !cfg.Proxy.DisableTLS && cfg.Proxy.DisableALPNSNIListener {
			listeners.tls, err = multiplexer.NewWebListener(multiplexer.WebListenerConfig{
				Listener: tls.NewListener(listeners.web, tlsConfigWeb),
			})
			if err != nil {
				return trace.Wrap(err)
			}
			listeners.web = listeners.tls.Web()
			listeners.db.tls = listeners.tls.DB()

			process.RegisterCriticalFunc("proxy.tls", func() error {
				log.Infof("TLS multiplexer is starting on %v.", cfg.Proxy.WebAddr.Addr)
				if err := listeners.tls.Serve(); !trace.IsConnectionProblem(err) {
					log.WithError(err).Warn("TLS multiplexer error.")
				}
				log.Info("TLS multiplexer exited.")
				return nil
			})
		}

		webServer, err = web.NewServer(web.ServerConfig{
			Server: &http.Server{
				Handler: utils.ChainHTTPMiddlewares(
					webHandler,
					makeXForwardedForMiddleware(cfg),
					limiter.MakeMiddleware(proxyLimiter),
					httplib.MakeTracingMiddleware(teleport.ComponentProxy),
				),
				// Note: read/write timeouts *should not* be set here because it
				// will break some application access use-cases.
				ReadHeaderTimeout: defaults.ReadHeadersTimeout,
				IdleTimeout:       apidefaults.DefaultIdleTimeout,
				ErrorLog:          utils.NewStdlogger(log.Error, teleport.ComponentProxy),
				ConnState:         ingress.HTTPConnStateReporter(ingress.Web, ingressReporter),
				ConnContext: func(ctx context.Context, c net.Conn) context.Context {
					return authz.ContextWithClientAddrs(ctx, c.RemoteAddr(), c.LocalAddr())
				},
			},
			Handler: webHandler,
			Log:     log,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		process.RegisterCriticalFunc("proxy.web", func() error {
			log.Infof("Web proxy service %s:%s is starting on %v.", teleport.Version, teleport.Gitref, cfg.Proxy.WebAddr.Addr)
			defer webHandler.Close()
			process.BroadcastEvent(Event{Name: ProxyWebServerReady, Payload: webHandler})
			if err := webServer.Serve(listeners.web); err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, http.ErrServerClosed) {
				log.Warningf("Error while serving web requests: %v", err)
			}
			log.Info("Exited.")
			return nil
		})

		if listeners.reverseTunnelMux != nil {
			if minimalWebServer, err = process.initMinimalReverseTunnel(listeners, tlsConfigWeb, cfg, webConfig, log); err != nil {
				return trace.Wrap(err)
			}
		}
	} else {
		log.Info("Web UI is disabled.")
	}

	// Register ALPN handler that will be accepting connections for plain
	// TCP applications.
	if alpnRouter != nil {
		alpnRouter.Add(alpnproxy.HandlerDecs{
			MatchFunc: alpnproxy.MatchByProtocol(alpncommon.ProtocolTCP),
			Handler:   webServer.HandleConnection,
		})
	}

	var peerAddrString string
	var proxyServer *peer.Server
	if !process.Config.Proxy.DisableReverseTunnel && listeners.proxyPeer != nil {
		peerAddr, err := process.Config.Proxy.PublicPeerAddr()
		if err != nil {
			return trace.Wrap(err)
		}
		peerAddrString = peerAddr.String()
		proxyServer, err = peer.NewServer(peer.ServerConfig{
			AccessCache:   accessPoint,
			Listener:      listeners.proxyPeer,
			TLSConfig:     serverTLSConfig,
			ClusterDialer: clusterdial.NewClusterDialer(tsrv),
			Log:           log,
			ClusterName:   clusterName,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		process.RegisterCriticalFunc("proxy.peer", func() error {
			if _, err := process.WaitForEvent(process.ExitContext(), ProxyReverseTunnelReady); err != nil {
				log.Debugf("Process exiting: failed to start peer proxy service waiting for reverse tunnel server")
				return nil
			}

			log.Infof("Peer proxy service is starting on %s", listeners.proxyPeer.Addr().String())
			err := proxyServer.Serve()
			if err != nil {
				return trace.Wrap(err)
			}

			return nil
		})
	}

	staticLabels := make(map[string]string, 2)
	if cfg.Proxy.ProxyGroupID != "" {
		staticLabels[types.ProxyGroupIDLabel] = cfg.Proxy.ProxyGroupID
	}
	if cfg.Proxy.ProxyGroupGeneration != 0 {
		staticLabels[types.ProxyGroupGenerationLabel] = strconv.FormatUint(cfg.Proxy.ProxyGroupGeneration, 10)
	}
	if len(staticLabels) > 0 {
		log.Infof("Enabling proxy group labels: group ID = %q, generation = %v.", cfg.Proxy.ProxyGroupID, cfg.Proxy.ProxyGroupGeneration)
	}

	sshProxy, err := regular.New(
		process.ExitContext(),
		cfg.SSH.Addr,
		cfg.Hostname,
		[]ssh.Signer{conn.ServerIdentity.KeySigner},
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
		ClusterName: clusterName,
		AccessPoint: accessPoint,
		LockWatcher: lockWatcher,
		Logger:      log,
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

	tlscfg := serverTLSConfig.Clone()
	tlscfg.ClientAuth = tls.RequireAndVerifyClientCert
	if lib.IsInsecureDevMode() {
		tlscfg.InsecureSkipVerify = true
		tlscfg.ClientAuth = tls.RequireAnyClientCert
	}

	// clientTLSConfigGenerator pre-generates specialized per-cluster client TLS config values
	clientTLSConfigGenerator, err := auth.NewClientTLSConfigGenerator(auth.ClientTLSConfigGeneratorConfig{
		TLS:                  tlscfg.Clone(),
		ClusterName:          clusterName,
		PermitRemoteClusters: true,
		AccessPoint:          accessPoint,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	tlscfg.GetConfigForClient = clientTLSConfigGenerator.GetConfigForClient

	creds, err := auth.NewTransportCredentials(auth.TransportCredentialsConfig{
		TransportCredentials: credentials.NewTLS(tlscfg),
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
		grpc.Creds(creds),
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
		Logger: process.log.WithField(trace.Component, "transport"),
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
		log.Infof("SSH proxy service %s:%s is starting on %v", teleport.Version, teleport.Gitref, sshListenerAddr)

		// start ssh server
		go func() {
			listener, err := proxyLimiter.WrapListener(listeners.ssh)
			if err != nil {
				log.WithError(err).Error("Failed to set up SSH proxy server", "error")
				return
			}
			if err := sshProxy.Serve(listener); err != nil && !utils.IsOKNetworkError(err) {
				log.WithError(err).Error("SSH proxy server terminated unexpectedly", "error")
			}
		}()

		// start grpc server
		go func() {
			listener, err := proxyLimiter.WrapListener(listeners.sshGRPC)
			if err != nil {
				log.WithError(err).Error("Failed to set up SSH proxy server", "error")
				return
			}
			if err := sshGRPCServer.Serve(listener); err != nil && !utils.IsOKNetworkError(err) && !errors.Is(err, grpc.ErrServerStopped) {
				log.WithError(err).Error("SSH gRPC server terminated unexpectedly", "error")
			}
		}()

		// broadcast that the proxy ssh server has started
		process.BroadcastEvent(Event{Name: ProxySSHReady, Payload: nil})
		return nil
	})

	rcWatchLog := logrus.WithFields(logrus.Fields{
		trace.Component: teleport.Component(teleport.ComponentReverseTunnelAgent, process.id),
	})

	// Create and register reverse tunnel AgentPool.
	rcWatcher, err := reversetunnel.NewRemoteClusterTunnelManager(reversetunnel.RemoteClusterTunnelManagerConfig{
		HostUUID:            conn.ServerIdentity.ID.HostUUID,
		AuthClient:          conn.Client,
		AccessPoint:         accessPoint,
		HostSigner:          conn.ServerIdentity.KeySigner,
		LocalCluster:        clusterName,
		KubeDialAddr:        utils.DialAddrFromListenAddr(kubeDialAddr(cfg.Proxy, clusterNetworkConfig.GetProxyListenerMode())),
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

	var kubeServer *kubeproxy.TLSServer
	if listeners.kube != nil && !process.Config.Proxy.DisableReverseTunnel {
		authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
			ClusterName: clusterName,
			AccessPoint: accessPoint,
			LockWatcher: lockWatcher,
			Logger:      log,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		// Register TLS endpoint of the Kube proxy service
		tlsConfig, err := conn.ServerIdentity.TLSConfig(cfg.CipherSuites)
		if err != nil {
			return trace.Wrap(err)
		}
		component := teleport.Component(teleport.ComponentProxy, teleport.ComponentProxyKube)
		kubeServiceType := kubeproxy.ProxyService
		if cfg.Proxy.Kube.LegacyKubeProxy {
			kubeServiceType = kubeproxy.LegacyProxyService
		}

		// kubeServerWatcher is used to watch for changes in the Kubernetes servers
		// and feed them to the kube proxy server so it can route the requests to
		// the correct kubernetes server.
		kubeServerWatcher, err := services.NewKubeServerWatcher(process.ExitContext(), services.KubeServerWatcherConfig{
			ResourceWatcherConfig: services.ResourceWatcherConfig{
				Component: component,
				Log:       log,
				Client:    accessPoint,
			},
		})
		if err != nil {
			return trace.Wrap(err)
		}

		kubeServer, err = kubeproxy.NewTLSServer(kubeproxy.TLSServerConfig{
			ForwarderConfig: kubeproxy.ForwarderConfig{
				Namespace:                     apidefaults.Namespace,
				Keygen:                        cfg.Keygen,
				ClusterName:                   clusterName,
				ReverseTunnelSrv:              tsrv,
				Authz:                         authorizer,
				AuthClient:                    conn.Client,
				Emitter:                       asyncEmitter,
				DataDir:                       cfg.DataDir,
				CachingAuthClient:             accessPoint,
				HostID:                        cfg.HostUUID,
				ClusterOverride:               cfg.Proxy.Kube.ClusterOverride,
				KubeconfigPath:                cfg.Proxy.Kube.KubeconfigPath,
				Component:                     component,
				KubeServiceType:               kubeServiceType,
				LockWatcher:                   lockWatcher,
				CheckImpersonationPermissions: cfg.Kube.CheckImpersonationPermissions,
				PROXYSigner:                   proxySigner,
				// ConnTLSConfig is used by the proxy authenticate to the upstream kubernetes
				// services or remote clustes to be able to send the client identity
				// using Impersonation headers. The upstream service will validate if
				// the provided connection certificate is from a proxy server and
				// will impersonate the identity of the user that is making the request.
				ConnTLSConfig:   tlsConfig.Clone(),
				ClusterFeatures: process.GetClusterFeatures,
			},
			TLS:                      tlsConfig.Clone(),
			LimiterConfig:            cfg.Proxy.Limiter,
			AccessPoint:              accessPoint,
			GetRotation:              process.GetRotation,
			OnHeartbeat:              process.OnHeartbeat(component),
			Log:                      log,
			IngressReporter:          ingressReporter,
			KubernetesServersWatcher: kubeServerWatcher,
			PROXYProtocolMode:        cfg.Proxy.PROXYProtocolMode,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		process.RegisterCriticalFunc("proxy.kube", func() error {
			log := logrus.WithFields(logrus.Fields{
				trace.Component: component,
			})

			kubeListenAddr := listeners.kube.Addr().String()
			if cfg.Proxy.Kube.ListenAddr.Addr != "" {
				kubeListenAddr = cfg.Proxy.Kube.ListenAddr.Addr
			}
			log.Infof("Starting Kube proxy on %v.", kubeListenAddr)

			var mopts []kubeproxy.ServeOption
			if cfg.Testing.KubeMultiplexerIgnoreSelfConnections {
				mopts = append(mopts, kubeproxy.WithMultiplexerIgnoreSelfConnections())
			}

			err := kubeServer.Serve(listeners.kube, mopts...)
			if err != nil && err != http.ErrServerClosed {
				log.Warningf("Kube TLS server exited with error: %v.", err)
			}
			return nil
		})
	}

	// Start the database proxy server that will be accepting connections from
	// the database clients (such as psql or mysql), authenticating them, and
	// then routing them to a respective database server over the reverse tunnel
	// framework.
	if (!listeners.db.Empty() || alpnRouter != nil) && !process.Config.Proxy.DisableReverseTunnel {
		authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
			ClusterName: clusterName,
			AccessPoint: accessPoint,
			LockWatcher: lockWatcher,
			Logger:      log,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		tlsConfig, err := conn.ServerIdentity.TLSConfig(cfg.CipherSuites)
		if err != nil {
			return trace.Wrap(err)
		}
		connLimiter, err := limiter.NewLimiter(process.Config.Databases.Limiter)
		if err != nil {
			return trace.Wrap(err)
		}

		connMonitor, err := srv.NewConnectionMonitor(srv.ConnectionMonitorConfig{
			AccessPoint:    accessPoint,
			LockWatcher:    lockWatcher,
			Clock:          process.Config.Clock,
			ServerID:       process.Config.HostUUID,
			Emitter:        asyncEmitter,
			EmitterContext: process.ExitContext(),
			Logger:         process.log,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		dbProxyServer, err := db.NewProxyServer(process.ExitContext(),
			db.ProxyServerConfig{
				AuthClient:         conn.Client,
				AccessPoint:        accessPoint,
				Authorizer:         authorizer,
				Tunnel:             tsrv,
				TLSConfig:          tlsConfig,
				Limiter:            connLimiter,
				IngressReporter:    ingressReporter,
				ConnectionMonitor:  connMonitor,
				MySQLServerVersion: process.Config.Proxy.MySQLServerVersion,
			})
		if err != nil {
			return trace.Wrap(err)
		}

		if alpnRouter != nil && !cfg.Proxy.DisableDatabaseProxy {
			alpnRouter.Add(alpnproxy.HandlerDecs{
				MatchFunc:           alpnproxy.MatchByALPNPrefix(string(alpncommon.ProtocolMySQL)),
				HandlerWithConnInfo: alpnproxy.ExtractMySQLEngineVersion(dbProxyServer.MySQLProxy().HandleConnection),
			})
			alpnRouter.Add(alpnproxy.HandlerDecs{
				MatchFunc: alpnproxy.MatchByProtocol(alpncommon.ProtocolMySQL),
				Handler:   dbProxyServer.MySQLProxy().HandleConnection,
			})
			alpnRouter.Add(alpnproxy.HandlerDecs{
				MatchFunc: alpnproxy.MatchByProtocol(alpncommon.ProtocolPostgres),
				Handler:   dbProxyServer.PostgresProxy().HandleConnection,
			})
			alpnRouter.Add(alpnproxy.HandlerDecs{
				// For the following protocols ALPN Proxy will handle the
				// connection internally (terminate wrapped TLS traffic) and
				// route extracted connection to ALPN Proxy DB TLS Handler.
				MatchFunc: alpnproxy.MatchByProtocol(
					alpncommon.ProtocolMongoDB,
					alpncommon.ProtocolOracle,
					alpncommon.ProtocolRedisDB,
					alpncommon.ProtocolSnowflake,
					alpncommon.ProtocolSQLServer,
					alpncommon.ProtocolCassandra),
			})
		}

		log := process.log.WithField(trace.Component, teleport.Component(teleport.ComponentDatabase))
		if listeners.db.postgres != nil {
			process.RegisterCriticalFunc("proxy.db.postgres", func() error {
				log.Infof("Starting Database Postgres proxy server on %v.", listeners.db.postgres.Addr())
				if err := dbProxyServer.ServePostgres(listeners.db.postgres); err != nil {
					log.WithError(err).Warn("Postgres proxy server exited with error.")
				}
				return nil
			})
		}
		if listeners.db.mysql != nil {
			process.RegisterCriticalFunc("proxy.db.mysql", func() error {
				log.Infof("Starting MySQL proxy server on %v.", cfg.Proxy.MySQLAddr.Addr)
				if err := dbProxyServer.ServeMySQL(listeners.db.mysql); err != nil {
					log.WithError(err).Warn("MySQL proxy server exited with error.")
				}
				return nil
			})
		}
		if listeners.db.tls != nil {
			process.RegisterCriticalFunc("proxy.db.tls", func() error {
				log.Infof("Starting Database TLS proxy server on %v.", cfg.Proxy.WebAddr.Addr)
				if err := dbProxyServer.ServeTLS(listeners.db.tls); err != nil {
					log.WithError(err).Warn("Database TLS proxy server exited with error.")
				}
				return nil
			})
		}

		if listeners.db.mongo != nil {
			process.RegisterCriticalFunc("proxy.db.mongo", func() error {
				log.Infof("Starting Database Mongo proxy server on %v.", cfg.Proxy.MongoAddr.Addr)
				if err := dbProxyServer.ServeMongo(listeners.db.mongo, tlsConfigWeb.Clone()); err != nil {
					log.WithError(err).Warn("Database Mongo proxy server exited with error.")
				}
				return nil
			})
		}
	}

	var (
		grpcServerPublic *grpc.Server
		grpcServerMTLS   *grpc.Server
	)
	if alpnRouter != nil {
		grpcServerPublic = process.initPublicGRPCServer(proxyLimiter, conn, listeners.grpcPublic)

		grpcServerMTLS, err = process.initSecureGRPCServer(
			initSecureGRPCServerCfg{
				limiter:     proxyLimiter,
				conn:        conn,
				listener:    listeners.grpcMTLS,
				accessPoint: accessPoint,
				lockWatcher: lockWatcher,
				emitter:     asyncEmitter,
			},
		)
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
		identityTLSConf, err := conn.ServerIdentity.TLSConfig(cfg.CipherSuites)
		if err != nil {
			return trace.Wrap(err)
		}
		alpnServer, err = alpnproxy.New(alpnproxy.ProxyConfig{
			WebTLSConfig:      tlsConfigWeb.Clone(),
			IdentityTLSConfig: identityTLSConf,
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
			log.Infof("Starting TLS ALPN SNI proxy server on %v.", listeners.alpn.Addr())
			if err := alpnServer.Serve(process.ExitContext()); err != nil {
				log.WithError(err).Warn("TLS ALPN SNI proxy proxy server exited with error.")
			}
			return nil
		})

		if reverseTunnelALPNRouter != nil {
			reverseTunnelALPNServer, err = alpnproxy.New(alpnproxy.ProxyConfig{
				WebTLSConfig:      tlsConfigWeb.Clone(),
				IdentityTLSConfig: identityTLSConf,
				Router:            reverseTunnelALPNRouter,
				Listener:          listeners.reverseTunnelALPN,
				ClusterName:       clusterName,
				AccessPoint:       accessPoint,
			})
			if err != nil {
				return trace.Wrap(err)
			}

			process.RegisterCriticalFunc("proxy.tls.alpn.sni.proxy.reverseTunnel", func() error {
				log.Infof("Starting TLS ALPN SNI reverse tunnel proxy server on %v.", listeners.reverseTunnelALPN.Addr())
				if err := reverseTunnelALPNServer.Serve(process.ExitContext()); err != nil {
					log.WithError(err).Warn("TLS ALPN SNI proxy proxy on reverse tunnel server exited with error.")
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
			log.Infof("Shutting down immediately.")
			if tsrv != nil {
				warnOnErr(tsrv.Close(), log)
			}
			warnOnErr(rcWatcher.Close(), log)
			if proxyServer != nil {
				warnOnErr(proxyServer.Close(), log)
			}
			if webServer != nil {
				warnOnErr(webServer.Close(), log)
			}
			if minimalWebServer != nil {
				warnOnErr(minimalWebServer.Close(), log)
			}
			if peerClient != nil {
				warnOnErr(peerClient.Stop(), log)
			}
			warnOnErr(sshProxy.Close(), log)
			sshGRPCServer.Stop()
			if kubeServer != nil {
				warnOnErr(kubeServer.Close(), log)
			}
			if grpcServerPublic != nil {
				grpcServerPublic.Stop()
			}
			if grpcServerMTLS != nil {
				grpcServerMTLS.Stop()
			}
			if alpnServer != nil {
				warnOnErr(alpnServer.Close(), log)
			}
			if reverseTunnelALPNServer != nil {
				warnOnErr(reverseTunnelALPNServer.Close(), log)
			}

			if clientTLSConfigGenerator != nil {
				clientTLSConfigGenerator.Close()
			}
		} else {
			log.Infof("Shutting down gracefully.")
			ctx := payloadContext(payload, log)
			if tsrv != nil {
				warnOnErr(tsrv.DrainConnections(ctx), log)
			}
			warnOnErr(sshProxy.Shutdown(ctx), log)
			sshGRPCServer.GracefulStop()
			if webServer != nil {
				warnOnErr(webServer.Shutdown(ctx), log)
			}
			if minimalWebServer != nil {
				warnOnErr(minimalWebServer.Shutdown(ctx), log)
			}
			if tsrv != nil {
				warnOnErr(tsrv.Shutdown(ctx), log)
			}
			warnOnErr(rcWatcher.Close(), log)
			if proxyServer != nil {
				warnOnErr(proxyServer.Shutdown(), log)
			}
			if peerClient != nil {
				peerClient.Shutdown(ctx)
			}
			if kubeServer != nil {
				warnOnErr(kubeServer.Shutdown(ctx), log)
			}
			if grpcServerPublic != nil {
				grpcServerPublic.GracefulStop()
			}
			if grpcServerMTLS != nil {
				grpcServerMTLS.GracefulStop()
			}
			if alpnServer != nil {
				warnOnErr(alpnServer.Close(), log)
			}
			if reverseTunnelALPNServer != nil {
				warnOnErr(reverseTunnelALPNServer.Close(), log)
			}

			// Explicitly deleting proxy heartbeats helps the behavior of
			// reverse tunnel agents during rollouts, as otherwise they'll keep
			// trying to reach proxies until the heartbeats expire.
			if services.ShouldDeleteServerHeartbeatsOnShutdown(ctx) {
				if err := conn.Client.DeleteProxy(ctx, process.Config.HostUUID); err != nil {
					if !trace.IsNotFound(err) {
						log.WithError(err).Warn("Failed to delete heartbeat.")
					} else {
						log.WithError(err).Debug("Failed to delete heartbeat.")
					}
				}
			}

			if clientTLSConfigGenerator != nil {
				clientTLSConfigGenerator.Close()
			}
		}
		warnOnErr(asyncEmitter.Close(), log)
		warnOnErr(conn.Close(), log)
		log.Infof("Exited.")
	})

	return nil
}

func (process *TeleportProcess) getPROXYSigner(ident *state.Identity) (multiplexer.PROXYHeaderSigner, error) {
	signer, err := utils.ParsePrivateKeyPEM(ident.KeyBytes)
	if err != nil {
		return nil, trace.Wrap(err, "could not parse identity's private key")
	}

	jwtSigner, err := services.GetJWTSigner(signer, ident.ClusterName, process.Clock)
	if err != nil {
		return nil, trace.Wrap(err, "could not create JWT signer")
	}

	proxySigner, err := multiplexer.NewPROXYSigner(ident.XCert, jwtSigner)
	if err != nil {
		return nil, trace.Wrap(err, "could not create PROXY signer")
	}
	return proxySigner, nil
}

func (process *TeleportProcess) initMinimalReverseTunnel(listeners *proxyListeners, tlsConfigWeb *tls.Config, cfg *servicecfg.Config, webConfig web.Config, log *logrus.Entry) (*web.Server, error) {
	internalListener := listeners.minimalWeb
	if !cfg.Proxy.DisableTLS {
		internalListener = tls.NewListener(internalListener, tlsConfigWeb)
	}

	minimalListener, err := multiplexer.NewWebListener(multiplexer.WebListenerConfig{
		Listener: internalListener,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	listeners.minimalTLS = minimalListener

	minimalProxyLimiter, err := limiter.NewLimiter(cfg.Proxy.Limiter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	webConfig.MinimalReverseTunnelRoutesOnly = true
	minimalWebHandler, err := web.NewHandler(webConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	minimalProxyLimiter.WrapHandle(minimalWebHandler)

	process.RegisterCriticalFunc("proxy.reversetunnel.tls", func() error {
		log.Infof("TLS multiplexer is starting on %v.", cfg.Proxy.ReverseTunnelListenAddr.Addr)
		if err := minimalListener.Serve(); !trace.IsConnectionProblem(err) {
			log.WithError(err).Warn("TLS multiplexer error.")
		}
		log.Info("TLS multiplexer exited.")
		return nil
	})

	minimalWebServer, err := web.NewServer(web.ServerConfig{
		Server: &http.Server{
			Handler:           httplib.MakeTracingHandler(minimalProxyLimiter, teleport.ComponentProxy),
			ReadTimeout:       apidefaults.DefaultIOTimeout,
			ReadHeaderTimeout: defaults.ReadHeadersTimeout,
			WriteTimeout:      apidefaults.DefaultIOTimeout,
			IdleTimeout:       apidefaults.DefaultIdleTimeout,
			ErrorLog:          utils.NewStdlogger(log.Error, teleport.ComponentReverseTunnelServer),
		},
		Handler: minimalWebHandler,
		Log:     log,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	process.RegisterCriticalFunc("proxy.reversetunnel.web", func() error {
		log.Infof("Minimal web proxy service %s:%s is starting on %v.", teleport.Version, teleport.Gitref, cfg.Proxy.ReverseTunnelListenAddr.Addr)
		defer minimalWebHandler.Close()
		if err := minimalWebServer.Serve(minimalListener.Web()); err != nil && err != http.ErrServerClosed {
			log.Warningf("Error while serving web requests: %v", err)
		}
		log.Info("Exited.")
		return nil
	})

	return minimalWebServer, nil
}

// kubeDialAddr returns Proxy Kube service address used for dialing local kube service
// by remote trusted cluster.
// If the proxy is running with Multiplex mode the WebPort is returned
// where connections are forwarded to kube service by ALPN SNI router.
func kubeDialAddr(config servicecfg.ProxyConfig, mode types.ProxyListenerMode) utils.NetAddr {
	if mode == types.ProxyListenerMode_Multiplex {
		return config.WebAddr
	}
	return config.Kube.ListenAddr
}

func (process *TeleportProcess) setupProxyTLSConfig(conn *Connector, tsrv reversetunnelclient.Server, accessPoint authclient.ReadProxyAccessPoint, clusterName string) (*tls.Config, error) {
	cfg := process.Config
	var tlsConfig *tls.Config
	acmeCfg := process.Config.Proxy.ACME
	if acmeCfg.Enabled {
		process.Config.Log.Infof("Managing certs using ACME https://datatracker.ietf.org/doc/rfc8555/.")

		acmePath := filepath.Join(process.Config.DataDir, teleport.ComponentACME)
		if err := os.MkdirAll(acmePath, teleport.PrivateDirMode); err != nil {
			return nil, trace.ConvertSystemError(err)
		}
		hostChecker, err := newHostPolicyChecker(hostPolicyCheckerConfig{
			publicAddrs: process.Config.Proxy.PublicAddrs,
			clt:         conn.Client,
			tun:         tsrv,
			clusterName: conn.ServerIdentity.Cert.Extensions[utils.CertExtensionAuthority],
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

func (process *TeleportProcess) setupALPNTLSConfigForWeb(serverTLSConfig *tls.Config, accessPoint authclient.ReadProxyAccessPoint, clusterName string) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(process.Config.CipherSuites)
	tlsConfig.Certificates = serverTLSConfig.Certificates

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

	if cfg.Proxy.Kube.Enabled {
		kubeListener := alpnproxy.NewMuxListenerWrapper(listeners.kube, listeners.web)
		router.AddKubeHandler(kubeListener.HandleConnection)
		listeners.kube = kubeListener
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
	router.Add(alpnproxy.HandlerDecs{
		MatchFunc: alpnproxy.MatchByProtocol(alpncommon.ProtocolProxySSH),
		Handler:   sshProxyListener.HandleConnection,
		TLSConfig: serverTLSConfig,
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

	webTLSDB := alpnproxy.NewMuxListenerWrapper(nil, listeners.web)
	router.AddDBTLSHandler(webTLSDB.HandleConnection)
	listeners.db.tls = webTLSDB

	return router, rtRouter
}

// waitForAppDepend waits until all dependencies for an application service
// are ready.
func (process *TeleportProcess) waitForAppDepend() {
	for _, event := range appDependEvents {
		_, err := process.WaitForEvent(process.ExitContext(), event)
		if err != nil {
			process.log.Debugf("Process is exiting.")
			break
		}
	}
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

	if cfg.Kube.Enabled {
		process.SetExpectedInstanceRole(types.RoleKube, KubeIdentityEvent)
	}

	if cfg.Apps.Enabled {
		process.SetExpectedInstanceRole(types.RoleApp, AppsIdentityEvent)
	}

	if cfg.Databases.Enabled {
		process.SetExpectedInstanceRole(types.RoleDatabase, DatabasesIdentityEvent)
	}

	if cfg.WindowsDesktop.Enabled {
		process.SetExpectedInstanceRole(types.RoleWindowsDesktop, WindowsDesktopIdentityEvent)
	}

	if cfg.Discovery.Enabled {
		process.SetExpectedInstanceRole(types.RoleDiscovery, DiscoveryIdentityEvent)
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
	log := process.log.WithField(trace.Component, component)

	process.RegisterCriticalFunc("apps.start", func() error {
		conn, err := process.WaitForConnector(AppsIdentityEvent, log)
		if conn == nil {
			return trace.Wrap(err)
		}

		shouldSkipCleanup := false
		defer func() {
			if !shouldSkipCleanup {
				warnOnErr(conn.Close(), log)
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
			log.Debugf("Waiting for application service dependencies to start.")
			process.waitForAppDepend()
			log.Debugf("Application service dependencies have started, continuing.")
		}

		clusterName := conn.ServerIdentity.ClusterName

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
			})
			if err != nil {
				return trace.Wrap(err)
			}

			applications = append(applications, a)
		}

		lockWatcher, err := services.NewLockWatcher(process.ExitContext(), services.LockWatcherConfig{
			ResourceWatcherConfig: services.ResourceWatcherConfig{
				Component: teleport.ComponentApp,
				Log:       log,
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
			Logger:      log,
			// Device authorization breaks browser-based access.
			DisableDeviceAuthorization: true,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		tlsConfig, err := conn.ServerIdentity.TLSConfig(nil)
		if err != nil {
			return trace.Wrap(err)
		}

		asyncEmitter, err := process.NewAsyncEmitter(conn.Client)
		if err != nil {
			return trace.Wrap(err)
		}
		defer func() {
			if !shouldSkipCleanup {
				warnOnErr(asyncEmitter.Close(), log)
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

		appServer, err := app.New(process.ExitContext(), &app.Config{
			Clock:                process.Config.Clock,
			DataDir:              process.Config.DataDir,
			AuthClient:           conn.Client,
			AccessPoint:          accessPoint,
			Authorizer:           authorizer,
			TLSConfig:            tlsConfig,
			CipherSuites:         process.Config.CipherSuites,
			HostID:               process.Config.HostUUID,
			Hostname:             process.Config.Hostname,
			GetRotation:          process.GetRotation,
			Apps:                 applications,
			CloudLabels:          process.cloudLabels,
			ResourceMatchers:     process.Config.Apps.ResourceMatchers,
			OnHeartbeat:          process.OnHeartbeat(teleport.ComponentApp),
			ConnectedProxyGetter: proxyGetter,
			Emitter:              asyncEmitter,
			ConnectionMonitor:    connMonitor,
			InventoryHandle:      process.inventoryHandle,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		defer func() {
			if !shouldSkipCleanup {
				warnOnErr(appServer.Close(), log)
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
				HostUUID:             conn.ServerIdentity.ID.HostUUID,
				Resolver:             tunnelAddrResolver,
				Client:               conn.Client,
				Server:               appServer,
				AccessPoint:          accessPoint,
				HostSigner:           conn.ServerIdentity.KeySigner,
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
		log.Infof("All applications successfully started.")

		// Cancel deferred cleanup actions, because we're going
		// to register an OnExit handler to take care of it
		shouldSkipCleanup = true

		// Execute this when process is asked to exit.
		process.OnExit("apps.stop", func(payload interface{}) {
			if payload == nil {
				log.Infof("Shutting down immediately.")
				warnOnErr(appServer.Close(), log)
			} else {
				log.Infof("Shutting down gracefully.")
				warnOnErr(appServer.Shutdown(payloadContext(payload, log)), log)
			}
			if asyncEmitter != nil {
				warnOnErr(asyncEmitter.Close(), log)
			}
			agentPool.Stop()
			warnOnErr(asyncEmitter.Close(), log)
			warnOnErr(conn.Close(), log)
			log.Infof("Exited.")
		})

		// Block and wait while the server and agent pool are running.
		if err := appServer.Wait(); err != nil && !errors.Is(err, context.Canceled) {
			return trace.Wrap(err)
		}
		agentPool.Wait()
		return nil
	})
}

func warnOnErr(err error, log logrus.FieldLogger) {
	if err != nil {
		// don't warn on double close, happens sometimes when
		// calling accept on a closed listener
		if utils.IsOKNetworkError(err) {
			return
		}
		log.WithError(err).Warn("Got error while cleaning up.")
	}
}

// initAuthStorage initializes the storage backend for the auth service.
func (process *TeleportProcess) initAuthStorage() (bk backend.Backend, err error) {
	ctx := context.TODO()
	bc := &process.Config.Auth.StorageConfig
	process.log.Debugf("Using %v backend.", bc.Type)
	switch bc.Type {
	// SQLite backend (or alt name dir).
	case lite.GetName():
		bk, err = lite.New(ctx, bc.Params)
	// Firestore backend:
	case firestore.GetName():
		bk, err = firestore.New(ctx, bc.Params, firestore.Options{})
	// DynamoDB backend.
	case dynamo.GetName():
		bk, err = dynamo.New(ctx, bc.Params)
	// etcd backend.
	case etcdbk.GetName():
		bk, err = etcdbk.New(ctx, bc.Params)
	// PostgreSQL backend
	case pgbk.Name, pgbk.AltName:
		bk, err = pgbk.NewFromParams(ctx, bc.Params)
	default:
		err = trace.BadParameter("unsupported secrets storage type: %q", bc.Type)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reporter, err := backend.NewReporter(backend.ReporterConfig{
		Component: teleport.ComponentBackend,
		Backend:   backend.NewSanitizer(bk),
		Tracer:    process.TracingProvider.Tracer(teleport.ComponentBackend),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	process.setReporter(reporter)
	return reporter, nil
}

func (process *TeleportProcess) setReporter(reporter *backend.Reporter) {
	process.Lock()
	defer process.Unlock()
	process.reporter = reporter
}

// WaitWithContext waits until all internal services stop.
func (process *TeleportProcess) WaitWithContext(ctx context.Context) {
	local, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()
		if err := process.Supervisor.Wait(); err != nil {
			process.log.Warnf("Error waiting for all services to complete: %v", err)
		}
	}()

	<-local.Done()
}

// StartShutdown launches non-blocking graceful shutdown process that signals
// completion, returns context that will be closed once the shutdown is done
func (process *TeleportProcess) StartShutdown(ctx context.Context) context.Context {
	// by the time we get here we've already extracted the parent pipe, which is
	// the only potential imported file descriptor that's not a listening
	// socket, so closing every imported FD with a prefix of "" will close all
	// imported listeners that haven't been used so far
	warnOnErr(process.closeImportedDescriptors(""), process.log)
	warnOnErr(process.stopListeners(), process.log)

	if len(process.getForkedPIDs()) == 0 {
		if process.inventoryHandle != nil {
			if err := process.inventoryHandle.SendGoodbye(ctx); err != nil {
				process.log.WithError(err).Warn("Failed sending inventory goodbye during shutdown")
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
			process.log.Warnf("Error waiting for all services to complete: %v", err)
		}
		process.log.Debug("All supervisor functions are completed.")

		if localAuth := process.getLocalAuth(); localAuth != nil {
			if err := localAuth.Close(); err != nil {
				process.log.Warningf("Failed closing auth server: %v.", err)
			}
		}

		if process.storage != nil {
			if err := process.storage.Close(); err != nil {
				process.log.Warningf("Failed closing process storage: %v.", err)
			}
		}

		if process.inventoryHandle != nil {
			process.inventoryHandle.Close()
		}
	}()
	go process.printShutdownStatus(localCtx)
	return localCtx
}

// Shutdown launches graceful shutdown process and waits
// for it to complete
func (process *TeleportProcess) Shutdown(ctx context.Context) {
	localCtx := process.StartShutdown(ctx)
	// wait until parent context closes
	<-localCtx.Done()
	process.log.Debug("Process completed.")
}

// Close broadcasts close signals and exits immediately
func (process *TeleportProcess) Close() error {
	process.BroadcastEvent(Event{Name: TeleportExitEvent})

	process.Config.Keygen.Close()

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

// initSelfSignedHTTPSCert generates and self-signs a TLS key+cert pair for https connection
// to the proxy server.
func initSelfSignedHTTPSCert(cfg *servicecfg.Config) (err error) {
	cfg.Log.Warningf("No TLS Keys provided, using self-signed certificate.")

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
	cfg.Log.Warningf("Generating self-signed key and cert to %v %v.", keyPath, certPath)

	hosts := []string{cfg.Hostname, "localhost"}
	var ips []string

	// add web public address hosts to self-signed cert
	for _, addr := range cfg.Proxy.PublicAddrs {
		proxyHost, _, err := net.SplitHostPort(addr.String())
		if err != nil {
			// log and skip error since this is a nice to have
			cfg.Log.Warnf("Error parsing proxy.public_address %v, skipping adding to self-signed cert: %v", addr.String(), err)
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

// initDebugApp starts a debug server that dumpers request headers.
func (process *TeleportProcess) initDebugApp() {
	process.RegisterFunc("debug.app.service", func() error {
		server := httptest.NewServer(http.HandlerFunc(dumperHandler))
		process.BroadcastEvent(Event{Name: DebugAppReady, Payload: server})

		process.OnExit("debug.app.shutdown", func(payload interface{}) {
			server.Close()
			process.log.Infof("Exited.")
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

// newHTTPFileSystem creates a new HTTP file system for the web handler.
// It uses external configuration to make the decision
func newHTTPFileSystem() (http.FileSystem, error) {
	fs, err := teleport.NewWebAssetsFilesystem() //nolint:staticcheck // linter fails on non-linux system as only linux implementation returns useful values.
	if err != nil {                              //nolint:staticcheck // linter fails on non-linux system as only linux implementation returns useful values.
		return nil, trace.Wrap(err)
	}
	return fs, nil
}

// readOrGenerateHostID tries to read the `host_uuid` from Kubernetes storage (if available) or local storage.
// If the read operation returns no `host_uuid`, this function tries to pick it from the first static identity provided.
// If no static identities were defined for the process, a new id is generated depending on the joining process:
// - types.JoinMethodEC2: we will use the EC2 NodeID: {accountID}-{nodeID}
// - Any other valid Joining method: a new UUID is generated.
// Finally, if a new id is generated, this function writes it into local storage and Kubernetes storage (if available).
// If kubeBackend is nil, the agent is not running in a Kubernetes Cluster.
func readOrGenerateHostID(ctx context.Context, cfg *servicecfg.Config, kubeBackend kubernetesBackend) (err error) {
	// Load `host_uuid` from different storages. If this process is running in a Kubernetes Cluster,
	// readHostUUIDFromStorages will try to read the `host_uuid` from the Kubernetes Secret. If the
	// key is empty or if not running in a Kubernetes Cluster, it will read the
	// `host_uuid` from local data directory.
	cfg.HostUUID, err = readHostIDFromStorages(ctx, cfg.DataDir, kubeBackend)
	if err != nil {
		if !trace.IsNotFound(err) {
			if errors.Is(err, fs.ErrPermission) {
				cfg.Log.Errorf("Teleport does not have permission to write to: %v. Ensure that you are running as a user with appropriate permissions.", cfg.DataDir)
			}
			return trace.Wrap(err)
		}
		// if there's no host uuid initialized yet, try to read one from the
		// one of the identities
		if len(cfg.Identities) != 0 {
			cfg.HostUUID = cfg.Identities[0].ID.HostUUID
			cfg.Log.Infof("Taking host UUID from first identity: %v.", cfg.HostUUID)
		} else {
			switch cfg.JoinMethod {
			case types.JoinMethodToken,
				types.JoinMethodUnspecified,
				types.JoinMethodIAM,
				types.JoinMethodCircleCI,
				types.JoinMethodKubernetes,
				types.JoinMethodGitHub,
				types.JoinMethodGitLab,
				types.JoinMethodAzure,
				types.JoinMethodGCP,
				types.JoinMethodTPM:
				// Checking error instead of the usual uuid.New() in case uuid generation
				// fails due to not enough randomness. It's been known to happen happen when
				// Teleport starts very early in the node initialization cycle and /dev/urandom
				// isn't ready yet.
				rawID, err := uuid.NewRandom()
				if err != nil {
					return trace.BadParameter("" +
						"Teleport failed to generate host UUID. " +
						"This may happen if randomness source is not fully initialized when the node is starting up. " +
						"Please try restarting Teleport again.")
				}
				cfg.HostUUID = rawID.String()
			case types.JoinMethodEC2:
				cfg.HostUUID, err = utils.GetEC2NodeID(ctx)
				if err != nil {
					return trace.Wrap(err)
				}
			default:
				return trace.BadParameter("unknown join method %q", cfg.JoinMethod)
			}
			cfg.Log.Infof("Generating new host UUID: %v.", cfg.HostUUID)
		}
		// persistHostUUIDToStorages will persist the host_uuid to the local storage
		// and to Kubernetes Secret if this process is running on a Kubernetes Cluster.
		if err := persistHostIDToStorages(ctx, cfg, kubeBackend); err != nil {
			return trace.Wrap(err)
		}
	} else if kubeBackend != nil && utils.HostUUIDExistsLocally(cfg.DataDir) {
		// This case is used when loading a Teleport pre-11 agent with storage attached.
		// In this case, we have to copy the "host_uuid" from the agent to the secret
		// in case storage is removed later.
		// loadHostIDFromKubeSecret will check if the `host_uuid` is already in the secret.
		if id, err := loadHostIDFromKubeSecret(ctx, kubeBackend); err != nil || len(id) == 0 {
			// Forces the copy of the host_uuid into the Kubernetes Secret if PV storage is enabled.
			// This is only required if PV storage is removed later.
			if err := writeHostIDToKubeSecret(ctx, kubeBackend, cfg.HostUUID); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

// kubernetesBackend interface for kube storage backend.
type kubernetesBackend interface {
	// Put puts value into backend (creates if it does not
	// exists, updates it otherwise)
	Put(ctx context.Context, i backend.Item) (*backend.Lease, error)
	// Get returns a single item or not found error
	Get(ctx context.Context, key []byte) (*backend.Item, error)
}

// readHostIDFromStorages tries to read the `host_uuid` value from different storages,
// depending on where the process is running.
// If the process is running in a Kubernetes Cluster, this function will attempt
// to read the `host_uuid` from the Kubernetes Secret. If it does not exist or
// if it is not running on a Kubernetes cluster the read is done from the local
// storage: `dataDir/host_uuid`.
func readHostIDFromStorages(ctx context.Context, dataDir string, kubeBackend kubernetesBackend) (string, error) {
	if kubeBackend != nil {
		if hostID, err := loadHostIDFromKubeSecret(ctx, kubeBackend); err == nil && len(hostID) > 0 {
			return hostID, nil
		}
	}
	// Even if running in Kubernetes fallback to local storage if `host_uuid` was
	// not found in secret.
	hostID, err := utils.ReadHostUUID(dataDir)
	return hostID, trace.Wrap(err)
}

// persistHostIDToStorages writes the cfg.HostUUID to local data and to
// Kubernetes Secret if this process is running on a Kubernetes Cluster.
func persistHostIDToStorages(ctx context.Context, cfg *servicecfg.Config, kubeBackend kubernetesBackend) error {
	if err := utils.WriteHostUUID(cfg.DataDir, cfg.HostUUID); err != nil {
		if errors.Is(err, fs.ErrPermission) {
			cfg.Log.Errorf("Teleport does not have permission to write to: %v. Ensure that you are running as a user with appropriate permissions.", cfg.DataDir)
		}
		return trace.Wrap(err)
	}

	// Persists the `host_uuid` into Kubernetes Secret for later reusage.
	// This is required because `host_uuid` is part of the client secret
	// and Auth connection will fail if we present a different `host_uuid`.
	if kubeBackend != nil {
		return trace.Wrap(writeHostIDToKubeSecret(ctx, kubeBackend, cfg.HostUUID))
	}
	return nil
}

// loadHostIDFromKubeSecret reads the host_uuid from the Kubernetes secret with
// the expected key: `/host_uuid`.
func loadHostIDFromKubeSecret(ctx context.Context, kubeBackend kubernetesBackend) (string, error) {
	item, err := kubeBackend.Get(ctx, backend.Key(utils.HostUUIDFile))
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(item.Value), nil
}

// writeHostIDToKubeSecret writes the `host_uuid` into the Kubernetes secret under
// the key `/host_uuid`.
func writeHostIDToKubeSecret(ctx context.Context, kubeBackend kubernetesBackend, id string) error {
	_, err := kubeBackend.Put(
		ctx,
		backend.Item{
			Key:   backend.Key(utils.HostUUIDFile),
			Value: []byte(id),
		},
	)
	return trace.Wrap(err)
}

// initPublicGRPCServer creates and registers a gRPC server that does not use client
// certificates for authentication. This is used by the join service, which nodes
// use to receive a signed certificate from the auth server.
func (process *TeleportProcess) initPublicGRPCServer(
	limiter *limiter.Limiter,
	conn *Connector,
	listener net.Listener,
) *grpc.Server {
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
	process.RegisterCriticalFunc("proxy.grpc.public", func() error {
		process.log.Infof("Starting proxy gRPC server on %v.", listener.Addr())
		return trace.Wrap(server.Serve(listener))
	})
	return server
}

// initSecureGRPCServer creates and registers a gRPC server that uses mTLS for
// authentication. This is used for the gRPC Kube service, which allows users to
// safely access Kubernetes clusters resources via Teleport without leaking certificates.
// The gRPC server handles the mTLS because we require the client certificate to be
// subject in order to determine his identity.
func (process *TeleportProcess) initSecureGRPCServer(cfg initSecureGRPCServerCfg) (*grpc.Server, error) {
	if !process.Config.Proxy.Kube.Enabled {
		return nil, nil
	}
	clusterName := cfg.conn.ServerIdentity.ClusterName
	serverTLSConfig, err := cfg.conn.ServerIdentity.TLSConfig(process.Config.CipherSuites)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: clusterName,
		AccessPoint: cfg.accessPoint,
		LockWatcher: cfg.lockWatcher,
		Logger: process.log.WithFields(logrus.Fields{
			trace.Component: teleport.Component(teleport.ComponentProxySecureGRPC, process.id),
		}),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// authMiddleware authenticates request assuming TLS client authentication
	// adds authentication information to the context
	// and passes it to the API server
	authMiddleware := &auth.Middleware{
		ClusterName:   clusterName,
		Limiter:       cfg.limiter,
		AcceptedUsage: []string{teleport.UsageKubeOnly},
	}

	tlsConf := copyAndConfigureTLS(serverTLSConfig, process.log, cfg.accessPoint, clusterName)
	creds, err := auth.NewTransportCredentials(auth.TransportCredentialsConfig{
		TransportCredentials: credentials.NewTLS(tlsConf),
		UserGetter:           authMiddleware,
		GetAuthPreference:    cfg.accessPoint.GetAuthPreference,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(authMiddleware.UnaryInterceptors()...),
		grpc.ChainStreamInterceptor(authMiddleware.StreamInterceptors()...),
		grpc.Creds(creds),
		grpc.MaxConcurrentStreams(defaults.GRPCMaxConcurrentStreams),
	)

	kubeServer, err := kubegrpc.New(kubegrpc.Config{
		Signer:      cfg.conn.Client,
		AccessPoint: cfg.accessPoint,
		Authz:       authorizer,
		Log:         process.log,
		Emitter:     cfg.emitter,
		// listener is using the underlying web listener, so we can just use its address.
		// since tls routing is enabled.
		KubeProxyAddr: cfg.listener.Addr().String(),
		ClusterName:   clusterName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	kubeproto.RegisterKubeServiceServer(server, kubeServer)

	process.RegisterCriticalFunc("proxy.grpc.secure", func() error {
		process.log.Infof("Starting proxy gRPC server on %v.", cfg.listener.Addr())
		return trace.Wrap(server.Serve(cfg.listener))
	})
	return server, nil
}

// initSecureGRPCServerCfg is a configuration for initSecureGRPCServer function.
type initSecureGRPCServerCfg struct {
	conn        *Connector
	limiter     *limiter.Limiter
	listener    net.Listener
	accessPoint authclient.ProxyAccessPoint
	lockWatcher *services.LockWatcher
	emitter     apievents.Emitter
}

// copyAndConfigureTLS can be used to copy and modify an existing *tls.Config
// for Teleport application proxy servers.
func copyAndConfigureTLS(config *tls.Config, log logrus.FieldLogger, accessPoint authclient.AccessCache, clusterName string) *tls.Config {
	tlsConfig := config.Clone()

	// Require clients to present a certificate
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

	// Configure function that will be used to fetch the CA that signed the
	// client's certificate to verify the chain presented. If the client does not
	// pass in the cluster name, this functions pulls back all CA to try and
	// match the certificate presented against any CA.
	tlsConfig.GetConfigForClient = authclient.WithClusterCAs(tlsConfig.Clone(), accessPoint, clusterName, log)

	return tlsConfig
}

func makeXForwardedForMiddleware(cfg *servicecfg.Config) utils.HTTPMiddleware {
	if cfg.Proxy.TrustXForwardedFor {
		return web.NewXForwardedForMiddleware
	}
	return utils.NoopHTTPMiddleware
}

func (process *TeleportProcess) newExternalAuditStorageConfigurator() (*externalauditstorage.Configurator, error) {
	watcher, err := local.NewClusterExternalAuditWatcher(process.GracefulExitContext(), local.ClusterExternalAuditStorageWatcherConfig{
		Backend: process.backend,
		OnChange: func() {
			// On change of cluster External Audit Storage, trigger teleport
			// reload, because s3 uploader and athena components don't support
			// live changes to their configuration.
			process.BroadcastEvent(Event{Name: TeleportReloadEvent})
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Wait for the watcher to init to avoid a race in case the external audit
	// storage config changes after the configurator loads it and before the
	// watcher initialized.
	watcher.WaitInit(process.GracefulExitContext())

	easSvc := local.NewExternalAuditStorageService(process.backend)
	integrationSvc, err := local.NewIntegrationsService(process.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	statusService := local.NewStatusService(process.backend)
	return externalauditstorage.NewConfigurator(process.ExitContext(), easSvc, integrationSvc, statusService)
}
