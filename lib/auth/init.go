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

package auth

import (
	"cmp"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	dbobjectimportrulev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/clusterconfig"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/autoupdate/autoupdatev1"
	"github.com/gravitational/teleport/lib/auth/dbobjectimportrule/dbobjectimportrulev1"
	igcredentials "github.com/gravitational/teleport/lib/auth/integration/credentials"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1"
	"github.com/gravitational/teleport/lib/auth/migration"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobjectimportrule"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var logger = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentAuth)

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
	KeyStoreConfig servicecfg.KeystoreConfig

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
	Trust services.TrustInternal

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
	ClusterConfiguration services.ClusterConfigurationInternal

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

	// WindowsDesktops is a service that manages Windows desktop resources.
	WindowsDesktops services.WindowsDesktops

	// DynamicWindowsServices is a service that manages dynamic Windows desktop resources.
	DynamicWindowsDesktops services.DynamicWindowsDesktops

	// SAMLIdPServiceProviders is a service that manages SAML IdP service providers.
	SAMLIdPServiceProviders services.SAMLIdPServiceProviders

	// UserGroups is a service that manages user groups.
	UserGroups services.UserGroups

	// CrownJewels is a service that manages CrownJewels.
	CrownJewels services.CrownJewels

	// Integrations is a service that manages Integrations.
	Integrations services.Integrations

	// UserTasks is a service that manages UserTasks.
	UserTasks services.UserTasks

	// DiscoveryConfigs is a service that manages DiscoveryConfigs.
	DiscoveryConfigs services.DiscoveryConfigs

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

	// DatabaseObjectImportRule is a service that manages database object import rules.
	DatabaseObjectImportRules services.DatabaseObjectImportRules

	// DatabaseObject is a service that manages database objects.
	DatabaseObjects services.DatabaseObjects

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

	// Tracer used to create spans.
	Tracer oteltrace.Tracer

	// AccessMonitoringEnabled is true if access monitoring is enabled.
	AccessMonitoringEnabled bool

	// AccessMonitoringRules is a service that manages access monitoring rules.
	AccessMonitoringRules services.AccessMonitoringRules

	// KubeWaitingContainers is a service that manages
	// Kubernetes ephemeral containers that are waiting
	// to be created until moderated session conditions are met.
	KubeWaitingContainers services.KubeWaitingContainer

	// Notifications is a service that manages notifications.
	Notifications services.Notifications

	// BotInstance is a service that manages Machine ID bot instances
	BotInstance services.BotInstance

	// SPIFFEFederations is a service that manages storing SPIFFE federations.
	SPIFFEFederations services.SPIFFEFederations

	// WorkloadIdentity is the service for storing and retrieving
	// WorkloadIdentity resources.
	WorkloadIdentity services.WorkloadIdentities

	// WorkloadIdentityX509Revocations is the service for storing and retrieving
	// WorkloadIdentityX509Revocations.
	WorkloadIdentityX509Revocations services.WorkloadIdentityX509Revocations

	// StaticHostUsers is a service that manages host users that should be
	// created on SSH nodes.
	StaticHostUsers services.StaticHostUser

	// ProvisioningStates is a service that manages the storage and retrieval of
	// downstream User and Access List provisioning records
	ProvisioningStates services.ProvisioningStates

	// Logger is the logger instance for the auth service to use.
	Logger *slog.Logger

	// IdentityCenter is the Identity Center state storage service to use in
	// this node.
	IdentityCenter services.IdentityCenter

	// PluginStaticCredentials handles credentials for integrations and plugins.
	PluginStaticCredentials services.PluginStaticCredentials

	// GitServers manages git servers.
	GitServers services.GitServers

	// StableUNIXUsers handles the storage for stable UNIX users.
	StableUNIXUsers services.StableUNIXUsersInternal
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
	if err := validateAndUpdateTeleportVersion(ctx, cfg.VersionStorage, teleport.SemVersion); err != nil {
		return trace.Wrap(err)
	}

	// if bootstrap resources are supplied, use them to bootstrap backend state
	// on initial startup.
	if len(cfg.BootstrapResources) > 0 {
		if firstStart {
			asrv.logger.InfoContext(ctx, "Applying bootstrap resources (first initialization)", "resource_count", len(cfg.BootstrapResources))
			if err := checkResourceConsistency(ctx, asrv.keyStore, domainName, cfg.BootstrapResources...); err != nil {
				return trace.Wrap(err, "refusing to bootstrap backend")
			}
			if err := local.CreateResources(ctx, cfg.Backend, cfg.BootstrapResources...); err != nil {
				return trace.Wrap(err, "backend bootstrap failed")
			}
		} else {
			asrv.logger.WarnContext(ctx, "Ignoring bootstrap resources (previously initialized)", "resource_count", len(cfg.BootstrapResources))
		}
	}

	// if apply-on-startup resources are supplied, apply them
	if len(cfg.ApplyOnStartupResources) > 0 {
		asrv.logger.InfoContext(ctx, "Applying resources (apply-on-startup)", "resource_count", len(cfg.ApplyOnStartupResources))

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
		if _, err := asrv.UpsertRole(ctx, role); err != nil {
			return trace.Wrap(err)
		}
		asrv.logger.InfoContext(ctx, "Created role", "role", role.GetName())
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
			asrv.logger.InfoContext(ctx, "Created trusted certificate authority",
				"ca_name", ca.GetName(),
				"ca_type", ca.GetType(),
			)
		}
	}
	for _, tunnel := range cfg.ReverseTunnels {
		if err := asrv.UpsertReverseTunnel(ctx, tunnel); err != nil {
			return trace.Wrap(err)
		}
		asrv.logger.InfoContext(ctx, "Created reverse tunnel", "tunnel", tunnel.GetName())
	}

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		ctx, span := cfg.Tracer.Start(gctx, "auth/SetClusterAuditConfig")
		defer span.End()
		return trace.Wrap(asrv.SetClusterAuditConfig(ctx, cfg.ClusterAuditConfig))
	})

	g.Go(func() error {
		ctx, span := cfg.Tracer.Start(gctx, "auth/InitializeClusterNetworkingConfig")
		defer span.End()
		return trace.Wrap(initializeClusterNetworkingConfig(ctx, asrv, cfg.ClusterNetworkingConfig))
	})

	g.Go(func() error {
		ctx, span := cfg.Tracer.Start(gctx, "auth/InitializeSessionRecordingConfig")
		defer span.End()
		return trace.Wrap(initializeSessionRecordingConfig(ctx, asrv, cfg.SessionRecordingConfig))
	})

	g.Go(func() error {
		ctx, span := cfg.Tracer.Start(gctx, "auth/InitializeAccessGraphSettings")
		defer span.End()
		return trace.Wrap(initializeAccessGraphSettings(ctx, asrv))
	})

	g.Go(func() error {
		ctx, span := cfg.Tracer.Start(gctx, "auth/initializeAuthPreference")
		defer span.End()
		return trace.Wrap(initializeAuthPreference(ctx, asrv, cfg.AuthPreference))
	})

	g.Go(func() error {
		_, span := cfg.Tracer.Start(gctx, "auth/SetStaticTokens")
		defer span.End()
		asrv.logger.InfoContext(ctx, "Updating cluster configuration", "static_tokens", cfg.StaticTokens)
		return trace.Wrap(asrv.SetStaticTokens(cfg.StaticTokens))
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
			const msg = "Cannot rename cluster: continuing with current cluster name. Teleport " +
				"clusters can not be renamed once they are created. You are seeing this " +
				"message for one of two reasons. Either you have not set \"cluster_name\" in " +
				"Teleport configuration and changed the hostname of the auth server or you " +
				"are trying to change the value of \"cluster_name\"."
			asrv.logger.ErrorContext(ctx, msg,
				"current_cluster_name", cn.GetClusterName(),
				"configured_cluster_name", cfg.ClusterName.GetClusterName(),
			)
		}

		asrv.logger.DebugContext(ctx, "Cluster configuration", "cluster_name", cn.GetClusterName())
		return nil
	})

	if err := g.Wait(); err != nil {
		return trace.Wrap(err)
	}

	// Override user passed in cluster name with what is in the backend.
	cfg.ClusterName = cn

	// Apply any outstanding migrations.
	if err := migration.Apply(ctx, asrv.logger, cfg.Backend); err != nil {
		return trace.Wrap(err, "applying migrations")
	}

	// generate certificate authorities if they don't exist
	if err := initializeAuthorities(ctx, asrv, &cfg); err != nil {
		return trace.Wrap(err)
	}

	if lib.IsInsecureDevMode() {
		const warningMessage = "Starting teleport in insecure mode. This is " +
			"dangerous! Sensitive information will be logged to console and " +
			"certificates will not be verified. Proceed with caution!"
		asrv.logger.WarnContext(ctx, warningMessage)
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

		span.AddEvent("creating preset database object import rules")
		if err := createPresetDatabaseObjectImportRule(ctx, asrv); err != nil {
			// merely raise a warning; this is not a fatal error.
			asrv.logger.WarnContext(ctx, "error creating preset database object import rules", "error", err)
		}
		span.AddEvent("completed creating database object import rules")
	} else {
		asrv.logger.InfoContext(ctx, "skipping preset role and user creation")
	}

	if !cfg.SkipPeriodicOperations {
		asrv.logger.InfoContext(ctx, "Auth server is running periodic operations")
		go asrv.runPeriodicOperations()
	} else {
		asrv.logger.InfoContext(ctx, "Auth server is skipping periodic operations")
	}

	return nil
}

