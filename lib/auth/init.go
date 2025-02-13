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

package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/ai"
	"github.com/gravitational/teleport/lib/ai/embedding"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/auth/migration"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentAuth,
})

// VersionStorage local storage for saving the version.
type VersionStorage interface {
	// GetTeleportVersion reads the last known Teleport version from storage.
	GetTeleportVersion(ctx context.Context) (*semver.Version, error)
	// WriteTeleportVersion writes the last known Teleport version to the storage.
	WriteTeleportVersion(ctx context.Context, version *semver.Version) error
}

// InitConfig is auth server init config
type InitConfig struct {
	// Backend is auth backend to use
	Backend backend.Backend

	// VersionStorage is a version storage for local process
	VersionStorage VersionStorage

	// Authority is key generator that we use
	Authority sshca.Authority

	// KeyStoreConfig is the config for the KeyStore which handles private CA
	// keys that may be held in an HSM.
	KeyStoreConfig keystore.Config

	// HostUUID is a UUID of this host
	HostUUID string

	// NodeName is the DNS name of the node
	NodeName string

	// ClusterName stores the FQDN of the signing CA (its certificate will have this
	// name embedded). It is usually set to the GUID of the host the Auth service runs on
	ClusterName types.ClusterName

	// Authorities is a list of pre-configured authorities to supply on first start
	Authorities []types.CertAuthority

	// ApplyOnStartupResources is a set of resources that should be applied
	// on each Teleport start.
	ApplyOnStartupResources []types.Resource

	// BootstrapResources is a list of previously backed-up resources used to
	// bootstrap backend on first start.
	BootstrapResources []types.Resource

	// AuthServiceName is a human-readable name of this CA. If several Auth services are running
	// (managing multiple teleport clusters) this field is used to tell them apart in UIs
	// It usually defaults to the hostname of the machine the Auth service runs on.
	AuthServiceName string

	// DataDir is the full path to the directory where keys, events and logs are kept
	DataDir string

	// ReverseTunnels is a list of reverse tunnels statically supplied
	// in configuration, so auth server will init the tunnels on the first start
	ReverseTunnels []types.ReverseTunnel

	// OIDCConnectors is a list of trusted OpenID Connect identity providers
	// in configuration, so auth server will init the tunnels on the first start
	OIDCConnectors []types.OIDCConnector

	// Trust is a service that manages users and credentials
	Trust services.Trust

	// Presence service is a discovery and heartbeat tracker
	Presence services.PresenceInternal

	// Provisioner is a service that keeps track of provisioning tokens
	Provisioner services.Provisioner

	// Identity is a service that manages users and credentials
	Identity services.Identity

	// Access is service controlling access to resources
	Access services.Access

	// DynamicAccessExt is a service that manages dynamic RBAC.
	DynamicAccessExt services.DynamicAccessExt

	// Events is an event service
	Events types.Events

	// ClusterConfiguration is a services that holds cluster wide configuration.
	ClusterConfiguration services.ClusterConfiguration

	// AutoUpdateService is a service of autoupdate configuration and version.
	AutoUpdateService services.AutoUpdateService

	// Restrictions is a service to access network restrictions, etc
	Restrictions services.Restrictions

	// Apps is a service that manages application resources.
	Apps services.Apps

	// Databases is a service that manages database resources.
	Databases services.Databases

	// DatabaseServices is a service that manages DatabaseService resources.
	DatabaseServices services.DatabaseServices

	// Status is a service that manages cluster status info.
	Status services.StatusInternal

	// Assist is a service that implements the Teleport Assist functionality.
	Assist services.Assistant

	// UserPreferences is a service that manages user preferences.
	UserPreferences services.UserPreferences

	// Roles is a set of roles to create
	Roles []types.Role

	// StaticTokens are pre-defined host provisioning tokens supplied via config file for
	// environments where paranoid security is not needed
	StaticTokens types.StaticTokens

	// AuthPreference defines the authentication type (local, oidc) and second
	// factor passed in from a configuration file.
	AuthPreference types.AuthPreference

	// AuditLog is used for emitting events to audit log.
	AuditLog events.AuditLogSessionStreamer

	// ClusterAuditConfig holds cluster audit configuration.
	ClusterAuditConfig types.ClusterAuditConfig

	// ClusterNetworkingConfig holds cluster networking configuration.
	ClusterNetworkingConfig types.ClusterNetworkingConfig

	// SessionRecordingConfig holds session recording configuration.
	SessionRecordingConfig types.SessionRecordingConfig

	// SkipPeriodicOperations turns off periodic operations
	// used in tests that don't need periodic operations.
	SkipPeriodicOperations bool

	// CipherSuites is a list of ciphersuites that the auth server supports.
	CipherSuites []uint16

	// Emitter is events emitter, used to submit discrete events
	Emitter apievents.Emitter

	// Streamer is events sessionstreamer, used to create continuous
	// session related streams
	Streamer events.Streamer

	// WindowsServices is a service that manages Windows desktop resources.
	WindowsDesktops services.WindowsDesktops

	// SAMLIdPServiceProviders is a service that manages SAML IdP service providers.
	SAMLIdPServiceProviders services.SAMLIdPServiceProviders

	// UserGroups is a service that manages user groups.
	UserGroups services.UserGroups

	// Integrations is a service that manages Integrations.
	Integrations services.Integrations

	// DiscoveryConfigs is a service that manages DiscoveryConfigs.
	DiscoveryConfigs services.DiscoveryConfigs

	// Embeddings is a service that manages Embeddings
	Embeddings services.Embeddings

	// SessionTrackerService is a service that manages trackers for all active sessions.
	SessionTrackerService services.SessionTrackerService

	// ConnectionsDiagnostic is a service that manages Connection Diagnostics resources.
	ConnectionsDiagnostic services.ConnectionsDiagnostic

	// LoadAllCAs tells tsh to load the host CAs for all clusters when trying to ssh into a node.
	LoadAllCAs bool

	// TraceClient is used to forward spans to the upstream telemetry collector
	TraceClient otlptrace.Client

	// Kubernetes is a service that manages kubernetes cluster resources.
	Kubernetes services.Kubernetes

	// AssertionReplayService is a service that mitigates SSO assertion replay.
	*local.AssertionReplayService

	// FIPS means FedRAMP/FIPS 140-2 compliant configuration was requested.
	FIPS bool

	// UsageReporter is a service that forwards cluster usage events.
	UsageReporter usagereporter.UsageReporter

	// Okta is a service that manages Okta resources.
	Okta services.Okta

	// AccessLists is a service that manages access list resources.
	AccessLists services.AccessLists

	// UserLoginStates is a service that manages user login states.
	UserLoginState services.UserLoginStates

	// SecReports is a service that manages security reports.
	SecReports services.SecReports

	// PluginData is a service that manages plugin data.
	PluginData services.PluginData

	// Clock is the clock instance auth uses. Typically you'd only want to set
	// this during testing.
	Clock clockwork.Clock

	// HTTPClientForAWSSTS overwrites the default HTTP client used for making
	// STS requests. Used in test.
	HTTPClientForAWSSTS utils.HTTPDoClient

	// EmbeddingRetriever is a retriever for embeddings.
	EmbeddingRetriever *ai.SimpleRetriever

	// EmbeddingClient is a client that allows generating embeddings.
	EmbeddingClient embedding.Embedder

	// Tracer used to create spans.
	Tracer oteltrace.Tracer

	// AccessMonitoringEnabled is true if access monitoring is enabled.
	AccessMonitoringEnabled bool

	// KubeWaitingContainers is a service that manages
	// Kubernetes ephemeral containers that are waiting
	// to be created until moderated session conditions are met.
	KubeWaitingContainers services.KubeWaitingContainer
}