func initializeAuthorities(ctx context.Context, asrv *Server, cfg *InitConfig) error {
	var (
		mu           sync.Mutex
		allKeysInUse [][]byte
	)
	usableKeysResults := make(map[types.CertAuthType]*keystore.UsableKeysResult)
	g, gctx := errgroup.WithContext(ctx)
	for _, caType := range types.CertAuthTypes {
		caType := caType
		g.Go(func() error {
			tctx, span := cfg.Tracer.Start(gctx, "auth/initializeAuthority", oteltrace.WithAttributes(attribute.String("type", string(caType))))
			defer span.End()

			caID := types.CertAuthID{Type: caType, DomainName: cfg.ClusterName.GetClusterName()}
			usableKeysResult, keysInUse, err := initializeAuthority(tctx, asrv, caID)
			if err != nil {
				return trace.Wrap(err)
			}

			mu.Lock()
			defer mu.Unlock()
			usableKeysResults[caType] = usableKeysResult
			allKeysInUse = append(allKeysInUse, keysInUse...)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return trace.Wrap(err)
	}

	if err := asrv.syncUsableKeysAlert(ctx, usableKeysResults); err != nil {
		return trace.Wrap(err)
	}

	// Collect CAs from integrations to avoid deleting them.
	err := clientutils.IterateResources(ctx, asrv.Services.ListIntegrations, func(ig types.Integration) error {
		caKeySet, err := igcredentials.GetIntegrationCertAuthorities(ctx, ig, asrv.Services)
		switch {
		case trace.IsNotImplemented(err):
		case err != nil:
			// This should not happen by design. In case integration is in a
			// bad state, log a warning instead of failing this initialization.
			asrv.logger.WarnContext(ctx, "Failed to fetch integration CAs", "ig", ig.GetName(), "error", err)
		default:
			allKeysInUse = append(allKeysInUse, collectKeysInUse(*caKeySet)...)
		}
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Delete any unused keys from the keyStore. This is to avoid exhausting
	// (or wasting) HSM resources.
	if err := asrv.keyStore.DeleteUnusedKeys(ctx, allKeysInUse); err != nil {
		// Key deletion is best-effort, log a warning if it fails and carry on.
		// We don't want to prevent a CA rotation, which may be necessary in
		// some cases where this would fail.
		asrv.logger.WarnContext(ctx, "An attempt to clean up unused HSM or KMS CA keys has failed unexpectedly", "error", err)
	}
	return nil
}

func initializeAuthority(ctx context.Context, asrv *Server, caID types.CertAuthID) (*keystore.UsableKeysResult, [][]byte, error) {
	ca, err := asrv.Services.GetCertAuthority(ctx, caID, true)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, nil, trace.Wrap(err)
		}

		asrv.logger.InfoContext(ctx, "First start: generating certificate authority", "ca_type", caID.Type)
		if ca, err = generateAuthority(ctx, asrv, caID); err != nil {
			return nil, nil, trace.Wrap(err)
		}

		if err := asrv.CreateCertAuthority(ctx, ca); err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	// Make sure the keystore has usable keys. This is a bit redundant if the CA
	// was just generated above, but cheap relative to generating the CA, and
	// it's nice to get the usableKeysResult.
	usableKeysResult, err := asrv.keyStore.HasUsableActiveKeys(ctx, ca)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if !usableKeysResult.CAHasUsableKeys {
		if ca.GetType() == types.HostCA {
			// We need to sign the local Admin identity to support auth startup
			// and local tctl. For this special case we add new
			// AdditionalTrustedKeys without any active keys. These keys will
			// sign the local Admin identity but nothing else (until a CA
			// rotation). Only the Host CA is necessary to issue the Admin
			// identity.
			//
			// We can only get here if all the active keys for this CA are in an
			// HSM or KMS that this auth is not configured to use. Because the
			// auth will not use PKCS#11 keys created by a different host UUID,
			// for clusters using HSM keys this includes cases where a new auth is
			// added to an HA cluster, or an existing auth's host UUID is reset.
			if err := asrv.ensureLocalAdditionalKeys(ctx, ca); err != nil {
				return nil, nil, trace.Wrap(err)
			}
			ca, err = asrv.Services.GetCertAuthority(ctx, caID, true)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			usableKeysResult, err = asrv.keyStore.HasUsableActiveKeys(ctx, ca)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
		} else {
			const msg = "This Auth Service is configured to use key types that the CA does not contain. " +
				"No new certificates can be signed with the existing keys. " +
				"You must perform a CA rotation to generate new keys, or adjust your configuration to use the existing keys."
			asrv.logger.WarnContext(ctx, msg,
				"configured_key_type", usableKeysResult.PreferredKeyType,
				"ca_type", caID.Type,
				"available_key_types", usableKeysResult.CAKeyTypes,
			)
		}
	} else if !usableKeysResult.CAHasPreferredKeyType {
		const msg = "This Auth Service is configured to use key types that the CA does not contain. " +
			"New certificates will continue to be signed with raw software keys but you must perform a CA rotation to begin using the new key type."
		asrv.logger.WarnContext(ctx, msg,
			"configured_key_type", usableKeysResult.PreferredKeyType,
			"ca_type", caID.Type,
			"available_key_types", usableKeysResult.CAKeyTypes,
		)
	}
	allKeyTypes := ca.AllKeyTypes()
	numKeyTypes := len(allKeyTypes)
	if numKeyTypes > 1 {
		const msg = "CA contains a combination of key types. If you are attempting to" +
			" configure HSM or KMS key storage, make sure it is configured on all auth servers in" +
			" this cluster and then perform a CA rotation: https://goteleport.com/docs/admin-guides/management/operations/ca-rotation/"
		asrv.logger.WarnContext(ctx, msg,
			"ca_type", caID.Type,
			"key_types", []string{strings.Join(allKeyTypes[:numKeyTypes-1], ", "), allKeyTypes[numKeyTypes-1]},
		)
	}

	keysInUse := collectKeysInUse(ca.GetActiveKeys(), ca.GetAdditionalTrustedKeys())
	return usableKeysResult, keysInUse, nil
}

func collectKeysInUse(cas ...types.CAKeySet) (keysInUse [][]byte) {
	for _, keySet := range cas {
		for _, sshKeyPair := range keySet.SSH {
			keysInUse = append(keysInUse, sshKeyPair.PrivateKey)
		}
		for _, tlsKeyPair := range keySet.TLS {
			keysInUse = append(keysInUse, tlsKeyPair.Key)
		}
		for _, jwtKeyPair := range keySet.JWT {
			keysInUse = append(keysInUse, jwtKeyPair.PrivateKey)
		}
	}
	return keysInUse
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

var secondFactorUpgradeInstructions = `
Teleport requires second factor authentication for local users.
The auth_service configuration should be updated to enable it.

auth_service:
  authentication:
    second_factor: on
    webauthn:
      rp_id: example.com

For more information:
- https://goteleport.com/docs/admin-guides/access-controls/guides/webauthn/
`

func initializeAuthPreference(ctx context.Context, asrv *Server, newAuthPref types.AuthPreference) error {
	const iterationLimit = 3
	for i := 0; i < iterationLimit; i++ {
		storedAuthPref, err := asrv.Services.GetAuthPreference(ctx)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}

		shouldReplace, err := shouldInitReplaceResourceWithOrigin(ctx, storedAuthPref, newAuthPref, asrv.logger)
		if err != nil {
			return trace.Wrap(err)
		}

		if !shouldReplace {
			if allowNoSecondFactor, _ := strconv.ParseBool(os.Getenv(teleport.EnvVarAllowNoSecondFactor)); allowNoSecondFactor {
				err := modules.ValidateResource(storedAuthPref)
				if errors.Is(err, modules.ErrCannotDisableSecondFactor) {
					return trace.Wrap(err, secondFactorUpgradeInstructions)
				}
				if err != nil {
					return trace.Wrap(err)
				}
			}
			return nil
		}

		switch {
		case storedAuthPref == nil:
			// This is a brand new cluster with no stored auth pref, set a
			// default signature algorithm suite.
			newAuthPref = newAuthPref.Clone()
			newAuthPref.SetDefaultSignatureAlgorithmSuite(types.SignatureAlgorithmSuiteParams{
				FIPS:          asrv.fips,
				UsingHSMOrKMS: asrv.keyStore.UsingHSMOrKMS(),
				Cloud:         modules.GetModules().Features().Cloud,
			})
		case newAuthPref.Origin() == types.OriginDefaults:
			// There is a stored auth preference which we are overwriting with a
			// default auth preference. Maintain the stored signature algorithm
			// suite to avoid automatically changing it on version upgrades, new
			// suites should always be opt-in for existing clusters.
			newAuthPref = newAuthPref.Clone()
			newAuthPref.SetSignatureAlgorithmSuite(storedAuthPref.GetSignatureAlgorithmSuite())
		case newAuthPref.GetSignatureAlgorithmSuite() == types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED:
			// There is a stored auth preference and the new auth preference is
			// coming from a config file where the signature_algorithm_suite is
			// unset. Maintain the stored signature algorithm suite otherwise we
			// would unset the default suite after the first auth restart.
			newAuthPref = newAuthPref.Clone()
			newAuthPref.SetSignatureAlgorithmSuite(storedAuthPref.GetSignatureAlgorithmSuite())
		}

		if storedAuthPref == nil {
			asrv.logger.InfoContext(ctx, "Creating cluster auth preference", "auth_preference", newAuthPref)
			_, err := asrv.CreateAuthPreference(ctx, newAuthPref)
			if trace.IsAlreadyExists(err) {
				continue
			}
			return trace.Wrap(err)
		}

		newAuthPref.SetRevision(storedAuthPref.GetRevision())
		_, err = asrv.UpdateAuthPreference(ctx, newAuthPref)
		if trace.IsCompareFailed(err) {
			continue
		}
		if errors.Is(err, modules.ErrCannotDisableSecondFactor) {
			return trace.Wrap(err, secondFactorUpgradeInstructions)
		}

		return trace.Wrap(err)
	}

	return trace.LimitExceeded("failed to initialize auth preference in %v iterations", iterationLimit)
}

func initializeClusterNetworkingConfig(ctx context.Context, asrv *Server, newNetConfig types.ClusterNetworkingConfig) error {
	const iterationLimit = 3
	for i := 0; i < 3; i++ {
		storedNetConfig, err := asrv.Services.GetClusterNetworkingConfig(ctx)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}

		shouldReplace, err := shouldInitReplaceResourceWithOrigin(ctx, storedNetConfig, newNetConfig, asrv.logger)
		if err != nil {
			return trace.Wrap(err)
		}

		if !shouldReplace {
			return nil
		}

		if storedNetConfig == nil {
			asrv.logger.InfoContext(ctx, "Creating cluster networking configuration", "networking_config", newNetConfig)
			_, err = asrv.CreateClusterNetworkingConfig(ctx, newNetConfig)
			if trace.IsAlreadyExists(err) {
				continue
			}

			return trace.Wrap(err)
		}

		asrv.logger.InfoContext(ctx, "Updating cluster networking configuration", "networking_config", newNetConfig)
		newNetConfig.SetRevision(storedNetConfig.GetRevision())
		_, err = asrv.UpdateClusterNetworkingConfig(ctx, newNetConfig)
		if trace.IsCompareFailed(err) {
			continue
		}

		return trace.Wrap(err)
	}

	return trace.LimitExceeded("failed to initialize cluster networking config in %v iterations", iterationLimit)
}

func initializeSessionRecordingConfig(ctx context.Context, asrv *Server, newRecConfig types.SessionRecordingConfig) error {
	const iterationLimit = 3
	for i := 0; i < iterationLimit; i++ {
		storedRecConfig, err := asrv.Services.GetSessionRecordingConfig(ctx)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}

		shouldReplace, err := shouldInitReplaceResourceWithOrigin(ctx, storedRecConfig, newRecConfig, asrv.logger)
		if err != nil {
			return trace.Wrap(err)
		}

		if !shouldReplace {
			return nil
		}

		if storedRecConfig == nil {
			asrv.logger.InfoContext(ctx, "Creating session recording config", "recording_config", newRecConfig)
			_, err := asrv.CreateSessionRecordingConfig(ctx, newRecConfig)
			if trace.IsAlreadyExists(err) {
				continue
			}

			return trace.Wrap(err)
		}

		asrv.logger.InfoContext(ctx, "Updating session recording config", "recording_config", newRecConfig)
		newRecConfig.SetRevision(storedRecConfig.GetRevision())
		_, err = asrv.UpdateSessionRecordingConfig(ctx, newRecConfig)
		if trace.IsCompareFailed(err) {
			continue
		}

		return trace.Wrap(err)
	}

	return trace.LimitExceeded("failed to initialize session recording config in %v iterations", iterationLimit)
}

func initializeAccessGraphSettings(ctx context.Context, asrv *Server) error {
	stored, err := asrv.Services.GetAccessGraphSettings(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if stored != nil {
		return nil
	}

	stored, err = clusterconfig.NewAccessGraphSettings(&clusterconfigpb.AccessGraphSettingsSpec{
		SecretsScanConfig: clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_DISABLED,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	asrv.logger.InfoContext(ctx, "Creating access graph settings", "settings", stored)
	_, err = asrv.CreateAccessGraphSettings(ctx, stored)
	if trace.IsAlreadyExists(err) {
		return nil
	}

	return trace.Wrap(err)
}

// shouldInitReplaceResourceWithOrigin determines whether the candidate
// resource should be used to replace the stored resource during auth server
// initialization.  Dynamically configured resources must not be overwritten
// when the corresponding file config is left unspecified (i.e., by defaults).
func shouldInitReplaceResourceWithOrigin(ctx context.Context, stored, candidate types.ResourceWithOrigin, logger *slog.Logger) (bool, error) {
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
			logger.WarnContext(ctx, "Stored resource that was configured dynamically is about to be discarded in favor of explicit file configuration", "resource", stored.GetKind())
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
func migrationStart(ctx context.Context, migrationName string, logger *slog.Logger) {
	logger.DebugContext(ctx, "Migration started", "migration_name", migrationName)
	migrations.WithLabelValues(migrationName).Set(1)
}

// migrationEnd marks the migration as inactive.
// It should be called when a migration ends.
func migrationEnd(ctx context.Context, migrationName string, logger *slog.Logger) {
	logger.DebugContext(ctx, "Migration ended", "migration_name", migrationName)
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
	CreateRole(ctx context.Context, role types.Role) (types.Role, error)
	// UpsertRole creates or updates a role and emits a related audit event.
	UpsertRole(ctx context.Context, role types.Role) (types.Role, error)
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
		services.NewSystemOktaAccessRole(),
		services.NewSystemOktaRequesterRole(),
		services.NewPresetTerraformProviderRole(),
		services.NewSystemIdentityCenterAccessRole(),
		services.NewPresetWildcardWorkloadIdentityIssuerRole(),
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
			// Specifically skip the Okta requester role, as it will be
			// modified by the Okta access list sync.
			if types.IsSystemResource(role) && role.GetName() != teleport.SystemOktaRequesterRoleName {
				// System resources *always* get reset on every auth startup
				if _, err := rm.UpsertRole(gctx, role); err != nil {
					return trace.Wrap(err, "failed upserting system role %s", role.GetName())
				}

				return nil
			}

			if _, err := rm.CreateRole(gctx, role); err != nil {
				if !trace.IsAlreadyExists(err) {
					return trace.WrapWithMessage(err, "failed to create preset role %v", role.GetName())
				}

				currentRole, err := rm.GetRole(gctx, role.GetName())
				if err != nil {
					return trace.Wrap(err)
				}

				role, err := services.AddRoleDefaults(gctx, currentRole)
				if trace.IsAlreadyExists(err) {
					return nil
				}
				if err != nil {
					return trace.Wrap(err)
				}

				if _, err := rm.UpsertRole(gctx, role); err != nil {
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
	CreateUser(ctx context.Context, user types.User) (types.User, error)
	// GetUser fetches a user from the repository by name, optionally fetching
	// any associated secrets.
	GetUser(ctx context.Context, username string, withSecrets bool) (types.User, error)
	// UpsertUser user creates or updates a user record as needed.
	UpsertUser(ctx context.Context, user types.User) (types.User, error)
}

// getPresetUsers returns a list of all preset users expected to be available on
// this cluster.
func getPresetUsers() []types.User {
	presets := []types.User{
		services.NewSystemAutomaticAccessBotUser(),
	}

	// Certain `New$FooUser()` functions will return a nil role if the
	// corresponding feature is disabled. They should be filtered out as they
	// are not actually made available on the cluster.
	return slices.DeleteFunc(presets, func(u types.User) bool { return u == nil })
}

// createPresetUsers creates all of the required user presets. No attempt is
// made to migrate any existing users to the lastest preset.
func createPresetUsers(ctx context.Context, um PresetUsers) error {
	users := getPresetUsers()
	for _, user := range users {
		// Some users are only valid for enterprise Teleport, and so will be
		// nil for an OSS build and can be skipped
		if user == nil {
			continue
		}

		if types.IsSystemResource(user) {
			// System resources *always* get reset on every auth startup
			if user, err := um.UpsertUser(ctx, user); err != nil {
				return trace.Wrap(err, "failed upserting system user %s", user.GetName())
			}
			continue
		}

		if user, err := um.CreateUser(ctx, user); err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err, "failed creating preset user %s", user.GetName())
		}
	}

	return nil
}

// createPresetDatabaseObjectImportRule will create a sample database object import rule if there are none.
func createPresetDatabaseObjectImportRule(ctx context.Context, rules services.DatabaseObjectImportRules) error {
	importRules, _, err := rules.ListDatabaseObjectImportRules(ctx, 100, "")
	if err != nil {
		return trace.Wrap(err, "failed listing available database object import rules")
	}
	if len(importRules) > 0 {
		// If the single rule is the old preset, we assume the user hasn't used
		// DB DAC feature yet since the old preset alone is usually not enough
		// to make things work. Replace it with the new preset.
		//
		// Creating and updating the database object import rule is handled on
		// a best-effort basis, so it’s not included in backend migrations.
		//
		// TODO(greedy52) DELETE in 18.0
		if len(importRules) == 1 && databaseobjectimportrule.IsOldImportAllObjectsRulePreset(importRules[0]) {
			rule := databaseobjectimportrule.NewPresetImportAllObjectsRule()
			if rule == nil {
				return nil
			}

			_, err = rules.UpsertDatabaseObjectImportRule(ctx, rule)
			if err != nil {
				return trace.Wrap(err, "failed to update the default database object import rule")
			}
		}
		return nil
	}

	rule := databaseobjectimportrule.NewPresetImportAllObjectsRule()
	if rule == nil {
		return nil
	}

	_, err = rules.CreateDatabaseObjectImportRule(ctx, rule)
	if err != nil {
		return trace.Wrap(err, "failed to create default database object import rule")
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
			case types.DatabaseCA, types.DatabaseClientCA, types.SAMLIDPCA, types.SPIFFECA:
				_, _, signerErr = keyStore.GetTLSCertAndSigner(ctx, r)
			case types.JWTSigner, types.OIDCIdPCA, types.OktaCA:
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
	key, err := cryptosuites.GenerateKey(context.Background(), cryptosuites.GetCurrentSuiteFromAuthPreference(a), cryptosuites.HostIdentity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshPub, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsPub, err := keys.MarshalPublicKey(key.Public())
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
			PublicSSHKey:         ssh.MarshalAuthorizedKey(sshPub),
			PublicTLSKey:         tlsPub,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	priv, err := keys.MarshalPrivateKey(key)
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
	migrationStart(ctx, "remote_clusters", asrv.logger)
	defer migrationEnd(ctx, "remote_clusters", asrv.logger)

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
			asrv.logger.DebugContext(ctx, "Migrations: skipping local cluster cert authority", "cert_authority", certAuthority.GetName())
			continue
		}
		// remote cluster already exists
		_, err = asrv.Services.GetRemoteCluster(ctx, certAuthority.GetName())
		if err == nil {
			asrv.logger.DebugContext(ctx, "Migrations: remote cluster already exists for cert authority", "cert_authority", certAuthority.GetName())
			continue
		}
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		// the cert authority is associated with trusted cluster
		_, err = asrv.Services.GetTrustedCluster(ctx, certAuthority.GetName())
		if err == nil {
			asrv.logger.DebugContext(ctx, "Migrations: trusted cluster resource exists for cert authority", "cert_authority", certAuthority.GetName())
			continue
		}
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		remoteCluster, err := types.NewRemoteCluster(certAuthority.GetName())
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = asrv.CreateRemoteCluster(ctx, remoteCluster)
		if err != nil {
			if !trace.IsAlreadyExists(err) {
				return trace.Wrap(err)
			}
		}
		asrv.logger.InfoContext(ctx, "Migrations: added remote cluster resource for cert authority", "cert_authority", certAuthority.GetName())
	}

	return nil
}

// ResourceApplyPriority specifies in which order the resources must be applied
// to avoid consistency issues. A lower priority means the resource is applied
// before.
var ResourceApplyPriority = map[string]int{
	types.KindRole:                    1,
	types.KindUser:                    2, // Users must be applied after Roles
	types.KindToken:                   3,
	types.KindClusterNetworkingConfig: 3,
	types.KindClusterAuthPreference:   3,
	// Bots should be applied after users and roles as at the moment they are actually converted to users and roles.
	// This will ensure that Bot Users/Roles are properly created regardless of the Teleport version from which the
	// resources have been exported.
	types.KindBot: 3,
}

// Unlike when resources are loaded via --bootstrap, we're inserting elements via their service.
// This means consistency is checked. This function support applying resources
// with dependencies (like a user referring to a role).
func applyResources(ctx context.Context, service *Services, resources []types.Resource) error {
	var err error
	slices.SortFunc(resources, func(a, b types.Resource) int {
		priorityA := ResourceApplyPriority[a.GetKind()]
		priorityB := ResourceApplyPriority[b.GetKind()]
		return cmp.Compare(priorityA, priorityB)
	})
	for _, resource := range resources {
		// Unwrap "new style" resources.
		// We always want to switch over the underlying type.
		var res any = resource
		if w, ok := res.(interface{ Unwrap() types.Resource153 }); ok {
			res = w.Unwrap()
		}
		switch r := res.(type) {
		case types.ProvisionToken:
			err = service.Provisioner.UpsertToken(ctx, r)
		case types.User:
			err = services.ValidateUserRoles(ctx, r, service.Access)
			if err != nil {
				return trace.Wrap(err)
			}
			_, err = service.Identity.UpsertUser(ctx, r)
		case types.Role:
			_, err = service.Access.UpsertRole(ctx, r)
		case types.ClusterNetworkingConfig:
			_, err = service.ClusterConfigurationInternal.UpsertClusterNetworkingConfig(ctx, r)
		case types.AuthPreference:
			_, err = service.ClusterConfigurationInternal.UpsertAuthPreference(ctx, r)
		case *machineidv1pb.Bot:
			_, err = machineidv1.UpsertBot(ctx, service, r, time.Now(), "system")
		case *dbobjectimportrulev1pb.DatabaseObjectImportRule:
			_, err = dbobjectimportrulev1.UpsertDatabaseObjectImportRule(ctx, service, r)
		case *autoupdatev1pb.AutoUpdateConfig:
			_, err = autoupdatev1.UpsertAutoUpdateConfig(ctx, service, r)
		case *autoupdatev1pb.AutoUpdateVersion:
			_, err = autoupdatev1.UpsertAutoUpdateVersion(ctx, service, r)
		default:
			return trace.NotImplemented("cannot apply resource of type %T", resource)
		}
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