// Init instantiates and configures an instance of AuthServer
func Init(ctx context.Context, cfg InitConfig, opts ...ServerOption) (*Server, error) {
	ctx, span := cfg.Tracer.Start(ctx, "auth/Init")
	defer span.End()

	if cfg.DataDir == "" {
		return nil, trace.BadParameter("DataDir: data dir can not be empty")
	}
	if cfg.HostUUID == "" {
		return nil, trace.BadParameter("HostUUID: host UUID can not be empty")
	}

	asrv, err := NewServer(&cfg, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	domainName := cfg.ClusterName.GetClusterName()
	if err := backend.RunWhileLocked(ctx,
		backend.RunWhileLockedConfig{
			LockConfiguration: backend.LockConfiguration{
				Backend:            cfg.Backend,
				LockNameComponents: []string{domainName},
				TTL:                30 * time.Second,
			},
			RefreshLockInterval: 20 * time.Second,
		}, func(ctx context.Context) error {
			return trace.Wrap(initCluster(ctx, cfg, asrv))
		}); err != nil {
		return nil, trace.Wrap(err)
	}

	return asrv, nil
}

// initCluster configures the cluster based on the user provided configuration. This should
// only be called when the init lock is held to prevent multiple instances of Auth from attempting
// to bootstrap the cluster at the same time.
func initCluster(ctx context.Context, cfg InitConfig, asrv *Server) error {
	span := oteltrace.SpanFromContext(ctx)
	domainName := cfg.ClusterName.GetClusterName()
	firstStart, err := isFirstStart(ctx, asrv, cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := validateAndUpdateTeleportVersion(ctx, cfg.VersionStorage, teleport.SemVersion, firstStart); err != nil {
		return trace.Wrap(err)
	}

	// if bootstrap resources are supplied, use them to bootstrap backend state
	// on initial startup.
	if len(cfg.BootstrapResources) > 0 {
		if firstStart {
			log.Infof("Applying %v bootstrap resources (first initialization)", len(cfg.BootstrapResources))
			if err := checkResourceConsistency(ctx, asrv.keyStore, domainName, cfg.BootstrapResources...); err != nil {
				return trace.Wrap(err, "refusing to bootstrap backend")
			}
			if err := local.CreateResources(ctx, cfg.Backend, cfg.BootstrapResources...); err != nil {
				return trace.Wrap(err, "backend bootstrap failed")
			}
		} else {
			log.Warnf("Ignoring %v bootstrap resources (previously initialized)", len(cfg.BootstrapResources))
		}
	}

	// if apply-on-startup resources are supplied, apply them
	if len(cfg.ApplyOnStartupResources) > 0 {
		log.Infof("Applying %v resources (apply-on-startup)", len(cfg.ApplyOnStartupResources))

		if err := applyResources(ctx, asrv.Services, cfg.ApplyOnStartupResources); err != nil {
			return trace.Wrap(err, "applying resources failed")
		}
	}

	// Set the ciphersuites that this auth server supports.
	asrv.cipherSuites = cfg.CipherSuites

	// INTERNAL: Authorities (plus Roles) and ReverseTunnels don't follow the
	// same pattern as the rest of the configuration (they are not configuration
	// singletons). However, we need to keep them around while Telekube uses them.
	for _, role := range cfg.Roles {
		if err := asrv.UpsertRole(ctx, role); err != nil {
			return trace.Wrap(err)
		}
		log.Infof("Created role: %v.", role)
	}
	for i := range cfg.Authorities {
		ca := cfg.Authorities[i]

		// Remove private key from leaf clusters.
		if domainName != ca.GetClusterName() {
			ca = ca.Clone()
			types.RemoveCASecrets(ca)
		}
		// Don't re-create CA if it already exists, otherwise
		// the existing cluster configuration will be corrupted;
		// this part of code is only used in tests.
		if err := asrv.CreateCertAuthority(ctx, ca); err != nil {
			if !trace.IsAlreadyExists(err) {
				return trace.Wrap(err)
			}
		} else {
			log.Infof("Created trusted certificate authority: %q, type: %q.", ca.GetName(), ca.GetType())
		}
	}
	for _, tunnel := range cfg.ReverseTunnels {
		if err := asrv.UpsertReverseTunnel(tunnel); err != nil {
			return trace.Wrap(err)
		}
		log.Infof("Created reverse tunnel: %v.", tunnel)
	}

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		ctx, span := cfg.Tracer.Start(gctx, "auth/SetClusterAuditConfig")
		defer span.End()
		return trace.Wrap(asrv.SetClusterAuditConfig(ctx, cfg.ClusterAuditConfig))
	})

	g.Go(func() error {
		ctx, span := cfg.Tracer.Start(gctx, "auth/SetClusterNetworkingConfig")
		defer span.End()
		return trace.Wrap(initSetClusterNetworkingConfig(ctx, asrv, cfg.ClusterNetworkingConfig))
	})

	g.Go(func() error {
		ctx, span := cfg.Tracer.Start(gctx, "auth/SetSessionRecordingConfig")
		defer span.End()
		return trace.Wrap(initSetSessionRecordingConfig(ctx, asrv, cfg.SessionRecordingConfig))
	})

	g.Go(func() error {
		ctx, span := cfg.Tracer.Start(gctx, "auth/SetAuthPreference")
		defer span.End()
		return trace.Wrap(initSetAuthPreference(ctx, asrv, cfg.AuthPreference))
	})

	g.Go(func() error {
		_, span := cfg.Tracer.Start(gctx, "auth/SetStaticTokens")
		defer span.End()
		log.Infof("Updating cluster configuration: %v.", cfg.StaticTokens)
		return trace.Wrap(asrv.SetStaticTokens(cfg.StaticTokens))
	})

	g.Go(func() error {
		_, span := cfg.Tracer.Start(gctx, "auth/SetClusterNamespace")
		defer span.End()
		log.Infof("Creating namespace: %q.", apidefaults.Namespace)
		return trace.Wrap(asrv.UpsertNamespace(types.DefaultNamespace()))
	})

	var cn types.ClusterName
	g.Go(func() error {
		_, span := cfg.Tracer.Start(gctx, "auth/SetClusterName")
		defer span.End()

		// The first Auth Server that starts gets to set the name of the cluster.
		// If a cluster name/ID is already stored in the backend, the attempt to set
		// a new name returns an AlreadyExists error.
		err := asrv.SetClusterName(cfg.ClusterName)
		if err == nil {
			cn = cfg.ClusterName
			return nil
		}

		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}

		// If the cluster name has already been set, log a warning if the user
		// is trying to change the name.
		cn, err = asrv.Services.GetClusterName()
		if err != nil {
			return trace.Wrap(err)
		}
		if cn.GetClusterName() != cfg.ClusterName.GetClusterName() {
			msg := "Cannot rename cluster: continuing with current cluster name. Teleport " +
				"clusters can not be renamed once they are created. You are seeing this " +
				"message for one of two reasons. Either you have not set \"cluster_name\" in " +
				"Teleport configuration and changed the hostname of the auth server or you " +
				"are trying to change the value of \"cluster_name\"."
			log.WithFields(logrus.Fields{
				"current_cluster_name":    cn.GetClusterName(),
				"configured_cluster_name": cfg.ClusterName.GetClusterName(),
			}).Error(msg)
		}

		log.Debugf("Cluster configuration: %v.", cn.GetClusterName())
		return nil
	})

	if err := g.Wait(); err != nil {
		return trace.Wrap(err)
	}

	// Override user passed in cluster name with what is in the backend.
	cfg.ClusterName = cn

	// Apply any outstanding migrations.
	if err := migration.Apply(ctx, cfg.Backend); err != nil {
		return trace.Wrap(err, "applying migrations")
	}
	span.AddEvent("migrating db_client_authority")
	err = migrateDBClientAuthority(ctx, asrv.Trust, cfg.ClusterName.GetClusterName())
	if err != nil {
		return trace.Wrap(err)
	}
	span.AddEvent("completed migration db_client_authority")

	// generate certificate authorities if they don't exist
	var (
		mu         sync.Mutex
		activeKeys [][]byte
	)
	g, gctx = errgroup.WithContext(ctx)
	for _, caType := range types.CertAuthTypes {
		caType := caType
		g.Go(func() error {
			ctx, span := cfg.Tracer.Start(gctx, "auth/initializeAuthority", oteltrace.WithAttributes(attribute.String("type", string(caType))))
			defer span.End()

			caID := types.CertAuthID{Type: caType, DomainName: cfg.ClusterName.GetClusterName()}
			ca, err := asrv.Services.GetCertAuthority(ctx, caID, true)
			if err != nil {
				if !trace.IsNotFound(err) {
					return trace.Wrap(err)
				}

				log.Infof("First start: generating %s certificate authority.", caID.Type)
				if ca, err = generateAuthority(ctx, asrv, caID); err != nil {
					return trace.Wrap(err)
				}

				if err := asrv.CreateCertAuthority(ctx, ca); err != nil {
					return trace.Wrap(err)
				}
			} else {
				// Already have a CA. Make sure the keyStore has usable keys.
				hasUsableActiveKeys, err := asrv.keyStore.HasUsableActiveKeys(ctx, ca)
				if err != nil {
					return trace.Wrap(err)
				}
				if !hasUsableActiveKeys {
					// This could be one of a few cases:
					// 1. A new auth server with an HSM being added to an HA cluster.
					// 2. A new auth server with no HSM being added to an HA cluster
					//    where all current auth servers have HSMs.
					// 3. An existing auth server has restarted with a new HSM configured.
					// 4. An existing HSM auth server has restarted no HSM configured.
					// 5. An existing HSM auth server has restarted with a new UUID.
					if ca.GetType() == types.HostCA {
						// We need local keys to sign the Admin identity to support
						// tctl. For this special case we add AdditionalTrustedKeys
						// without any active keys. These keys will not be used for
						// any signing operations until a CA rotation. Only the Host
						// CA is necessary to issue the Admin identity.
						if err := asrv.ensureLocalAdditionalKeys(ctx, ca); err != nil {
							return trace.Wrap(err)
						}
						// reload updated CA for below checks
						if ca, err = asrv.Services.GetCertAuthority(ctx, caID, true); err != nil {
							return trace.Wrap(err)
						}
					}
				}
				hasUsableActiveKeys, err = asrv.keyStore.HasUsableActiveKeys(ctx, ca)
				if err != nil {
					return trace.Wrap(err)
				}
				hasUsableAdditionalKeys, err := asrv.keyStore.HasUsableAdditionalKeys(ctx, ca)
				if err != nil {
					return trace.Wrap(err)
				}
				if !hasUsableActiveKeys && hasUsableAdditionalKeys {
					log.Warn("This auth server has a newly added or removed HSM and will not " +
						"be able to perform any signing operations. You must rotate all CAs " +
						"before routing traffic to this auth server. See https://goteleport.com/docs/management/operations/ca-rotation/")
				}
				allKeyTypes := ca.AllKeyTypes()
				numKeyTypes := len(allKeyTypes)
				if numKeyTypes > 1 {
					log.Warnf("%s CA contains a combination of %s and %s keys. If you are attempting to"+
						" configure HSM or KMS support, make sure it is configured on all auth servers in"+
						" this cluster and then perform a CA rotation: https://goteleport.com/docs/management/operations/ca-rotation/",
						caID.Type, strings.Join(allKeyTypes[:numKeyTypes-1], ", "), allKeyTypes[numKeyTypes-1])
				}
			}

			mu.Lock()
			defer mu.Unlock()
			for _, keySet := range []types.CAKeySet{ca.GetActiveKeys(), ca.GetAdditionalTrustedKeys()} {
				for _, sshKeyPair := range keySet.SSH {
					activeKeys = append(activeKeys, sshKeyPair.PrivateKey)
				}
				for _, tlsKeyPair := range keySet.TLS {
					activeKeys = append(activeKeys, tlsKeyPair.Key)
				}
				for _, jwtKeyPair := range keySet.JWT {
					activeKeys = append(activeKeys, jwtKeyPair.PrivateKey)
				}
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return trace.Wrap(err)
	}

	// Delete any unused keys from the keyStore. This is to avoid exhausting
	// (or wasting) HSM resources.
	if err := asrv.keyStore.DeleteUnusedKeys(ctx, activeKeys); err != nil {
		// Key deletion is best-effort, log a warning if it fails and carry on.
		// We don't want to prevent a CA rotation, which may be necessary in
		// some cases where this would fail.
		log.Warnf("An attempt to clean up unused HSM or KMS CA keys has failed unexpectedly: %v", err)
	}

	if lib.IsInsecureDevMode() {
		warningMessage := "Starting teleport in insecure mode. This is " +
			"dangerous! Sensitive information will be logged to console and " +
			"certificates will not be verified. Proceed with caution!"
		log.Warn(warningMessage)
	}

	span.AddEvent("migrating legacy resources")
	// Migrate any legacy resources to new format.
	if err := migrateLegacyResources(ctx, asrv); err != nil {
		return trace.Wrap(err)
	}
	span.AddEvent("completed migration legacy resources")

	span.AddEvent("checking certificate authority cluster names")
	if err := checkAuthorityClusterNames(ctx, asrv); err != nil {
		return trace.Wrap(err)
	}
	span.AddEvent("completed checking certificate authority cluster names")

	// Create presets - convenience and example resources.
	if !services.IsDashboard(*modules.GetModules().Features().ToProto()) {
		span.AddEvent("creating preset roles")
		if err := createPresetRoles(ctx, asrv); err != nil {
			return trace.Wrap(err)
		}
		span.AddEvent("completed creating preset roles")

		span.AddEvent("creating preset users")
		if err := createPresetUsers(ctx, asrv); err != nil {
			return trace.Wrap(err)
		}
		span.AddEvent("completed creating preset users")
	} else {
		log.Info("skipping preset role and user creation")
	}

	if !cfg.SkipPeriodicOperations {
		log.Infof("Auth server is running periodic operations.")
		go asrv.runPeriodicOperations()
	} else {
		log.Infof("Auth server is skipping periodic operations.")
	}

	return nil
}

// generateAuthority creates a new self-signed authority of the provided type
// and returns it to the caller. It is the responsibility of callers to persist
// the authority.
func generateAuthority(ctx context.Context, asrv *Server, caID types.CertAuthID) (types.CertAuthority, error) {
	keySet, err := newKeySet(ctx, asrv.keyStore, caID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        caID.Type,
		ClusterName: caID.DomainName,
		ActiveKeys:  keySet,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ca, nil
}

func checkAuthorityClusterNames(ctx context.Context, asrv *Server) error {
	for _, caType := range types.CertAuthTypes {
		authorities, err := asrv.Services.GetCertAuthorities(ctx, caType, false)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, ca := range authorities {
			caClusterName := ca.GetClusterName()
			// sanity check that the cluster name in the CA certificates
			// matches the authority resource's cluster name
			for _, keyPair := range ca.GetTrustedTLSKeyPairs() {
				cert, err := tlsca.ParseCertificatePEM(keyPair.Cert)
				if err != nil {
					return trace.Wrap(err)
				}
				clusterName, err := tlsca.ClusterName(cert.Subject)
				if err != nil {
					return trace.Wrap(err)
				}
				if clusterName != caClusterName {
					return trace.BadParameter("CA certificate of type %s has cluster name %q that does not match the cluster name %q found in the subject", ca.GetType(), caClusterName, clusterName)
				}
			}
		}
	}

	return nil
}

func initSetAuthPreference(ctx context.Context, asrv *Server, newAuthPref types.AuthPreference) error {
	storedAuthPref, err := asrv.Services.GetAuthPreference(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	shouldReplace, err := shouldInitReplaceResourceWithOrigin(storedAuthPref, newAuthPref)
	if err != nil {
		return trace.Wrap(err)
	}
	if shouldReplace {
		if err := asrv.SetAuthPreference(ctx, newAuthPref); err != nil {
			return trace.Wrap(err)
		}
		log.Infof("Updating cluster auth preference: %v.", newAuthPref)
	}
	return nil
}

func initSetClusterNetworkingConfig(ctx context.Context, asrv *Server, newNetConfig types.ClusterNetworkingConfig) error {
	storedNetConfig, err := asrv.Services.GetClusterNetworkingConfig(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	shouldReplace, err := shouldInitReplaceResourceWithOrigin(storedNetConfig, newNetConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	if shouldReplace {
		if err := asrv.SetClusterNetworkingConfig(ctx, newNetConfig); err != nil {
			return trace.Wrap(err)
		}
		log.Infof("Updating cluster networking configuration: %v.", newNetConfig)
	}
	return nil
}

func initSetSessionRecordingConfig(ctx context.Context, asrv *Server, newRecConfig types.SessionRecordingConfig) error {
	storedRecConfig, err := asrv.Services.GetSessionRecordingConfig(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	shouldReplace, err := shouldInitReplaceResourceWithOrigin(storedRecConfig, newRecConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	if shouldReplace {
		if err := asrv.SetSessionRecordingConfig(ctx, newRecConfig); err != nil {
			return trace.Wrap(err)
		}
		log.Infof("Updating session recording configuration: %v.", newRecConfig)
	}
	return nil
}

// shouldInitReplaceResourceWithOrigin determines whether the candidate
// resource should be used to replace the stored resource during auth server
// initialization.  Dynamically configured resources must not be overwritten
// when the corresponding file config is left unspecified (i.e., by defaults).
func shouldInitReplaceResourceWithOrigin(stored, candidate types.ResourceWithOrigin) (bool, error) {
	if candidate == nil || (candidate.Origin() != types.OriginDefaults && candidate.Origin() != types.OriginConfigFile) {
		return false, trace.BadParameter("candidate origin must be either defaults or config-file (this is a bug)")
	}

	// If there is no resource stored in the backend or it was not dynamically
	// configured, the candidate resource should be stored in the backend.
	if stored == nil || stored.Origin() != types.OriginDynamic {
		return true, nil
	}

	// If the candidate resource is explicitly configured in the config file,
	// store this config-file resource in the backend no matter what.
	if candidate.Origin() == types.OriginConfigFile {
		// Log a warning when about to overwrite a dynamically configured resource.
		if stored.Origin() == types.OriginDynamic {
			log.Warnf("Stored %v resource that was configured dynamically is about to be discarded in favor of explicit file configuration.", stored.GetKind())
		}
		return true, nil
	}

	// The resource in the backend was configured dynamically, and there is no
	// more authoritative file configuration to replace it.  Keep the stored
	// dynamic resource.
	return false, nil
}

// migrationStart marks the migration as active.
// It should be called when a migration starts.
func migrationStart(ctx context.Context, migrationName string) {
	log.Debugf("Migrations: %q migration started.", migrationName)
	migrations.WithLabelValues(migrationName).Set(1)
}

// migrationEnd marks the migration as inactive.
// It should be called when a migration ends.
func migrationEnd(ctx context.Context, migrationName string) {
	log.Debugf("Migrations: %q migration ended.", migrationName)
	migrations.WithLabelValues(migrationName).Set(0)
}

func migrateLegacyResources(ctx context.Context, asrv *Server) error {
	if err := migrateRemoteClusters(ctx, asrv); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// PresetRoleManager contains the required Role Management methods to create a Preset Role.
type PresetRoleManager interface {
	// GetRole returns role by name.
	GetRole(ctx context.Context, name string) (types.Role, error)
	// CreateRole creates a role.
	CreateRole(ctx context.Context, role types.Role) error
	// UpsertRole creates or updates a role and emits a related audit event.
	UpsertRole(ctx context.Context, role types.Role) error
}

// GetPresetRoles returns a list of all preset roles expected to be available on
// this cluster.
func GetPresetRoles() []types.Role {
	presets := []types.Role{
		services.NewPresetGroupAccessRole(),
		services.NewPresetEditorRole(),
		services.NewPresetAccessRole(),
		services.NewPresetAuditorRole(),
		services.NewPresetReviewerRole(),
		services.NewPresetRequesterRole(),
		services.NewSystemAutomaticAccessApproverRole(),
		services.NewPresetDeviceAdminRole(),
		services.NewPresetDeviceEnrollRole(),
		services.NewPresetRequireTrustedDeviceRole(),
	}

	// Certain `New$FooRole()` functions will return a nil role if the
	// corresponding feature is disabled. They should be filtered out as they
	// are not actually made available on the cluster.
	return slices.DeleteFunc(presets, func(r types.Role) bool { return r == nil })
}

// createPresetRoles creates preset role resources
func createPresetRoles(ctx context.Context, rm PresetRoleManager) error {
	roles := GetPresetRoles()

	g, gctx := errgroup.WithContext(ctx)
	for _, role := range roles {
		// If the role is nil, skip because it doesn't apply to this Teleport installation.
		if role == nil {
			continue
		}

		role := role
		g.Go(func() error {
			if types.IsSystemResource(role) {
				// System resources *always* get reset on every auth startup
				if err := rm.UpsertRole(gctx, role); err != nil {
					return trace.Wrap(err, "failed upserting system role %s", role.GetName())
				}

				return nil
			}

			if err := rm.CreateRole(gctx, role); err != nil {
				if !trace.IsAlreadyExists(err) {
					return trace.WrapWithMessage(err, "failed to create preset role %v", role.GetName())
				}

				currentRole, err := rm.GetRole(gctx, role.GetName())
				if err != nil {
					return trace.Wrap(err)
				}

				role, err := services.AddRoleDefaults(currentRole)
				if trace.IsAlreadyExists(err) {
					return nil
				}
				if err != nil {
					return trace.Wrap(err)
				}

				if err := rm.UpsertRole(gctx, role); err != nil {
					return trace.WrapWithMessage(err, "failed to update preset role %v", role.GetName())
				}
			}
			return nil
		})
	}
	return trace.Wrap(g.Wait())
}

// PresetUsers contains the required User Management methods to
// create a preset User. Method names represent the appropriate
// subset
type PresetUsers interface {
	// CreateUser creates a new user record based on the supplied `user` instance.
	CreateUser(ctx context.Context, user types.User) error
	// GetUser fetches a user from the repository by name, optionally fetching
	// any associated secrets
	GetUser(username string, withSecrets bool) (types.User, error)
	// Upsert user creates or updates a user record as needed
	UpsertUser(user types.User) error
}

// createPresetUsers creates all of the required user presets. No attempt is
// made to migrate any existing users to the lastest preset.
func createPresetUsers(ctx context.Context, um PresetUsers) error {
	users := []types.User{
		services.NewSystemAutomaticAccessBotUser(),
	}
	for _, user := range users {
		// Some users are only valid for enterprise Teleport, and so will be
		// nil for an OSS build and can be skipped
		if user == nil {
			continue
		}

		if types.IsSystemResource(user) {
			// System resources *always* get reset on every auth startup
			if err := um.UpsertUser(user); err != nil {
				return trace.Wrap(err, "failed upserting system user %s", user.GetName())
			}
			continue
		}

		if err := um.CreateUser(ctx, user); err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err, "failed creating preset user %s", user.GetName())
		}
	}

	return nil
}

// isFirstStart returns 'true' if the auth server is starting for the 1st time
// on this server.
func isFirstStart(ctx context.Context, authServer *Server, cfg InitConfig) (bool, error) {
	// check if the CA exists?
	_, err := authServer.Services.GetCertAuthority(
		ctx,
		types.CertAuthID{
			DomainName: cfg.ClusterName.GetClusterName(),
			Type:       types.HostCA,
		}, false)
	if err != nil {
		if !trace.IsNotFound(err) {
			return false, trace.Wrap(err)
		}
		// If a CA was not found, that means this is the first start.
		return true, nil
	}
	return false, nil
}

// checkResourceConsistency checks far basic conflicting state issues.
func checkResourceConsistency(ctx context.Context, keyStore *keystore.Manager, clusterName string, resources ...types.Resource) error {
	for _, rsc := range resources {
		switch r := rsc.(type) {
		case types.CertAuthority:
			// check that signing CAs have expected cluster name and that
			// all CAs for this cluster do having signing keys.
			caClusterName := r.GetClusterName()
			seemsLocal := caClusterName == clusterName

			var hasKeys bool
			var signerErr error
			switch r.GetType() {
			case types.HostCA, types.UserCA, types.OpenSSHCA:
				_, signerErr = keyStore.GetSSHSigner(ctx, r)
			case types.DatabaseCA, types.DatabaseClientCA, types.SAMLIDPCA:
				_, _, signerErr = keyStore.GetTLSCertAndSigner(ctx, r)
			case types.JWTSigner, types.OIDCIdPCA:
				_, signerErr = keyStore.GetJWTSigner(ctx, r)
			default:
				return trace.BadParameter("unexpected cert_authority type %s for cluster %v", r.GetType(), clusterName)
			}
			switch {
			case signerErr == nil:
				hasKeys = true
			case trace.IsNotFound(signerErr):
				hasKeys = false
			default:
				return trace.Wrap(signerErr)
			}

			if seemsLocal && !hasKeys {
				return trace.BadParameter("ca for local cluster %q missing signing keys", clusterName)
			}
			if !seemsLocal && hasKeys {
				return trace.BadParameter("unexpected cluster name %q for signing ca (expected %q)", r.GetClusterName(), clusterName)
			}
			if !seemsLocal {
				continue
			}
		case types.TrustedCluster:
			if r.GetName() == clusterName {
				return trace.BadParameter("trusted cluster has same name as local cluster (%q)", clusterName)
			}
		case types.Role:
			// Some options are only available with enterprise subscription
			if err := checkRoleFeatureSupport(r); err != nil {
				return trace.Wrap(err)
			}
		default:
			// No validation checks for this resource type
		}
	}
	return nil
}

// GenerateIdentity generates identity for the auth server
func GenerateIdentity(a *Server, id state.IdentityID, additionalPrincipals, dnsNames []string) (*state.Identity, error) {
	priv, pub, err := native.GenerateKeyPair()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsPub, err := PrivateKeyToPublicKeyTLS(priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certs, err := a.GenerateHostCerts(context.Background(),
		&proto.HostCertsRequest{
			HostID:               id.HostUUID,
			NodeName:             id.NodeName,
			Role:                 id.Role,
			AdditionalPrincipals: additionalPrincipals,
			DNSNames:             dnsNames,
			PublicSSHKey:         pub,
			PublicTLSKey:         tlsPub,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return state.ReadIdentityFromKeyPair(priv, certs)
}

// CertInfo returns diagnostic information about certificate
func CertInfo(cert *x509.Certificate) string {
	return fmt.Sprintf("cert(%v issued by %v:%v)", cert.Subject.CommonName, cert.Issuer.CommonName, cert.Issuer.SerialNumber)
}

// TLSCertInfo returns diagnostic information about certificate
func TLSCertInfo(cert *tls.Certificate) string {
	x509cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return err.Error()
	}
	return CertInfo(x509cert)
}

// CertAuthorityInfo returns debugging information about certificate authority
func CertAuthorityInfo(ca types.CertAuthority) string {
	var out []string
	for _, keyPair := range ca.GetTrustedTLSKeyPairs() {
		cert, err := tlsca.ParseCertificatePEM(keyPair.Cert)
		if err != nil {
			out = append(out, err.Error())
		} else {
			out = append(out, fmt.Sprintf("trust root(%v:%v)", cert.Subject.CommonName, cert.Subject.SerialNumber))
		}
	}
	return fmt.Sprintf("cert authority(state: %v, phase: %v, roots: %v)", ca.GetRotation().State, ca.GetRotation().Phase, strings.Join(out, ", "))
}

// DELETE IN: 2.7.0
// NOTE: Sadly, our integration tests depend on this logic
// because they create remote cluster resource. Our integration
// tests should be migrated to use trusted clusters instead of manually
// creating tunnels.
// This migration adds remote cluster resource migrating from 2.5.0
// where the presence of remote cluster was identified only by presence
// of host certificate authority with cluster name not equal local cluster name
func migrateRemoteClusters(ctx context.Context, asrv *Server) error {
	migrationStart(ctx, "remote_clusters")
	defer migrationEnd(ctx, "remote_clusters")

	clusterName, err := asrv.Services.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}
	certAuthorities, err := asrv.Services.GetCertAuthorities(ctx, types.HostCA, false)
	if err != nil {
		return trace.Wrap(err)
	}
	// loop over all roles and make sure any v3 roles have permit port
	// forward and forward agent allowed
	for _, certAuthority := range certAuthorities {
		if certAuthority.GetName() == clusterName.GetClusterName() {
			log.Debugf("Migrations: skipping local cluster cert authority %q.", certAuthority.GetName())
			continue
		}
		// remote cluster already exists
		_, err = asrv.Services.GetRemoteCluster(certAuthority.GetName())
		if err == nil {
			log.Debugf("Migrations: remote cluster already exists for cert authority %q.", certAuthority.GetName())
			continue
		}
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		// the cert authority is associated with trusted cluster
		_, err = asrv.Services.GetTrustedCluster(ctx, certAuthority.GetName())
		if err == nil {
			log.Debugf("Migrations: trusted cluster resource exists for cert authority %q.", certAuthority.GetName())
			continue
		}
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		remoteCluster, err := types.NewRemoteCluster(certAuthority.GetName())
		if err != nil {
			return trace.Wrap(err)
		}
		err = asrv.CreateRemoteCluster(remoteCluster)
		if err != nil {
			if !trace.IsAlreadyExists(err) {
				return trace.Wrap(err)
			}
		}
		log.Infof("Migrations: added remote cluster resource for cert authority %q.", certAuthority.GetName())
	}

	return nil
}

// Unlike when resources are loaded via --bootstrap, we're inserting elements via their service.
// This means consistency is checked. This function does not currently support applying resources
// with dependencies (like a user referring to a role) as it won't necessarily apply them in the
// right order.
func applyResources(ctx context.Context, service *Services, resources []types.Resource) error {
	var err error
	for _, resource := range resources {
		switch r := resource.(type) {
		case types.ProvisionToken:
			err = service.Provisioner.UpsertToken(ctx, r)
		case types.ClusterNetworkingConfig:
			err = service.ClusterConfiguration.SetClusterNetworkingConfig(ctx, r)
		case types.AuthPreference:
			err = service.ClusterConfiguration.SetAuthPreference(ctx, r)
		default:
			return trace.NotImplemented("cannot apply resource of type %T", resource)
		}
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// migrateDBClientAuthority copies Database CA as Database Client CA.
// Does nothing if the Database Client CA already exists.
//
// TODO(gavin): DELETE IN 16.0.0
func migrateDBClientAuthority(ctx context.Context, trustSvc services.Trust, cluster string) error {
	migrationStart(ctx, "db_client_authority")
	defer migrationEnd(ctx, "db_client_authority")
	err := migration.MigrateDBClientAuthority(ctx, trustSvc, cluster)
	return trace.Wrap(err)
}
