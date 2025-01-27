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

// Package auth implements certificate signing authority and access control server
// Authority server is composed of several parts:
//
// * Authority server itself that implements signing and acl logic
// * HTTP server wrapper for authority server
// * HTTP client wrapper
package auth

import (
	"bytes"
	"cmp"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/subtle"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	mathrand "math/rand/v2"
	"net"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	liblicense "github.com/gravitational/license"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/ssh"
	"golang.org/x/exp/maps"
	"golang.org/x/time/rate"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/retryutils"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/auth/userloginstate"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/bitbucket"
	"github.com/gravitational/teleport/lib/cache"
	"github.com/gravitational/teleport/lib/circleci"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/devicetrust/assertserver"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/gcp"
	"github.com/gravitational/teleport/lib/githubactions"
	"github.com/gravitational/teleport/lib/gitlab"
	"github.com/gravitational/teleport/lib/inventory"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/kubernetestoken"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/loginrule"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/release"
	"github.com/gravitational/teleport/lib/resourceusage"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/spacelift"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/terraformcloud"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/tpm"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
	vc "github.com/gravitational/teleport/lib/versioncontrol"
	"github.com/gravitational/teleport/lib/versioncontrol/github"
	uw "github.com/gravitational/teleport/lib/versioncontrol/upgradewindow"
)

const (
	ErrFieldKeyUserMaxedAttempts = "maxed-attempts"

	// MaxFailedAttemptsErrMsg is a user friendly error message that tells a user that they are locked.
	MaxFailedAttemptsErrMsg = "too many incorrect attempts, please try again later"
)

const (
	// githubCacheTimeout is how long Github org entries are cached.
	githubCacheTimeout = time.Hour

	// mfaDeviceNameMaxLen is the maximum length of a device name.
	mfaDeviceNameMaxLen = 30
)

const (
	OSSDesktopsCheckPeriod  = 5 * time.Minute
	OSSDesktopsAlertID      = "oss-desktops"
	OSSDesktopsAlertMessage = "Your cluster is beyond its allocation of 5 non-Active Directory Windows desktops. " +
		"Reach out for unlimited desktops with Teleport Enterprise."

	OSSDesktopsAlertLink     = "https://goteleport.com/r/upgrade-community?utm_campaign=CTA_windows_local"
	OSSDesktopsAlertLinkText = "Contact Sales"
	OSSDesktopsLimit         = 5
)

const (
	dynamicLabelCheckPeriod  = time.Hour
	dynamicLabelAlertID      = "dynamic-labels-in-deny-rules"
	dynamicLabelAlertMessage = "One or more roles has deny rules that include dynamic/ labels. " +
		"This is not recommended due to the volatility of dynamic/ labels and is not allowed for new roles. " +
		"(hint: use 'tctl get roles' to find roles that need updating)"
)

const (
	notificationsPageReadInterval = 5 * time.Millisecond
	notificationsWriteInterval    = 40 * time.Millisecond
)

var ErrRequiresEnterprise = services.ErrRequiresEnterprise

// ServerOption allows setting options as functional arguments to Server
type ServerOption func(*Server) error

// NewServer creates and configures a new Server instance
func NewServer(cfg *InitConfig, opts ...ServerOption) (*Server, error) {
	err := metrics.RegisterPrometheusCollectors(prometheusCollectors...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.VersionStorage == nil {
		return nil, trace.BadParameter("version storage is not set")
	}
	if cfg.Trust == nil {
		cfg.Trust = local.NewCAService(cfg.Backend)
	}
	if cfg.Presence == nil {
		cfg.Presence = local.NewPresenceService(cfg.Backend)
	}
	if cfg.Provisioner == nil {
		cfg.Provisioner = local.NewProvisioningService(cfg.Backend)
	}
	if cfg.Identity == nil {
		cfg.Identity, err = local.NewIdentityServiceV2(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.Access == nil {
		cfg.Access = local.NewAccessService(cfg.Backend)
	}
	if cfg.DynamicAccessExt == nil {
		cfg.DynamicAccessExt = local.NewDynamicAccessService(cfg.Backend)
	}
	if cfg.ClusterConfiguration == nil {
		clusterConfig, err := local.NewClusterConfigurationService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cfg.ClusterConfiguration = clusterConfig
	}
	if cfg.AutoUpdateService == nil {
		cfg.AutoUpdateService, err = local.NewAutoUpdateService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.Restrictions == nil {
		cfg.Restrictions = local.NewRestrictionsService(cfg.Backend)
	}
	if cfg.Apps == nil {
		cfg.Apps = local.NewAppService(cfg.Backend)
	}
	if cfg.Databases == nil {
		cfg.Databases = local.NewDatabasesService(cfg.Backend)
	}
	if cfg.DatabaseServices == nil {
		cfg.DatabaseServices = local.NewDatabaseServicesService(cfg.Backend)
	}
	if cfg.Kubernetes == nil {
		cfg.Kubernetes = local.NewKubernetesService(cfg.Backend)
	}
	if cfg.Status == nil {
		cfg.Status = local.NewStatusService(cfg.Backend)
	}
	if cfg.Events == nil {
		cfg.Events = local.NewEventsService(cfg.Backend)
	}
	if cfg.AuditLog == nil {
		cfg.AuditLog = events.NewDiscardAuditLog()
	}
	if cfg.Emitter == nil {
		cfg.Emitter = events.NewDiscardEmitter()
	}
	if cfg.Streamer == nil {
		cfg.Streamer = events.NewDiscardStreamer()
	}
	if cfg.WindowsDesktops == nil {
		cfg.WindowsDesktops = local.NewWindowsDesktopService(cfg.Backend)
	}
	if cfg.DynamicWindowsDesktops == nil {
		cfg.DynamicWindowsDesktops, err = local.NewDynamicWindowsDesktopService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.SAMLIdPServiceProviders == nil {
		cfg.SAMLIdPServiceProviders, err = local.NewSAMLIdPServiceProviderService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.UserGroups == nil {
		cfg.UserGroups, err = local.NewUserGroupService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if cfg.CrownJewels == nil {
		cfg.CrownJewels, err = local.NewCrownJewelsService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.ConnectionsDiagnostic == nil {
		cfg.ConnectionsDiagnostic = local.NewConnectionsDiagnosticService(cfg.Backend)
	}
	if cfg.SessionTrackerService == nil {
		cfg.SessionTrackerService, err = local.NewSessionTrackerService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.AssertionReplayService == nil {
		cfg.AssertionReplayService = local.NewAssertionReplayService(cfg.Backend)
	}
	if cfg.TraceClient == nil {
		cfg.TraceClient = tracing.NewNoopClient()
	}
	if cfg.UsageReporter == nil {
		cfg.UsageReporter = usagereporter.DiscardUsageReporter{}
	}
	if cfg.Okta == nil {
		cfg.Okta, err = local.NewOktaService(cfg.Backend, cfg.Clock)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.SecReports == nil {
		cfg.SecReports, err = local.NewSecReportsService(cfg.Backend, cfg.Clock)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.AccessLists == nil {
		cfg.AccessLists, err = local.NewAccessListService(cfg.Backend, cfg.Clock)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.DatabaseObjectImportRules == nil {
		cfg.DatabaseObjectImportRules, err = local.NewDatabaseObjectImportRuleService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.DatabaseObjects == nil {
		cfg.DatabaseObjects, err = local.NewDatabaseObjectService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.PluginData == nil {
		cfg.PluginData = local.NewPluginData(cfg.Backend, cfg.DynamicAccessExt)
	}
	if cfg.Integrations == nil {
		cfg.Integrations, err = local.NewIntegrationsService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.PluginStaticCredentials == nil {
		cfg.PluginStaticCredentials, err = local.NewPluginStaticCredentialsService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.UserTasks == nil {
		cfg.UserTasks, err = local.NewUserTasksService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.DiscoveryConfigs == nil {
		cfg.DiscoveryConfigs, err = local.NewDiscoveryConfigService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.UserPreferences == nil {
		cfg.UserPreferences = local.NewUserPreferencesService(cfg.Backend)
	}
	if cfg.UserLoginState == nil {
		cfg.UserLoginState, err = local.NewUserLoginStateService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.ProvisioningStates == nil {
		cfg.ProvisioningStates, err = local.NewProvisioningStateService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.IdentityCenter == nil {
		svcCfg := local.IdentityCenterServiceConfig{Backend: cfg.Backend}
		cfg.IdentityCenter, err = local.NewIdentityCenterService(svcCfg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.CloudClients == nil {
		cfg.CloudClients, err = cloud.NewClients()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.Notifications == nil {
		cfg.Notifications, err = local.NewNotificationsService(cfg.Backend, cfg.Clock)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.BotInstance == nil {
		cfg.BotInstance, err = local.NewBotInstanceService(cfg.Backend, cfg.Clock)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if cfg.SPIFFEFederations == nil {
		cfg.SPIFFEFederations, err = local.NewSPIFFEFederationService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err, "creating SPIFFEFederation service")
		}
	}
	if cfg.WorkloadIdentity == nil {
		workloadIdentity, err := local.NewWorkloadIdentityService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err, "creating WorkloadIdentity service")
		}
		cfg.WorkloadIdentity = workloadIdentity
	}
	if cfg.GitServers == nil {
		cfg.GitServers, err = local.NewGitServerService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err, "creating GitServer service")
		}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, teleport.ComponentAuth)
	}

	limiter := limiter.NewConnectionsLimiter(defaults.LimiterMaxConcurrentSignatures)

	keystoreOpts := &keystore.Options{
		HostUUID:             cfg.HostUUID,
		ClusterName:          cfg.ClusterName,
		AuthPreferenceGetter: cfg.ClusterConfiguration,
		FIPS:                 cfg.FIPS,
	}
	if cfg.KeyStoreConfig.PKCS11 != (servicecfg.PKCS11Config{}) {
		if !modules.GetModules().Features().GetEntitlement(entitlements.HSM).Enabled {
			return nil, fmt.Errorf("PKCS11 HSM support requires a license with the HSM feature enabled: %w", ErrRequiresEnterprise)
		}
	} else if cfg.KeyStoreConfig.GCPKMS != (servicecfg.GCPKMSConfig{}) {
		if !modules.GetModules().Features().GetEntitlement(entitlements.HSM).Enabled {
			return nil, fmt.Errorf("Google Cloud KMS support requires a license with the HSM feature enabled: %w", ErrRequiresEnterprise)
		}
	} else if cfg.KeyStoreConfig.AWSKMS != nil {
		if !modules.GetModules().Features().GetEntitlement(entitlements.HSM).Enabled {
			return nil, fmt.Errorf("AWS KMS support requires a license with the HSM feature enabled: %w", ErrRequiresEnterprise)
		}
	}
	keyStore, err := keystore.NewManager(context.Background(), &cfg.KeyStoreConfig, keystoreOpts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.KubeWaitingContainers == nil {
		cfg.KubeWaitingContainers, err = local.NewKubeWaitingContainerService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if cfg.AccessMonitoringRules == nil {
		cfg.AccessMonitoringRules, err = local.NewAccessMonitoringRulesService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if cfg.StaticHostUsers == nil {
		cfg.StaticHostUsers, err = local.NewStaticHostUserService(cfg.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	closeCtx, cancelFunc := context.WithCancel(context.TODO())
	services := &Services{
		TrustInternal:             cfg.Trust,
		PresenceInternal:          cfg.Presence,
		Provisioner:               cfg.Provisioner,
		Identity:                  cfg.Identity,
		Access:                    cfg.Access,
		DynamicAccessExt:          cfg.DynamicAccessExt,
		ClusterConfiguration:      cfg.ClusterConfiguration,
		AutoUpdateService:         cfg.AutoUpdateService,
		Restrictions:              cfg.Restrictions,
		Apps:                      cfg.Apps,
		Kubernetes:                cfg.Kubernetes,
		Databases:                 cfg.Databases,
		DatabaseServices:          cfg.DatabaseServices,
		AuditLogSessionStreamer:   cfg.AuditLog,
		Events:                    cfg.Events,
		WindowsDesktops:           cfg.WindowsDesktops,
		DynamicWindowsDesktops:    cfg.DynamicWindowsDesktops,
		SAMLIdPServiceProviders:   cfg.SAMLIdPServiceProviders,
		UserGroups:                cfg.UserGroups,
		SessionTrackerService:     cfg.SessionTrackerService,
		ConnectionsDiagnostic:     cfg.ConnectionsDiagnostic,
		Integrations:              cfg.Integrations,
		UserTasks:                 cfg.UserTasks,
		DiscoveryConfigs:          cfg.DiscoveryConfigs,
		Okta:                      cfg.Okta,
		AccessLists:               cfg.AccessLists,
		DatabaseObjectImportRules: cfg.DatabaseObjectImportRules,
		DatabaseObjects:           cfg.DatabaseObjects,
		SecReports:                cfg.SecReports,
		UserLoginStates:           cfg.UserLoginState,
		StatusInternal:            cfg.Status,
		UsageReporter:             cfg.UsageReporter,
		UserPreferences:           cfg.UserPreferences,
		PluginData:                cfg.PluginData,
		KubeWaitingContainer:      cfg.KubeWaitingContainers,
		Notifications:             cfg.Notifications,
		AccessMonitoringRules:     cfg.AccessMonitoringRules,
		CrownJewels:               cfg.CrownJewels,
		BotInstance:               cfg.BotInstance,
		SPIFFEFederations:         cfg.SPIFFEFederations,
		StaticHostUser:            cfg.StaticHostUsers,
		ProvisioningStates:        cfg.ProvisioningStates,
		IdentityCenter:            cfg.IdentityCenter,
		WorkloadIdentities:        cfg.WorkloadIdentity,
		PluginStaticCredentials:   cfg.PluginStaticCredentials,
		GitServers:                cfg.GitServers,
	}

	as := Server{
		bk:                      cfg.Backend,
		clock:                   cfg.Clock,
		limiter:                 limiter,
		Authority:               cfg.Authority,
		AuthServiceName:         cfg.AuthServiceName,
		ServerID:                cfg.HostUUID,
		cancelFunc:              cancelFunc,
		closeCtx:                closeCtx,
		emitter:                 cfg.Emitter,
		Streamer:                cfg.Streamer,
		Unstable:                local.NewUnstableService(cfg.Backend, cfg.AssertionReplayService),
		Services:                services,
		Cache:                   services,
		keyStore:                keyStore,
		traceClient:             cfg.TraceClient,
		fips:                    cfg.FIPS,
		loadAllCAs:              cfg.LoadAllCAs,
		httpClientForAWSSTS:     cfg.HTTPClientForAWSSTS,
		accessMonitoringEnabled: cfg.AccessMonitoringEnabled,
		logger:                  cfg.Logger,
	}
	as.inventory = inventory.NewController(&as, services,
		inventory.WithAuthServerID(cfg.HostUUID),
		inventory.WithOnConnect(func(s string) {
			if g, ok := connectedResourceGauges[s]; ok {
				g.Inc()
			} else {
				log.Warnf("missing connected resources gauge for keep alive %s (this is a bug)", s)
			}
		}),
		inventory.WithOnDisconnect(func(s string, c int) {
			if g, ok := connectedResourceGauges[s]; ok {
				g.Sub(float64(c))
			} else {
				log.Warnf("missing connected resources gauge for keep alive %s (this is a bug)", s)
			}
		}),
	)
	for _, o := range opts {
		if err := o(&as); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if as.clock == nil {
		as.clock = clockwork.NewRealClock()
	}
	as.githubOrgSSOCache, err = utils.NewFnCache(utils.FnCacheConfig{
		TTL: githubCacheTimeout,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	as.ttlCache, err = utils.NewFnCache(utils.FnCacheConfig{
		TTL: time.Second * 3,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, cacheEnabled := as.getCache()

	// cluster config ttl cache *must* be set up after `opts` has been applied to the server because
	// the Cache field starts off as a pointer to the local backend services and is only switched
	// over to being a proper cache during option processing.
	as.ReadOnlyCache, err = readonly.NewCache(readonly.CacheConfig{
		Upstream:    as.Cache,
		Disabled:    !cacheEnabled,
		ReloadOnErr: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if as.ghaIDTokenValidator == nil {
		as.ghaIDTokenValidator = githubactions.NewIDTokenValidator(
			githubactions.IDTokenValidatorConfig{
				Clock: as.clock,
			},
		)
	}
	if as.ghaIDTokenJWKSValidator == nil {
		as.ghaIDTokenJWKSValidator = githubactions.ValidateTokenWithJWKS
	}
	if as.spaceliftIDTokenValidator == nil {
		as.spaceliftIDTokenValidator = spacelift.NewIDTokenValidator(
			spacelift.IDTokenValidatorConfig{
				Clock: as.clock,
			},
		)
	}
	if as.gitlabIDTokenValidator == nil {
		as.gitlabIDTokenValidator, err = gitlab.NewIDTokenValidator(
			gitlab.IDTokenValidatorConfig{
				Clock:             as.clock,
				ClusterNameGetter: services,
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if as.circleCITokenValidate == nil {
		as.circleCITokenValidate = func(
			ctx context.Context, organizationID, token string,
		) (*circleci.IDTokenClaims, error) {
			return circleci.ValidateToken(
				ctx, as.clock, circleci.IssuerURLTemplate, organizationID, token,
			)
		}
	}
	if as.tpmValidator == nil {
		as.tpmValidator = tpm.Validate
	}
	if as.k8sTokenReviewValidator == nil {
		as.k8sTokenReviewValidator = &kubernetestoken.TokenReviewValidator{}
	}
	if as.k8sJWKSValidator == nil {
		as.k8sJWKSValidator = kubernetestoken.ValidateTokenWithJWKS
	}

	if as.gcpIDTokenValidator == nil {
		as.gcpIDTokenValidator = gcp.NewIDTokenValidator(
			gcp.IDTokenValidatorConfig{
				Clock: as.clock,
			},
		)
	}

	if as.terraformIDTokenValidator == nil {
		as.terraformIDTokenValidator = terraformcloud.NewIDTokenValidator(terraformcloud.IDTokenValidatorConfig{
			Clock: as.clock,
		})
	}

	if as.bitbucketIDTokenValidator == nil {
		as.bitbucketIDTokenValidator = bitbucket.NewIDTokenValidator(as.clock)
	}

	// Add in a login hook for generating state during user login.
	as.ulsGenerator, err = userloginstate.NewGenerator(userloginstate.GeneratorConfig{
		Log:         log,
		AccessLists: &as,
		Access:      &as,
		UsageEvents: &as,
		Clock:       cfg.Clock,
		Emitter:     as.emitter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	as.RegisterLoginHook(as.ulsGenerator.LoginHook(services.UserLoginStates))

	if _, ok := as.getCache(); !ok {
		log.Warn("Auth server starting without cache (may have negative performance implications).")
	}

	return &as, nil
}

// Services is a collection of services that are used by the auth server.
// Avoid using this type as a dependency and instead depend on the actual
// methods/services you need. It should really only be necessary to directly
// reference this type on auth.Server itself and on code that manages
// the lifecycle of the auth server.
type Services struct {
	services.TrustInternal
	services.PresenceInternal
	services.Provisioner
	services.Identity
	services.Access
	services.DynamicAccessExt
	services.ClusterConfiguration
	services.Restrictions
	services.Apps
	services.Kubernetes
	services.Databases
	services.DatabaseServices
	services.WindowsDesktops
	services.DynamicWindowsDesktops
	services.SAMLIdPServiceProviders
	services.UserGroups
	services.SessionTrackerService
	services.ConnectionsDiagnostic
	services.StatusInternal
	services.Integrations
	services.IntegrationsTokenGenerator
	services.UserTasks
	services.DiscoveryConfigs
	services.Okta
	services.AccessLists
	services.DatabaseObjectImportRules
	services.DatabaseObjects
	services.UserLoginStates
	services.UserPreferences
	services.PluginData
	services.SCIM
	services.Notifications
	usagereporter.UsageReporter
	types.Events
	events.AuditLogSessionStreamer
	services.SecReports
	services.KubeWaitingContainer
	services.AccessMonitoringRules
	services.CrownJewels
	services.BotInstance
	services.AccessGraphSecretsGetter
	services.DevicesGetter
	services.SPIFFEFederations
	services.StaticHostUser
	services.AutoUpdateService
	services.ProvisioningStates
	services.IdentityCenter
	services.WorkloadIdentities
	services.PluginStaticCredentials
	services.GitServers
}

// GetWebSession returns existing web session described by req.
// Implements ReadAccessPoint
func (r *Services) GetWebSession(ctx context.Context, req types.GetWebSessionRequest) (types.WebSession, error) {
	return r.Identity.WebSessions().Get(ctx, req)
}

// GetWebToken returns existing web token described by req.
// Implements ReadAccessPoint
func (r *Services) GetWebToken(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error) {
	return r.Identity.WebTokens().Get(ctx, req)
}

// GenerateAWSOIDCToken generates a token to be used to execute an AWS OIDC Integration action.
func (r *Services) GenerateAWSOIDCToken(ctx context.Context, integration string) (string, error) {
	return r.IntegrationsTokenGenerator.GenerateAWSOIDCToken(ctx, integration)
}

var (
	generateRequestsCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: teleport.MetricGenerateRequests,
			Help: "Number of requests to generate new server keys",
		},
	)
	generateThrottledRequestsCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: teleport.MetricGenerateRequestsThrottled,
			Help: "Number of throttled requests to generate new server keys",
		},
	)
	generateRequestsCurrent = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: teleport.MetricGenerateRequestsCurrent,
			Help: "Number of current generate requests for server keys",
		},
	)
	generateRequestsLatencies = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: teleport.MetricGenerateRequestsHistogram,
			Help: "Latency for generate requests for server keys",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
	)
	// UserLoginCount counts user logins
	UserLoginCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: teleport.MetricUserLoginCount,
			Help: "Number of times there was a user login",
		},
	)

	heartbeatsMissedByAuth = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: teleport.MetricHeartbeatsMissed,
			Help: "Number of heartbeats missed by auth server",
		},
	)

	roleCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "roles_total",
			Help:      "Number of roles that exist in the cluster",
		},
	)

	registeredAgents = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricRegisteredServers,
			Help:      "The number of Teleport services that are connected to an auth server.",
		},
		[]string{
			teleport.TagVersion,
			teleport.TagAutomaticUpdates,
		},
	)

	registeredAgentsInstallMethod = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricRegisteredServersByInstallMethods,
			Help:      "The number of Teleport services that are connected to an auth server by install method.",
		},
		[]string{teleport.TagInstallMethods},
	)

	migrations = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricMigrations,
			Help:      "Migrations tracks for each migration if it is active (1) or not (0).",
		},
		[]string{teleport.TagMigration},
	)

	totalInstancesMetric = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricTotalInstances,
			Help:      "Total teleport instances",
		},
	)

	enrolledInUpgradesMetric = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricEnrolledInUpgrades,
			Help:      "Number of instances enrolled in automatic upgrades",
		},
	)

	upgraderCountsMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricUpgraderCounts,
			Help:      "Tracks the number of instances advertising each upgrader",
		},
		[]string{
			teleport.TagUpgrader,
			teleport.TagVersion,
		},
	)

	accessRequestsCreatedMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricAccessRequestsCreated,
			Help:      "Tracks the number of created access requests",
		},
		[]string{teleport.TagRoles, teleport.TagResources},
	)

	userCertificatesGeneratedMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricUserCertificatesGenerated,
			Help:      "Tracks the number of user certificates generated",
		},
		[]string{teleport.TagPrivateKeyPolicy},
	)

	prometheusCollectors = []prometheus.Collector{
		generateRequestsCount, generateThrottledRequestsCount,
		generateRequestsCurrent, generateRequestsLatencies, UserLoginCount, heartbeatsMissedByAuth,
		registeredAgents, migrations,
		totalInstancesMetric, enrolledInUpgradesMetric, upgraderCountsMetric,
		accessRequestsCreatedMetric,
		registeredAgentsInstallMethod,
		userCertificatesGeneratedMetric,
		roleCount,
	}
)

// LoginHook is a function that will be called on a successful login. This will likely be used
// for enterprise services that need to add in feature specific operations after a user has been
// successfully authenticated. An example would be creating objects based on the user.
type LoginHook func(context.Context, types.User) error

// CreateDeviceWebTokenFunc creates a new DeviceWebToken for the logged in user.
//
// Used during a successful Web login, after the user was verified and the
// WebSession created.
//
// May return `nil, nil` if device trust isn't supported (OSS), disabled, or if
// the user has no suitable trusted device.
type CreateDeviceWebTokenFunc func(context.Context, *devicepb.DeviceWebToken) (*devicepb.DeviceWebToken, error)

// CreateDeviceAssertionFunc creates a new device assertion ceremony to authenticate
// a trusted device.
type CreateDeviceAssertionFunc func() (assertserver.Ceremony, error)

// ReadOnlyCache is a type alias used to assist with embedding [readonly.Cache] in places
// where it would have a naming conflict with other types named Cache.
type ReadOnlyCache = readonly.Cache

// Server keeps the cluster together. It acts as a certificate authority (CA) for
// a cluster and:
//   - generates the keypair for the node it's running on
//   - invites other SSH nodes to a cluster, by issuing invite tokens
//   - adds other SSH nodes to a cluster, by checking their token and signing their keys
//   - same for users and their sessions
//   - checks public keys to see if they're signed by it (can be trusted or not)
type Server struct {
	lock  sync.RWMutex
	clock clockwork.Clock
	bk    backend.Backend

	closeCtx   context.Context
	cancelFunc context.CancelFunc

	samlAuthService SAMLService
	oidcAuthService OIDCService

	releaseService release.Client

	loginRuleEvaluator loginrule.Evaluator

	sshca.Authority

	upgradeWindowStartHourGetter func(context.Context) (int64, error)

	// AuthServiceName is a human-readable name of this CA. If several Auth services are running
	// (managing multiple teleport clusters) this field is used to tell them apart in UIs
	// It usually defaults to the hostname of the machine the Auth service runs on.
	AuthServiceName string

	// ServerID is the server ID of this auth server.
	ServerID string

	// Unstable implements Unstable backend methods not suitable
	// for inclusion in Services.
	Unstable local.UnstableService

	// Services encapsulate services - provisioner, trust, etc. used by the auth
	// server in a separate structure. Reads through Services hit the backend.
	*Services

	// Cache should either be the same as Services, or a caching layer over it.
	// As it's an interface (and thus directly implementing all of its methods)
	// its embedding takes priority over Services (which only indirectly
	// implements its methods), thus any implemented GetFoo method on both Cache
	// and Services will call the one from Cache. To bypass the cache, call the
	// method on Services instead.
	authclient.Cache

	// ReadOnlyCache is a specialized cache that provides read-only shared references
	// in certain performance-critical paths where deserialization/cloning may be too
	// expensive at scale.
	*ReadOnlyCache

	// privateKey is used in tests to use pre-generated private keys
	privateKey []byte

	// cipherSuites is a list of ciphersuites that the auth server supports.
	cipherSuites []uint16

	// limiter limits the number of active connections per client IP.
	limiter *limiter.ConnectionsLimiter

	// Emitter is events emitter, used to submit discrete events
	emitter apievents.Emitter

	// Streamer is an events session streamer, used to create continuous
	// session related streams
	events.Streamer

	// keyStore manages all CA private keys, which  may or may not be backed by
	// HSMs
	keyStore *keystore.Manager

	// lockWatcher is a lock watcher, used to verify cert generation requests.
	lockWatcher *services.LockWatcher

	// UnifiedResourceCache is a cache of multiple resource kinds to be presented
	// in a unified manner in the web UI.
	UnifiedResourceCache *services.UnifiedResourceCache

	// AccessRequestCache is a cache of access requests that specifically provides
	// custom sorting options not available via the standard backend.
	AccessRequestCache *services.AccessRequestCache

	// UserNotificationCache is a cache of user-specific notifications.
	UserNotificationCache *services.UserNotificationCache

	// GlobalNotificationCache is a cache of global notifications.
	GlobalNotificationCache *services.GlobalNotificationCache

	inventory *inventory.Controller

	// githubOrgSSOCache is used to cache whether Github organizations use
	// external SSO or not.
	githubOrgSSOCache *utils.FnCache

	// ttlCache is a generic ttl cache. typed keys must be used.
	ttlCache *utils.FnCache

	// traceClient is used to forward spans to the upstream collector for components
	// within the cluster that don't have a direct connection to said collector
	traceClient otlptrace.Client

	// fips means FedRAMP/FIPS 140-2 compliant configuration was requested.
	fips bool

	// ghaIDTokenValidator allows ID tokens from GitHub Actions to be validated
	// by the auth server. It can be overridden for the purpose of tests.
	ghaIDTokenValidator ghaIDTokenValidator
	// ghaIDTokenJWKSValidator allows ID tokens from GitHub Actions to be
	// validated by the auth server using a known JWKS. It can be overridden for
	// the purpose of tests.
	ghaIDTokenJWKSValidator ghaIDTokenJWKSValidator

	// spaceliftIDTokenValidator allows ID tokens from Spacelift to be validated
	// by the auth server. It can be overridden for the purpose of tests.
	spaceliftIDTokenValidator spaceliftIDTokenValidator

	// gitlabIDTokenValidator allows ID tokens from GitLab CI to be validated by
	// the auth server. It can be overridden for the purpose of tests.
	gitlabIDTokenValidator gitlabIDTokenValidator

	// tpmValidator allows TPMs to be validated by the auth server. It can be
	// overridden for the purpose of tests.
	tpmValidator func(
		ctx context.Context, log *slog.Logger, params tpm.ValidateParams,
	) (*tpm.ValidatedTPM, error)

	// circleCITokenValidate allows ID tokens from CircleCI to be validated by
	// the auth server. It can be overridden for the purpose of tests.
	circleCITokenValidate func(ctx context.Context, organizationID, token string) (*circleci.IDTokenClaims, error)

	// k8sTokenReviewValidator allows tokens from Kubernetes to be validated
	// by the auth server using k8s Token Review API. It can be overridden for
	// the purpose of tests.
	k8sTokenReviewValidator k8sTokenReviewValidator
	// k8sJWKSValidator allows tokens from Kubernetes to be validated
	// by the auth server using a known JWKS. It can be overridden for the
	// purpose of tests.
	k8sJWKSValidator k8sJWKSValidator

	// gcpIDTokenValidator allows ID tokens from GCP to be validated by the auth
	// server. It can be overridden for the purpose of tests.
	gcpIDTokenValidator gcpIDTokenValidator

	// terraformIDTokenValidator allows JWTs from Terraform Cloud to be
	// validated by the auth server using a known JWKS. It can be overridden for
	// the purpose of tests.
	terraformIDTokenValidator terraformCloudIDTokenValidator

	bitbucketIDTokenValidator bitbucketIDTokenValidator

	// loadAllCAs tells tsh to load the host CAs for all clusters when trying to ssh into a node.
	loadAllCAs bool

	// license is the Teleport Enterprise license used to start the auth server
	license *liblicense.License

	// headlessAuthenticationWatcher is a headless authentication watcher,
	// used to catch and propagate headless authentication request changes.
	headlessAuthenticationWatcher *local.HeadlessAuthenticationWatcher

	loginHooksMu sync.RWMutex
	// loginHooks are a list of hooks that will be called on login.
	loginHooks []LoginHook

	// httpClientForAWSSTS overwrites the default HTTP client used for making
	// STS requests.
	httpClientForAWSSTS utils.HTTPDoClient

	// accessMonitoringEnabled is a flag that indicates whether access monitoring is enabled.
	accessMonitoringEnabled bool

	// ulsGenerator is the user login state generator.
	ulsGenerator *userloginstate.Generator

	// createDeviceWebTokenFunc is the CreateDeviceWebToken implementation.
	// Is nil on OSS clusters.
	createDeviceWebTokenFunc CreateDeviceWebTokenFunc

	// deviceAssertionServer holds the server-side implementation of device assertions.
	//
	// It is used to authenticate devices previously enrolled in the cluster. The goal
	// is to provide an API for devices to authenticate with the cluster without the need
	// for valid user credentials, e.g. when running `tsh scan keys`.
	//
	// The value is nil on OSS clusters.
	deviceAssertionServer CreateDeviceAssertionFunc

	// bcryptCostOverride overrides the bcrypt cost for operations executed
	// directly by [Server].
	// Used for testing.
	bcryptCostOverride *int

	// GithubUserAndTeamsOverride overrides the user and teams that would
	// normally be fetched from the GitHub API. Used for testing.
	GithubUserAndTeamsOverride func() (*GithubUserResponse, []GithubTeamResponse, error)

	// logger is the logger used by the auth server.
	logger *slog.Logger
}

// SetSAMLService registers svc as the SAMLService that provides the SAML
// connector implementation. If a SAMLService has already been registered, this
// will override the previous registration.
func (a *Server) SetSAMLService(svc SAMLService) {
	a.samlAuthService = svc
}

// SetOIDCService registers svc as the OIDCService that provides the OIDC
// connector implementation. If a OIDCService has already been registered, this
// will override the previous registration.
func (a *Server) SetOIDCService(svc OIDCService) {
	a.oidcAuthService = svc
}

// SetLicense sets the license
func (a *Server) SetLicense(license *liblicense.License) {
	a.license = license
}

// SetReleaseService sets the release service
func (a *Server) SetReleaseService(svc release.Client) {
	a.releaseService = svc
}

// SetUpgradeWindowStartHourGetter sets the getter used to sync the ClusterMaintenanceConfig resource
// with the cloud UpgradeWindowStartHour value.
func (a *Server) SetUpgradeWindowStartHourGetter(fn func(context.Context) (int64, error)) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.upgradeWindowStartHourGetter = fn
}

func (a *Server) getUpgradeWindowStartHourGetter() func(context.Context) (int64, error) {
	a.lock.Lock()
	defer a.lock.Unlock()
	return a.upgradeWindowStartHourGetter
}

// SetLoginRuleEvaluator sets the login rule evaluator.
func (a *Server) SetLoginRuleEvaluator(l loginrule.Evaluator) {
	a.loginRuleEvaluator = l
}

// GetLoginRuleEvaluator returns the login rule evaluator. It is guaranteed not
// to return nil, if no evaluator has been installed it will return
// [loginrule.NullEvaluator].
func (a *Server) GetLoginRuleEvaluator() loginrule.Evaluator {
	if a.loginRuleEvaluator == nil {
		return loginrule.NullEvaluator{}
	}
	return a.loginRuleEvaluator
}

// RegisterLoginHook will register a login hook with the auth server.
func (a *Server) RegisterLoginHook(hook LoginHook) {
	a.loginHooksMu.Lock()
	defer a.loginHooksMu.Unlock()

	a.loginHooks = append(a.loginHooks, hook)
}

// CallLoginHooks will call the registered login hooks.
func (a *Server) CallLoginHooks(ctx context.Context, user types.User) error {
	// Make a copy of the login hooks to operate on.
	a.loginHooksMu.RLock()
	loginHooks := make([]LoginHook, len(a.loginHooks))
	copy(loginHooks, a.loginHooks)
	a.loginHooksMu.RUnlock()

	if len(loginHooks) == 0 {
		return nil
	}

	var errs []error
	for _, hook := range loginHooks {
		errs = append(errs, hook(ctx, user))
	}

	return trace.NewAggregate(errs...)
}

// ResetLoginHooks will clear out the login hooks.
func (a *Server) ResetLoginHooks() {
	a.loginHooksMu.Lock()
	a.loginHooks = nil
	a.loginHooksMu.Unlock()
}

// CloseContext returns the close context
func (a *Server) CloseContext() context.Context {
	return a.closeCtx
}

// SetUnifiedResourcesCache sets the unified resource cache.
func (a *Server) SetUnifiedResourcesCache(unifiedResourcesCache *services.UnifiedResourceCache) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.UnifiedResourceCache = unifiedResourcesCache
}

// SetAccessRequestCache sets the access request cache.
func (a *Server) SetAccessRequestCache(accessRequestCache *services.AccessRequestCache) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.AccessRequestCache = accessRequestCache
}

// SetUserNotificationsCache sets the user notification cache.
func (a *Server) SetUserNotificationCache(userNotificationCache *services.UserNotificationCache) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.UserNotificationCache = userNotificationCache
}

// SetGlobalNotificationsCache sets the global notification cache.
func (a *Server) SetGlobalNotificationCache(globalNotificationCache *services.GlobalNotificationCache) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.GlobalNotificationCache = globalNotificationCache
}

func (a *Server) SetLockWatcher(lockWatcher *services.LockWatcher) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.lockWatcher = lockWatcher
}

func (a *Server) checkLockInForce(mode constants.LockingMode, targets []types.LockTarget) error {
	a.lock.RLock()
	defer a.lock.RUnlock()
	if a.lockWatcher == nil {
		return trace.BadParameter("lockWatcher is not set")
	}
	return a.lockWatcher.CheckLockInForce(mode, targets...)
}

func (a *Server) SetHeadlessAuthenticationWatcher(headlessAuthenticationWatcher *local.HeadlessAuthenticationWatcher) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.headlessAuthenticationWatcher = headlessAuthenticationWatcher
}

// SetDeviceAssertionServer sets the device assertion implementation.
func (a *Server) SetDeviceAssertionServer(f CreateDeviceAssertionFunc) {
	a.lock.Lock()
	a.deviceAssertionServer = f
	a.lock.Unlock()
}

// GetDeviceAssertionServer returns the device assertion implementation.
// On OSS clusters, this will return a non nil function that returns an error.
func (a *Server) GetDeviceAssertionServer() CreateDeviceAssertionFunc {
	a.lock.RLock()
	defer a.lock.RUnlock()
	if a.deviceAssertionServer == nil {
		return func() (assertserver.Ceremony, error) {
			return nil, trace.NotImplemented("device assertions are not supported on OSS clusters")
		}
	}
	return a.deviceAssertionServer
}

func (a *Server) SetCreateDeviceWebTokenFunc(f CreateDeviceWebTokenFunc) {
	a.lock.Lock()
	a.createDeviceWebTokenFunc = f
	a.lock.Unlock()
}

// createDeviceWebToken safely calls the underlying [CreateDeviceWebTokenFunc].
func (a *Server) createDeviceWebToken(ctx context.Context, webToken *devicepb.DeviceWebToken) (*devicepb.DeviceWebToken, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()
	if a.createDeviceWebTokenFunc == nil {
		return nil, nil
	}
	token, err := a.createDeviceWebTokenFunc(ctx, webToken)
	return token, trace.Wrap(err)
}

func (a *Server) bcryptCost() int {
	if cost := a.bcryptCostOverride; cost != nil {
		return *cost
	}
	return bcrypt.DefaultCost
}

// syncUpgradeWindowStartHour attempts to load the cloud UpgradeWindowStartHour value and set
// the ClusterMaintenanceConfig resource's AgentUpgrade.UTCStartHour field to match it.
func (a *Server) syncUpgradeWindowStartHour(ctx context.Context) error {
	getter := a.getUpgradeWindowStartHourGetter()
	if getter == nil {
		return trace.Errorf("getter has not been registered")
	}

	startHour, err := getter(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	cmc, err := a.GetClusterMaintenanceConfig(ctx)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}

		// create an empty maintenance config resource on NotFound
		cmc = types.NewClusterMaintenanceConfig()
	}

	agentWindow, _ := cmc.GetAgentUpgradeWindow()

	agentWindow.UTCStartHour = uint32(startHour)

	cmc.SetAgentUpgradeWindow(agentWindow)

	if err := a.UpdateClusterMaintenanceConfig(ctx, cmc); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// periodicIntervalKey is used to uniquely identify the subintervals registered with
// the interval.MultiInterval instance that we use for managing periodics operations.

type periodicIntervalKey int

const (
	heartbeatCheckKey periodicIntervalKey = 1 + iota
	rotationCheckKey
	metricsKey
	releaseCheckKey
	localReleaseCheckKey
	instancePeriodicsKey
	dynamicLabelsCheckKey
	notificationsCleanupKey
	desktopCheckKey
	upgradeWindowCheckKey
	roleCountKey
)

// runPeriodicOperations runs some periodic bookkeeping operations
// performed by auth server
func (a *Server) runPeriodicOperations() {
	firstReleaseCheck := retryutils.FullJitter(time.Hour * 6)

	// this environment variable is "unstable" since it will be deprecated
	// by an upcoming tctl command. currently exists for testing purposes only.
	if os.Getenv("TELEPORT_UNSTABLE_VC_SYNC_ON_START") == "yes" {
		firstReleaseCheck = retryutils.HalfJitter(time.Second * 10)
	}

	// run periodic functions with a semi-random period
	// to avoid contention on the database in case if there are multiple
	// auth servers running - so they don't compete trying
	// to update the same resources.
	period := retryutils.HalfJitter(2 * defaults.HighResPollingPeriod)

	ticker := interval.NewMulti(
		a.GetClock(),
		interval.SubInterval[periodicIntervalKey]{
			Key:      rotationCheckKey,
			Duration: period,
		},
		interval.SubInterval[periodicIntervalKey]{
			Key:           metricsKey,
			Duration:      defaults.PrometheusScrapeInterval,
			FirstDuration: 5 * time.Second,
			Jitter:        retryutils.SeventhJitter,
		},
		interval.SubInterval[periodicIntervalKey]{
			Key:           instancePeriodicsKey,
			Duration:      9 * time.Minute,
			FirstDuration: retryutils.HalfJitter(time.Minute),
			Jitter:        retryutils.SeventhJitter,
		},
		interval.SubInterval[periodicIntervalKey]{
			Key:           notificationsCleanupKey,
			Duration:      48 * time.Hour,
			FirstDuration: retryutils.FullJitter(time.Hour),
			Jitter:        retryutils.SeventhJitter,
		},
		interval.SubInterval[periodicIntervalKey]{
			Key:           roleCountKey,
			Duration:      12 * time.Hour,
			FirstDuration: retryutils.FullJitter(time.Minute),
			Jitter:        retryutils.SeventhJitter,
		},
	)

	defer ticker.Stop()

	// Prevent some periodic operations from running for dashboard tenants.
	if !services.IsDashboard(*modules.GetModules().Features().ToProto()) {
		ticker.Push(interval.SubInterval[periodicIntervalKey]{
			Key:           dynamicLabelsCheckKey,
			Duration:      dynamicLabelCheckPeriod,
			FirstDuration: retryutils.HalfJitter(10 * time.Second),
			Jitter:        retryutils.SeventhJitter,
		})
		ticker.Push(interval.SubInterval[periodicIntervalKey]{
			Key:      heartbeatCheckKey,
			Duration: apidefaults.ServerKeepAliveTTL() * 2,
			Jitter:   retryutils.SeventhJitter,
		})
		ticker.Push(interval.SubInterval[periodicIntervalKey]{
			Key:           releaseCheckKey,
			Duration:      24 * time.Hour,
			FirstDuration: firstReleaseCheck,
			// note the use of FullJitter for the releases check interval. this lets us ensure
			// that frequent restarts don't prevent checks from happening despite the infrequent
			// effective check rate.
			Jitter: retryutils.FullJitter,
		})
		// more frequent release check that just re-calculates alerts based on previously
		// pulled versioning info.
		ticker.Push(interval.SubInterval[periodicIntervalKey]{
			Key:           localReleaseCheckKey,
			Duration:      10 * time.Minute,
			FirstDuration: retryutils.HalfJitter(10 * time.Second),
			Jitter:        retryutils.HalfJitter,
		})
	}

	if modules.GetModules().IsOSSBuild() {
		ticker.Push(interval.SubInterval[periodicIntervalKey]{
			Key:           desktopCheckKey,
			Duration:      OSSDesktopsCheckPeriod,
			FirstDuration: retryutils.HalfJitter(10 * time.Second),
			Jitter:        retryutils.HalfJitter,
		})
	} else if err := a.DeleteClusterAlert(a.closeCtx, OSSDesktopsAlertID); err != nil && !trace.IsNotFound(err) {
		log.Warnf("Can't delete OSS non-AD desktops limit alert: %v", err)
	}

	// isolate the schedule of potentially long-running refreshRemoteClusters() from other tasks
	go func() {
		// reasonably small interval to ensure that users observe clusters as online within 1 minute of adding them.
		remoteClustersRefresh := interval.New(interval.Config{
			Duration: time.Second * 40,
			Jitter:   retryutils.SeventhJitter,
		})
		defer remoteClustersRefresh.Stop()

		for {
			select {
			case <-a.closeCtx.Done():
				return
			case <-remoteClustersRefresh.Next():
				a.refreshRemoteClusters(a.closeCtx)
			}
		}
	}()

	// cloud auth servers need to periodically sync the upgrade window
	// from the cloud db.
	if modules.GetModules().Features().Cloud {
		ticker.Push(interval.SubInterval[periodicIntervalKey]{
			Key:           upgradeWindowCheckKey,
			Duration:      3 * time.Minute,
			FirstDuration: retryutils.FullJitter(30 * time.Second),
			Jitter:        retryutils.SeventhJitter,
		})
	}

	for {
		select {
		case <-a.closeCtx.Done():
			return
		case tick := <-ticker.Next():
			switch tick.Key {
			case rotationCheckKey:
				go func() {
					if err := a.AutoRotateCertAuthorities(a.closeCtx); err != nil {
						if trace.IsCompareFailed(err) {
							log.Debugf("Cert authority has been updated concurrently: %v.", err)
						} else {
							log.Errorf("Failed to perform cert rotation check: %v.", err)
						}
					}
				}()
			case heartbeatCheckKey:
				go func() {
					req := &proto.ListUnifiedResourcesRequest{Kinds: []string{types.KindNode}, SortBy: types.SortBy{Field: types.ResourceKind}}

					for {
						_, next, err := a.UnifiedResourceCache.IterateUnifiedResources(a.closeCtx,
							func(rwl types.ResourceWithLabels) (bool, error) {
								srv, ok := rwl.(types.Server)
								if !ok {
									return false, nil
								}
								if services.NodeHasMissedKeepAlives(srv) {
									heartbeatsMissedByAuth.Inc()
								}

								if srv.GetSubKind() != types.SubKindOpenSSHNode {
									return false, nil
								}
								// TODO(tross) DELETE in v20.0.0 - all invalid hostnames should have been sanitized by then.
								if !validServerHostname(srv.GetHostname()) {
									logger := a.logger.With("server", srv.GetName(), "hostname", srv.GetHostname())

									logger.DebugContext(a.closeCtx, "sanitizing invalid static SSH server hostname")
									// Any existing static hosts will not have their
									// hostname sanitized since they don't heartbeat.
									if err := sanitizeHostname(srv); err != nil {
										logger.WarnContext(a.closeCtx, "failed to sanitize static SSH server hostname", "error", err)
										return false, nil
									}

									if _, err := a.Services.UpdateNode(a.closeCtx, srv); err != nil && !trace.IsCompareFailed(err) {
										logger.WarnContext(a.closeCtx, "failed to update SSH server hostname", "error", err)
									}
								} else if oldHostname, ok := srv.GetLabel(replacedHostnameLabel); ok && validServerHostname(oldHostname) {
									// If the hostname has been replaced by a sanitized version, revert it back to the original
									// if the original is valid under the most recent rules.
									logger := a.logger.With("server", srv.GetName(), "old_hostname", oldHostname, "sanitized_hostname", srv.GetHostname())
									if err := restoreSanitizedHostname(srv); err != nil {
										logger.WarnContext(a.closeCtx, "failed to restore sanitized static SSH server hostname", "error", err)
										return false, nil
									}
									if _, err := a.Services.UpdateNode(a.closeCtx, srv); err != nil && !trace.IsCompareFailed(err) {
										log.Warnf("Failed to update node hostname: %v", err)
									}
								}

								return false, nil
							},
							req,
						)
						if err != nil {
							log.Errorf("Failed to load nodes for heartbeat metric calculation: %v", err)
							return
						}

						req.StartKey = next
						if req.StartKey == "" {
							break
						}
					}
				}()
			case metricsKey:
				go a.updateAgentMetrics()
			case releaseCheckKey:
				go a.syncReleaseAlerts(a.closeCtx, true)
			case localReleaseCheckKey:
				go a.syncReleaseAlerts(a.closeCtx, false)
			case instancePeriodicsKey:
				go a.doInstancePeriodics(a.closeCtx)
			case desktopCheckKey:
				go a.syncDesktopsLimitAlert(a.closeCtx)
			case dynamicLabelsCheckKey:
				go a.syncDynamicLabelsAlert(a.closeCtx)
			case notificationsCleanupKey:
				go a.CleanupNotifications(a.closeCtx)
			case upgradeWindowCheckKey:
				go a.syncUpgradeWindowStartHour(a.closeCtx)
			case roleCountKey:
				go a.tallyRoles(a.closeCtx)
			}
		}
	}
}

func (a *Server) tallyRoles(ctx context.Context) {
	var count = 0
	a.logger.DebugContext(ctx, "tallying roles")
	defer func() {
		a.logger.DebugContext(ctx, "tallying roles completed", "role_count", count)
	}()

	req := &proto.ListRolesRequest{Limit: 20}

	readLimiter := time.NewTicker(20 * time.Millisecond)
	defer readLimiter.Stop()

	for {
		resp, err := a.Cache.ListRoles(ctx, req)
		if err != nil {
			return
		}

		count += len(resp.Roles)
		req.StartKey = resp.NextKey

		if req.StartKey == "" {
			break
		}

		select {
		case <-readLimiter.C:
		case <-ctx.Done():
			return
		}
	}

	roleCount.Set(float64(count))
}

func (a *Server) doInstancePeriodics(ctx context.Context) {
	const slowRate = time.Millisecond * 200 // 5 reads per second
	const fastRate = time.Millisecond * 5   // 200 reads per second
	const dynamicPeriod = time.Minute * 3

	instances := a.GetInstances(ctx, types.InstanceFilter{})

	// dynamically scale the rate-limiting we apply to reading instances
	// s.t. we read at a progressively faster rate as we observe larger
	// connected instance counts. this isn't a perfect metric, but it errs
	// on the side of slowness, which is preferable for this kind of periodic.
	instanceRate := slowRate
	if ci := a.inventory.ConnectedInstances(); ci > 0 {
		localDynamicRate := dynamicPeriod / time.Duration(ci)
		if localDynamicRate < fastRate {
			localDynamicRate = fastRate
		}

		if localDynamicRate < instanceRate {
			instanceRate = localDynamicRate
		}
	}

	limiter := rate.NewLimiter(rate.Every(instanceRate), 100)
	instances = stream.RateLimit(instances, func() error {
		return limiter.Wait(ctx)
	})

	// cloud deployments shouldn't include control-plane elements in
	// metrics since information about them is not actionable and may
	// produce misleading/confusing results.
	skipControlPlane := modules.GetModules().Features().Cloud

	// set up aggregators for our periodics
	uep := newUpgradeEnrollPeriodic()

	// stream all instances to all aggregators
	for instances.Next() {
		if skipControlPlane {
			for _, service := range instances.Item().GetServices() {
				if service.IsControlPlane() {
					continue
				}
			}
		}

		uep.VisitInstance(instances.Item())
	}

	if err := instances.Done(); err != nil {
		log.Warnf("Failed stream instances for periodics: %v", err)
		return
	}

	// create/delete upgrade enroll prompt as appropriate
	enrollMsg, shouldPrompt := uep.GenerateEnrollPrompt()
	a.handleUpgradeEnrollPrompt(ctx, enrollMsg, shouldPrompt)
}

const (
	upgradeEnrollAlertID = "auto-upgrade-enroll"
)

func (a *Server) handleUpgradeEnrollPrompt(ctx context.Context, msg string, shouldPrompt bool) {
	const alertTTL = time.Minute * 30

	if !shouldPrompt {
		if err := a.DeleteClusterAlert(ctx, upgradeEnrollAlertID); err != nil && !trace.IsNotFound(err) {
			log.Warnf("Failed to delete %s alert: %v", upgradeEnrollAlertID, err)
		}
		return
	}
	alert, err := types.NewClusterAlert(
		upgradeEnrollAlertID,
		msg,
		// Defaulting to "low" severity level. We may want to make this dynamic
		// in the future depending on the distance from up-to-date.
		types.WithAlertSeverity(types.AlertSeverity_LOW),
		types.WithAlertLabel(types.AlertVerbPermit, fmt.Sprintf("%s:%s", types.KindInstance, types.VerbRead)),
		// hide the normal upgrade alert for users who can see this alert as it is
		// generally more actionable/specific.
		types.WithAlertLabel(types.AlertSupersedes, releaseAlertID),
		types.WithAlertLabel(types.AlertOnLogin, "yes"),
		types.WithAlertExpires(a.clock.Now().Add(alertTTL)),
	)
	if err != nil {
		log.Warnf("Failed to build %s alert: %v (this is a bug)", upgradeEnrollAlertID, err)
		return
	}
	if err := a.UpsertClusterAlert(ctx, alert); err != nil {
		log.Warnf("Failed to set %s alert: %v", upgradeEnrollAlertID, err)
		return
	}
}

const (
	releaseAlertID = "upgrade-suggestion"
	secAlertID     = "security-patch-available"
	verInUseLabel  = "teleport.internal/ver-in-use"
)

// syncReleaseAlerts calculates alerts related to new teleport releases. When checkRemote
// is true it pulls the latest release info from GitHub.  Otherwise, it loads the versions used
// for the most recent alerts and re-syncs with latest cluster state.
func (a *Server) syncReleaseAlerts(ctx context.Context, checkRemote bool) {
	log.Debug("Checking for new teleport releases via github api.")

	// NOTE: essentially everything in this function is going to be
	// scrapped/replaced once the inventory and version-control systems
	// are a bit further along.

	current := vc.NewTarget(vc.Normalize(teleport.Version))

	// this environment variable is "unstable" since it will be deprecated
	// by an upcoming tctl command. currently exists for testing purposes only.
	if t := vc.NewTarget(os.Getenv("TELEPORT_UNSTABLE_VC_VERSION")); t.Ok() {
		current = t
	}

	visitor := vc.Visitor{
		Current: current,
	}

	// users cannot upgrade their own auth instances in cloud, so it isn't helpful
	// to generate alerts for releases newer than the current auth server version.
	if modules.GetModules().Features().Cloud {
		visitor.NotNewerThan = current
	}

	var loadFailed bool

	if checkRemote {
		// scrape the github releases API with our visitor
		if err := github.Visit(&visitor); err != nil {
			log.Warnf("Failed to load github releases: %v (this will not impact teleport functionality)", err)
			loadFailed = true
		}
	} else {
		if err := a.visitCachedAlertVersions(ctx, &visitor); err != nil {
			log.Warnf("Failed to load release alert into: %v (this will not impact teleport functionality)", err)
			loadFailed = true
		}
	}

	a.doReleaseAlertSync(ctx, current, visitor, !loadFailed)
}

// visitCachedAlertVersions updates the visitor with targets reconstructed from the metadata
// of existing alerts. This lets us "reevaluate" the alerts based on newer cluster state without
// re-pulling the releases page. Future version of teleport will cache actual full release
// descriptions, rending this unnecessary.
func (a *Server) visitCachedAlertVersions(ctx context.Context, visitor *vc.Visitor) error {
	// reconstruct the target for the "latest stable" alert if it exists.
	alert, err := a.getClusterAlert(ctx, releaseAlertID)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if err == nil {
		if t := vc.NewTarget(alert.Metadata.Labels[verInUseLabel]); t.Ok() {
			visitor.Visit(t)
		}
	}

	// reconstruct the target for the "latest sec patch" alert if it exists.
	alert, err = a.getClusterAlert(ctx, secAlertID)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if err == nil {
		if t := vc.NewTarget(alert.Metadata.Labels[verInUseLabel], vc.SecurityPatch(true)); t.Ok() {
			visitor.Visit(t)
		}
	}
	return nil
}

func (a *Server) getClusterAlert(ctx context.Context, id string) (types.ClusterAlert, error) {
	alerts, err := a.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		AlertID: id,
	})
	if err != nil {
		return types.ClusterAlert{}, trace.Wrap(err)
	}
	if len(alerts) == 0 {
		return types.ClusterAlert{}, trace.NotFound("cluster alert %q not found", id)
	}
	return alerts[0], nil
}

func (a *Server) doReleaseAlertSync(ctx context.Context, current vc.Target, visitor vc.Visitor, cleanup bool) {
	const alertTTL = time.Minute * 30
	// use visitor to find the oldest version among connected instances.
	// TODO(fspmarshall): replace this check as soon as we have a backend inventory repr. using
	// connected instances is a poor approximation and may lead to missed notifications if auth
	// server is up to date, but instances not connected to this auth need update.
	var instanceVisitor vc.Visitor
	a.inventory.Iter(func(handle inventory.UpstreamHandle) {
		v := vc.Normalize(handle.Hello().Version)
		instanceVisitor.Visit(vc.NewTarget(v))
	})

	if sp := visitor.NewestSecurityPatch(); sp.Ok() && sp.NewerThan(current) && !sp.SecurityPatchAltOf(current) {
		// explicit security patch alerts have a more limited audience, so we generate
		// them as their own separate alert.
		log.Warnf("A newer security patch has been detected. current=%s, patch=%s", current.Version(), sp.Version())
		secMsg := fmt.Sprintf("A security patch is available for Teleport. Please upgrade your Cluster to %s or newer.", sp.Version())

		alert, err := types.NewClusterAlert(
			secAlertID,
			secMsg,
			types.WithAlertLabel(types.AlertOnLogin, "yes"),
			// TODO(fspmarshall): permit alert to be shown to those with inventory management
			// permissions once we have RBAC around that. For now, token:write is a decent
			// approximation and will ensure that alerts are shown to the editor role.
			types.WithAlertLabel(types.AlertVerbPermit, fmt.Sprintf("%s:%s", types.KindToken, types.VerbCreate)),
			// hide the normal upgrade alert for users who can see this alert in order to
			// improve its visibility and reduce clutter.
			types.WithAlertLabel(types.AlertSupersedes, releaseAlertID),
			types.WithAlertSeverity(types.AlertSeverity_HIGH),
			types.WithAlertLabel(verInUseLabel, sp.Version()),
			types.WithAlertExpires(a.clock.Now().Add(alertTTL)),
		)
		if err != nil {
			log.Warnf("Failed to build %s alert: %v (this is a bug)", secAlertID, err)
			return
		}

		if err := a.UpsertClusterAlert(ctx, alert); err != nil {
			log.Warnf("Failed to set %s alert: %v", secAlertID, err)
			return
		}
	} else if cleanup {
		err := a.DeleteClusterAlert(ctx, secAlertID)
		if err != nil && !trace.IsNotFound(err) {
			log.Warnf("Failed to delete %s alert: %v", secAlertID, err)
		}
	}
}

func (a *Server) updateAgentMetrics() {
	imp := newInstanceMetricsPeriodic()

	a.inventory.Iter(func(handle inventory.UpstreamHandle) {
		imp.VisitInstance(handle.Hello(), handle.AgentMetadata())
	})

	totalInstancesMetric.Set(float64(imp.TotalInstances()))
	enrolledInUpgradesMetric.Set(float64(imp.TotalEnrolledInUpgrades()))

	// reset the gauges so that any versions that fall off are removed from exported metrics
	registeredAgents.Reset()
	for agent, count := range imp.RegisteredAgentsCount() {
		registeredAgents.With(prometheus.Labels{
			teleport.TagVersion:          agent.version,
			teleport.TagAutomaticUpdates: agent.automaticUpdates,
		}).Set(float64(count))
	}

	// reset the gauges so that any versions that fall off are removed from exported metrics
	registeredAgentsInstallMethod.Reset()
	for installMethod, count := range imp.InstallMethodCounts() {
		registeredAgentsInstallMethod.WithLabelValues(installMethod).Set(float64(count))
	}

	// reset the gauges so that any type+version that fall off are removed from exported metrics
	upgraderCountsMetric.Reset()
	for metadata, count := range imp.UpgraderCounts() {
		upgraderCountsMetric.With(prometheus.Labels{
			teleport.TagUpgrader: metadata.upgraderType,
			teleport.TagVersion:  metadata.version,
		}).Set(float64(count))
	}
}

var (
	// remoteClusterRefreshLimit is the maximum number of backend updates that will be performed
	// during periodic remote cluster connection status refresh.
	remoteClusterRefreshLimit = 50

	// remoteClusterRefreshBuckets is the maximum number of refresh cycles that should guarantee the status update
	// of all remote clusters if their number exceeds remoteClusterRefreshLimit × remoteClusterRefreshBuckets.
	remoteClusterRefreshBuckets = 12
)

// refreshRemoteClusters updates connection status of all remote clusters.
func (a *Server) refreshRemoteClusters(ctx context.Context) {
	remoteClusters, err := a.Services.GetRemoteClusters(ctx)
	if err != nil {
		log.WithError(err).Error("Failed to load remote clusters for status refresh")
		return
	}

	netConfig, err := a.GetClusterNetworkingConfig(ctx)
	if err != nil {
		log.WithError(err).Error("Failed to load networking config for remote cluster status refresh")
		return
	}

	// randomize the order to optimize for multiple auth servers running in parallel
	mathrand.Shuffle(len(remoteClusters), func(i, j int) {
		remoteClusters[i], remoteClusters[j] = remoteClusters[j], remoteClusters[i]
	})

	// we want to limit the number of backend updates performed on each refresh to avoid overwhelming the backend.
	updateLimit := remoteClusterRefreshLimit
	if dynamicLimit := (len(remoteClusters) / remoteClusterRefreshBuckets) + 1; dynamicLimit > updateLimit {
		// if the number of remote clusters is larger than remoteClusterRefreshLimit × remoteClusterRefreshBuckets,
		// bump the limit to make sure all remote clusters will be updated within reasonable time.
		updateLimit = dynamicLimit
	}

	var updateCount int
	for _, remoteCluster := range remoteClusters {
		if updated, err := a.updateRemoteClusterStatus(ctx, netConfig, remoteCluster); err != nil {
			log.WithError(err).Error("Failed to perform remote cluster status refresh")
		} else if updated {
			updateCount++
		}

		if updateCount >= updateLimit {
			break
		}
	}
}

func (a *Server) Close() error {
	a.cancelFunc()

	var errs []error

	if err := a.inventory.Close(); err != nil {
		errs = append(errs, err)
	}

	if a.Services.AuditLogSessionStreamer != nil {
		if err := a.Services.AuditLogSessionStreamer.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if a.bk != nil {
		if err := a.bk.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if a.AccessRequestCache != nil {
		if err := a.AccessRequestCache.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if a.UserNotificationCache != nil {
		if err := a.UserNotificationCache.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if a.GlobalNotificationCache != nil {
		if err := a.GlobalNotificationCache.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	return trace.NewAggregate(errs...)
}

func (a *Server) GetClock() clockwork.Clock {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.clock
}

// SetClock sets clock, used in tests
func (a *Server) SetClock(clock clockwork.Clock) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.clock = clock
}

func (a *Server) SetSCIMService(scim services.SCIM) {
	a.Services.SCIM = scim
}

// SetAccessGraphSecretService sets the server's access graph secret service
func (a *Server) SetAccessGraphSecretService(s services.AccessGraphSecretsGetter) {
	a.Services.AccessGraphSecretsGetter = s
}

// SetDevicesGetter sets the server's device service
func (a *Server) SetDevicesGetter(s services.DevicesGetter) {
	a.Services.DevicesGetter = s
}

// SetAuditLog sets the server's audit log
func (a *Server) SetAuditLog(auditLog events.AuditLogSessionStreamer) {
	a.Services.AuditLogSessionStreamer = auditLog
}

// GetEmitter fetches the current audit log emitter implementation.
func (a *Server) GetEmitter() apievents.Emitter {
	return a.emitter
}

// SetEmitter sets the current audit log emitter. Note that this is only safe to
// use before main server start.
func (a *Server) SetEmitter(emitter apievents.Emitter) {
	a.emitter = emitter
}

// EmitAuditEvent implements [apievents.Emitter] by delegating to its dedicated
// emitter rather than falling back to the implementation from [Services] (using
// the audit log directly, which is almost never what you want).
func (a *Server) EmitAuditEvent(ctx context.Context, e apievents.AuditEvent) error {
	return trace.Wrap(a.emitter.EmitAuditEvent(context.WithoutCancel(ctx), e))
}

// SetUsageReporter sets the server's usage reporter. Note that this is only
// safe to use before server start.
func (a *Server) SetUsageReporter(reporter usagereporter.UsageReporter) {
	a.Services.UsageReporter = reporter
}

// GetClusterID returns the cluster ID.
func (a *Server) GetClusterID(ctx context.Context, opts ...services.MarshalOption) (string, error) {
	clusterName, err := a.GetClusterName(opts...)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return clusterName.GetClusterID(), nil
}

// GetAnonymizationKey returns the anonymization key that identifies this client.
// The anonymization key may be any of the following, in order of precedence:
// - (Teleport Cloud) a key provided by the Teleport Cloud API
// - a key embedded in the license file
// - the cluster's UUID
func (a *Server) GetAnonymizationKey(ctx context.Context, opts ...services.MarshalOption) (string, error) {
	if key := modules.GetModules().Features().CloudAnonymizationKey; len(key) > 0 {
		return string(key), nil
	}

	if a.license != nil && len(a.license.AnonymizationKey) > 0 {
		return string(a.license.AnonymizationKey), nil
	}
	id, err := a.GetClusterID(ctx, opts...)
	return id, trace.Wrap(err)
}

// GetDomainName returns the domain name that identifies this authority server.
// Also known as "cluster name"
func (a *Server) GetDomainName() (string, error) {
	clusterName, err := a.GetClusterName()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return clusterName.GetClusterName(), nil
}

// GetClusterCACert returns the PEM-encoded TLS certs for the local cluster. If
// the cluster has multiple TLS certs, they will all be concatenated.
func (a *Server) GetClusterCACert(ctx context.Context) (*proto.GetClusterCACertResponse, error) {
	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Extract the TLS CA for this cluster.
	hostCA, err := a.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName.GetClusterName(),
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs := services.GetTLSCerts(hostCA)
	if len(certs) < 1 {
		return nil, trace.NotFound("no tls certs found in host CA")
	}
	allCerts := bytes.Join(certs, []byte("\n"))

	return &proto.GetClusterCACertResponse{
		TLSCA: allCerts,
	}, nil
}

// GenerateHostCert uses the private key of the CA to sign the public key of the host
// (along with meta data like host ID, node name, roles, and ttl) to generate a host certificate.
func (a *Server) GenerateHostCert(ctx context.Context, hostPublicKey []byte, hostID, nodeName string, principals []string, clusterName string, role types.SystemRole, ttl time.Duration) ([]byte, error) {
	domainName, err := a.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// get the certificate authority that will be signing the public key of the host
	ca, err := a.Services.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: domainName,
	}, true)
	if err != nil {
		return nil, trace.BadParameter("failed to load host CA for %q: %v", domainName, err)
	}

	caSigner, err := a.keyStore.GetSSHSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// create and sign!
	return a.generateHostCert(ctx, sshca.HostCertificateRequest{
		CASigner:      caSigner,
		PublicHostKey: hostPublicKey,
		HostID:        hostID,
		NodeName:      nodeName,
		TTL:           ttl,
		Identity: sshca.Identity{
			Principals:  principals,
			ClusterName: clusterName,
			SystemRole:  role,
		},
	})
}

func (a *Server) generateHostCert(
	ctx context.Context, req sshca.HostCertificateRequest,
) ([]byte, error) {
	readOnlyAuthPref, err := a.GetReadOnlyAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var locks []types.LockTarget
	switch req.Identity.SystemRole {
	case types.RoleNode:
		// Node role is a special case because it was previously suported as a
		// lock target that only locked the `ssh_service`. If the same Teleport server
		// had multiple roles, Node lock would only lock the `ssh_service` while
		// other roles would be able to generate certificates without a problem.
		// To remove the ambiguity, we now lock the entire Teleport server for
		// all roles using the LockTarget.ServerID field and `Node` field is
		// deprecated.
		// In order to support legacy behavior, we need fill in both `ServerID`
		// and `Node` fields if the role is `Node` so that the previous behavior
		// is preserved.
		// This is a legacy behavior that we need to support for backwards compatibility.
		locks = []types.LockTarget{{ServerID: req.HostID, Node: req.HostID}, {ServerID: HostFQDN(req.HostID, req.Identity.ClusterName), Node: HostFQDN(req.HostID, req.Identity.ClusterName)}}
	default:
		locks = []types.LockTarget{{ServerID: req.HostID}, {ServerID: HostFQDN(req.HostID, req.Identity.ClusterName)}}
	}
	if lockErr := a.checkLockInForce(readOnlyAuthPref.GetLockingMode(),
		locks,
	); lockErr != nil {
		return nil, trace.Wrap(lockErr)
	}

	return a.Authority.GenerateHostCert(req)
}

// GetKeyStore returns the KeyStore used by the auth server
func (a *Server) GetKeyStore() *keystore.Manager {
	return a.keyStore
}

type certRequest struct {
	// sshPublicKey is a public key in SSH authorized_keys format. If set it
	// will be used as the subject public key for the returned SSH certificate.
	sshPublicKey []byte
	// tlsPublicKey is a PEM-encoded public key in PKCS#1 or PKIX ASN.1 DER
	// form. If set it will be used as the subject public key for the returned
	// TLS certificate.
	tlsPublicKey []byte
	// sshPublicKeyAttestationStatement is an attestation statement associated with sshPublicKey.
	sshPublicKeyAttestationStatement *keys.AttestationStatement
	// tlsPublicKeyAttestationStatement is an attestation statement associated with tlsPublicKey.
	tlsPublicKeyAttestationStatement *keys.AttestationStatement

	// user is a user to generate certificate for
	user services.UserState
	// impersonator is a user who generates the certificate,
	// is set when different from the user in the certificate
	impersonator string
	// checker is used to perform RBAC checks.
	checker services.AccessChecker
	// ttl is Duration of the certificate
	ttl time.Duration
	// compatibility is compatibility mode
	compatibility string
	// overrideRoleTTL is used for requests when the requested TTL should not be
	// adjusted based off the role of the user. This is used by tctl to allow
	// creating long lived user certs.
	overrideRoleTTL bool
	// usage is a list of acceptable usages to be encoded in X509 certificate,
	// is used to limit ways the certificate can be used, for example
	// the cert can be only used against kubernetes endpoint, and not auth endpoint,
	// no usage means unrestricted (to keep backwards compatibility)
	usage []string
	// routeToCluster is an optional teleport cluster name to route the
	// certificate requests to, this teleport cluster name will be used to
	// route the requests to in case of kubernetes
	routeToCluster string
	// kubernetesCluster specifies the target kubernetes cluster for TLS
	// identities. This can be empty on older Teleport clients.
	kubernetesCluster string
	// traits hold claim data used to populate a role at runtime.
	traits wrappers.Traits
	// activeRequests tracks privilege escalation requests applied
	// during the construction of the certificate.
	activeRequests []string
	// appSessionID is the session ID of the application session.
	appSessionID string
	// appPublicAddr is the public address of the application.
	appPublicAddr string
	// appClusterName is the name of the cluster this application is in.
	appClusterName string
	// appName is the name of the application to generate cert for.
	appName string
	// appURI is the URI of the app. This is the internal endpoint where the application is running and isn't user-facing.
	appURI string
	// appTargetPort signifies that the cert should grant access to a specific port in a multi-port
	// TCP app, as long as the port is defined in the app spec. Used only for routing, should not be
	// used in other contexts (e.g., access requests).
	appTargetPort int
	// awsRoleARN is the role ARN to generate certificate for.
	awsRoleARN string
	// azureIdentity is the Azure identity to generate certificate for.
	azureIdentity string
	// gcpServiceAccount is the GCP service account to generate certificate for.
	gcpServiceAccount string
	// dbService identifies the name of the database service requests will
	// be routed to.
	dbService string
	// dbProtocol specifies the protocol of the database a certificate will
	// be issued for.
	dbProtocol string
	// dbUser is the optional database user which, if provided, will be used
	// as a default username.
	dbUser string
	// dbName is the optional database name which, if provided, will be used
	// as a default database.
	dbName string
	// dbRoles is the optional list of database roles which, if provided, will
	// be used instead of all database roles granted for the target database.
	dbRoles []string
	// mfaVerified is the UUID of an MFA device when this certRequest was
	// created immediately after an MFA check.
	mfaVerified string
	// previousIdentityExpires is the expiry time of the identity/cert that this
	// identity/cert was derived from. It is used to determine a session's hard
	// deadline in cases where both require_session_mfa and disconnect_expired_cert
	// are enabled. See https://github.com/gravitational/teleport/issues/18544.
	previousIdentityExpires time.Time
	// loginIP is an IP of the client requesting the certificate.
	loginIP string
	// pinIP flags that client's login IP should be pinned in the certificate
	pinIP bool
	// disallowReissue flags that a cert should not be allowed to issue future
	// certificates.
	disallowReissue bool
	// renewable indicates that the certificate can be renewed,
	// having its TTL increased
	renewable bool
	// includeHostCA indicates that host CA certs should be included in the
	// returned certs
	includeHostCA bool
	// generation indicates the number of times this certificate has been
	// renewed.
	generation uint64
	// connectionDiagnosticID contains the ID of the ConnectionDiagnostic.
	// The Node/Agent will append connection traces to this instance.
	connectionDiagnosticID string
	// deviceExtensions holds device-aware user certificate extensions.
	deviceExtensions DeviceExtensions
	// botName is the name of the bot requesting this cert, if any
	botName string
	// botInstanceID is the unique identifier of the bot instance associated
	// with this cert, if any
	botInstanceID string
	// joinAttributes holds attributes derived from attested metadata from the
	// join process, should any exist.
	joinAttributes *workloadidentityv1pb.JoinAttrs
}

// check verifies the cert request is valid.
func (r *certRequest) check() error {
	if r.user == nil {
		return trace.BadParameter("missing parameter user")
	}
	if r.checker == nil {
		return trace.BadParameter("missing parameter checker")
	}

	// When generating certificate for MongoDB access, database username must
	// be encoded into it. This is required to be able to tell which database
	// user to authenticate the connection as.
	if r.dbProtocol == defaults.ProtocolMongoDB {
		if r.dbUser == "" {
			return trace.BadParameter("must provide database user name to generate certificate for database %q", r.dbService)
		}
	}

	if r.sshPublicKey == nil && r.tlsPublicKey == nil {
		return trace.BadParameter("must provide a public key")
	}

	return nil
}

type certRequestOption func(*certRequest)

func certRequestPreviousIdentityExpires(previousIdentityExpires time.Time) certRequestOption {
	return func(r *certRequest) { r.previousIdentityExpires = previousIdentityExpires }
}

func certRequestLoginIP(ip string) certRequestOption {
	return func(r *certRequest) { r.loginIP = ip }
}

func certRequestDeviceExtensions(ext tlsca.DeviceExtensions) certRequestOption {
	return func(r *certRequest) {
		r.deviceExtensions = DeviceExtensions(ext)
	}
}

// GetUserOrLoginState will return the given user or the login state associated with the user.
func (a *Server) GetUserOrLoginState(ctx context.Context, username string) (services.UserState, error) {
	return services.GetUserOrLoginState(ctx, a, username)
}

func (a *Server) GenerateOpenSSHCert(ctx context.Context, req *proto.OpenSSHCertRequest) (*proto.OpenSSHCert, error) {
	if req.User == nil {
		return nil, trace.BadParameter("user is empty")
	}
	if len(req.PublicKey) == 0 {
		return nil, trace.BadParameter("public key is empty")
	}
	if req.TTL == 0 {
		readOnlyAuthPref, err := a.GetReadOnlyAuthPreference(ctx)
		if err != nil {
			return nil, trace.BadParameter("cert request does not specify a TTL and the cluster_auth_preference is not available: %v", err)
		}
		req.TTL = proto.Duration(readOnlyAuthPref.GetDefaultSessionTTL())
	}
	if req.TTL < 0 {
		return nil, trace.BadParameter("TTL must be positive")
	}
	if req.Cluster == "" {
		return nil, trace.BadParameter("cluster is empty")
	}

	// add implicit roles to the set and build a checker
	accessInfo := services.AccessInfoFromUserState(req.User)
	roles := make([]types.Role, len(req.Roles))
	for i := range req.Roles {
		var err error
		roles[i], err = services.ApplyTraits(req.Roles[i], req.User.GetTraits())
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	roleSet := services.NewRoleSet(roles...)

	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker := services.NewAccessCheckerWithRoleSet(accessInfo, clusterName.GetClusterName(), roleSet)

	sessionTTL := time.Duration(req.TTL)

	// OpenSSH certs and their corresponding keys are held strictly by the proxy,
	// so we can attest them as "web_session" to bypass Hardware Key support
	// requirements that are unattainable from the Proxy.
	sshPublicKey, _, _, _, err := ssh.ParseAuthorizedKey(req.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cryptoPublicKey, ok := sshPublicKey.(ssh.CryptoPublicKey)
	if !ok {
		return nil, trace.BadParameter("unsupported SSH public key type %q", sshPublicKey.Type())
	}
	webAttData, err := services.NewWebSessionAttestationData(cryptoPublicKey.CryptoPublicKey())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = a.UpsertKeyAttestationData(ctx, webAttData, sessionTTL); err != nil {
		return nil, trace.Wrap(err)
	}

	certs, err := a.generateOpenSSHCert(ctx, certRequest{
		user:            req.User,
		sshPublicKey:    req.PublicKey,
		compatibility:   constants.CertificateFormatStandard,
		checker:         checker,
		ttl:             sessionTTL,
		traits:          req.User.GetTraits(),
		routeToCluster:  req.Cluster,
		disallowReissue: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.OpenSSHCert{
		Cert: certs.SSH,
	}, nil
}

// GenerateUserTestCertsRequest is a request to generate test certificates.
type GenerateUserTestCertsRequest struct {
	SSHPubKey               []byte
	TLSPubKey               []byte
	Username                string
	TTL                     time.Duration
	Compatibility           string
	RouteToCluster          string
	PinnedIP                string
	MFAVerified             string
	SSHAttestationStatement *keys.AttestationStatement
	TLSAttestationStatement *keys.AttestationStatement
	AppName                 string
	AppSessionID            string
}

// GenerateUserTestCerts is used to generate user certificate, used internally for tests
func (a *Server) GenerateUserTestCerts(req GenerateUserTestCertsRequest) ([]byte, []byte, error) {
	ctx := context.Background()
	userState, err := a.GetUserOrLoginState(ctx, req.Username)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	accessInfo := services.AccessInfoFromUserState(userState)
	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	checker, err := services.NewAccessChecker(accessInfo, clusterName.GetClusterName(), a)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	certs, err := a.generateUserCert(ctx, certRequest{
		user:                             userState,
		ttl:                              req.TTL,
		compatibility:                    req.Compatibility,
		sshPublicKey:                     req.SSHPubKey,
		tlsPublicKey:                     req.TLSPubKey,
		routeToCluster:                   req.RouteToCluster,
		checker:                          checker,
		traits:                           userState.GetTraits(),
		loginIP:                          req.PinnedIP,
		pinIP:                            req.PinnedIP != "",
		mfaVerified:                      req.MFAVerified,
		sshPublicKeyAttestationStatement: req.SSHAttestationStatement,
		tlsPublicKeyAttestationStatement: req.TLSAttestationStatement,
		appName:                          req.AppName,
		appSessionID:                     req.AppSessionID,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return certs.SSH, certs.TLS, nil
}

// AppTestCertRequest combines parameters for generating a test app access cert.
type AppTestCertRequest struct {
	// PublicKey is the public key to sign, in PEM-encoded PKCS#1 or PKIX DER format.
	PublicKey []byte
	// Username is the Teleport user name to sign certificate for.
	Username string
	// TTL is the test certificate validity period.
	TTL time.Duration
	// PublicAddr is the application public address. Used for routing.
	PublicAddr string
	// TargetPort is the port to which connections to multi-port TCP apps should be routed to.
	TargetPort int
	// ClusterName is the name of the cluster application resides in. Used for routing.
	ClusterName string
	// SessionID is the optional session ID to encode. Used for routing.
	SessionID string
	// AWSRoleARN is optional AWS role ARN a user wants to assume to encode.
	AWSRoleARN string
	// AzureIdentity is the optional Azure identity a user wants to assume to encode.
	AzureIdentity string
	// GCPServiceAccount is optional GCP service account a user wants to assume to encode.
	GCPServiceAccount string
	// PinnedIP is optional IP to pin certificate to.
	PinnedIP string
	// LoginTrait is the login to include in the cert
	LoginTrait string
}

// GenerateUserAppTestCert generates an application specific certificate, used
// internally for tests.
func (a *Server) GenerateUserAppTestCert(req AppTestCertRequest) ([]byte, error) {
	ctx := context.Background()
	userState, err := a.GetUserOrLoginState(ctx, req.Username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accessInfo := services.AccessInfoFromUserState(userState)
	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := services.NewAccessChecker(accessInfo, clusterName.GetClusterName(), a)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	login := req.LoginTrait
	if login == "" {
		login = uuid.New().String()
	}

	certs, err := a.generateUserCert(ctx, certRequest{
		user:         userState,
		tlsPublicKey: req.PublicKey,
		checker:      checker,
		ttl:          req.TTL,
		// Set the login to be a random string. Application certificates are never
		// used to log into servers but SSH certificate generation code requires a
		// principal be in the certificate.
		traits: wrappers.Traits(map[string][]string{
			constants.TraitLogins: {login},
		}),
		// Only allow this certificate to be used for applications.
		usage: []string{teleport.UsageAppsOnly},
		// Add in the application routing information.
		appSessionID:      sessionID,
		appPublicAddr:     req.PublicAddr,
		appTargetPort:     req.TargetPort,
		appClusterName:    req.ClusterName,
		awsRoleARN:        req.AWSRoleARN,
		azureIdentity:     req.AzureIdentity,
		gcpServiceAccount: req.GCPServiceAccount,
		pinIP:             req.PinnedIP != "",
		loginIP:           req.PinnedIP,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return certs.TLS, nil
}

// DatabaseTestCertRequest combines parameters for generating test database
// access certificate.
type DatabaseTestCertRequest struct {
	// PublicKey is the public key to sign, in PEM-encoded PKCS#1 or PKIX format.
	PublicKey []byte
	// Cluster is the Teleport cluster name.
	Cluster string
	// Username is the Teleport username.
	Username string
	// RouteToDatabase contains database routing information.
	RouteToDatabase tlsca.RouteToDatabase
	// PinnedIP is an IP new certificate should be pinned to.
	PinnedIP string
}

// GenerateDatabaseTestCert generates a database access certificate for the
// provided parameters. Used only internally in tests.
func (a *Server) GenerateDatabaseTestCert(req DatabaseTestCertRequest) ([]byte, error) {
	ctx := context.Background()
	userState, err := a.GetUserOrLoginState(ctx, req.Username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accessInfo := services.AccessInfoFromUserState(userState)
	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := services.NewAccessChecker(accessInfo, clusterName.GetClusterName(), a)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs, err := a.generateUserCert(ctx, certRequest{
		user:         userState,
		tlsPublicKey: req.PublicKey,
		loginIP:      req.PinnedIP,
		pinIP:        req.PinnedIP != "",
		checker:      checker,
		ttl:          time.Hour,
		traits: map[string][]string{
			constants.TraitLogins: {req.Username},
		},
		routeToCluster: req.Cluster,
		dbService:      req.RouteToDatabase.ServiceName,
		dbProtocol:     req.RouteToDatabase.Protocol,
		dbUser:         req.RouteToDatabase.Username,
		dbName:         req.RouteToDatabase.Database,
		dbRoles:        req.RouteToDatabase.Roles,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return certs.TLS, nil
}

// DeviceExtensions hold device-aware user certificate extensions.
// Device extensions are a part of Device Trust, a feature exclusive to Teleport
// Enterprise.
type DeviceExtensions tlsca.DeviceExtensions

// AugmentUserCertificateOpts aggregates options for extending user
// certificates.
// See [AugmentContextUserCertificates].
type AugmentUserCertificateOpts struct {
	// SSHAuthorizedKey is an SSH certificate, in the authorized key format, to
	// augment with opts.
	// The SSH certificate must be issued for the current authenticated user,
	// and either:
	// - the public key must match their TLS certificate, or
	// - SSHKeySatisfiedChallenge must be true.
	SSHAuthorizedKey []byte
	// SSHKeySatisfiedChallenge will be true if the user has already
	// proven that they own the private key associated with SSHAuthorizedKey by
	// satisfying a signature challenge.
	SSHKeySatisfiedChallenge bool
	// DeviceExtensions are the device-aware extensions to add to the certificates
	// being augmented.
	DeviceExtensions *DeviceExtensions
}

// AugmentContextUserCertificates augments the context user certificates with
// the given extensions. It requires the user's TLS certificate to be present
// in the [ctx], in addition to the [authCtx] itself.
//
// Any additional certificates to augment, such as the SSH certificate, must be
// valid and fully match the certificate used to authenticate (likely the user's
// mTLS cert).
//
// Used by Device Trust to add device extensions to the user certificate.
func (a *Server) AugmentContextUserCertificates(
	ctx context.Context,
	authCtx *authz.Context,
	opts *AugmentUserCertificateOpts,
) (*proto.Certs, error) {
	switch {
	case authCtx == nil:
		return nil, trace.BadParameter("authCtx required")
	case opts == nil:
		return nil, trace.BadParameter("opts required")
	}

	// Fetch user TLS certificate.
	x509Cert, err := authz.UserCertificateFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	identity := authCtx.Identity.GetIdentity()

	return a.augmentUserCertificates(ctx, augmentUserCertificatesOpts{
		checker:          authCtx.Checker,
		x509Cert:         x509Cert,
		x509Identity:     &identity,
		sshAuthorizedKey: opts.SSHAuthorizedKey,
		sshKeyVerified:   opts.SSHKeySatisfiedChallenge,
		deviceExtensions: opts.DeviceExtensions,
	})
}

// AugmentWebSessionCertificatesOpts aggregates arguments for
// [AugmentWebSessionCertificates].
type AugmentWebSessionCertificatesOpts struct {
	// WebSessionID is the identifier for the WebSession.
	WebSessionID string
	// User is the owner of the WebSession.
	User string
	// DeviceExtensions are the device-aware extensions to add to the certificates
	// being augmented.
	DeviceExtensions *DeviceExtensions
}

// AugmentWebSessionCertificates is a variant of
// [AugmentContextUserCertificates] that operates directly in the certificates
// stored in a WebSession.
//
// On success the WebSession is updated with device extension certificates.
func (a *Server) AugmentWebSessionCertificates(ctx context.Context, opts *AugmentWebSessionCertificatesOpts) error {
	switch {
	case opts == nil:
		return trace.BadParameter("opts required")
	case opts.WebSessionID == "":
		return trace.BadParameter("opts.WebSessionID required")
	case opts.User == "":
		return trace.BadParameter("opts.User required")
	}

	// Get and validate session.
	sessions := a.WebSessions()
	session, err := sessions.Get(ctx, types.GetWebSessionRequest{
		User:      opts.User,
		SessionID: opts.WebSessionID,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Coerce session before doing more expensive operations.
	sessionV2, ok := session.(*types.WebSessionV2)
	if !ok {
		return trace.BadParameter("unexpected WebSession type: %T", session)
	}

	// Parse X.509 certificate.
	block, _ := pem.Decode(session.GetTLSCert())
	if block == nil {
		return trace.BadParameter("cannot decode session TLS certificate")
	}
	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return trace.Wrap(err)
	}
	x509Identity, err := tlsca.FromSubject(x509Cert.Subject, x509Cert.NotAfter)
	if err != nil {
		return trace.Wrap(err)
	}

	// Prepare the AccessChecker for the WebSession identity.
	clusterName, err := a.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}
	accessInfo, err := services.AccessInfoFromLocalIdentity(*x509Identity, a)
	if err != nil {
		return trace.Wrap(err)
	}
	checker, err := services.NewAccessChecker(accessInfo, clusterName.GetClusterName(), a)
	if err != nil {
		return trace.Wrap(err)
	}

	// We consider this SSH key to be verified because we take it directly from
	// the web session. The user doesn't need to verify they own it because the
	// don't: we own it.
	const sshKeyVerified = true

	// Augment certificates.
	newCerts, err := a.augmentUserCertificates(ctx, augmentUserCertificatesOpts{
		checker:          checker,
		x509Cert:         x509Cert,
		x509Identity:     x509Identity,
		sshAuthorizedKey: session.GetPub(),
		sshKeyVerified:   sshKeyVerified,
		deviceExtensions: opts.DeviceExtensions,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Update WebSession.
	sessionV2.Spec.Pub = newCerts.SSH
	sessionV2.Spec.TLSCert = newCerts.TLS
	sessionV2.Spec.HasDeviceExtensions = true
	return trace.Wrap(sessions.Upsert(ctx, sessionV2))
}

type augmentUserCertificatesOpts struct {
	checker          services.AccessChecker
	x509Cert         *x509.Certificate
	x509Identity     *tlsca.Identity
	sshAuthorizedKey []byte
	// sshKeyVerified means that either the user has proven that they control
	// the private key associated with sshAuthorizedKey (by signing a
	// challenge), or it comes from a web session where we know that the cluster
	// controls the key.
	sshKeyVerified   bool
	deviceExtensions *DeviceExtensions
}

func (a *Server) augmentUserCertificates(
	ctx context.Context,
	opts augmentUserCertificatesOpts,
) (*proto.Certs, error) {
	// Is at least one extension present?
	// Are the extensions valid?
	dev := opts.deviceExtensions
	switch {
	case dev == nil: // Only extension that currently exists.
		return nil, trace.BadParameter("at least one opts extension must be present")
	case dev.DeviceID == "":
		return nil, trace.BadParameter("opts.DeviceExtensions.DeviceID required")
	case dev.AssetTag == "":
		return nil, trace.BadParameter("opts.DeviceExtensions.AssetTag required")
	case dev.CredentialID == "":
		return nil, trace.BadParameter("opts.DeviceExtensions.CredentialID required")
	}

	x509Cert := opts.x509Cert
	x509Identity := opts.x509Identity

	// Sanity check: x509Cert identity matches x509Identity.
	if x509Cert.Subject.CommonName != x509Identity.Username {
		return nil, trace.BadParameter("identity and x509 user mismatch")
	}

	// Do not reissue if device extensions are already present.
	// Note that the certIdentity extensions could differ from the "current"
	// identity extensions if this was not the cert used to authenticate.
	if x509Identity.DeviceExtensions.DeviceID != "" ||
		x509Identity.DeviceExtensions.AssetTag != "" ||
		x509Identity.DeviceExtensions.CredentialID != "" {
		return nil, trace.BadParameter("device extensions already present")
	}

	// Parse and verify SSH certificate.
	sshAuthorizedKey := opts.sshAuthorizedKey
	var sshCert *ssh.Certificate
	if len(sshAuthorizedKey) > 0 {
		var err error
		sshCert, err = apisshutils.ParseCertificate(sshAuthorizedKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		xPubKey, err := ssh.NewPublicKey(x509Cert.PublicKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// filter and sort TLS and SSH principals for comparison.
		// Order does not matter and "-teleport-*" principals are filtered out.
		filterAndSortPrincipals := func(s []string) []string {
			res := make([]string, 0, len(s))
			for _, principal := range s {
				// Ignore -teleport- internal principals.
				if strings.HasPrefix(principal, "-teleport-") {
					continue
				}
				res = append(res, principal)
			}
			sort.Strings(res)
			return res
		}

		// Verify SSH certificate against identity.
		// The SSH certificate isn't used to establish the connection that
		// eventually reaches this method, so we check it more thoroughly.
		// In the end it still has to be signed by the Teleport CA and share the
		// TLS public key, but we verify most fields to be safe.
		switch {
		case sshCert.CertType != ssh.UserCert:
			return nil, trace.BadParameter("ssh cert type mismatch")
		case sshCert.KeyId != x509Identity.Username:
			return nil, trace.BadParameter("identity and SSH user mismatch")
		case !slices.Equal(filterAndSortPrincipals(sshCert.ValidPrincipals), filterAndSortPrincipals(x509Identity.Principals)):
			return nil, trace.BadParameter("identity and SSH principals mismatch")
		case !opts.sshKeyVerified && !apisshutils.KeysEqual(sshCert.Key, xPubKey):
			return nil, trace.BadParameter("x509 and SSH public key mismatch and SSH challenge unsatisfied")
		// Do not reissue if device extensions are already present.
		case sshCert.Extensions[teleport.CertExtensionDeviceID] != "",
			sshCert.Extensions[teleport.CertExtensionDeviceAssetTag] != "",
			sshCert.Extensions[teleport.CertExtensionDeviceCredentialID] != "":
			return nil, trace.BadParameter("device extensions already present")
		}
	}

	// Fetch TLS CA and SSH signer.
	domainName, err := a.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCA, sshSigner, _, err := a.getSigningCAs(ctx, domainName, types.UserCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Verify TLS certificate against CA.
	now := a.clock.Now()
	roots := x509.NewCertPool()
	roots.AddCert(tlsCA.Cert)
	if _, err := x509Cert.Verify(x509.VerifyOptions{
		Roots:       roots,
		CurrentTime: now,
		KeyUsages: []x509.ExtKeyUsage{
			// Extensions added by tlsca.
			// See https://github.com/gravitational/teleport/blob/master/lib/tlsca/ca.go#L963.
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	// Verify SSH certificate against CA.
	if sshCert != nil {
		// ValidPrincipals are checked against identity above.
		// Pick the first one from the cert here.
		var principal string
		if len(sshCert.ValidPrincipals) > 0 {
			principal = sshCert.ValidPrincipals[0]
		}

		certChecker := &ssh.CertChecker{
			Clock: a.clock.Now,
		}
		if err := certChecker.CheckCert(principal, sshCert); err != nil {
			return nil, trace.Wrap(err)
		}

		// CheckCert verifies the signature but not the CA.
		// Do that here.
		if !apisshutils.KeysEqual(sshCert.SignatureKey, sshSigner.PublicKey()) {
			return nil, trace.BadParameter("ssh certificate signed by unknown authority")
		}
	}

	// Verify locks right before we re-issue any certificates.
	readOnlyAuthPref, err := a.GetReadOnlyAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.verifyLocksForUserCerts(verifyLocksForUserCertsReq{
		checker:              opts.checker,
		defaultMode:          readOnlyAuthPref.GetLockingMode(),
		username:             x509Identity.Username,
		mfaVerified:          x509Identity.MFAVerified,
		activeAccessRequests: x509Identity.ActiveRequests,
		deviceID:             dev.DeviceID, // Check lock against requested device.
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	// Augment TLS certificate.
	newIdentity := x509Identity
	newIdentity.DeviceExtensions.DeviceID = dev.DeviceID
	newIdentity.DeviceExtensions.AssetTag = dev.AssetTag
	newIdentity.DeviceExtensions.CredentialID = dev.CredentialID
	subj, err := newIdentity.Subject()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	notAfter := x509Cert.NotAfter
	newTLSCert, err := tlsCA.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     a.clock,
		PublicKey: x509Cert.PublicKey,
		Subject:   subj,
		// Use the same expiration as the original cert.
		NotAfter: notAfter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Augment SSH certificate.
	var newAuthorizedKey []byte
	if sshCert != nil {
		// Add some leeway to validAfter to avoid time skew errors.
		validAfter := a.clock.Now().UTC().Add(-1 * time.Minute)
		newSSHCert := &ssh.Certificate{
			Key:             sshCert.Key,
			CertType:        ssh.UserCert,
			KeyId:           sshCert.KeyId,
			ValidPrincipals: sshCert.ValidPrincipals,
			ValidAfter:      uint64(validAfter.Unix()),
			// Use the same expiration as the x509 cert.
			ValidBefore: uint64(notAfter.Unix()),
			Permissions: sshCert.Permissions,
		}
		newSSHCert.Extensions[teleport.CertExtensionDeviceID] = dev.DeviceID
		newSSHCert.Extensions[teleport.CertExtensionDeviceAssetTag] = dev.AssetTag
		newSSHCert.Extensions[teleport.CertExtensionDeviceCredentialID] = dev.CredentialID
		if err := newSSHCert.SignCert(rand.Reader, sshSigner); err != nil {
			return nil, trace.Wrap(err)
		}
		newAuthorizedKey = ssh.MarshalAuthorizedKey(newSSHCert)
	}

	// Issue audit event on success, same as [Server.generateCert].
	a.emitCertCreateEvent(ctx, newIdentity, notAfter)

	return &proto.Certs{
		SSH: newAuthorizedKey,
		TLS: newTLSCert,
	}, nil
}

// submitCertificateIssuedEvent submits a certificate issued usage event to the
// usage reporting service.
func (a *Server) submitCertificateIssuedEvent(req *certRequest, attestedKeyPolicy keys.PrivateKeyPolicy) {
	var database, app, kubernetes, desktop bool

	if req.dbService != "" {
		database = true
	}

	if req.appName != "" {
		app = true
	}

	if req.kubernetesCluster != "" {
		kubernetes = true
	}

	// Bot users are regular Teleport users, but have a special internal label.
	bot := req.user.IsBot()

	// Unfortunately the only clue we have about Windows certs is the usage
	// restriction: `RouteToWindowsDesktop` isn't actually passed along to the
	// certRequest.
	for _, usage := range req.usage {
		switch usage {
		case teleport.UsageWindowsDesktopOnly:
			desktop = true
		}
	}

	// For usage reporting, we care about the impersonator rather than the user
	// being impersonated (if any).
	user := req.user.GetName()
	if req.impersonator != "" {
		user = req.impersonator
	}

	a.AnonymizeAndSubmit(&usagereporter.UserCertificateIssuedEvent{
		UserName:         user,
		Ttl:              durationpb.New(req.ttl),
		IsBot:            bot,
		UsageDatabase:    database,
		UsageApp:         app,
		UsageKubernetes:  kubernetes,
		UsageDesktop:     desktop,
		PrivateKeyPolicy: string(attestedKeyPolicy),
	})
}

// generateUserCert generates certificates signed with User CA
func (a *Server) generateUserCert(ctx context.Context, req certRequest) (*proto.Certs, error) {
	return generateCert(ctx, a, req, types.UserCA)
}

// generateOpenSSHCert generates certificates signed with OpenSSH CA
func (a *Server) generateOpenSSHCert(ctx context.Context, req certRequest) (*proto.Certs, error) {
	return generateCert(ctx, a, req, types.OpenSSHCA)
}

func generateCert(ctx context.Context, a *Server, req certRequest, caType types.CertAuthType) (*proto.Certs, error) {
	err := req.check()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(req.checker.GetAllowedResourceIDs()) > 0 && modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil, fmt.Errorf("Resource Access Requests: %w", ErrRequiresEnterprise)
	}

	// Reject the cert request if there is a matching lock in force.
	readOnlyAuthPref, err := a.GetReadOnlyAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.verifyLocksForUserCerts(verifyLocksForUserCertsReq{
		checker:              req.checker,
		defaultMode:          readOnlyAuthPref.GetLockingMode(),
		username:             req.user.GetName(),
		mfaVerified:          req.mfaVerified,
		activeAccessRequests: req.activeRequests,
		deviceID:             req.deviceExtensions.DeviceID,
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	// extract the passed in certificate format. if nothing was passed in, fetch
	// the certificate format from the role.
	certificateFormat, err := utils.CheckCertificateFormatFlag(req.compatibility)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if certificateFormat == teleport.CertificateFormatUnspecified {
		certificateFormat = req.checker.CertificateFormat()
	}

	var sessionTTL time.Duration
	var allowedLogins []string

	if req.ttl == 0 {
		req.ttl = time.Duration(readOnlyAuthPref.GetDefaultSessionTTL())
	}

	// If the role TTL is ignored, do not restrict session TTL and allowed logins.
	// The only caller setting this parameter should be "tctl auth sign".
	// Otherwise, set the session TTL to the smallest of all roles and
	// then only grant access to allowed logins based on that.
	if req.overrideRoleTTL {
		// Take whatever was passed in. Pass in 0 to CheckLoginDuration so all
		// logins are returned for the role set.
		sessionTTL = req.ttl
		allowedLogins, err = req.checker.CheckLoginDuration(0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		// Adjust session TTL to the smaller of two values: the session TTL requested
		// in tsh (possibly using default_session_ttl) or the session TTL for the
		// role.
		sessionTTL = req.checker.AdjustSessionTTL(req.ttl)
		// Return a list of logins that meet the session TTL limit. This means if
		// the requested session TTL is larger than the max session TTL for a login,
		// that login will not be included in the list of allowed logins.
		allowedLogins, err = req.checker.CheckLoginDuration(sessionTTL)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	notAfter := a.clock.Now().UTC().Add(sessionTTL)

	attestedKeyPolicy := keys.PrivateKeyPolicyNone
	requiredKeyPolicy, err := req.checker.PrivateKeyPolicy(readOnlyAuthPref.GetPrivateKeyPolicy())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if requiredKeyPolicy != keys.PrivateKeyPolicyNone {
		var (
			sshAttestedKeyPolicy keys.PrivateKeyPolicy
			tlsAttestedKeyPolicy keys.PrivateKeyPolicy
		)
		if req.sshPublicKey != nil {
			sshCryptoPubKey, err := sshutils.CryptoPublicKey(req.sshPublicKey)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			sshAttestedKeyPolicy, err = a.attestHardwareKey(ctx, &attestHardwareKeyParams{
				requiredKeyPolicy:    requiredKeyPolicy,
				pubKey:               sshCryptoPubKey,
				attestationStatement: req.sshPublicKeyAttestationStatement,
				sessionTTL:           sessionTTL,
				readOnlyAuthPref:     readOnlyAuthPref,
				userName:             req.user.GetName(),
				userTraits:           req.checker.Traits(),
			})
			if err != nil {
				return nil, trace.Wrap(err, "attesting SSH key")
			}
		}
		if req.tlsPublicKey != nil {
			tlsCryptoPubKey, err := keys.ParsePublicKey(req.tlsPublicKey)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			tlsAttestedKeyPolicy, err = a.attestHardwareKey(ctx, &attestHardwareKeyParams{
				requiredKeyPolicy:    requiredKeyPolicy,
				pubKey:               tlsCryptoPubKey,
				attestationStatement: req.tlsPublicKeyAttestationStatement,
				sessionTTL:           sessionTTL,
				readOnlyAuthPref:     readOnlyAuthPref,
				userName:             req.user.GetName(),
				userTraits:           req.checker.Traits(),
			})
			if err != nil {
				return nil, trace.Wrap(err, "attesting TLS key")
			}
		}
		if req.sshPublicKey != nil && req.tlsPublicKey != nil && sshAttestedKeyPolicy != tlsAttestedKeyPolicy {
			return nil, trace.BadParameter("SSH attested key policy %q does not match TLS attested key policy %q, this not supported",
				sshAttestedKeyPolicy, tlsAttestedKeyPolicy)
		}
		attestedKeyPolicy = cmp.Or(sshAttestedKeyPolicy, tlsAttestedKeyPolicy)
	}

	clusterName, err := a.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if req.routeToCluster == "" {
		req.routeToCluster = clusterName
	}
	if req.routeToCluster != clusterName {
		// Authorize access to a remote cluster.
		rc, err := a.GetRemoteCluster(ctx, req.routeToCluster)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := req.checker.CheckAccessToRemoteCluster(rc); err != nil {
			if trace.IsAccessDenied(err) {
				return nil, trace.NotFound("remote cluster %q not found", req.routeToCluster)
			}
			return nil, trace.Wrap(err)
		}
	}

	// Add the special join-only principal used for joining sessions.
	// All users have access to this and join RBAC rules are checked after the connection is established.
	allowedLogins = append(allowedLogins, teleport.SSHSessionJoinPrincipal)

	pinnedIP := ""
	if caType == types.UserCA && (req.checker.PinSourceIP() || req.pinIP) {
		if req.loginIP == "" {
			return nil, trace.BadParameter("IP pinning is enabled for user %q but there is no client IP information", req.user.GetName())
		}

		pinnedIP = req.loginIP
	}

	ca, err := a.GetCertAuthority(ctx, types.CertAuthID{
		Type:       caType,
		DomainName: clusterName,
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// At most one GitHub identity expected.
	var githubUserID, githubUsername string
	if githubIdentities := req.user.GetGithubIdentities(); len(githubIdentities) > 0 {
		githubUserID = githubIdentities[0].UserID
		githubUsername = githubIdentities[0].Username
	}

	var signedSSHCert []byte
	if req.sshPublicKey != nil {
		sshSigner, err := a.keyStore.GetSSHSigner(ctx, ca)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		params := sshca.UserCertificateRequest{
			CASigner:          sshSigner,
			PublicUserKey:     req.sshPublicKey,
			TTL:               sessionTTL,
			CertificateFormat: certificateFormat,
			Identity: sshca.Identity{
				Username:                req.user.GetName(),
				Impersonator:            req.impersonator,
				Principals:              allowedLogins,
				Roles:                   req.checker.RoleNames(),
				PermitPortForwarding:    req.checker.CanPortForward(),
				PermitAgentForwarding:   req.checker.CanForwardAgents(),
				PermitX11Forwarding:     req.checker.PermitX11Forwarding(),
				RouteToCluster:          req.routeToCluster,
				Traits:                  req.traits,
				ActiveRequests:          req.activeRequests,
				MFAVerified:             req.mfaVerified,
				PreviousIdentityExpires: req.previousIdentityExpires,
				LoginIP:                 req.loginIP,
				PinnedIP:                pinnedIP,
				DisallowReissue:         req.disallowReissue,
				Renewable:               req.renewable,
				Generation:              req.generation,
				BotName:                 req.botName,
				BotInstanceID:           req.botInstanceID,
				CertificateExtensions:   req.checker.CertificateExtensions(),
				AllowedResourceIDs:      req.checker.GetAllowedResourceIDs(),
				ConnectionDiagnosticID:  req.connectionDiagnosticID,
				PrivateKeyPolicy:        attestedKeyPolicy,
				DeviceID:                req.deviceExtensions.DeviceID,
				DeviceAssetTag:          req.deviceExtensions.AssetTag,
				DeviceCredentialID:      req.deviceExtensions.CredentialID,
				GitHubUserID:            githubUserID,
				GitHubUsername:          githubUsername,
			},
		}
		signedSSHCert, err = a.GenerateUserCert(params)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	kubeGroups, kubeUsers, err := req.checker.CheckKubeGroupsAndUsers(sessionTTL, req.overrideRoleTTL)
	// NotFound errors are acceptable - this user may have no k8s access
	// granted and that shouldn't prevent us from issuing a TLS cert.
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	// Ensure that the Kubernetes cluster name specified in the request exists
	// when the certificate is intended for a local Kubernetes cluster.
	// If the certificate is targeting a trusted Teleport cluster, it is the
	// responsibility of the cluster to ensure its existence.
	if req.routeToCluster == clusterName && req.kubernetesCluster != "" {
		if err := kubeutils.CheckKubeCluster(a.closeCtx, a, req.kubernetesCluster); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// See which database names and users this user is allowed to use.
	dbNames, dbUsers, err := req.checker.CheckDatabaseNamesAndUsers(sessionTTL, req.overrideRoleTTL)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	// See which AWS role ARNs this user is allowed to assume.
	roleARNs, err := req.checker.CheckAWSRoleARNs(sessionTTL, req.overrideRoleTTL)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	// See which Azure identities this user is allowed to assume.
	azureIdentities, err := req.checker.CheckAzureIdentities(sessionTTL, req.overrideRoleTTL)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	// Enumerate allowed GCP service accounts.
	gcpAccounts, err := req.checker.CheckGCPServiceAccounts(sessionTTL, req.overrideRoleTTL)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	identity := tlsca.Identity{
		Username:          req.user.GetName(),
		Impersonator:      req.impersonator,
		Groups:            req.checker.RoleNames(),
		Principals:        allowedLogins,
		Usage:             req.usage,
		RouteToCluster:    req.routeToCluster,
		KubernetesCluster: req.kubernetesCluster,
		Traits:            req.traits,
		KubernetesGroups:  kubeGroups,
		KubernetesUsers:   kubeUsers,
		RouteToApp: tlsca.RouteToApp{
			SessionID:         req.appSessionID,
			URI:               req.appURI,
			TargetPort:        req.appTargetPort,
			PublicAddr:        req.appPublicAddr,
			ClusterName:       req.appClusterName,
			Name:              req.appName,
			AWSRoleARN:        req.awsRoleARN,
			AzureIdentity:     req.azureIdentity,
			GCPServiceAccount: req.gcpServiceAccount,
		},
		TeleportCluster: clusterName,
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: req.dbService,
			Protocol:    req.dbProtocol,
			Username:    req.dbUser,
			Database:    req.dbName,
			Roles:       req.dbRoles,
		},
		DatabaseNames:           dbNames,
		DatabaseUsers:           dbUsers,
		MFAVerified:             req.mfaVerified,
		PreviousIdentityExpires: req.previousIdentityExpires,
		LoginIP:                 req.loginIP,
		PinnedIP:                pinnedIP,
		AWSRoleARNs:             roleARNs,
		AzureIdentities:         azureIdentities,
		GCPServiceAccounts:      gcpAccounts,
		ActiveRequests:          req.activeRequests,
		DisallowReissue:         req.disallowReissue,
		Renewable:               req.renewable,
		Generation:              req.generation,
		BotName:                 req.botName,
		BotInstanceID:           req.botInstanceID,
		AllowedResourceIDs:      req.checker.GetAllowedResourceIDs(),
		PrivateKeyPolicy:        attestedKeyPolicy,
		ConnectionDiagnosticID:  req.connectionDiagnosticID,
		DeviceExtensions: tlsca.DeviceExtensions{
			DeviceID:     req.deviceExtensions.DeviceID,
			AssetTag:     req.deviceExtensions.AssetTag,
			CredentialID: req.deviceExtensions.CredentialID,
		},
		UserType:       req.user.GetUserType(),
		JoinAttributes: req.joinAttributes,
	}

	var signedTLSCert []byte
	if req.tlsPublicKey != nil {
		tlsCryptoPubKey, err := keys.ParsePublicKey(req.tlsPublicKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tlsCert, tlsSigner, err := a.keyStore.GetTLSCertAndSigner(ctx, ca)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tlsCA, err := tlsca.FromCertAndSigner(tlsCert, tlsSigner)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		subject, err := identity.Subject()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certRequest := tlsca.CertificateRequest{
			Clock:     a.clock,
			PublicKey: tlsCryptoPubKey,
			Subject:   subject,
			NotAfter:  notAfter,
		}
		signedTLSCert, err = tlsCA.GenerateCertificate(certRequest)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	a.emitCertCreateEvent(ctx, &identity, notAfter)

	// create certs struct to return to user
	certs := &proto.Certs{
		SSH: signedSSHCert,
		TLS: signedTLSCert,
	}

	// always include specified CA
	cas := []types.CertAuthority{ca}

	// also include host CA certs if requested
	if req.includeHostCA {
		hostCA, err := a.GetCertAuthority(ctx, types.CertAuthID{
			Type:       types.HostCA,
			DomainName: clusterName,
		}, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cas = append(cas, hostCA)
	}

	for _, ca := range cas {
		certs.TLSCACerts = append(certs.TLSCACerts, services.GetTLSCerts(ca)...)
		certs.SSHCACerts = append(certs.SSHCACerts, services.GetSSHCheckingKeys(ca)...)
	}

	a.submitCertificateIssuedEvent(&req, attestedKeyPolicy)
	userCertificatesGeneratedMetric.WithLabelValues(string(attestedKeyPolicy)).Inc()

	return certs, nil
}

type attestHardwareKeyParams struct {
	requiredKeyPolicy    keys.PrivateKeyPolicy
	pubKey               crypto.PublicKey
	attestationStatement *keys.AttestationStatement
	sessionTTL           time.Duration
	readOnlyAuthPref     readonly.AuthPreference
	userName             string
	userTraits           map[string][]string
}

func (a *Server) attestHardwareKey(ctx context.Context, params *attestHardwareKeyParams) (attestedKeyPolicy keys.PrivateKeyPolicy, err error) {
	// Try to attest the given hardware key using the given attestation statement.
	attestationData, err := modules.GetModules().AttestHardwareKey(ctx, a, params.attestationStatement, params.pubKey, params.sessionTTL)
	if trace.IsNotFound(err) {
		return attestedKeyPolicy, keys.NewPrivateKeyPolicyError(params.requiredKeyPolicy)
	} else if err != nil {
		return attestedKeyPolicy, trace.Wrap(err)
	}

	// verify that the required private key policy for the requested identity
	// is met by the provided attestation statement.
	attestedKeyPolicy = attestationData.PrivateKeyPolicy
	if !params.requiredKeyPolicy.IsSatisfiedBy(attestedKeyPolicy) {
		return attestedKeyPolicy, keys.NewPrivateKeyPolicyError(params.requiredKeyPolicy)
	}

	var validateSerialNumber bool
	hksnv, err := params.readOnlyAuthPref.GetHardwareKeySerialNumberValidation()
	if err == nil {
		validateSerialNumber = hksnv.Enabled
	}

	// Validate the serial number if enabled, unless this is a web session.
	if validateSerialNumber && attestedKeyPolicy != keys.PrivateKeyPolicyWebSession {
		const defaultSerialNumberTraitName = "hardware_key_serial_numbers"
		// Note: currently only yubikeys are supported as hardware keys. If we extend
		// support to more hardware keys, we can add prefixes to serial numbers.
		// Ex: solokey_12345678 or s_12345678.
		// When prefixes are added, we can default to assuming that serial numbers
		// without prefixes are for yubikeys, meaning there will be no backwards
		// compatibility issues.
		serialNumberTraitName := hksnv.SerialNumberTraitName
		if serialNumberTraitName == "" {
			serialNumberTraitName = defaultSerialNumberTraitName
		}

		// Check that the attested hardware key serial number matches
		// a serial number in the user's traits, if any are set.
		registeredSerialNumbers, ok := params.userTraits[serialNumberTraitName]
		if !ok || len(registeredSerialNumbers) == 0 {
			log.Debugf("user %q tried to sign in with hardware key support, but has no known hardware keys. A user's known hardware key serial numbers should be set \"in user.traits.%v\"", params.userName, serialNumberTraitName)
			return attestedKeyPolicy, trace.BadParameter("cannot generate certs for user with no known hardware keys")
		}

		attestedSerialNumber := strconv.Itoa(int(attestationData.SerialNumber))
		// serial number traits can be a comma separated list, or a list of comma separated lists.
		// e.g. [["12345678,87654321"], ["13572468"]].
		if !slices.ContainsFunc(registeredSerialNumbers, func(s string) bool {
			return slices.Contains(strings.Split(s, ","), attestedSerialNumber)
		}) {
			log.Debugf("user %q tried to sign in with hardware key support with an unknown hardware key and was denied: YubiKey serial number %q", params.userName, attestedSerialNumber)
			return attestedKeyPolicy, trace.BadParameter("cannot generate certs for user with unknown hardware key: YubiKey serial number %q", attestedSerialNumber)
		}
	}
	return attestedKeyPolicy, nil
}

type verifyLocksForUserCertsReq struct {
	checker services.AccessChecker

	// defaultMode is the default locking mode, as recorded in the cluster
	// Auth Preferences.
	defaultMode constants.LockingMode
	// username is the Teleport username.
	// Eg: tlsca.Identity.Username.
	username string
	// mfaVerified is the UUID of the MFA device used to authenticate the user.
	// Eg: tlsca.Identity.MFAVerified.
	mfaVerified string
	// activeAccessRequests are the UUIDs of active access requests for the user.
	// Eg: tlsca.Identity.ActiveRequests.
	activeAccessRequests []string
	// deviceID is the trusted device ID.
	// Eg: tlsca.Identity.DeviceExtensions.DeviceID
	deviceID string
}

// verifyLocksForUserCerts verifies if any locks are in place before issuing new
// user certificates.
func (a *Server) verifyLocksForUserCerts(req verifyLocksForUserCertsReq) error {
	checker := req.checker
	lockingMode := checker.LockingMode(req.defaultMode)

	lockTargets := []types.LockTarget{
		{User: req.username},
		{MFADevice: req.mfaVerified},
		{Device: req.deviceID},
	}
	lockTargets = append(lockTargets,
		services.RolesToLockTargets(checker.RoleNames())...,
	)
	lockTargets = append(lockTargets,
		services.AccessRequestsToLockTargets(req.activeAccessRequests)...,
	)

	return trace.Wrap(a.checkLockInForce(lockingMode, lockTargets))
}

// getSigningCAs returns the necessary resources to issue/sign new certificates.
func (a *Server) getSigningCAs(ctx context.Context, domainName string, caType types.CertAuthType) (*tlsca.CertAuthority, ssh.Signer, types.CertAuthority, error) {
	const loadKeys = true
	ca, err := a.GetCertAuthority(ctx, types.CertAuthID{
		Type:       caType,
		DomainName: domainName,
	}, loadKeys)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	tlsCert, tlsSigner, err := a.keyStore.GetTLSCertAndSigner(ctx, ca)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	tlsCA, err := tlsca.FromCertAndSigner(tlsCert, tlsSigner)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	sshSigner, err := a.keyStore.GetSSHSigner(ctx, ca)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	return tlsCA, sshSigner, ca, nil
}

func (a *Server) emitCertCreateEvent(ctx context.Context, identity *tlsca.Identity, notAfter time.Time) {
	eventIdentity := identity.GetEventIdentity()
	eventIdentity.Expires = notAfter
	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.CertificateCreate{
		Metadata: apievents.Metadata{
			Type: events.CertificateCreateEvent,
			Code: events.CertificateCreateCode,
		},
		CertificateType: events.CertificateTypeUser,
		Identity:        &eventIdentity,
		ClientMetadata: apievents.ClientMetadata{
			// TODO(greedy52) currently only user-agent from GRPC clients are
			// fetched. Need to propagate user-agent from HTTP calls.
			UserAgent: trimUserAgent(metadata.UserAgentFromContext(ctx)),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit certificate create event.")
	}
}

// WithUserLock executes function authenticateFn that performs user authentication
// if authenticateFn returns non nil error, the login attempt will be logged in as failed.
// The only exception to this rule is ConnectionProblemError, in case if it occurs
// access will be denied, but login attempt will not be recorded
// this is done to avoid potential user lockouts due to backend failures
// In case if user exceeds defaults.MaxLoginAttempts
// the user account will be locked for defaults.AccountLockInterval
func (a *Server) WithUserLock(ctx context.Context, username string, authenticateFn func() error) error {
	user, err := a.Services.GetUser(ctx, username, false)
	if err != nil {
		if trace.IsNotFound(err) {
			// If user is not found, still call authenticateFn. It should
			// always return an error. This prevents username oracles and
			// timing attacks.
			return authenticateFn()
		}
		return trace.Wrap(err)
	}
	status := user.GetStatus()
	if status.IsLocked {
		if status.LockExpires.After(a.clock.Now().UTC()) {
			log.Debugf("%v exceeds %v failed login attempts, locked until %v",
				user.GetName(), defaults.MaxLoginAttempts, apiutils.HumanTimeFormat(status.LockExpires))

			err := trace.AccessDenied(MaxFailedAttemptsErrMsg)
			return trace.WithField(err, ErrFieldKeyUserMaxedAttempts, true)
		}
	}
	fnErr := authenticateFn()
	if fnErr == nil {
		// upon successful login, reset the failed attempt counter
		err = a.DeleteUserLoginAttempts(username)
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}

		return nil
	}
	// do not lock user in case if DB is flaky or down
	if trace.IsConnectionProblem(err) {
		return trace.Wrap(fnErr)
	}
	// log failed attempt and possibly lock user
	attempt := services.LoginAttempt{Time: a.clock.Now().UTC(), Success: false}
	err = a.AddUserLoginAttempt(username, attempt, defaults.AttemptTTL)
	if err != nil {
		log.Error(trace.DebugReport(err))
		return trace.Wrap(fnErr)
	}
	loginAttempts, err := a.GetUserLoginAttempts(username)
	if err != nil {
		log.Error(trace.DebugReport(err))
		return trace.Wrap(fnErr)
	}
	if !services.LastFailed(defaults.MaxLoginAttempts, loginAttempts) {
		log.Debugf("%v user has less than %v failed login attempts", username, defaults.MaxLoginAttempts)
		return trace.Wrap(fnErr)
	}
	lockUntil := a.clock.Now().UTC().Add(defaults.AccountLockInterval)
	log.Debug(fmt.Sprintf("%v exceeds %v failed login attempts, locked until %v",
		username, defaults.MaxLoginAttempts, apiutils.HumanTimeFormat(lockUntil)))
	user.SetLocked(lockUntil, "user has exceeded maximum failed login attempts")
	_, err = a.UpsertUser(ctx, user)
	if err != nil {
		log.Error(trace.DebugReport(err))
		return trace.Wrap(fnErr)
	}

	retErr := trace.AccessDenied(MaxFailedAttemptsErrMsg)
	return trace.WithField(retErr, ErrFieldKeyUserMaxedAttempts, true)
}

// CreateAuthenticateChallenge implements AuthService.CreateAuthenticateChallenge.
func (a *Server) CreateAuthenticateChallenge(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
	var username string

	challengeExtensions := &mfav1.ChallengeExtensions{}
	if req.ChallengeExtensions != nil {
		challengeExtensions = req.ChallengeExtensions
	}

	validateAndSetScope := func(challengeExtensions *mfav1.ChallengeExtensions, expectedScope mfav1.ChallengeScope) error {
		if challengeExtensions.Scope == mfav1.ChallengeScope_CHALLENGE_SCOPE_UNSPECIFIED {
			challengeExtensions.Scope = expectedScope
		} else if challengeExtensions.Scope != expectedScope {
			// scope doesn't need to be specified when the challenge request type is
			// tied to a specific scope, but we validate it anyways as a sanity check.
			return trace.BadParameter("invalid scope %q, expected %q", challengeExtensions.Scope, expectedScope)
		}

		return nil
	}

	switch req.GetRequest().(type) {
	case *proto.CreateAuthenticateChallengeRequest_UserCredentials:
		username = req.GetUserCredentials().GetUsername()

		if err := a.WithUserLock(ctx, username, func() error {
			return a.checkPasswordWOToken(ctx, username, req.GetUserCredentials().GetPassword())
		}); err != nil {
			// This is only ever used as a means to acquire a login challenge, so
			// let's issue an authentication failure event.
			if err := a.emitAuthAuditEvent(ctx, authAuditProps{
				username: username,
				authErr:  err,
			}); err != nil {
				log.WithError(err).Warn("Failed to emit login event")
				// err swallowed on purpose.
			}
			return nil, trace.Wrap(err)
		}

		if err := validateAndSetScope(challengeExtensions, mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN); err != nil {
			return nil, trace.Wrap(ErrDone)
		}

	case *proto.CreateAuthenticateChallengeRequest_RecoveryStartTokenID:
		token, err := a.GetUserToken(ctx, req.GetRecoveryStartTokenID())
		if err != nil {
			log.Error(trace.DebugReport(err))
			return nil, trace.AccessDenied("invalid token")
		}

		if err := a.verifyUserToken(token, authclient.UserTokenTypeRecoveryStart); err != nil {
			return nil, trace.Wrap(err)
		}

		username = token.GetUser()

		if err := validateAndSetScope(challengeExtensions, mfav1.ChallengeScope_CHALLENGE_SCOPE_ACCOUNT_RECOVERY); err != nil {
			return nil, trace.Wrap(ErrDone)
		}

	case *proto.CreateAuthenticateChallengeRequest_Passwordless:
		if err := validateAndSetScope(challengeExtensions, mfav1.ChallengeScope_CHALLENGE_SCOPE_PASSWORDLESS_LOGIN); err != nil {
			return nil, trace.Wrap(ErrDone)
		}
	default: // unset or CreateAuthenticateChallengeRequest_ContextUser.

		// Require that a scope was provided.
		if challengeExtensions.Scope == mfav1.ChallengeScope_CHALLENGE_SCOPE_UNSPECIFIED {
			return nil, trace.BadParameter("scope not present in request")
		}

		var err error
		username, err = authz.GetClientUsername(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	challenges, err := a.mfaAuthChallenge(ctx, username, req.SSOClientRedirectURL, challengeExtensions)
	if err != nil {
		// Do not obfuscate config-related errors.
		if errors.Is(err, types.ErrPasswordlessRequiresWebauthn) || errors.Is(err, types.ErrPasswordlessDisabledBySettings) {
			return nil, trace.Wrap(err)
		}

		log.Error(trace.DebugReport(err))
		return nil, trace.AccessDenied("unable to create MFA challenges")
	}

	return challenges, nil
}

// CreateRegisterChallenge implements AuthService.CreateRegisterChallenge.
func (a *Server) CreateRegisterChallenge(ctx context.Context, req *proto.CreateRegisterChallengeRequest) (*proto.MFARegisterChallenge, error) {
	var token types.UserToken
	var username string
	switch {
	case req.TokenID != "": // Web UI or account recovery flows.
		var err error
		token, err = a.GetUserToken(ctx, req.GetTokenID())
		if err != nil {
			log.Error(trace.DebugReport(err))
			return nil, trace.AccessDenied("invalid token")
		}

		allowedTokenTypes := []string{
			authclient.UserTokenTypePrivilege,
			authclient.UserTokenTypePrivilegeException,
			authclient.UserTokenTypeResetPassword,
			authclient.UserTokenTypeResetPasswordInvite,
			authclient.UserTokenTypeRecoveryApproved,
		}
		if err := a.verifyUserToken(token, allowedTokenTypes...); err != nil {
			return nil, trace.AccessDenied("invalid token")
		}
		username = token.GetUser()

	default: // Authenticated user without token, tsh.
		var err error
		username, err = authz.GetClientUsername(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		requiredExt := &mfav1.ChallengeExtensions{Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES}
		if _, err := a.validateMFAAuthResponseForRegister(ctx, req.ExistingMFAResponse, username, requiredExt); err != nil {
			return nil, trace.Wrap(err)
		}

		// Create a special token for OTP registrations. The token doubles as
		// temporary storage for the OTP secret, like in the branch above.
		// This is OK because the user just did an MFA check.
		if req.GetDeviceType() != proto.DeviceType_DEVICE_TYPE_TOTP {
			break // break from switch
		}

		token, err = a.createTOTPPrivilegeToken(ctx, username)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	regChal, err := a.createRegisterChallenge(ctx, &newRegisterChallengeRequest{
		username:    username,
		token:       token,
		deviceType:  req.GetDeviceType(),
		deviceUsage: req.GetDeviceUsage(),
	})
	return regChal, trace.Wrap(err)
}

func (a *Server) createTOTPPrivilegeToken(ctx context.Context, username string) (types.UserToken, error) {
	tokenReq := authclient.CreateUserTokenRequest{
		Name: username,
		Type: userTokenTypePrivilegeOTP,
	}
	if err := tokenReq.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := a.newUserToken(tokenReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err = a.CreateUserToken(ctx, token)
	return token, trace.Wrap(err)
}

type newRegisterChallengeRequest struct {
	username    string
	deviceType  proto.DeviceType
	deviceUsage proto.DeviceUsage

	// token is a user token resource.
	// It is used as following:
	//  - TOTP:
	//    - create a UserTokenSecrets resource
	//    - store by token's ID using Server's IdentityService.
	//  - MFA:
	//    - store challenge by the token's ID
	//    - store by token's ID using Server's IdentityService.
	// This field can be empty to use storage overrides.
	token types.UserToken

	// webIdentityOverride is an optional RegistrationIdentity override to be used
	// to store webauthn challenge. A common override is decorating the regular
	// Identity with an in-memory SessionData storage.
	// Defaults to the Server's IdentityService.
	webIdentityOverride wanlib.RegistrationIdentity
}

func (a *Server) createRegisterChallenge(ctx context.Context, req *newRegisterChallengeRequest) (*proto.MFARegisterChallenge, error) {
	switch req.deviceType {
	case proto.DeviceType_DEVICE_TYPE_TOTP:
		if req.token == nil {
			return nil, trace.BadParameter("all TOTP registrations require a privilege token")
		}

		otpKey, otpOpts, err := a.newTOTPKey(req.username)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		token := req.token
		secrets, err := a.createTOTPUserTokenSecrets(ctx, token, otpKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &proto.MFARegisterChallenge{
			Request: &proto.MFARegisterChallenge_TOTP{
				TOTP: &proto.TOTPRegisterChallenge{
					Secret:        otpKey.Secret(),
					Issuer:        otpKey.Issuer(),
					PeriodSeconds: uint32(otpOpts.Period),
					Algorithm:     otpOpts.Algorithm.String(),
					Digits:        uint32(otpOpts.Digits.Length()),
					Account:       otpKey.AccountName(),
					QRCode:        secrets.GetQRCode(),
					ID:            token.GetName(),
				},
			},
		}, nil

	case proto.DeviceType_DEVICE_TYPE_WEBAUTHN:
		cap, err := a.GetAuthPreference(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		webConfig, err := cap.GetWebauthn()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		identity := req.webIdentityOverride
		if identity == nil {
			identity = a.Services
		}

		webRegistration := &wanlib.RegistrationFlow{
			Webauthn: webConfig,
			Identity: identity,
		}

		passwordless := req.deviceUsage == proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS
		credentialCreation, err := webRegistration.Begin(ctx, req.username, passwordless)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &proto.MFARegisterChallenge{Request: &proto.MFARegisterChallenge_Webauthn{
			Webauthn: wantypes.CredentialCreationToProto(credentialCreation),
		}}, nil

	default:
		return nil, trace.BadParameter("MFA device type %q unsupported", req.deviceType.String())
	}
}

// GetMFADevices returns all mfa devices for the user defined in the token or the user defined in context.
func (a *Server) GetMFADevices(ctx context.Context, req *proto.GetMFADevicesRequest) (*proto.GetMFADevicesResponse, error) {
	var username string

	if req.GetTokenID() != "" {
		token, err := a.GetUserToken(ctx, req.GetTokenID())
		if err != nil {
			log.Error(trace.DebugReport(err))
			return nil, trace.AccessDenied("invalid token")
		}

		if err := a.verifyUserToken(token, authclient.UserTokenTypeRecoveryApproved); err != nil {
			return nil, trace.Wrap(err)
		}

		username = token.GetUser()
	}

	if username == "" {
		var err error
		username, err = authz.GetClientUsername(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	devs, err := a.Services.GetMFADevices(ctx, username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.GetMFADevicesResponse{
		Devices: devs,
	}, nil
}

// DeleteMFADeviceSync implements AuthService.DeleteMFADeviceSync.
func (a *Server) DeleteMFADeviceSync(ctx context.Context, req *proto.DeleteMFADeviceSyncRequest) error {
	var user string
	switch {
	case req.TokenID != "":
		token, err := a.GetUserToken(ctx, req.TokenID)
		if err != nil {
			log.Error(trace.DebugReport(err))
			return trace.AccessDenied("invalid token")
		}
		user = token.GetUser()

		if err := a.verifyUserToken(token, authclient.UserTokenTypeRecoveryApproved, authclient.UserTokenTypePrivilege); err != nil {
			return trace.Wrap(err)
		}

	case req.ExistingMFAResponse != nil:
		var err error
		user, err = authz.GetClientUsername(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		requiredExt := &mfav1.ChallengeExtensions{Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES}
		if _, err := a.ValidateMFAAuthResponse(ctx, req.ExistingMFAResponse, user, requiredExt); err != nil {
			return trace.Wrap(err)
		}

	default:
		return trace.BadParameter(
			"deleting an MFA device requires either a privilege token or a solved authentication challenge")
	}

	_, err := a.deleteMFADeviceSafely(ctx, user, req.DeviceName)
	return trace.Wrap(err)
}

// deleteMFADeviceSafely deletes the user's mfa device while preventing users
// from locking themselves out of their account.
//
// Deletes are not allowed in the following situations:
//   - Last MFA device when the cluster requires MFA
//   - Last resident key credential in a passwordless-capable cluster (avoids
//     passwordless users from locking themselves out).
func (a *Server) deleteMFADeviceSafely(ctx context.Context, user, deviceName string) (*types.MFADevice, error) {
	mfaDevices, err := a.Services.GetMFADevices(ctx, user, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	readOnlyAuthPref, err := a.GetReadOnlyAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	isPasskey := func(d *types.MFADevice) bool {
		return d.GetWebauthn() != nil && d.GetWebauthn().ResidentKey
	}

	var deviceToDelete *types.MFADevice
	remainingDevices := make(map[types.SecondFactorType]int)
	var remainingPasskeys int

	// Find the device to delete and count devices.
	for _, d := range mfaDevices {
		// Match device by name or ID.
		if d.GetName() == deviceName || d.Id == deviceName {
			deviceToDelete = d
			switch d.Device.(type) {
			case *types.MFADevice_Totp, *types.MFADevice_U2F, *types.MFADevice_Webauthn:
			case *types.MFADevice_Sso:
				return nil, trace.BadParameter("cannot delete ephemeral SSO MFA device")
			default:
				return nil, trace.NotImplemented("cannot delete device of type %T", d.Device)
			}
			continue
		}

		switch d.Device.(type) {
		case *types.MFADevice_Totp:
			remainingDevices[types.SecondFactorType_SECOND_FACTOR_TYPE_OTP]++
		case *types.MFADevice_U2F, *types.MFADevice_Webauthn:
			remainingDevices[types.SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN]++
		case *types.MFADevice_Sso:
			remainingDevices[types.SecondFactorType_SECOND_FACTOR_TYPE_SSO]++
		default:
			log.Warnf("Ignoring unknown device with type %T in deletion.", d.Device)
			continue
		}

		if isPasskey(d) {
			remainingPasskeys++
		}
	}
	if deviceToDelete == nil {
		return nil, trace.NotFound("MFA device %q does not exist", deviceName)
	}

	var remainingAllowedDevices int
	for _, sf := range readOnlyAuthPref.GetSecondFactors() {
		remainingAllowedDevices += remainingDevices[sf]
	}

	// Prevent users from deleting their last allowed device for clusters that require second factors.
	if readOnlyAuthPref.IsSecondFactorEnforced() && remainingAllowedDevices == 0 {
		return nil, trace.BadParameter("cannot delete the last MFA device for this user; add a replacement device first to avoid getting locked out")
	}

	// Check whether the device to delete is the last passwordless device,
	// and whether deleting it would lockout the user from login.
	//
	// Note: the user may already be locked out from login if a password
	// is not set and passwordless is disabled. Prevent them from deleting
	// their last passkey to prevent them from being locked out further,
	// in the case of passwordless being re-enabled.
	if isPasskey(deviceToDelete) && remainingPasskeys == 0 {
		u, err := a.Services.GetUser(ctx, user, false /* withSecrets */)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if u.GetUserType() != types.UserTypeSSO && u.GetPasswordState() != types.PasswordState_PASSWORD_STATE_SET {
			return nil, trace.BadParameter("cannot delete last passwordless credential for user")
		}
	}

	if err := a.DeleteMFADevice(ctx, user, deviceToDelete.Id); err != nil {
		return nil, trace.Wrap(err)
	}

	// Emit deleted event.
	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.MFADeviceDelete{
		Metadata: apievents.Metadata{
			Type:        events.MFADeviceDeleteEvent,
			Code:        events.MFADeviceDeleteEventCode,
			ClusterName: clusterName.GetClusterName(),
		},
		UserMetadata:       authz.ClientUserMetadataWithUser(ctx, user),
		MFADeviceMetadata:  mfaDeviceEventMetadata(deviceToDelete),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	return deviceToDelete, nil
}

// AddMFADeviceSync implements AuthService.AddMFADeviceSync.
func (a *Server) AddMFADeviceSync(ctx context.Context, req *proto.AddMFADeviceSyncRequest) (*proto.AddMFADeviceSyncResponse, error) {
	// Use either the explicitly provided token or the TOTP token created by
	// CreateRegisterChallenge.
	token := req.GetTokenID()
	if token == "" {
		token = req.GetNewMFAResponse().GetTOTP().GetID()
	}

	var username string
	switch {
	case token != "":
		privilegeToken, err := a.GetUserToken(ctx, token)
		if err != nil {
			log.Error(trace.DebugReport(err))
			return nil, trace.AccessDenied("invalid token")
		}

		if err := a.verifyUserToken(
			privilegeToken,
			authclient.UserTokenTypePrivilege,
			authclient.UserTokenTypePrivilegeException,
			userTokenTypePrivilegeOTP,
		); err != nil {
			return nil, trace.Wrap(err)
		}
		username = privilegeToken.GetUser()

	default: // ContextUser
		var err error
		username, err = authz.GetClientUsername(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	dev, err := a.verifyMFARespAndAddDevice(ctx, &newMFADeviceFields{
		username:      username,
		newDeviceName: req.GetNewDeviceName(),
		tokenID:       token,
		deviceResp:    req.GetNewMFAResponse(),
		deviceUsage:   req.DeviceUsage,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.AddMFADeviceSyncResponse{Device: dev}, nil
}

type newMFADeviceFields struct {
	username      string
	newDeviceName string
	// tokenID is the ID of a reset/invite/recovery/privilege token.
	// It is generally used to recover the TOTP secret stored in the token.
	tokenID string

	// webIdentityOverride is an optional RegistrationIdentity override to be used
	// for device registration. A common override is decorating the regular
	// Identity with an in-memory SessionData storage.
	// Defaults to the Server's IdentityService.
	webIdentityOverride wanlib.RegistrationIdentity
	// deviceResp is the register response from the new device.
	deviceResp *proto.MFARegisterResponse
	// deviceUsage describes the intended usage of the new device.
	deviceUsage proto.DeviceUsage
}

// verifyMFARespAndAddDevice validates MFA register response and on success adds the new MFA device.
func (a *Server) verifyMFARespAndAddDevice(ctx context.Context, req *newMFADeviceFields) (*types.MFADevice, error) {
	if len(req.newDeviceName) > mfaDeviceNameMaxLen {
		return nil, trace.BadParameter("device name must be %v characters or less", mfaDeviceNameMaxLen)
	}

	cap, err := a.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cap.IsSecondFactorEnabled() {
		return nil, trace.BadParameter("second factor disabled by cluster configuration")
	}

	var dev *types.MFADevice
	switch req.deviceResp.GetResponse().(type) {
	case *proto.MFARegisterResponse_TOTP:
		dev, err = a.registerTOTPDevice(ctx, req.deviceResp, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case *proto.MFARegisterResponse_Webauthn:
		dev, err = a.registerWebauthnDevice(ctx, req.deviceResp, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		return nil, trace.BadParameter("MFARegisterResponse is an unknown response type %T", req.deviceResp.Response)
	}

	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.MFADeviceAdd{
		Metadata: apievents.Metadata{
			Type:        events.MFADeviceAddEvent,
			Code:        events.MFADeviceAddEventCode,
			ClusterName: clusterName.GetClusterName(),
		},
		UserMetadata:       authz.ClientUserMetadataWithUser(ctx, req.username),
		MFADeviceMetadata:  mfaDeviceEventMetadata(dev),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		log.WithError(err).Warn("Failed to emit add mfa device event.")
	}

	return dev, nil
}

func (a *Server) registerTOTPDevice(ctx context.Context, regResp *proto.MFARegisterResponse, req *newMFADeviceFields) (*types.MFADevice, error) {
	cap, err := a.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cap.IsSecondFactorTOTPAllowed() {
		return nil, trace.BadParameter("second factor TOTP not allowed by cluster")
	}

	if req.tokenID == "" {
		return nil, trace.BadParameter("missing TOTP secret")
	}

	secrets, err := a.GetUserTokenSecrets(ctx, req.tokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	secret := secrets.GetOTPKey()

	dev, err := services.NewTOTPDevice(req.newDeviceName, secret, a.clock.Now())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.checkTOTP(ctx, req.username, regResp.GetTOTP().GetCode(), dev); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.UpsertMFADevice(ctx, req.username, dev); err != nil {
		return nil, trace.Wrap(err)
	}
	return dev, nil
}

func (a *Server) registerWebauthnDevice(ctx context.Context, regResp *proto.MFARegisterResponse, req *newMFADeviceFields) (*types.MFADevice, error) {
	cap, err := a.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cap.IsSecondFactorWebauthnAllowed() {
		return nil, trace.BadParameter("second factor webauthn not allowed by cluster")
	}

	webConfig, err := cap.GetWebauthn()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity := req.webIdentityOverride // Override Identity, if supplied.
	if identity == nil {
		identity = a.Services
	}
	webRegistration := &wanlib.RegistrationFlow{
		Webauthn: webConfig,
		Identity: identity,
	}
	// Finish upserts the device on success.
	dev, err := webRegistration.Finish(ctx, wanlib.RegisterResponse{
		User:             req.username,
		DeviceName:       req.newDeviceName,
		CreationResponse: wantypes.CredentialCreationResponseFromProto(regResp.GetWebauthn()),
		Passwordless:     req.deviceUsage == proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS,
	})
	return dev, trace.Wrap(err)
}

// GetWebSession returns existing web session described by req. Explicitly
// delegating to Services as it's directly implemented by Cache as well.
func (a *Server) GetWebSession(ctx context.Context, req types.GetWebSessionRequest) (types.WebSession, error) {
	return a.Services.GetWebSession(ctx, req)
}

// GetWebToken returns existing web token described by req. Explicitly
// delegating to Services as it's directly implemented by Cache as well.
func (a *Server) GetWebToken(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error) {
	return a.Services.GetWebToken(ctx, req)
}

// ExtendWebSession creates a new web session for a user based on a valid previous (current) session.
//
// If there is an approved access request, additional roles are appended to the roles that were
// extracted from identity. The new session expiration time will not exceed the expiration time
// of the previous session.
//
// If there is a switchback request, the roles will switchback to user's default roles and
// the expiration time is derived from users recently logged in time.
func (a *Server) ExtendWebSession(ctx context.Context, req authclient.WebSessionReq, identity tlsca.Identity) (types.WebSession, error) {
	prevSession, err := a.GetWebSession(ctx, types.GetWebSessionRequest{
		User:      req.User,
		SessionID: req.PrevSessionID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// consider absolute expiry time that may be set for this session
	// by some external identity service, so we can not renew this session
	// anymore without extra logic for renewal with external OIDC provider
	expiresAt := prevSession.GetExpiryTime()
	if !expiresAt.IsZero() && expiresAt.Before(a.clock.Now().UTC()) {
		return nil, trace.NotFound("web session has expired")
	}

	accessInfo, err := services.AccessInfoFromLocalIdentity(identity, a)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roles := accessInfo.Roles
	traits := accessInfo.Traits
	allowedResourceIDs := accessInfo.AllowedResourceIDs
	accessRequests := identity.ActiveRequests

	if req.ReloadUser {
		// We don't call from the cache layer because we want to
		// retrieve the recently updated user. Otherwise, the cache
		// returns stale data.
		user, err := a.Identity.GetUser(ctx, req.User, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Make sure to refresh the user login state.
		userState, err := a.ulsGenerator.Refresh(ctx, user, a.UserLoginStates)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Updating traits is needed for guided SSH flow in Discover.
		traits = userState.GetTraits()
		// Updating roles is needed for guided Connect My Computer flow in Discover.
		roles = userState.GetRoles()

	} else if req.AccessRequestID != "" {
		accessRequest, err := a.getValidatedAccessRequest(ctx, identity, req.User, req.AccessRequestID)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		roles = append(roles, accessRequest.GetRoles()...)
		roles = apiutils.Deduplicate(roles)
		accessRequests = apiutils.Deduplicate(append(accessRequests, req.AccessRequestID))

		if len(accessRequest.GetRequestedResourceIDs()) > 0 {
			// There's not a consistent way to merge multiple resource access
			// requests, a user may be able to request access to different resources
			// with different roles which should not overlap.
			if len(allowedResourceIDs) > 0 {
				return nil, trace.BadParameter("user is already logged in with a resource access request, cannot assume another")
			}
			allowedResourceIDs = accessRequest.GetRequestedResourceIDs()
		}

		webSessionTTL := a.getWebSessionTTL(accessRequest)

		// Let the session expire with the shortest expiry time.
		if expiresAt.After(webSessionTTL) {
			expiresAt = webSessionTTL
		}
	} else if req.Switchback {
		if prevSession.GetLoginTime().IsZero() {
			return nil, trace.BadParameter("Unable to switchback, log in time was not recorded.")
		}

		// Get default/static roles.
		userState, err := a.GetUserOrLoginState(ctx, req.User)
		if err != nil {
			return nil, trace.Wrap(err, "failed to switchback")
		}

		// Reset any search-based access requests
		allowedResourceIDs = nil

		// Calculate expiry time.
		roleSet, err := services.FetchRoles(userState.GetRoles(), a, userState.GetTraits())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		sessionTTL := roleSet.AdjustSessionTTL(apidefaults.CertDuration)

		// Set default roles and expiration.
		expiresAt = prevSession.GetLoginTime().UTC().Add(sessionTTL)
		roles = userState.GetRoles()
		accessRequests = nil
	}

	// Create a new web session with the same private key. This way, if the
	// original session was an attested web session, the extended session will
	// also be an attested web session.
	prevSSHKey, err := keys.ParsePrivateKey(prevSession.GetSSHPriv())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	prevTLSKey, err := keys.ParsePrivateKey(prevSession.GetTLSPriv())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Keep existing device extensions in the new session.
	opts := &newWebSessionOpts{}
	if prevSession.GetHasDeviceExtensions() {
		var err error
		opts.deviceExtensions, err = decodeDeviceExtensionsFromSession(prevSession)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	sessionTTL := utils.ToTTL(a.clock, expiresAt)
	sess, _, err := a.newWebSession(ctx, NewWebSessionRequest{
		User:                 req.User,
		LoginIP:              identity.LoginIP,
		Roles:                roles,
		Traits:               traits,
		SessionTTL:           sessionTTL,
		AccessRequests:       accessRequests,
		RequestedResourceIDs: allowedResourceIDs,
		SSHPrivateKey:        prevSSHKey,
		TLSPrivateKey:        prevTLSKey,
	}, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Keep preserving the login time.
	sess.SetLoginTime(prevSession.GetLoginTime())

	sess.SetConsumedAccessRequestID(req.AccessRequestID)

	if err := a.upsertWebSession(ctx, sess); err != nil {
		return nil, trace.Wrap(err)
	}

	return sess, nil
}

func decodeDeviceExtensionsFromSession(webSession types.WebSession) (*tlsca.DeviceExtensions, error) {
	// Reading the extensions from the session itself means we are always taking
	// them for a legitimate source (ie, certificates issued by Auth).
	// We don't re-validate the certificates when decoding the extensions.

	block, _ := pem.Decode(webSession.GetTLSCert())
	if block == nil {
		return nil, trace.BadParameter("failed to decode session TLS certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certIdentity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &certIdentity.DeviceExtensions, nil
}

// getWebSessionTTL returns the earliest expiration time of allowed in the access request.
func (a *Server) getWebSessionTTL(accessRequest types.AccessRequest) time.Time {
	webSessionTTL := accessRequest.GetAccessExpiry()
	sessionTTL := accessRequest.GetSessionTLL()
	if sessionTTL.IsZero() {
		return webSessionTTL
	}

	// Session TTL contains the time when the session should end.
	// We need to subtract it from the creation time to get the
	// session duration.
	sessionDuration := sessionTTL.Sub(accessRequest.GetCreationTime())
	// Calculate the adjusted session TTL.
	adjustedSessionTTL := a.clock.Now().UTC().Add(sessionDuration)
	// Adjusted TTL can't exceed webSessionTTL.
	if adjustedSessionTTL.Before(webSessionTTL) {
		return adjustedSessionTTL
	}
	return webSessionTTL
}

func (a *Server) getValidatedAccessRequest(ctx context.Context, identity tlsca.Identity, user string, accessRequestID string) (types.AccessRequest, error) {
	reqFilter := types.AccessRequestFilter{
		User: user,
		ID:   accessRequestID,
	}

	reqs, err := a.GetAccessRequests(ctx, reqFilter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(reqs) < 1 {
		return nil, trace.NotFound("access request %q not found", accessRequestID)
	}

	req := reqs[0]

	if !req.GetState().IsApproved() {
		if req.GetState().IsDenied() {
			return nil, trace.AccessDenied("access request %q has been denied", accessRequestID)
		}
		if req.GetState().IsPromoted() {
			return nil, trace.AccessDenied("access request %q has been promoted. Use access list to access resources.", accessRequestID)
		}
		return nil, trace.AccessDenied("access request %q is awaiting approval", accessRequestID)
	}

	if err := services.ValidateAccessRequestForUser(ctx, a.clock, a, req, identity); err != nil {
		return nil, trace.Wrap(err)
	}

	accessExpiry := req.GetAccessExpiry()
	if accessExpiry.Before(a.GetClock().Now()) {
		return nil, trace.BadParameter("access request %q has expired", accessRequestID)
	}

	if req.GetAssumeStartTime() != nil && req.GetAssumeStartTime().After(a.GetClock().Now()) {
		return nil, trace.BadParameter("access request %q can not be assumed until %v", accessRequestID, req.GetAssumeStartTime())
	}

	return req, nil
}

// CreateWebSession creates a new web session for user without any
// checks, is used by admins
func (a *Server) CreateWebSession(ctx context.Context, user string) (types.WebSession, error) {
	u, err := a.GetUserOrLoginState(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session, err := a.CreateWebSessionFromReq(ctx, NewWebSessionRequest{
		User:      user,
		Roles:     u.GetRoles(),
		Traits:    u.GetTraits(),
		LoginTime: a.clock.Now().UTC(),
	})
	return session, trace.Wrap(err)
}

// ExtractHostID returns host id based on the hostname
func ExtractHostID(hostName string, clusterName string) (string, error) {
	suffix := "." + clusterName
	if !strings.HasSuffix(hostName, suffix) {
		return "", trace.BadParameter("expected suffix %q in %q", suffix, hostName)
	}
	return strings.TrimSuffix(hostName, suffix), nil
}

// HostFQDN consists of host UUID and cluster name joined via .
func HostFQDN(hostUUID, clusterName string) string {
	return fmt.Sprintf("%v.%v", hostUUID, clusterName)
}

// GenerateHostCerts generates new host certificates (signed
// by the host certificate authority) for a node.
func (a *Server) GenerateHostCerts(ctx context.Context, req *proto.HostCertsRequest) (*proto.Certs, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.Role.Check(); err != nil {
		return nil, err
	}

	if err := a.limiter.AcquireConnection(req.Role.String()); err != nil {
		generateThrottledRequestsCount.Inc()
		log.Debugf("Node %q [%v] is rate limited: %v.", req.NodeName, req.HostID, req.Role)
		return nil, trace.Wrap(err)
	}
	defer a.limiter.ReleaseConnection(req.Role.String())

	// only observe latencies for non-throttled requests
	start := a.clock.Now()
	defer func() { generateRequestsLatencies.Observe(time.Since(start).Seconds()) }()

	generateRequestsCount.Inc()
	generateRequestsCurrent.Inc()
	defer generateRequestsCurrent.Dec()

	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If the request contains 0.0.0.0, this implies an advertise IP was not
	// specified on the node. Try and guess what the address by replacing 0.0.0.0
	// with the RemoteAddr as known to the Auth Server.
	if slices.Contains(req.AdditionalPrincipals, defaults.AnyAddress) {
		remoteHost, err := utils.Host(req.RemoteAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		req.AdditionalPrincipals = utils.ReplaceInSlice(
			req.AdditionalPrincipals,
			defaults.AnyAddress,
			remoteHost)
	}

	if _, _, _, _, err := ssh.ParseAuthorizedKey(req.PublicSSHKey); err != nil {
		return nil, trace.BadParameter("failed to parse SSH public key")
	}
	cryptoPubKey, err := keys.ParsePublicKey(req.PublicTLSKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// get the certificate authority that will be signing the public key of the host,
	client := a.Cache
	if req.NoCache {
		client = a.Services
	}
	ca, err := client.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return nil, trace.BadParameter("failed to load host CA for %q: %v", clusterName.GetClusterName(), err)
	}

	// could be a couple of scenarios, either client data is out of sync,
	// or auth server is out of sync, either way, for now check that
	// cache is out of sync, this will result in higher read rate
	// to the backend, which is a fine tradeoff
	if !req.NoCache && !req.Rotation.IsZero() && !req.Rotation.Matches(ca.GetRotation()) {
		log.Debugf("Client sent rotation state %v, cache state is %v, using state from the DB.", req.Rotation, ca.GetRotation())
		ca, err = a.Services.GetCertAuthority(ctx, types.CertAuthID{
			Type:       types.HostCA,
			DomainName: clusterName.GetClusterName(),
		}, true)
		if err != nil {
			return nil, trace.BadParameter("failed to load host CA for %q: %v", clusterName.GetClusterName(), err)
		}
		if !req.Rotation.Matches(ca.GetRotation()) {
			return nil, trace.BadParameter(""+
				"the client expected state is out of sync, server rotation state: %v, "+
				"client rotation state: %v, re-register the client from scratch to fix the issue.",
				ca.GetRotation(), req.Rotation)
		}
	}

	isAdminRole := req.Role == types.RoleAdmin

	cert, signer, err := a.keyStore.GetTLSCertAndSigner(ctx, ca)
	if trace.IsNotFound(err) && isAdminRole {
		// If there is no local TLS signer found in the host CA ActiveKeys, this
		// auth server may have a newly configured HSM and has only populated
		// local keys in the AdditionalTrustedKeys until the next CA rotation.
		// This is the only case where we should be able to get a signer from
		// AdditionalTrustedKeys but not ActiveKeys.
		cert, signer, err = a.keyStore.GetAdditionalTrustedTLSCertAndSigner(ctx, ca)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsAuthority, err := tlsca.FromCertAndSigner(cert, signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caSigner, err := a.keyStore.GetSSHSigner(ctx, ca)
	if trace.IsNotFound(err) && isAdminRole {
		// If there is no local SSH signer found in the host CA ActiveKeys, this
		// auth server may have a newly configured HSM and has only populated
		// local keys in the AdditionalTrustedKeys until the next CA rotation.
		// This is the only case where we should be able to get a signer from
		// AdditionalTrustedKeys but not ActiveKeys.
		caSigner, err = a.keyStore.GetAdditionalTrustedSSHSigner(ctx, ca)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// generate host SSH certificate
	hostSSHCert, err := a.generateHostCert(ctx, sshca.HostCertificateRequest{
		CASigner:      caSigner,
		PublicHostKey: req.PublicSSHKey,
		HostID:        req.HostID,
		NodeName:      req.NodeName,
		Identity: sshca.Identity{
			ClusterName: clusterName.GetClusterName(),
			SystemRole:  req.Role,
			Principals:  req.AdditionalPrincipals,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Role == types.RoleInstance && len(req.SystemRoles) == 0 {
		return nil, trace.BadParameter("cannot generate instance cert with no system roles")
	}

	systemRoles := make([]string, 0, len(req.SystemRoles))
	for _, r := range req.SystemRoles {
		systemRoles = append(systemRoles, string(r))
	}

	// generate host TLS certificate
	identity := tlsca.Identity{
		Username:        authclient.HostFQDN(req.HostID, clusterName.GetClusterName()),
		Groups:          []string{req.Role.String()},
		TeleportCluster: clusterName.GetClusterName(),
		SystemRoles:     systemRoles,
	}
	subject, err := identity.Subject()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certRequest := tlsca.CertificateRequest{
		Clock:     a.clock,
		PublicKey: cryptoPubKey,
		Subject:   subject,
		NotAfter:  a.clock.Now().UTC().Add(defaults.CATTL),
		DNSNames:  append([]string{}, req.AdditionalPrincipals...),
	}

	// API requests need to specify a DNS name, which must be present in the certificate's DNS Names.
	// The target DNS is not always known in advance, so we add a default one to all certificates.
	certRequest.DNSNames = append(certRequest.DNSNames, DefaultDNSNamesForRole(req.Role)...)
	// Unlike additional principals, DNS Names is x509 specific and is limited
	// to services with TLS endpoints (e.g. auth, proxies, kubernetes)
	if (types.SystemRoles{req.Role}).IncludeAny(types.RoleAuth, types.RoleAdmin, types.RoleProxy, types.RoleKube, types.RoleWindowsDesktop) {
		certRequest.DNSNames = append(certRequest.DNSNames, req.DNSNames...)
	}
	hostTLSCert, err := tlsAuthority.GenerateCertificate(certRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.Certs{
		SSH:        hostSSHCert,
		TLS:        hostTLSCert,
		TLSCACerts: services.GetTLSCerts(ca),
		SSHCACerts: services.GetSSHCheckingKeys(ca),
	}, nil
}

// AssertSystemRole is used by agents to prove that they have a given system role when their credentials
// originate from multiple separate join tokens so that they can be issued an instance certificate that
// encompasses all of their capabilities. This method will be deprecated once we have a more comprehensive
// model for join token joining/replacement.
func (a *Server) AssertSystemRole(ctx context.Context, req proto.SystemRoleAssertion) error {
	return trace.Wrap(a.Unstable.AssertSystemRole(ctx, req))
}

// GetSystemRoleAssertions is used in validated claims made by older instances to prove that they hold a given
// system role. This method will be deprecated once we have a more comprehensive model for join token
// joining/replacement.
func (a *Server) GetSystemRoleAssertions(ctx context.Context, serverID string, assertionID string) (proto.SystemRoleAssertionSet, error) {
	set, err := a.Unstable.GetSystemRoleAssertions(ctx, serverID, assertionID)
	return set, trace.Wrap(err)
}

func (a *Server) RegisterInventoryControlStream(ics client.UpstreamInventoryControlStream, hello proto.UpstreamInventoryHello) error {
	// upstream hello is pulled and checked at rbac layer. we wait to send the downstream hello until we get here
	// in order to simplify creation of in-memory streams when dealing with local auth (note: in theory we could
	// send hellos simultaneously to slightly improve perf, but there is a potential benefit to having the
	// downstream hello serve double-duty as an indicator of having successfully transitioned the rbac layer).
	downstreamHello := proto.DownstreamInventoryHello{
		Version:  teleport.Version,
		ServerID: a.ServerID,
		Capabilities: &proto.DownstreamInventoryHello_SupportedCapabilities{
			NodeHeartbeats:       true,
			AppHeartbeats:        true,
			AppCleanup:           true,
			DatabaseHeartbeats:   true,
			DatabaseCleanup:      true,
			KubernetesHeartbeats: true,
			KubernetesCleanup:    true,
		},
	}
	if err := ics.Send(a.CloseContext(), downstreamHello); err != nil {
		return trace.Wrap(err)
	}
	a.inventory.RegisterControlStream(ics, hello)
	return nil
}

// MakeLocalInventoryControlStream sets up an in-memory control stream which automatically registers with this auth
// server upon hello exchange.
func (a *Server) MakeLocalInventoryControlStream(opts ...client.ICSPipeOption) client.DownstreamInventoryControlStream {
	upstream, downstream := client.InventoryControlStreamPipe(opts...)
	go func() {
		select {
		case msg := <-upstream.Recv():
			hello, ok := msg.(proto.UpstreamInventoryHello)
			if !ok {
				upstream.CloseWithError(trace.BadParameter("expected upstream hello, got: %T", msg))
				return
			}
			if err := a.RegisterInventoryControlStream(upstream, hello); err != nil {
				upstream.CloseWithError(err)
				return
			}
		case <-upstream.Done():
		case <-a.CloseContext().Done():
			upstream.Close()
		}
	}()
	return downstream
}

func (a *Server) GetInventoryStatus(ctx context.Context, req proto.InventoryStatusRequest) (proto.InventoryStatusSummary, error) {
	var rsp proto.InventoryStatusSummary
	if req.Connected {
		a.inventory.Iter(func(handle inventory.UpstreamHandle) {
			rsp.Connected = append(rsp.Connected, handle.Hello())
		})

		// connected instance list is a special case, don't bother aggregating heartbeats
		return rsp, nil
	}

	rsp.VersionCounts = make(map[string]uint32)
	rsp.UpgraderCounts = make(map[string]uint32)
	rsp.ServiceCounts = make(map[string]uint32)

	ins := a.GetInstances(ctx, types.InstanceFilter{})

	for ins.Next() {
		rsp.InstanceCount++

		rsp.VersionCounts[vc.Normalize(ins.Item().GetTeleportVersion())]++

		upgrader := ins.Item().GetExternalUpgrader()
		if upgrader == "" {
			upgrader = "none"
		}

		rsp.UpgraderCounts[upgrader]++

		for _, service := range ins.Item().GetServices() {
			rsp.ServiceCounts[string(service)]++
		}
	}

	return rsp, ins.Done()
}

// GetInventoryConnectedServiceCounts returns the counts of each connected service seen in the inventory.
func (a *Server) GetInventoryConnectedServiceCounts() proto.InventoryConnectedServiceCounts {
	return proto.InventoryConnectedServiceCounts{
		ServiceCounts: a.inventory.ConnectedServiceCounts(),
	}
}

// GetInventoryConnectedServiceCount returns the counts of a particular connected service seen in the inventory.
func (a *Server) GetInventoryConnectedServiceCount(service types.SystemRole) uint64 {
	return a.inventory.ConnectedServiceCount(service)
}

func (a *Server) PingInventory(ctx context.Context, req proto.InventoryPingRequest) (proto.InventoryPingResponse, error) {
	stream, ok := a.inventory.GetControlStream(req.ServerID)
	if !ok {
		return proto.InventoryPingResponse{}, trace.NotFound("no control stream found for server %q", req.ServerID)
	}

	id := mathrand.Uint64()

	if req.ControlLog { //nolint:staticcheck // SA1019. Checking deprecated field that may be sent by older clients.
		return proto.InventoryPingResponse{}, trace.BadParameter("ControlLog pings are not supported")
	}

	d, err := stream.Ping(ctx, id)
	if err != nil {
		return proto.InventoryPingResponse{}, trace.Wrap(err)
	}

	return proto.InventoryPingResponse{
		Duration: d,
	}, nil
}

// UpdateLabels updates the labels on an instance over the inventory control
// stream.
func (a *Server) UpdateLabels(ctx context.Context, req proto.InventoryUpdateLabelsRequest) error {
	stream, ok := a.inventory.GetControlStream(req.ServerID)
	if !ok {
		return trace.NotFound("no control stream found for server %q", req.ServerID)
	}
	return trace.Wrap(stream.UpdateLabels(ctx, req.Kind, req.Labels))
}

// TokenExpiredOrNotFound is a special message returned by the auth server when provisioning
// tokens are either past their TTL, or could not be found.
const TokenExpiredOrNotFound = "token expired or not found"

// ValidateToken takes a provisioning token value and finds if it's valid. Returns
// a list of roles this token allows its owner to assume and token labels, or an error if the token
// cannot be found.
func (a *Server) ValidateToken(ctx context.Context, token string) (types.ProvisionToken, error) {
	tkns, err := a.GetStaticTokens()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// First check if the token is a static token. If it is, return right away.
	// Static tokens have no expiration.
	for _, st := range tkns.GetStaticTokens() {
		if subtle.ConstantTimeCompare([]byte(st.GetName()), []byte(token)) == 1 {
			return st, nil
		}
	}

	// If it's not a static token, check if it's a ephemeral token in the backend.
	// If a ephemeral token is found, make sure it's still valid.
	tok, err := a.GetToken(ctx, token)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.AccessDenied(TokenExpiredOrNotFound)
		}
		return nil, trace.Wrap(err)
	}
	if !a.checkTokenTTL(tok) {
		return nil, trace.AccessDenied(TokenExpiredOrNotFound)
	}

	return tok, nil
}

// checkTokenTTL checks if the token is still valid. If it is not, the token
// is removed from the backend and returns false. Otherwise returns true.
func (a *Server) checkTokenTTL(tok types.ProvisionToken) bool {
	// Always accept tokens without an expiry configured.
	if tok.Expiry().IsZero() {
		return true
	}

	now := a.clock.Now().UTC()
	if tok.Expiry().Before(now) {
		// Tidy up the expired token in background if it has expired.
		go func() {
			ctx, cancel := context.WithTimeout(a.CloseContext(), time.Second*30)
			defer cancel()
			if err := a.DeleteToken(ctx, tok.GetName()); err != nil {
				if !trace.IsNotFound(err) {
					log.Warnf("Unable to delete token from backend: %v.", err)
				}
			}
		}()
		return false
	}
	return true
}

func (a *Server) DeleteToken(ctx context.Context, token string) (err error) {
	tkns, err := a.GetStaticTokens()
	if err != nil {
		return trace.Wrap(err)
	}

	// is this a static token?
	for _, st := range tkns.GetStaticTokens() {
		if subtle.ConstantTimeCompare([]byte(st.GetName()), []byte(token)) == 1 {
			return trace.BadParameter("token %s is statically configured and cannot be removed", backend.MaskKeyName(token))
		}
	}
	// Delete a user token.
	if err = a.DeleteUserToken(ctx, token); err == nil {
		return nil
	}
	// delete node token:
	if err = a.Services.DeleteToken(ctx, token); err == nil {
		return nil
	}
	return trace.Wrap(err)
}

// GetTokens returns all tokens (machine provisioning ones and user tokens). Machine
// tokens usually have "node roles", like auth,proxy,node and user invitation tokens have 'signup' role
func (a *Server) GetTokens(ctx context.Context, opts ...services.MarshalOption) (tokens []types.ProvisionToken, err error) {
	// get node tokens:
	tokens, err = a.Services.GetTokens(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// get static tokens:
	tkns, err := a.GetStaticTokens()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if err == nil {
		tokens = append(tokens, tkns.GetStaticTokens()...)
	}
	// get user tokens:
	userTokens, err := a.GetUserTokens(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// convert user tokens to machine tokens:
	for _, t := range userTokens {
		roles := types.SystemRoles{types.RoleSignup}
		tok, err := types.NewProvisionToken(t.GetName(), roles, t.Expiry())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tokens = append(tokens, tok)
	}
	return tokens, nil
}

// GetWebSessionInfo returns the web session specified with sessionID for the given user.
// The session is stripped of any authentication details.
// Implements auth.WebUIService
func (a *Server) GetWebSessionInfo(ctx context.Context, user, sessionID string) (types.WebSession, error) {
	sess, err := a.GetWebSession(ctx, types.GetWebSessionRequest{User: user, SessionID: sessionID})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sess.WithoutSecrets(), nil
}

func (a *Server) DeleteNamespace(namespace string) error {
	ctx := context.TODO()
	if namespace == apidefaults.Namespace {
		return trace.AccessDenied("can't delete default namespace")
	}
	nodes, err := a.GetNodes(ctx, namespace)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(nodes) != 0 {
		return trace.BadParameter("can't delete namespace %v that has %v registered nodes", namespace, len(nodes))
	}
	return a.Services.DeleteNamespace(namespace)
}

// IterateRoles is a helper used to read a page of roles with a custom matcher, used by access-control logic to handle
// per-resource read permissions.
func (a *Server) IterateRoles(ctx context.Context, req *proto.ListRolesRequest, match func(*types.RoleV6) (bool, error)) ([]*types.RoleV6, string, error) {
	const maxIterations = 100_000

	if req.Limit == 0 {
		req.Limit = apidefaults.DefaultChunkSize
	}

	req.Limit++
	defer func() {
		req.Limit--
	}()

	var filtered []*types.RoleV6
	var iterations int

Outer:
	for {
		iterations++
		if iterations > maxIterations {
			return nil, "", trace.Errorf("too many role page iterations (%d), this is likely a bug", iterations)
		}

		rsp, err := a.Cache.ListRoles(ctx, req)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

	Inner:
		for _, role := range rsp.Roles {
			ok, err := match(role)
			if err != nil {
				return nil, "", trace.Wrap(err)
			}

			if !ok {
				continue Inner
			}

			filtered = append(filtered, role)
			if len(filtered) == int(req.Limit) {
				break Outer
			}
		}

		req.StartKey = rsp.NextKey

		if req.StartKey == "" {
			break Outer
		}
	}

	var nextKey string
	if len(filtered) == int(req.Limit) {
		nextKey = filtered[req.Limit-1].GetName()
		filtered = filtered[:req.Limit-1]
	}

	return filtered, nextKey, nil
}

// ListAccessRequests is an access request getter with pagination and sorting options.
func (a *Server) ListAccessRequests(ctx context.Context, req *proto.ListAccessRequestsRequest) (*proto.ListAccessRequestsResponse, error) {
	// most access request methods target the backend directly since access requests are frequently read
	// immediately after writing, but listing requires support for custom sort orders so we route it to
	// a special cache. note that the access request cache will still end up forwarding single-request
	// reads to the real backend due to the read after write issue.
	return a.AccessRequestCache.ListAccessRequests(ctx, req)
}

// ListMatchingAccessRequests is equivalent to ListAccessRequests except that it adds the ability to provide an arbitrary matcher function. This method
// should be preferred when using custom filtering (e.g. access-controls), since the paginations keys used by the access request cache are non-standard.
func (a *Server) ListMatchingAccessRequests(ctx context.Context, req *proto.ListAccessRequestsRequest, match func(*types.AccessRequestV3) bool) (*proto.ListAccessRequestsResponse, error) {
	// most access request methods target the backend directly since access requests are frequently read
	// immediately after writing, but listing requires support for custom sort orders so we route it to
	// a special cache. note that the access request cache will still end up forwarding single-request
	// reads to the real backend due to the read after write issue.
	return a.AccessRequestCache.ListMatchingAccessRequests(ctx, req, match)
}

func (a *Server) CreateAccessRequestV2(ctx context.Context, req types.AccessRequest, identity tlsca.Identity) (types.AccessRequest, error) {
	now := a.clock.Now().UTC()

	req.SetCreationTime(now)

	// Always perform variable expansion on creation only; this ensures the
	// access request that is reviewed is the same that is approved.
	expandOpts := services.ExpandVars(true)
	if err := services.ValidateAccessRequestForUser(ctx, a.clock, a, req, identity, expandOpts); err != nil {
		return nil, trace.Wrap(err)
	}

	// Look for user groups and associated applications to the request.
	requestedResourceIDs, err := a.appendImplicitlyRequiredResources(ctx, req.GetRequestedResourceIDs())
	if err != nil {
		return nil, trace.Wrap(err, "adding additional implicitly required resources")
	}
	req.SetRequestedResourceIDs(requestedResourceIDs)

	if req.GetDryRun() {
		_, promotions := a.generateAccessRequestPromotions(ctx, req)
		// update the request with additional reviewers if possible.
		updateAccessRequestWithAdditionalReviewers(ctx, req, a.AccessLists, promotions)
		// Made it this far with no errors, return before creating the request
		// if this is a dry run.
		return req, nil
	}

	if err := a.verifyAccessRequestMonthlyLimit(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	log.Debugf("Creating Access Request %v with expiry %v.", req.GetName(), req.Expiry())

	if _, err := a.Services.CreateAccessRequestV2(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}

	var annotations *apievents.Struct
	if sa := req.GetSystemAnnotations(); len(sa) > 0 {
		var err error
		annotations, err = apievents.EncodeMapStrings(sa)
		if err != nil {
			log.WithError(err).Debug("Failed to encode access request annotations.")
		}
	}

	err = a.emitter.EmitAuditEvent(a.closeCtx, &apievents.AccessRequestCreate{
		Metadata: apievents.Metadata{
			Type: events.AccessRequestCreateEvent,
			Code: events.AccessRequestCreateCode,
		},
		UserMetadata: authz.ClientUserMetadataWithUser(ctx, req.GetUser()),
		ResourceMetadata: apievents.ResourceMetadata{
			Expires: req.GetAccessExpiry(),
		},
		Roles:                req.GetRoles(),
		RequestedResourceIDs: apievents.ResourceIDs(req.GetRequestedResourceIDs()),
		RequestID:            req.GetName(),
		RequestState:         req.GetState().String(),
		Reason:               req.GetRequestReason(),
		MaxDuration:          req.GetMaxDuration(),
		Annotations:          annotations,
	})
	if err != nil {
		log.WithError(err).Warn("Failed to emit access request create event.")
	}

	// Create a notification.
	var notificationText string
	// If this is a resource request.
	if len(req.GetRequestedResourceIDs()) > 0 {
		notificationText = fmt.Sprintf("%s requested access to %d resources.", req.GetUser(), len(req.GetRequestedResourceIDs()))
		if len(req.GetRequestedResourceIDs()) == 1 {
			notificationText = fmt.Sprintf("%s requested access to a resource.", req.GetUser())
		}
		// If this is a role request.
	} else {
		notificationText = fmt.Sprintf("%s requested access to the '%s' role.", req.GetUser(), req.GetRoles()[0])
		if len(req.GetRoles()) > 1 {
			notificationText = fmt.Sprintf("%s requested access to %d roles.", req.GetUser(), len(req.GetRoles()))
		}
	}

	_, err = a.Services.CreateGlobalNotification(ctx, &notificationsv1.GlobalNotification{
		Spec: &notificationsv1.GlobalNotificationSpec{
			Matcher: &notificationsv1.GlobalNotificationSpec_ByPermissions{
				ByPermissions: &notificationsv1.ByPermissions{
					RoleConditions: []*types.RoleConditions{
						{
							ReviewRequests: &types.AccessReviewConditions{
								Roles: req.GetOriginalRoles(),
							},
						},
					},
				},
			},
			// Prevent the requester from seeing the notification for their own access request.
			ExcludeUsers: []string{req.GetUser()},
			Notification: &notificationsv1.Notification{
				Spec:    &notificationsv1.NotificationSpec{},
				SubKind: types.NotificationAccessRequestPendingSubKind,
				Metadata: &headerv1.Metadata{
					Labels:  map[string]string{types.NotificationTitleLabel: notificationText, "request-id": req.GetName()},
					Expires: timestamppb.New(req.Expiry()),
				},
			},
		},
	})
	if err != nil {
		log.WithError(err).Warn("Failed to create access request notification")
	}

	// calculate the promotions
	reqCopy, promotions := a.generateAccessRequestPromotions(ctx, req)
	if promotions != nil {
		// Create the promotion entry even if the allowed promotion is empty. Otherwise, we won't
		// be able to distinguish between an allowed empty set and generation failure.
		if err := a.Services.CreateAccessRequestAllowedPromotions(ctx, reqCopy, promotions); err != nil {
			log.WithError(err).Warn("Failed to update access request with promotions.")
		}
	}

	accessRequestsCreatedMetric.WithLabelValues(
		strconv.Itoa(len(req.GetRoles())),
		strconv.Itoa(len(req.GetRequestedResourceIDs()))).Inc()
	return req, nil
}

// appendImplicitlyRequiredResources examines the set of requested resources and adds
// any extra resources that are implicitly required by the request.
func (a *Server) appendImplicitlyRequiredResources(ctx context.Context, resources []types.ResourceID) ([]types.ResourceID, error) {
	addedApps := utils.NewSet[string]()
	var userGroups []types.ResourceID
	var accountAssignments []types.ResourceID

	for _, resource := range resources {
		switch resource.Kind {
		case types.KindApp:
			addedApps.Add(resource.Name)
		case types.KindUserGroup:
			userGroups = append(userGroups, resource)
		case types.KindIdentityCenterAccountAssignment:
			accountAssignments = append(accountAssignments, resource)
		}
	}

	for _, resource := range userGroups {
		userGroup, err := a.GetUserGroup(ctx, resource.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, app := range userGroup.GetApplications() {
			// Only add to the request if we haven't already added it.
			if !addedApps.Contains(app) {
				resources = append(resources, types.ResourceID{
					ClusterName: resource.ClusterName,
					Kind:        types.KindApp,
					Name:        app,
				})
				addedApps.Add(app)
			}
		}
	}

	icAccounts := utils.NewSet[string]()
	for _, resource := range accountAssignments {
		// The UI needs access to the account associated with an Account Assignment
		// in order to display the enclosing Account, otherwise the user will not
		// be able to see their assigned permission sets.
		assignmentID := services.IdentityCenterAccountAssignmentID(resource.Name)
		asmt, err := a.Services.IdentityCenter.GetAccountAssignment(ctx, assignmentID)
		if err != nil {
			return nil, trace.Wrap(err, "fetching identity center account assignment")
		}

		if icAccounts.Contains(asmt.GetSpec().GetAccountId()) {
			continue
		}

		resources = append(resources, types.ResourceID{
			ClusterName: resource.ClusterName,
			Kind:        types.KindIdentityCenterAccount,
			Name:        asmt.GetSpec().GetAccountId(),
		})
		icAccounts.Add(asmt.GetSpec().GetAccountId())
	}

	return resources, nil
}

// generateAccessRequestPromotions will return potential access list promotions for an access request. On error, this function will log
// the error and return whatever it has. The caller is expected to deal with the possibility of a nil promotions object.
func (a *Server) generateAccessRequestPromotions(ctx context.Context, req types.AccessRequest) (types.AccessRequest, *types.AccessRequestAllowedPromotions) {
	reqCopy := req.Copy()
	promotions, err := modules.GetModules().GenerateAccessRequestPromotions(ctx, a.Cache, reqCopy)
	if err != nil {
		// Do not fail the request if the promotions failed to generate.
		// The request promotion will be blocked, but the request can still be approved.
		log.WithError(err).Warn("Failed to generate access list promotions.")
	}
	return reqCopy, promotions
}

// updateAccessRequestWithAdditionalReviewers will update the given access request with additional reviewers given the promotions
// created for the access request.
func updateAccessRequestWithAdditionalReviewers(ctx context.Context, req types.AccessRequest, accessLists services.AccessListsGetter, promotions *types.AccessRequestAllowedPromotions) {
	if promotions == nil {
		return
	}

	// For promotions, add in access list owners as additional suggested reviewers
	additionalReviewers := map[string]struct{}{}

	// Iterate through the promotions, adding the owners of the corresponding access lists as reviewers.
	for _, promotion := range promotions.Promotions {
		allOwners, err := accessLists.GetAccessListOwners(ctx, promotion.AccessListName)
		if err != nil {
			log.WithError(err).Warnf("Failed to get nested access list owners for %v, skipping additional reviewers", promotion.AccessListName)
			break
		}

		for _, owner := range allOwners {
			additionalReviewers[owner.Name] = struct{}{}
		}
	}

	// Only modify the original request if additional reviewers were found.
	if len(additionalReviewers) > 0 {
		req.SetSuggestedReviewers(append(req.GetSuggestedReviewers(), maps.Keys(additionalReviewers)...))
	}
}

func (a *Server) DeleteAccessRequest(ctx context.Context, name string) error {
	if err := a.Services.DeleteAccessRequest(ctx, name); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.AccessRequestDelete{
		Metadata: apievents.Metadata{
			Type: events.AccessRequestDeleteEvent,
			Code: events.AccessRequestDeleteCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		RequestID:    name,
	}); err != nil {
		log.WithError(err).Warn("Failed to emit access request delete event.")
	}
	return nil
}

func (a *Server) SetAccessRequestState(ctx context.Context, params types.AccessRequestUpdate) error {
	req, err := a.Services.SetAccessRequestState(ctx, params)
	if err != nil {
		return trace.Wrap(err)
	}
	event := &apievents.AccessRequestCreate{
		Metadata: apievents.Metadata{
			Type: events.AccessRequestUpdateEvent,
			Code: events.AccessRequestUpdateCode,
		},
		ResourceMetadata: apievents.ResourceMetadata{
			UpdatedBy: authz.ClientUsername(ctx),
			Expires:   req.GetAccessExpiry(),
		},
		RequestID:       params.RequestID,
		RequestState:    params.State.String(),
		Reason:          params.Reason,
		Roles:           params.Roles,
		AssumeStartTime: params.AssumeStartTime,
	}
	if sa := req.GetSystemAnnotations(); len(sa) > 0 {
		var err error
		event.Annotations, err = apievents.EncodeMapStrings(sa)
		if err != nil {
			log.WithError(err).Debug("Failed to encode access request annotations.")
		}
	}

	if delegator := apiutils.GetDelegator(ctx); delegator != "" {
		event.Delegator = delegator
	}

	if len(params.Annotations) > 0 {
		annotations, err := apievents.EncodeMapStrings(params.Annotations)
		if err != nil {
			log.WithError(err).Debugf("Failed to encode access request annotations.")
		} else {
			event.Annotations = annotations
		}
	}
	err = a.emitter.EmitAuditEvent(a.closeCtx, event)
	if err != nil {
		log.WithError(err).Warn("Failed to emit access request update event.")
	}
	return trace.Wrap(err)
}

// SubmitAccessReview is used to process a review of an Access Request.
// This is implemented by Server.submitAccessRequest but this method exists
// to provide a matching signature with the auth client. This allows the
// hosted plugins to use the Server struct directly as a client.
func (a *Server) SubmitAccessReview(
	ctx context.Context,
	params types.AccessReviewSubmission,
) (types.AccessRequest, error) {
	// identity is passed as nil as we do not know which user has triggered
	// this action.
	return a.submitAccessReview(ctx, params, nil)
}

// submitAccessReview implements submitting a review of an Access Request.
// The `identity` parameter should be the identity of the user that has called
// an RPC that has invoked this, if applicable. It may be nil if this is
// unknown.
func (a *Server) submitAccessReview(
	ctx context.Context,
	params types.AccessReviewSubmission,
	identity *tlsca.Identity,
) (types.AccessRequest, error) {
	// When promoting a request, the access list name must be set.
	if params.Review.ProposedState.IsPromoted() && params.Review.GetAccessListName() == "" {
		return nil, trace.BadParameter("promoted access list can be only set when promoting access requests")
	}

	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// set up a checker for the review author
	checker, err := services.NewReviewPermissionChecker(ctx, a, params.Review.Author, identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// don't bother continuing if the author has no allow directives
	if !checker.HasAllowDirectives() {
		return nil, trace.AccessDenied("user %q cannot submit reviews", params.Review.Author)
	}

	// final permission checks and review application must be done by the local backend
	// service, as their validity depends upon optimistic locking.
	req, err := a.ApplyAccessReview(ctx, params, checker)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	event := &apievents.AccessRequestCreate{
		Metadata: apievents.Metadata{
			Type:        events.AccessRequestReviewEvent,
			Code:        events.AccessRequestReviewCode,
			ClusterName: clusterName.GetClusterName(),
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Expires: req.GetAccessExpiry(),
		},
		RequestID:              params.RequestID,
		RequestState:           req.GetState().String(),
		ProposedState:          params.Review.ProposedState.String(),
		Reason:                 params.Review.Reason,
		Reviewer:               params.Review.Author,
		MaxDuration:            req.GetMaxDuration(),
		PromotedAccessListName: req.GetPromotedAccessListName(),
	}

	// Create a notification.
	if !req.GetState().IsPending() {
		_, err = a.Services.CreateUserNotification(ctx, generateAccessRequestReviewedNotification(req, params))
		if err != nil {
			log.WithError(err).Debugf("Failed to emit access request reviewed notification.")
		}
	}

	if len(params.Review.Annotations) > 0 {
		annotations, err := apievents.EncodeMapStrings(params.Review.Annotations)
		if err != nil {
			log.WithError(err).Debugf("Failed to encode access request annotations.")
		} else {
			event.Annotations = annotations
		}
	}
	if err := a.emitter.EmitAuditEvent(a.closeCtx, event); err != nil {
		log.WithError(err).Warn("Failed to emit access request update event.")
	}

	return req, nil
}

// generateAccessRequestReviewedNotification returns the notification object for a notification notifying a user of their
// access request being approved or denied.
func generateAccessRequestReviewedNotification(req types.AccessRequest, params types.AccessReviewSubmission) *notificationsv1.Notification {
	var subKind string
	var reviewVerb string

	if req.GetState().IsApproved() {
		subKind = types.NotificationAccessRequestApprovedSubKind
		reviewVerb = "approved"
	} else if req.GetState().IsPromoted() {
		subKind = types.NotificationAccessRequestPromotedSubKind
	} else {
		subKind = types.NotificationAccessRequestDeniedSubKind
		reviewVerb = "denied"
	}

	var notificationText string
	if req.GetState().IsPromoted() {
		notificationText = fmt.Sprintf("%s promoted your access request to long-term access.", params.Review.Author)
	} else {
		// If this was a resource request.
		if len(req.GetRequestedResourceIDs()) > 0 {
			notificationText = fmt.Sprintf("%s %s your access request for %d resources.", params.Review.Author, reviewVerb, len(req.GetRequestedResourceIDs()))
			if len(req.GetRequestedResourceIDs()) == 1 {
				notificationText = fmt.Sprintf("%s %s your access request for a resource.", params.Review.Author, reviewVerb)
			}
			// If this was a role request.
		} else {
			notificationText = fmt.Sprintf("%s %s your access request for the '%s' role.", params.Review.Author, reviewVerb, req.GetRoles()[0])
			if len(req.GetRoles()) > 1 {
				notificationText = fmt.Sprintf("%s %s your access request for %d roles.", params.Review.Author, reviewVerb, len(req.GetRoles()))
			}
		}
	}

	assumableTime := ""
	if req.GetAssumeStartTime() != nil {
		assumableTime = req.GetAssumeStartTime().Format("2006-01-02T15:04:05.000Z0700")
	}

	return &notificationsv1.Notification{
		Spec: &notificationsv1.NotificationSpec{
			Username: req.GetUser(),
		},
		SubKind: subKind,
		Metadata: &headerv1.Metadata{
			Labels: map[string]string{
				types.NotificationTitleLabel: notificationText,
				"request-id":                 params.RequestID,
				"roles":                      strings.Join(req.GetRoles(), ","),
				"assumable-time":             assumableTime,
			},
			Expires: timestamppb.New(req.Expiry()),
		},
	}
}

func (a *Server) GetAccessCapabilities(ctx context.Context, req types.AccessCapabilitiesRequest) (*types.AccessCapabilities, error) {
	user, err := authz.UserFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caps, err := services.CalculateAccessCapabilities(ctx, a.clock, a, user.GetIdentity(), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return caps, nil
}

func (a *Server) getCache() (c *cache.Cache, ok bool) {
	c, ok = a.Cache.(*cache.Cache)
	return
}

func (a *Server) NewStream(ctx context.Context, watch types.Watch) (stream.Stream[types.Event], error) {
	if cache, ok := a.getCache(); ok {
		// cache exposes a native stream implementation
		return cache.NewStream(ctx, watch)
	}

	// fallback to wrapping a watcher in a stream.Stream adapter
	watcher, err := a.Cache.NewWatcher(ctx, watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	closer := func() {
		watcher.Close()
	}

	return stream.Func(func() (types.Event, error) {
		select {
		case event := <-watcher.Events():
			return event, nil
		case <-watcher.Done():
			err := watcher.Error()
			if err == nil {
				// stream.Func needs an error to signal end of stream. io.EOF is
				// the expected "happy" end of stream singnal.
				err = io.EOF
			}
			return types.Event{}, trace.Wrap(err)
		}
	}, closer), nil
}

// NewKeepAliver returns a new instance of keep aliver
func (a *Server) NewKeepAliver(ctx context.Context) (types.KeepAliver, error) {
	cancelCtx, cancel := context.WithCancel(ctx)
	k := &authKeepAliver{
		a:           a,
		ctx:         cancelCtx,
		cancel:      cancel,
		keepAlivesC: make(chan types.KeepAlive),
	}
	go k.forwardKeepAlives()
	return k, nil
}

// KeepAliveServer implements [services.Presence] by delegating to
// [Server.Services] and potentially emitting a [usagereporter] event.
func (a *Server) KeepAliveServer(ctx context.Context, h types.KeepAlive) error {
	if err := a.Services.KeepAliveServer(ctx, h); err != nil {
		return trace.Wrap(err)
	}

	// ResourceHeartbeatEvent only cares about a few KeepAlive types
	kind := usagereporter.ResourceKindFromKeepAliveType(h.Type)
	if kind == 0 {
		return nil
	}
	a.AnonymizeAndSubmit(&usagereporter.ResourceHeartbeatEvent{
		Name:   h.Name,
		Kind:   kind,
		Static: h.Expires.IsZero(),
	})

	return nil
}

const (
	serverHostnameMaxLen       = 256
	serverHostnameRegexPattern = `^[a-zA-Z0-9]+[a-zA-Z0-9\.-]*$`
	replacedHostnameLabel      = types.TeleportInternalLabelPrefix + "invalid-hostname"
)

var serverHostnameRegex = regexp.MustCompile(serverHostnameRegexPattern)

// validServerHostname returns false if the hostname is longer than 256 characters or
// does not entirely consist of alphanumeric characters as well as '-' and '.'. A valid hostname also
// cannot begin with a symbol.
func validServerHostname(hostname string) bool {
	return len(hostname) <= serverHostnameMaxLen && serverHostnameRegex.MatchString(hostname)
}

func sanitizeHostname(server types.Server) error {
	invalidHostname := server.GetHostname()

	replacedHostname := server.GetName()
	if server.GetSubKind() == types.SubKindOpenSSHNode {
		host, _, err := net.SplitHostPort(server.GetAddr())
		if err != nil || !validServerHostname(host) {
			id, err := uuid.NewRandom()
			if err != nil {
				return trace.Wrap(err)
			}

			host = id.String()
		}

		replacedHostname = host
	}

	switch s := server.(type) {
	case *types.ServerV2:
		s.Spec.Hostname = replacedHostname

		if s.Metadata.Labels == nil {
			s.Metadata.Labels = map[string]string{}
		}

		s.Metadata.Labels[replacedHostnameLabel] = invalidHostname
	default:
		return trace.BadParameter("invalid server provided")
	}

	return nil
}

// restoreSanitizedHostname restores the original hostname of a server and removes the label.
func restoreSanitizedHostname(server types.Server) error {
	oldHostname, ok := server.GetLabels()[replacedHostnameLabel]
	// if the label is not present or the hostname is invalid under the most recent rules, do nothing.
	if !ok || !validServerHostname(oldHostname) {
		return nil
	}

	switch s := server.(type) {
	case *types.ServerV2:
		// restore the original hostname and remove the label.
		s.Spec.Hostname = oldHostname
		delete(s.Metadata.Labels, replacedHostnameLabel)
	default:
		return trace.BadParameter("invalid server provided")
	}

	return nil
}

// UpsertNode implements [services.Presence] by delegating to [Server.Services]
// and potentially emitting a [usagereporter] event.
func (a *Server) UpsertNode(ctx context.Context, server types.Server) (*types.KeepAlive, error) {
	if !validServerHostname(server.GetHostname()) {
		a.logger.DebugContext(a.closeCtx, "sanitizing invalid server hostname",
			"server", server.GetName(),
			"hostname", server.GetHostname(),
		)
		if err := sanitizeHostname(server); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	lease, err := a.Services.UpsertNode(ctx, server)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	kind := usagereporter.ResourceKindNode
	switch server.GetSubKind() {
	case types.SubKindOpenSSHNode:
		kind = usagereporter.ResourceKindNodeOpenSSH
	case types.SubKindOpenSSHEICENode:
		kind = usagereporter.ResourceKindNodeOpenSSHEICE
	}

	a.AnonymizeAndSubmit(&usagereporter.ResourceHeartbeatEvent{
		Name:   server.GetName(),
		Kind:   kind,
		Static: server.Expiry().IsZero(),
	})

	return lease, nil
}

// enforceLicense checks if the license allows the given resource type to be
// created.
func enforceLicense(t string) error {
	switch t {
	case types.KindKubeServer, types.KindKubernetesCluster:
		if !modules.GetModules().Features().GetEntitlement(entitlements.K8s).Enabled {
			return trace.AccessDenied(
				"this Teleport cluster is not licensed for Kubernetes, please contact the cluster administrator")
		}
	}
	return nil
}

// UpsertKubernetesServer implements [services.Presence] by delegating to
// [Server.Services] and then potentially emitting a [usagereporter] event.
func (a *Server) UpsertKubernetesServer(ctx context.Context, server types.KubeServer) (*types.KeepAlive, error) {
	if err := enforceLicense(types.KindKubeServer); err != nil {
		return nil, trace.Wrap(err)
	}

	k, err := a.Services.UpsertKubernetesServer(ctx, server)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	a.AnonymizeAndSubmit(&usagereporter.ResourceHeartbeatEvent{
		// the name of types.KubeServer might include a -proxy_service suffix
		Name:   server.GetCluster().GetName(),
		Kind:   usagereporter.ResourceKindKubeServer,
		Static: server.Expiry().IsZero(),
	})

	return k, nil
}

// UpsertApplicationServer implements [services.Presence] by delegating to
// [Server.Services] and then potentially emitting a [usagereporter] event.
func (a *Server) UpsertApplicationServer(ctx context.Context, server types.AppServer) (*types.KeepAlive, error) {
	lease, err := a.Services.UpsertApplicationServer(ctx, server)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	a.AnonymizeAndSubmit(&usagereporter.ResourceHeartbeatEvent{
		Name:   server.GetName(),
		Kind:   usagereporter.ResourceKindAppServer,
		Static: server.Expiry().IsZero(),
	})

	return lease, nil
}

// UpsertDatabaseServer implements [services.Presence] by delegating to
// [Server.Services] and then potentially emitting a [usagereporter] event.
func (a *Server) UpsertDatabaseServer(ctx context.Context, server types.DatabaseServer) (*types.KeepAlive, error) {
	lease, err := a.Services.UpsertDatabaseServer(ctx, server)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	a.AnonymizeAndSubmit(&usagereporter.ResourceHeartbeatEvent{
		Name:   server.GetName(),
		Kind:   usagereporter.ResourceKindDBServer,
		Static: server.Expiry().IsZero(),
	})

	return lease, nil
}

func (a *Server) DeleteWindowsDesktop(ctx context.Context, hostID, name string) error {
	if err := a.Services.DeleteWindowsDesktop(ctx, hostID, name); err != nil {
		return trace.Wrap(err)
	}
	if _, err := a.desktopsLimitExceeded(ctx); err != nil {
		log.Warnf("Can't check OSS non-AD desktops limit: %v", err)
	}
	return nil
}

// CreateWindowsDesktop implements [services.WindowsDesktops] by delegating to
// [Server.Services] and then potentially emitting a [usagereporter] event.
func (a *Server) CreateWindowsDesktop(ctx context.Context, desktop types.WindowsDesktop) error {
	if err := a.Services.CreateWindowsDesktop(ctx, desktop); err != nil {
		return trace.Wrap(err)
	}

	a.AnonymizeAndSubmit(&usagereporter.ResourceHeartbeatEvent{
		Name:   desktop.GetName(),
		Kind:   usagereporter.ResourceKindWindowsDesktop,
		Static: desktop.Expiry().IsZero(),
	})

	return nil
}

// UpdateWindowsDesktop implements [services.WindowsDesktops] by delegating to
// [Server.Services] and then potentially emitting a [usagereporter] event.
func (a *Server) UpdateWindowsDesktop(ctx context.Context, desktop types.WindowsDesktop) error {
	if err := a.Services.UpdateWindowsDesktop(ctx, desktop); err != nil {
		return trace.Wrap(err)
	}

	a.AnonymizeAndSubmit(&usagereporter.ResourceHeartbeatEvent{
		Name:   desktop.GetName(),
		Kind:   usagereporter.ResourceKindWindowsDesktop,
		Static: desktop.Expiry().IsZero(),
	})

	return nil
}

// UpsertWindowsDesktop implements [services.WindowsDesktops] by delegating to
// [Server.Services] and then potentially emitting a [usagereporter] event.
func (a *Server) UpsertWindowsDesktop(ctx context.Context, desktop types.WindowsDesktop) error {
	if err := a.Services.UpsertWindowsDesktop(ctx, desktop); err != nil {
		return trace.Wrap(err)
	}

	a.AnonymizeAndSubmit(&usagereporter.ResourceHeartbeatEvent{
		Name:   desktop.GetName(),
		Kind:   usagereporter.ResourceKindWindowsDesktop,
		Static: desktop.Expiry().IsZero(),
	})

	return nil
}

func (a *Server) streamWindowsDesktops(ctx context.Context, startKey string) stream.Stream[types.WindowsDesktop] {
	var done bool
	return stream.PageFunc(func() ([]types.WindowsDesktop, error) {
		if done {
			return nil, io.EOF
		}
		resp, err := a.ListWindowsDesktops(ctx, types.ListWindowsDesktopsRequest{
			Limit:    50,
			StartKey: startKey,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		startKey = resp.NextKey
		done = startKey == ""
		return resp.Desktops, nil
	})
}

func (a *Server) syncDesktopsLimitAlert(ctx context.Context) {
	exceeded, err := a.desktopsLimitExceeded(ctx)
	if err != nil {
		log.Warnf("Can't check OSS non-AD desktops limit: %v", err)
	}
	if !exceeded {
		return
	}
	alert, err := types.NewClusterAlert(OSSDesktopsAlertID, OSSDesktopsAlertMessage,
		types.WithAlertSeverity(types.AlertSeverity_MEDIUM),
		types.WithAlertLabel(types.AlertOnLogin, "yes"),
		types.WithAlertLabel(types.AlertPermitAll, "yes"),
		types.WithAlertLabel(types.AlertLink, OSSDesktopsAlertLink),
		types.WithAlertLabel(types.AlertLinkText, OSSDesktopsAlertLinkText),
		types.WithAlertExpires(time.Now().Add(OSSDesktopsCheckPeriod)))
	if err != nil {
		log.Warnf("Can't create OSS non-AD desktops limit alert: %v", err)
	}
	if err := a.UpsertClusterAlert(ctx, alert); err != nil {
		log.Warnf("Can't upsert OSS non-AD desktops limit alert: %v", err)
	}
}

// desktopsLimitExceeded checks if number of non-AD desktops exceeds limit for OSS distribution. Returns always false for Enterprise.
func (a *Server) desktopsLimitExceeded(ctx context.Context) (bool, error) {
	if modules.GetModules().IsEnterpriseBuild() {
		return false, nil
	}

	desktops := stream.FilterMap(
		a.streamWindowsDesktops(ctx, ""),
		func(d types.WindowsDesktop) (struct{}, bool) {
			return struct{}{}, d.NonAD()
		},
	)
	count := 0
	for desktops.Next() {
		count++
		if count > OSSDesktopsLimit {
			desktops.Done()
			return true, nil
		}
	}
	return false, trace.Wrap(desktops.Done())
}

func (a *Server) syncDynamicLabelsAlert(ctx context.Context) {
	roles, err := a.GetRoles(ctx)
	if err != nil {
		log.Warnf("Can't get roles: %v", err)
	}
	var rolesWithDynamicDenyLabels bool
	for _, role := range roles {
		err := services.CheckDynamicLabelsInDenyRules(role)
		if trace.IsBadParameter(err) {
			rolesWithDynamicDenyLabels = true
			break
		}
		if err != nil {
			log.Warnf("Error checking labels in role %s: %v", role.GetName(), err)
			continue
		}
	}
	if !rolesWithDynamicDenyLabels {
		return
	}
	alert, err := types.NewClusterAlert(
		dynamicLabelAlertID,
		dynamicLabelAlertMessage,
		types.WithAlertSeverity(types.AlertSeverity_MEDIUM),
		types.WithAlertLabel(types.AlertVerbPermit, fmt.Sprintf("%s:%s", types.KindRole, types.VerbRead)),
	)
	if err != nil {
		log.Warnf("Failed to build %s alert: %v (this is a bug)", dynamicLabelAlertID, err)
	}
	if err := a.UpsertClusterAlert(ctx, alert); err != nil {
		log.Warnf("Failed to set %s alert: %v", dynamicLabelAlertID, err)
	}
}

// CleanupNotifications deletes all expired user notifications and global notifications, as well as any associated notification states, for all users.
func (a *Server) CleanupNotifications(ctx context.Context) {
	var userNotifications []*notificationsv1.Notification
	var userNotificationsPageKey string
	userNotificationsReadLimiter := time.NewTicker(notificationsPageReadInterval)
	defer userNotificationsReadLimiter.Stop()
	for {
		select {
		case <-userNotificationsReadLimiter.C:
		case <-ctx.Done():
			return
		}
		response, nextKey, err := a.Cache.ListUserNotifications(ctx, 20, userNotificationsPageKey)
		if err != nil {
			slog.WarnContext(ctx, "failed to list user notifications for periodic cleanup", "error", err)
		}
		userNotifications = append(userNotifications, response...)
		if nextKey == "" {
			break
		}
		userNotificationsPageKey = nextKey
	}

	var globalNotifications []*notificationsv1.GlobalNotification
	var globalNotificationsPageKey string
	globalNotificationsReadLimiter := time.NewTicker(notificationsPageReadInterval)
	defer globalNotificationsReadLimiter.Stop()
	for {
		select {
		case <-globalNotificationsReadLimiter.C:
		case <-ctx.Done():
			return
		}
		response, nextKey, err := a.Cache.ListGlobalNotifications(ctx, 20, globalNotificationsPageKey)
		if err != nil {
			slog.WarnContext(ctx, "failed to list global notifications for periodic cleanup", "error", err)
		}
		globalNotifications = append(globalNotifications, response...)
		if nextKey == "" {
			break
		}
		globalNotificationsPageKey = nextKey
	}

	timeNow := a.clock.Now()

	notificationsDeleteLimiter := time.NewTicker(notificationsWriteInterval)
	defer notificationsDeleteLimiter.Stop()

	// Initialize a map for non-expired notifications where the key is the notification id.
	nonExpiredGlobalNotificationsByID := make(map[string]*notificationsv1.GlobalNotification)
	for _, gn := range globalNotifications {
		notificationID := gn.GetMetadata().GetName()
		expiry := gn.GetSpec().GetNotification().GetMetadata().GetExpires()

		if timeNow.After(expiry.AsTime()) {
			select {
			case <-notificationsDeleteLimiter.C:
			case <-ctx.Done():
				return
			}
			if err := a.DeleteGlobalNotification(ctx, notificationID); err != nil && !trace.IsNotFound(err) {
				slog.WarnContext(ctx, "encountered error attempting to cleanup global notification", "error", err, "notification_id", notificationID)
			}
		} else {
			nonExpiredGlobalNotificationsByID[notificationID] = gn
		}
	}

	// Initialize a map for non-expired notifications where the key is the notification id.
	nonExpiredUserNotificationsByID := make(map[string]*notificationsv1.Notification)
	for _, un := range userNotifications {
		notificationID := un.GetMetadata().GetName()
		user := un.GetSpec().GetUsername()
		expiry := un.GetMetadata().GetExpires()

		if timeNow.After(expiry.AsTime()) {
			select {
			case <-notificationsDeleteLimiter.C:
			case <-ctx.Done():
				return
			}
			if err := a.DeleteUserNotification(ctx, user, notificationID); err != nil && !trace.IsNotFound(err) {
				slog.WarnContext(ctx, "encountered error attempting to cleanup user notification", "error", err, "notification_id", notificationID, "target_user", user)
			}
		} else {
			nonExpiredUserNotificationsByID[notificationID] = un
		}
	}

	var userNotificationStates []*notificationsv1.UserNotificationState
	var userNotificationStatesPageKey string
	notificationStatesTicker := time.NewTicker(notificationsPageReadInterval)
	defer notificationStatesTicker.Stop()
	for {
		select {
		case <-notificationStatesTicker.C:
		case <-ctx.Done():
			return
		}
		response, nextKey, err := a.ListNotificationStatesForAllUsers(ctx, 20, userNotificationStatesPageKey)
		if err != nil {
			slog.WarnContext(ctx, "encountered error attempting to list notification states for cleanup", "error", err)
		}
		userNotificationStates = append(userNotificationStates, response...)
		if nextKey == "" {
			break
		}
		userNotificationStatesPageKey = nextKey
	}

	for _, uns := range userNotificationStates {
		id := uns.GetSpec().GetNotificationId()
		username := uns.GetSpec().GetUsername()

		// If this notification state is for a notification which doesn't exist in either the non-expired global notifications map or
		// the non-expired user notifications map, then delete it.
		if nonExpiredGlobalNotificationsByID[id] == nil && nonExpiredUserNotificationsByID[id] == nil {
			select {
			case <-notificationsDeleteLimiter.C:
			case <-ctx.Done():
				return
			}
			if err := a.DeleteUserNotificationState(ctx, username, id); err != nil {
				slog.WarnContext(ctx, "encountered error attempting to cleanup notification state", "error", err, "user", username, "id", id)
			}
		}
	}
}

// GenerateCertAuthorityCRL generates an empty CRL for the local CA of a given type.
func (a *Server) GenerateCertAuthorityCRL(ctx context.Context, caType types.CertAuthType) ([]byte, error) {
	// Generate a CRL for the current cluster CA.
	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := a.GetCertAuthority(ctx, types.CertAuthID{
		Type:       caType,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(awly): this will only create a CRL for an active signer.
	// If there are multiple signers (multiple HSMs), we won't have the full CRL coverage.
	// Generate a CRL per signer and return all of them separately.

	cert, signer, err := a.keyStore.GetTLSCertAndSigner(ctx, ca)
	if trace.IsNotFound(err) {
		// If there is no local TLS signer found in the host CA ActiveKeys, this
		// auth server may have a newly configured HSM and has only populated
		// local keys in the AdditionalTrustedKeys until the next CA rotation.
		// This is the only case where we should be able to get a signer from
		// AdditionalTrustedKeys but not ActiveKeys.
		cert, signer, err = a.keyStore.GetAdditionalTrustedTLSCertAndSigner(ctx, ca)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsAuthority, err := tlsca.FromCertAndSigner(cert, signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Empty CRL valid for 1yr.
	template := &x509.RevocationList{
		Number:     big.NewInt(1),
		ThisUpdate: time.Now().Add(-1 * time.Minute), // 1 min in the past to account for clock skew.
		NextUpdate: time.Now().Add(365 * 24 * time.Hour),
	}
	crl, err := x509.CreateRevocationList(rand.Reader, template, tlsAuthority.Cert, tlsAuthority.Signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return crl, nil
}

// ErrDone indicates that resource iteration is complete
var ErrDone = errors.New("done iterating")

// IterateResources loads all resources matching the provided request and passes them one by one to the provided
// callback function. To stop iteration callers may return ErrDone from the callback function, which will result in
// a nil return from IterateResources. Any other errors returned from the callback function cause iteration to stop
// and the error to be returned.
func (a *Server) IterateResources(ctx context.Context, req proto.ListResourcesRequest, f func(resource types.ResourceWithLabels) error) error {
	for {
		resp, err := a.ListResources(ctx, req)
		if err != nil {
			return trace.Wrap(err)
		}

		for _, resource := range resp.Resources {
			if err := f(resource); err != nil {
				if errors.Is(err, ErrDone) {
					return nil
				}
				return trace.Wrap(err)
			}
		}

		if resp.NextKey == "" {
			return nil
		}

		req.StartKey = resp.NextKey
	}
}

// CreateApp creates a new application resource.
func (a *Server) CreateApp(ctx context.Context, app types.Application) error {
	if err := a.Services.CreateApp(ctx, app); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.AppCreate{
		Metadata: apievents.Metadata{
			Type: events.AppCreateEvent,
			Code: events.AppCreateCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    app.GetName(),
			Expires: app.Expiry(),
		},
		AppMetadata: apievents.AppMetadata{
			AppURI:        app.GetURI(),
			AppPublicAddr: app.GetPublicAddr(),
			AppLabels:     app.GetStaticLabels(),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit app create event.")
	}
	return nil
}

// UpdateApp updates an existing application resource.
func (a *Server) UpdateApp(ctx context.Context, app types.Application) error {
	if err := a.Services.UpdateApp(ctx, app); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.AppUpdate{
		Metadata: apievents.Metadata{
			Type: events.AppUpdateEvent,
			Code: events.AppUpdateCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    app.GetName(),
			Expires: app.Expiry(),
		},
		AppMetadata: apievents.AppMetadata{
			AppURI:        app.GetURI(),
			AppPublicAddr: app.GetPublicAddr(),
			AppLabels:     app.GetStaticLabels(),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit app update event.")
	}
	return nil
}

// DeleteApp deletes an application resource.
func (a *Server) DeleteApp(ctx context.Context, name string) error {
	if err := a.Services.DeleteApp(ctx, name); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.AppDelete{
		Metadata: apievents.Metadata{
			Type: events.AppDeleteEvent,
			Code: events.AppDeleteCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: name,
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit app delete event.")
	}
	return nil
}

// CreateSessionTracker creates a tracker resource for an active session.
func (a *Server) CreateSessionTracker(ctx context.Context, tracker types.SessionTracker) (types.SessionTracker, error) {
	// Don't allow sessions that require moderation without the enterprise feature enabled.
	for _, policySet := range tracker.GetHostPolicySets() {
		if len(policySet.RequireSessionJoin) != 0 {
			if modules.GetModules().BuildType() != modules.BuildEnterprise {
				return nil, fmt.Errorf("Moderated Sessions: %w", ErrRequiresEnterprise)
			}
		}
	}

	return a.Services.CreateSessionTracker(ctx, tracker)
}

// CreateDatabase creates a new database resource.
func (a *Server) CreateDatabase(ctx context.Context, database types.Database) error {
	if err := a.Services.CreateDatabase(ctx, database); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.DatabaseCreate{
		Metadata: apievents.Metadata{
			Type: events.DatabaseCreateEvent,
			Code: events.DatabaseCreateCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    database.GetName(),
			Expires: database.Expiry(),
		},
		DatabaseMetadata: apievents.DatabaseMetadata{
			DatabaseProtocol:             database.GetProtocol(),
			DatabaseURI:                  database.GetURI(),
			DatabaseLabels:               database.GetStaticLabels(),
			DatabaseAWSRegion:            database.GetAWS().Region,
			DatabaseAWSRedshiftClusterID: database.GetAWS().Redshift.ClusterID,
			DatabaseGCPProjectID:         database.GetGCP().ProjectID,
			DatabaseGCPInstanceID:        database.GetGCP().InstanceID,
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit database create event.")
	}
	return nil
}

// UpdateDatabase updates an existing database resource.
func (a *Server) UpdateDatabase(ctx context.Context, database types.Database) error {
	if err := a.Services.UpdateDatabase(ctx, database); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.DatabaseUpdate{
		Metadata: apievents.Metadata{
			Type: events.DatabaseUpdateEvent,
			Code: events.DatabaseUpdateCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    database.GetName(),
			Expires: database.Expiry(),
		},
		DatabaseMetadata: apievents.DatabaseMetadata{
			DatabaseProtocol:             database.GetProtocol(),
			DatabaseURI:                  database.GetURI(),
			DatabaseLabels:               database.GetStaticLabels(),
			DatabaseAWSRegion:            database.GetAWS().Region,
			DatabaseAWSRedshiftClusterID: database.GetAWS().Redshift.ClusterID,
			DatabaseGCPProjectID:         database.GetGCP().ProjectID,
			DatabaseGCPInstanceID:        database.GetGCP().InstanceID,
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit database update event.")
	}
	return nil
}

// DeleteDatabase deletes a database resource.
func (a *Server) DeleteDatabase(ctx context.Context, name string) error {
	if err := a.Services.DeleteDatabase(ctx, name); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.DatabaseDelete{
		Metadata: apievents.Metadata{
			Type: events.DatabaseDeleteEvent,
			Code: events.DatabaseDeleteCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: name,
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit database delete event.")
	}
	return nil
}

// ListResources returns paginated resources depending on the resource type..
func (a *Server) ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	// Because WindowsDesktopService does not contain the desktop resources,
	// this is not implemented at the cache level and requires the workaround
	// here in order to support KindWindowsDesktop for ListResources.
	if req.ResourceType == types.KindWindowsDesktop {
		wResp, err := a.ListWindowsDesktops(ctx, types.ListWindowsDesktopsRequest{
			WindowsDesktopFilter: req.WindowsDesktopFilter,
			Limit:                int(req.Limit),
			StartKey:             req.StartKey,
			PredicateExpression:  req.PredicateExpression,
			Labels:               req.Labels,
			SearchKeywords:       req.SearchKeywords,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &types.ListResourcesResponse{
			Resources: types.WindowsDesktops(wResp.Desktops).AsResources(),
			NextKey:   wResp.NextKey,
		}, nil
	}
	if req.ResourceType == types.KindWindowsDesktopService {
		wResp, err := a.ListWindowsDesktopServices(ctx, types.ListWindowsDesktopServicesRequest{
			Limit:               int(req.Limit),
			StartKey:            req.StartKey,
			PredicateExpression: req.PredicateExpression,
			Labels:              req.Labels,
			SearchKeywords:      req.SearchKeywords,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &types.ListResourcesResponse{
			Resources: types.WindowsDesktopServices(wResp.DesktopServices).AsResources(),
			NextKey:   wResp.NextKey,
		}, nil
	}
	return a.Cache.ListResources(ctx, req)
}

// CreateKubernetesCluster creates a new kubernetes cluster resource.
func (a *Server) CreateKubernetesCluster(ctx context.Context, kubeCluster types.KubeCluster) error {
	if err := enforceLicense(types.KindKubernetesCluster); err != nil {
		return trace.Wrap(err)
	}
	if err := a.Services.CreateKubernetesCluster(ctx, kubeCluster); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.KubernetesClusterCreate{
		Metadata: apievents.Metadata{
			Type: events.KubernetesClusterCreateEvent,
			Code: events.KubernetesClusterCreateCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    kubeCluster.GetName(),
			Expires: kubeCluster.Expiry(),
		},
		KubeClusterMetadata: apievents.KubeClusterMetadata{
			KubeLabels: kubeCluster.GetStaticLabels(),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit kube cluster create event.")
	}
	return nil
}

// UpdateKubernetesCluster updates an existing kubernetes cluster resource.
func (a *Server) UpdateKubernetesCluster(ctx context.Context, kubeCluster types.KubeCluster) error {
	if err := enforceLicense(types.KindKubernetesCluster); err != nil {
		return trace.Wrap(err)
	}
	if err := a.Kubernetes.UpdateKubernetesCluster(ctx, kubeCluster); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.KubernetesClusterUpdate{
		Metadata: apievents.Metadata{
			Type: events.KubernetesClusterUpdateEvent,
			Code: events.KubernetesClusterUpdateCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    kubeCluster.GetName(),
			Expires: kubeCluster.Expiry(),
		},
		KubeClusterMetadata: apievents.KubeClusterMetadata{
			KubeLabels: kubeCluster.GetStaticLabels(),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit kube cluster update event.")
	}
	return nil
}

// DeleteKubernetesCluster deletes a kubernetes cluster resource.
func (a *Server) DeleteKubernetesCluster(ctx context.Context, name string) error {
	if err := a.Kubernetes.DeleteKubernetesCluster(ctx, name); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.KubernetesClusterDelete{
		Metadata: apievents.Metadata{
			Type: events.KubernetesClusterDeleteEvent,
			Code: events.KubernetesClusterDeleteCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: name,
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit kube cluster delete event.")
	}
	return nil
}

// SubmitUsageEvent submits an external usage event.
func (a *Server) SubmitUsageEvent(ctx context.Context, req *proto.SubmitUsageEventRequest) error {
	username, err := authz.GetClientUsername(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	userIsSSO, err := authz.GetClientUserIsSSO(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	userMetadata := usagereporter.UserMetadata{
		Username: username,
		IsSSO:    userIsSSO,
	}

	event, err := usagereporter.ConvertUsageEvent(req.GetEvent(), userMetadata)
	if err != nil {
		return trace.Wrap(err)
	}

	a.AnonymizeAndSubmit(event)

	return nil
}

// Ping gets basic info about the auth server.
// Please note that Ping is publicly accessible (not protected by any RBAC) by design,
// and thus PingResponse must never contain any sensitive information.
func (a *Server) Ping(ctx context.Context) (proto.PingResponse, error) {
	cn, err := a.GetClusterName()
	if err != nil {
		return proto.PingResponse{}, trace.Wrap(err)
	}
	features := modules.GetModules().Features().ToProto()

	authPref, err := a.GetAuthPreference(ctx)
	if err != nil {
		return proto.PingResponse{}, nil
	}

	licenseExpiry := modules.GetModules().LicenseExpiry()

	return proto.PingResponse{
		ClusterName:             cn.GetClusterName(),
		ServerVersion:           teleport.Version,
		ServerFeatures:          features,
		ProxyPublicAddr:         a.getProxyPublicAddr(),
		IsBoring:                modules.GetModules().IsBoringBinary(),
		LoadAllCAs:              a.loadAllCAs,
		SignatureAlgorithmSuite: authPref.GetSignatureAlgorithmSuite(),
		LicenseExpiry:           &licenseExpiry,
	}, nil
}

type maintenanceWindowCacheKey struct {
	key string
}

// agentWindowLookahead is the number of upgrade windows, starting from 'today', that we export
// when compiling agent upgrade schedules. The choice is arbitrary. We must export at least 2, because upgraders
// treat a schedule value whose windows all end in the past to be stale and therefore a sign that the agent is
// unhealthy. 3 was picked to give us some leeway in terms of how long an agent can be turned off before its
// upgrader starts complaining of a stale schedule.
const agentWindowLookahead = 3

// exportUpgradeWindowsCached generates the export value of all upgrade window schedule types. Since schedules
// are reloaded frequently in large clusters and export incurs string/json encoding, we use the ttl cache to store
// the encoded schedule values for a few seconds.
func (a *Server) exportUpgradeWindowsCached(ctx context.Context) (proto.ExportUpgradeWindowsResponse, error) {
	return utils.FnCacheGet(ctx, a.ttlCache, maintenanceWindowCacheKey{"export"}, func(ctx context.Context) (proto.ExportUpgradeWindowsResponse, error) {
		var rsp proto.ExportUpgradeWindowsResponse
		cmc, err := a.GetClusterMaintenanceConfig(ctx)
		if err != nil {
			if trace.IsNotFound(err) {
				// "not found" is treated as an empty schedule value
				return rsp, nil
			}
			return rsp, trace.Wrap(err)
		}

		agentWindow, ok := cmc.GetAgentUpgradeWindow()
		if !ok {
			// "unconfigured" is treated as an empty schedule value
			return rsp, nil
		}

		sched := agentWindow.Export(time.Now(), agentWindowLookahead)

		rsp.CanonicalSchedule = &sched

		rsp.KubeControllerSchedule, err = uw.EncodeKubeControllerSchedule(sched)
		if err != nil {
			log.Warnf("Failed to encode kube controller maintenance schedule: %v", err)
		}

		rsp.SystemdUnitSchedule, err = uw.EncodeSystemdUnitSchedule(sched)
		if err != nil {
			log.Warnf("Failed to encode systemd unit maintenance schedule: %v", err)
		}

		return rsp, nil
	})
}

func (a *Server) ExportUpgradeWindows(ctx context.Context, req proto.ExportUpgradeWindowsRequest) (proto.ExportUpgradeWindowsResponse, error) {
	var rsp proto.ExportUpgradeWindowsResponse

	// get the cached collection of all export values
	cached, err := a.exportUpgradeWindowsCached(ctx)
	if err != nil {
		return rsp, nil
	}

	switch req.UpgraderKind {
	case "":
		rsp.CanonicalSchedule = cached.CanonicalSchedule.Clone()
	case types.UpgraderKindKubeController:
		rsp.KubeControllerSchedule = cached.KubeControllerSchedule

		if sched := os.Getenv("TELEPORT_UNSTABLE_KUBE_UPGRADE_SCHEDULE"); sched != "" {
			rsp.KubeControllerSchedule = sched
		}
	case types.UpgraderKindSystemdUnit:
		rsp.SystemdUnitSchedule = cached.SystemdUnitSchedule

		if sched := os.Getenv("TELEPORT_UNSTABLE_SYSTEMD_UPGRADE_SCHEDULE"); sched != "" {
			rsp.SystemdUnitSchedule = sched
		}
	default:
		return rsp, trace.NotImplemented("unsupported upgrader kind %q in upgrade window export request", req.UpgraderKind)
	}

	return rsp, nil
}

// MFARequiredToBool translates a [proto.MFARequired] value to a simple
// "required bool".
func MFARequiredToBool(m proto.MFARequired) (required bool) {
	switch m {
	case proto.MFARequired_MFA_REQUIRED_NO:
		return false
	default: // _UNSPECIFIED or _YES are both treated as required.
		return true
	}
}

func (a *Server) isMFARequired(ctx context.Context, checker services.AccessChecker, req *proto.IsMFARequiredRequest) (resp *proto.IsMFARequiredResponse, err error) {
	// Assign Required as a function of MFARequired.
	defer func() {
		if resp != nil {
			resp.Required = MFARequiredToBool(resp.MFARequired)
		}
	}()

	authPref, err := a.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch state := checker.GetAccessState(authPref); state.MFARequired {
	case services.MFARequiredAlways:
		return &proto.IsMFARequiredResponse{
			MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
		}, nil
	case services.MFARequiredNever:
		return &proto.IsMFARequiredResponse{
			MFARequired: proto.MFARequired_MFA_REQUIRED_NO,
		}, nil
	}

	var noMFAAccessErr error
	switch t := req.Target.(type) {
	case *proto.IsMFARequiredRequest_Node:
		if t.Node.Node == "" {
			return nil, trace.BadParameter("empty Node field")
		}
		if t.Node.Login == "" {
			return nil, trace.BadParameter("empty Login field")
		}

		// state.MFARequired is "per-role", so if the user is joining
		// a session, MFA is required no matter what node they are
		// connecting to. We don't preform an RBAC check like we do
		// below when users are starting a session to selectively
		// require MFA because we don't know what session the user
		// is joining, nor do we know what role allowed the session
		// creator to start the session that is attempting to be joined.
		// We need this info to be able to selectively skip MFA in
		// this case.
		if t.Node.Login == teleport.SSHSessionJoinPrincipal {
			return &proto.IsMFARequiredResponse{
				MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
			}, nil
		}

		// Find the target node and check whether MFA is required.
		matches, err := client.GetResourcesWithFilters(ctx, a, proto.ListResourcesRequest{
			ResourceType:   types.KindNode,
			Namespace:      apidefaults.Namespace,
			SearchKeywords: []string{t.Node.Node},
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if len(matches) == 0 {
			// If t.Node.Node is not a known registered node, it may be an
			// unregistered host running OpenSSH with a certificate created via
			// `tctl auth sign`. In these cases, let the user through without
			// extra checks.
			//
			// If t.Node.Node turns out to be an alias for a real node (e.g.
			// private network IP), and MFA check was actually required, the
			// Node itself will check the cert extensions and reject the
			// connection.
			return &proto.IsMFARequiredResponse{
				MFARequired: proto.MFARequired_MFA_REQUIRED_NO,
			}, nil
		}

		// Check RBAC against all matching nodes and return the first error.
		// If at least one node requires MFA, we'll catch it.
		for _, n := range matches {
			srv, ok := n.(types.Server)
			if !ok {
				continue
			}

			// Filter out any matches on labels before checking access
			fieldVals := append(srv.GetPublicAddrs(), srv.GetName(), srv.GetHostname(), srv.GetAddr())
			if !types.MatchSearch(fieldVals, []string{t.Node.Node}, nil) {
				continue
			}

			err = checker.CheckAccess(
				n,
				services.AccessState{},
				services.NewLoginMatcher(t.Node.Login),
			)

			// Ignore other errors; they'll be caught on the real access attempt.
			if err != nil && errors.Is(err, services.ErrSessionMFARequired) {
				noMFAAccessErr = err
				break
			}
		}

	case *proto.IsMFARequiredRequest_KubernetesCluster:
		if t.KubernetesCluster == "" {
			return nil, trace.BadParameter("missing KubernetesCluster field in a kubernetes-only UserCertsRequest")
		}
		// Find the target cluster and check whether MFA is required.
		svcs, err := a.GetKubernetesServers(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var cluster types.KubeCluster
		for _, svc := range svcs {
			kubeCluster := svc.GetCluster()
			if kubeCluster.GetName() == t.KubernetesCluster {
				cluster = kubeCluster
				break
			}
		}
		if cluster == nil {
			return nil, trace.NotFound("kubernetes cluster %q not found", t.KubernetesCluster)
		}

		noMFAAccessErr = checker.CheckAccess(cluster, services.AccessState{})

	case *proto.IsMFARequiredRequest_Database:
		if t.Database.ServiceName == "" {
			return nil, trace.BadParameter("missing ServiceName field in a database-only UserCertsRequest")
		}
		servers, err := a.GetDatabaseServers(ctx, apidefaults.Namespace)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var db types.Database
		for _, server := range servers {
			if server.GetDatabase().GetName() == t.Database.ServiceName {
				db = server.GetDatabase()
				break
			}
		}
		if db == nil {
			return nil, trace.NotFound("database service %q not found", t.Database.ServiceName)
		}

		autoCreate, err := checker.DatabaseAutoUserMode(db)
		switch {
		case errors.Is(err, services.ErrSessionMFARequired):
			noMFAAccessErr = err
		case err != nil:
			return nil, trace.Wrap(err)
		default:
			dbRoleMatchers := role.GetDatabaseRoleMatchers(role.RoleMatchersConfig{
				Database:       db,
				DatabaseUser:   t.Database.Username,
				DatabaseName:   t.Database.GetDatabase(),
				AutoCreateUser: autoCreate.IsEnabled(),
			})
			noMFAAccessErr = checker.CheckAccess(
				db,
				services.AccessState{},
				dbRoleMatchers...,
			)
		}

	case *proto.IsMFARequiredRequest_WindowsDesktop:
		desktops, err := a.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{Name: t.WindowsDesktop.GetWindowsDesktop()})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if len(desktops) == 0 {
			return nil, trace.NotFound("windows desktop %q not found", t.WindowsDesktop.GetWindowsDesktop())
		}

		noMFAAccessErr = checker.CheckAccess(desktops[0],
			services.AccessState{},
			services.NewWindowsLoginMatcher(t.WindowsDesktop.GetLogin()))

	case *proto.IsMFARequiredRequest_App:
		if t.App.Name == "" {
			return nil, trace.BadParameter("missing Name field in an app-only UserCertsRequest")
		}

		servers, err := a.GetApplicationServers(ctx, apidefaults.Namespace)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		i := slices.IndexFunc(servers, func(server types.AppServer) bool {
			return server.GetApp().GetName() == t.App.Name
		})
		if i == -1 {
			return nil, trace.NotFound("application service %q not found", t.App.Name)
		}

		app := servers[i].GetApp()
		noMFAAccessErr = checker.CheckAccess(app, services.AccessState{})

	default:
		return nil, trace.BadParameter("unknown Target %T", req.Target)
	}
	// No error means that MFA is not required for this resource by
	// AccessChecker.
	if noMFAAccessErr == nil {
		return &proto.IsMFARequiredResponse{
			MFARequired: proto.MFARequired_MFA_REQUIRED_NO,
		}, nil
	}
	// Errors other than ErrSessionMFARequired mean something else is wrong,
	// most likely access denied.
	if !errors.Is(noMFAAccessErr, services.ErrSessionMFARequired) {
		if !trace.IsAccessDenied(noMFAAccessErr) {
			log.WithError(noMFAAccessErr).Warn("Could not determine MFA access")
		}

		// Mask the access denied errors by returning false to prevent resource
		// name oracles. Auth will be denied (and generate an audit log entry)
		// when the client attempts to connect.
		return &proto.IsMFARequiredResponse{
			MFARequired: proto.MFARequired_MFA_REQUIRED_NO,
		}, nil
	}
	// If we reach here, the error from AccessChecker was
	// ErrSessionMFARequired.

	return &proto.IsMFARequiredResponse{
		MFARequired: proto.MFARequired_MFA_REQUIRED_YES,
	}, nil
}

// mfaAuthChallenge constructs an MFAAuthenticateChallenge for all MFA devices
// registered by the user.
func (a *Server) mfaAuthChallenge(ctx context.Context, user string, ssoClientRedirectURL string, challengeExtensions *mfav1.ChallengeExtensions) (*proto.MFAAuthenticateChallenge, error) {
	isPasswordless := challengeExtensions.Scope == mfav1.ChallengeScope_CHALLENGE_SCOPE_PASSWORDLESS_LOGIN

	// Check what kind of MFA is enabled.
	apref, err := a.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	enableTOTP := apref.IsSecondFactorTOTPAllowed()
	enableWebauthn := apref.IsSecondFactorWebauthnAllowed()
	enableSSO := apref.IsSecondFactorSSOAllowed()

	// Fetch configurations. The IsSecondFactor*Allowed calls above already
	// include the necessary checks of config empty, disabled, etc.
	var u2fPref *types.U2F
	switch val, err := apref.GetU2F(); {
	case trace.IsNotFound(err): // OK, may happen.
	case err != nil: // NOK, unexpected.
		return nil, trace.Wrap(err)
	default:
		u2fPref = val
	}
	var webConfig *types.Webauthn
	switch val, err := apref.GetWebauthn(); {
	case trace.IsNotFound(err): // OK, may happen.
	case err != nil: // NOK, unexpected.
		return nil, trace.Wrap(err)
	default:
		webConfig = val
	}

	// Handle passwordless separately, it works differently from MFA.
	if isPasswordless {
		if !enableWebauthn {
			return nil, trace.Wrap(types.ErrPasswordlessRequiresWebauthn)
		}
		if !apref.GetAllowPasswordless() {
			return nil, trace.Wrap(types.ErrPasswordlessDisabledBySettings)
		}

		webLogin := &wanlib.PasswordlessFlow{
			Webauthn: webConfig,
			Identity: a.Services,
		}
		assertion, err := webLogin.Begin(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		clusterName, err := a.GetClusterName()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if err := a.emitter.EmitAuditEvent(ctx, &apievents.CreateMFAAuthChallenge{
			Metadata: apievents.Metadata{
				Type:        events.CreateMFAAuthChallengeEvent,
				Code:        events.CreateMFAAuthChallengeCode,
				ClusterName: clusterName.GetClusterName(),
			},
			UserMetadata:        authz.ClientUserMetadataWithUser(ctx, user),
			ChallengeScope:      challengeExtensions.Scope.String(),
			ChallengeAllowReuse: challengeExtensions.AllowReuse == mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES,
		}); err != nil {
			log.WithError(err).Warn("Failed to emit CreateMFAAuthChallenge event.")
		}

		return &proto.MFAAuthenticateChallenge{
			WebauthnChallenge: wantypes.CredentialAssertionToProto(assertion),
		}, nil
	}

	// User required for non-passwordless.
	if user == "" {
		return nil, trace.BadParameter("user required")
	}

	devs, err := a.Services.GetMFADevices(ctx, user, true /* withSecrets */)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	groupedDevs := groupByDeviceType(devs)
	challenge := &proto.MFAAuthenticateChallenge{}

	// TOTP challenge.
	if enableTOTP && groupedDevs.TOTP {
		challenge.TOTP = &proto.TOTPChallenge{}
	}

	// WebAuthn challenge.
	if enableWebauthn && len(groupedDevs.Webauthn) > 0 {
		webLogin := &wanlib.LoginFlow{
			U2F:      u2fPref,
			Webauthn: webConfig,
			Identity: wanlib.WithDevices(a.Services, groupedDevs.Webauthn),
		}
		assertion, err := webLogin.Begin(ctx, user, challengeExtensions)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		challenge.WebauthnChallenge = wantypes.CredentialAssertionToProto(assertion)
	}

	// If the user has an SSO device and the client provided a redirect URL to handle
	// the MFA SSO flow, create an SSO challenge.
	if enableSSO && groupedDevs.SSO != nil && ssoClientRedirectURL != "" {
		if challenge.SSOChallenge, err = a.beginSSOMFAChallenge(ctx, user, groupedDevs.SSO.GetSso(), ssoClientRedirectURL, challengeExtensions); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := a.emitter.EmitAuditEvent(ctx, &apievents.CreateMFAAuthChallenge{
		Metadata: apievents.Metadata{
			Type:        events.CreateMFAAuthChallengeEvent,
			Code:        events.CreateMFAAuthChallengeCode,
			ClusterName: clusterName.GetClusterName(),
		},
		UserMetadata:        authz.ClientUserMetadataWithUser(ctx, user),
		ChallengeScope:      challengeExtensions.Scope.String(),
		ChallengeAllowReuse: challengeExtensions.AllowReuse == mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES,
	}); err != nil {
		log.WithError(err).Warn("Failed to emit CreateMFAAuthChallenge event.")
	}

	return challenge, nil
}

type devicesByType struct {
	TOTP     bool
	Webauthn []*types.MFADevice
	SSO      *types.MFADevice
}

func groupByDeviceType(devs []*types.MFADevice) devicesByType {
	res := devicesByType{}
	for _, dev := range devs {
		switch dev.Device.(type) {
		case *types.MFADevice_Totp:
			res.TOTP = true
		case *types.MFADevice_U2F:
			res.Webauthn = append(res.Webauthn, dev)
		case *types.MFADevice_Webauthn:
			res.Webauthn = append(res.Webauthn, dev)
		case *types.MFADevice_Sso:
			res.SSO = dev
		default:
			log.Warningf("Skipping MFA device of unknown type %T.", dev.Device)
		}
	}
	return res
}

// validateMFAAuthResponseForRegister is akin to [validateMFAAuthResponse], but
// it allows users with no devices to supply a nil/empty response.
//
// The hasDevices response value can only be trusted in the absence of errors.
//
// Use only for registration purposes.
func (a *Server) validateMFAAuthResponseForRegister(ctx context.Context, resp *proto.MFAAuthenticateResponse, username string, requiredExtensions *mfav1.ChallengeExtensions) (hasDevices bool, err error) {
	// Let users without a useable device go through registration.
	if resp == nil || (resp.GetTOTP() == nil && resp.GetWebauthn() == nil && resp.GetSSO() == nil) {
		devices, err := a.Services.GetMFADevices(ctx, username, false /* withSecrets */)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if len(devices) == 0 {
			// Allowed, no devices registered.
			return false, nil
		}
		devsByType := groupByDeviceType(devices)

		authPref, err := a.GetAuthPreference(ctx)
		if err != nil {
			return false, trace.Wrap(err)
		}

		hasTOTP := authPref.IsSecondFactorTOTPAllowed() && devsByType.TOTP
		hasWebAuthn := authPref.IsSecondFactorWebauthnAllowed() && len(devsByType.Webauthn) > 0
		hasSSO := authPref.IsSecondFactorSSOAllowed() && devsByType.SSO != nil

		if hasTOTP || hasWebAuthn || hasSSO {
			return false, trace.BadParameter("second factor authentication required")
		}

		// Allowed, no useable devices registered.
		return false, nil
	}

	if err := a.WithUserLock(ctx, username, func() error {
		_, err := a.ValidateMFAAuthResponse(ctx, resp, username, requiredExtensions)
		return err
	}); err != nil {
		return false, trace.Wrap(err)
	}

	return true, nil
}

// ValidateMFAAuthResponse validates an MFA or passwordless challenge. The provided
// required challenge extensions will be checked against the stored challenge when
// applicable (webauthn only). Returns the authentication data derived from the solved
// challenge.
func (a *Server) ValidateMFAAuthResponse(
	ctx context.Context,
	resp *proto.MFAAuthenticateResponse,
	user string,
	requiredExtensions *mfav1.ChallengeExtensions,
) (*authz.MFAAuthData, error) {
	if requiredExtensions == nil {
		return nil, trace.BadParameter("required challenge extensions parameter required")
	}

	authData, validateErr := a.validateMFAAuthResponseInternal(ctx, resp, user, requiredExtensions)
	// validateErr handled after audit.

	// Read ClusterName for audit.
	var clusterName string
	if cn, err := a.GetClusterName(); err != nil {
		log.WithError(err).Warn("Failed to read cluster name")
		// err swallowed on purpose.
	} else {
		clusterName = cn.GetClusterName()
	}

	// Take the user from the authData if the user param is empty.
	// This happens for passwordless.
	if user == "" && authData != nil {
		user = authData.User
	}

	// Emit audit event.
	auditEvent := &apievents.ValidateMFAAuthResponse{
		Metadata: apievents.Metadata{
			Type:        events.ValidateMFAAuthResponseEvent,
			ClusterName: clusterName,
		},
		UserMetadata:   authz.ClientUserMetadataWithUser(ctx, user),
		ChallengeScope: requiredExtensions.Scope.String(),
	}
	if validateErr != nil {
		auditEvent.Code = events.ValidateMFAAuthResponseFailureCode
		auditEvent.Success = false
		auditEvent.UserMessage = validateErr.Error()
		auditEvent.Error = validateErr.Error()
	} else {
		auditEvent.Code = events.ValidateMFAAuthResponseCode
		auditEvent.Success = true
		deviceMetadata := mfaDeviceEventMetadata(authData.Device)
		auditEvent.MFADevice = &deviceMetadata
		auditEvent.ChallengeAllowReuse = authData.AllowReuse == mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES
	}
	if err := a.emitter.EmitAuditEvent(ctx, auditEvent); err != nil {
		log.WithError(err).Warn("Failed to emit ValidateMFAAuthResponse event")
		// err swallowed on purpose.
	}

	return authData, trace.Wrap(validateErr)
}

func (a *Server) validateMFAAuthResponseInternal(
	ctx context.Context,
	resp *proto.MFAAuthenticateResponse,
	user string,
	requiredExtensions *mfav1.ChallengeExtensions,
) (*authz.MFAAuthData, error) {
	if requiredExtensions == nil {
		return nil, trace.BadParameter("required challenge extensions parameter required")
	}

	isPasswordless := requiredExtensions.Scope == mfav1.ChallengeScope_CHALLENGE_SCOPE_PASSWORDLESS_LOGIN

	// Sanity check user/passwordless.
	if user == "" && !isPasswordless {
		return nil, trace.BadParameter("user required")
	}

	switch res := resp.Response.(type) {
	// cases in order of preference
	case *proto.MFAAuthenticateResponse_Webauthn:
		// Read necessary configurations.
		cap, err := a.GetAuthPreference(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		u2f, err := cap.GetU2F()
		switch {
		case trace.IsNotFound(err): // OK, may happen.
		case err != nil: // Unexpected.
			return nil, trace.Wrap(err)
		}
		webConfig, err := cap.GetWebauthn()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		assertionResp := wantypes.CredentialAssertionResponseFromProto(res.Webauthn)
		var loginData *wanlib.LoginData
		if isPasswordless {
			webLogin := &wanlib.PasswordlessFlow{
				Webauthn: webConfig,
				Identity: a.Services,
			}
			loginData, err = webLogin.Finish(ctx, assertionResp)

			// Disallow non-local users from logging in with passwordless.
			if err == nil {
				u, getErr := a.GetUser(ctx, loginData.User, false /* withSecrets */)
				if getErr != nil {
					err = trace.Wrap(getErr)
				} else if u.GetUserType() != types.UserTypeLocal {
					// Return the error unmodified, without the "MFA response validation
					// failed" prefix.
					return nil, trace.Wrap(types.ErrPassswordlessLoginBySSOUser)
				}
			}
		} else {
			webLogin := &wanlib.LoginFlow{
				U2F:      u2f,
				Webauthn: webConfig,
				Identity: a.Services,
			}
			loginData, err = webLogin.Finish(ctx, user, wantypes.CredentialAssertionResponseFromProto(res.Webauthn), requiredExtensions)
		}
		if err != nil {
			return nil, trace.AccessDenied("MFA response validation failed: %v", err)
		}

		return &authz.MFAAuthData{
			Device:     loginData.Device,
			User:       loginData.User,
			AllowReuse: loginData.AllowReuse,
		}, nil

	case *proto.MFAAuthenticateResponse_TOTP:
		dev, err := a.checkOTP(user, res.TOTP.Code)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &authz.MFAAuthData{
			Device: dev,
			User:   user,
			// We store the last used token so OTP reuse is never allowed.
			AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_NO,
		}, nil

	case *proto.MFAAuthenticateResponse_SSO:
		mfaAuthData, err := a.verifySSOMFASession(ctx, user, res.SSO.RequestId, res.SSO.Token, requiredExtensions)
		return mfaAuthData, trace.Wrap(err)
	default:
		return nil, trace.BadParameter("unknown or missing MFAAuthenticateResponse type %T", resp.Response)
	}
}

func mergeKeySets(a, b types.CAKeySet) types.CAKeySet {
	newKeySet := a.Clone()
	newKeySet.SSH = append(newKeySet.SSH, b.SSH...)
	newKeySet.TLS = append(newKeySet.TLS, b.TLS...)
	newKeySet.JWT = append(newKeySet.JWT, b.JWT...)
	return newKeySet
}

// addAdditionalTrustedKeysAtomic performs an atomic update to
// the given CA with newKeys added to the AdditionalTrustedKeys
func (a *Server) addAdditionalTrustedKeysAtomic(ctx context.Context, ca types.CertAuthority, newKeys types.CAKeySet, needsUpdate func(types.CertAuthority) (bool, error)) error {
	const maxIterations = 64

	for i := 0; i < maxIterations; i++ {
		if update, err := needsUpdate(ca); err != nil || !update {
			return trace.Wrap(err)
		}

		err := ca.SetAdditionalTrustedKeys(mergeKeySets(
			ca.GetAdditionalTrustedKeys(),
			newKeys,
		))
		if err != nil {
			return trace.Wrap(err)
		}

		if _, err := a.UpdateCertAuthority(ctx, ca); err == nil {
			return nil
		} else if !errors.Is(err, backend.ErrIncorrectRevision) {
			return trace.Wrap(err)
		}

		ca, err = a.Services.GetCertAuthority(ctx, ca.GetID(), true)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return trace.Errorf("too many conflicts attempting to set additional trusted keys for ca %q of type %q", ca.GetClusterName(), ca.GetType())
}

// newKeySet generates a new sets of keys for a given CA type.
// Keep this function in sync with lib/services/suite/suite.go:NewTestCAWithConfig().
func newKeySet(ctx context.Context, keyStore *keystore.Manager, caID types.CertAuthID) (types.CAKeySet, error) {
	var keySet types.CAKeySet

	// Add SSH keys if necessary.
	switch caID.Type {
	case types.UserCA, types.HostCA, types.OpenSSHCA:
		sshKeyPair, err := keyStore.NewSSHKeyPair(ctx, sshCAKeyPurpose(caID.Type))
		if err != nil {
			return keySet, trace.Wrap(err)
		}
		keySet.SSH = append(keySet.SSH, sshKeyPair)
	}

	// Add TLS keys if necessary.
	switch caID.Type {
	case types.UserCA, types.HostCA, types.DatabaseCA, types.DatabaseClientCA, types.SAMLIDPCA, types.SPIFFECA:
		tlsKeyPair, err := keyStore.NewTLSKeyPair(ctx, caID.DomainName, tlsCAKeyPurpose(caID.Type))
		if err != nil {
			return keySet, trace.Wrap(err)
		}
		keySet.TLS = append(keySet.TLS, tlsKeyPair)
	}

	// Add JWT keys if necessary.
	switch caID.Type {
	case types.JWTSigner, types.OIDCIdPCA, types.SPIFFECA, types.OktaCA:
		jwtKeyPair, err := keyStore.NewJWTKeyPair(ctx, jwtCAKeyPurpose(caID.Type))
		if err != nil {
			return keySet, trace.Wrap(err)
		}
		keySet.JWT = append(keySet.JWT, jwtKeyPair)
	}

	return keySet, nil
}

func sshCAKeyPurpose(caType types.CertAuthType) cryptosuites.KeyPurpose {
	switch caType {
	case types.UserCA:
		return cryptosuites.UserCASSH
	case types.HostCA:
		return cryptosuites.HostCASSH
	case types.OpenSSHCA:
		return cryptosuites.OpenSSHCASSH
	}
	return cryptosuites.KeyPurposeUnspecified
}

func tlsCAKeyPurpose(caType types.CertAuthType) cryptosuites.KeyPurpose {
	switch caType {
	case types.UserCA:
		return cryptosuites.UserCATLS
	case types.HostCA:
		return cryptosuites.HostCATLS
	case types.DatabaseCA:
		return cryptosuites.DatabaseCATLS
	case types.DatabaseClientCA:
		return cryptosuites.DatabaseClientCATLS
	case types.SAMLIDPCA:
		return cryptosuites.SAMLIdPCATLS
	case types.SPIFFECA:
		return cryptosuites.SPIFFECATLS
	}
	return cryptosuites.KeyPurposeUnspecified
}

func jwtCAKeyPurpose(caType types.CertAuthType) cryptosuites.KeyPurpose {
	switch caType {
	case types.JWTSigner:
		return cryptosuites.JWTCAJWT
	case types.OIDCIdPCA:
		return cryptosuites.OIDCIdPCAJWT
	case types.SPIFFECA:
		return cryptosuites.SPIFFECAJWT
	case types.OktaCA:
		return cryptosuites.OktaCAJWT
	}
	return cryptosuites.KeyPurposeUnspecified
}

// ensureLocalAdditionalKeys adds additional trusted keys to the CA if they are not
// already present.
func (a *Server) ensureLocalAdditionalKeys(ctx context.Context, ca types.CertAuthority) error {
	usableKeysResult, err := a.keyStore.HasUsableAdditionalKeys(ctx, ca)
	if err != nil {
		return trace.Wrap(err)
	}
	if usableKeysResult.CAHasPreferredKeyType {
		// Nothing to do.
		return nil
	}

	newKeySet, err := newKeySet(ctx, a.keyStore, ca.GetID())
	if err != nil {
		return trace.Wrap(err)
	}

	// The CA still needs an update while the CA does not contain any keys of
	// the preferred type.
	needsUpdate := func(ca types.CertAuthority) (bool, error) {
		usableKeysResult, err := a.keyStore.HasUsableAdditionalKeys(ctx, ca)
		return !usableKeysResult.CAHasPreferredKeyType, trace.Wrap(err)
	}
	err = a.addAdditionalTrustedKeysAtomic(ctx, ca, newKeySet, needsUpdate)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Infof("Successfully added locally usable additional trusted keys to %s CA.", ca.GetType())
	return nil
}

// GetLicense return the license used the start the teleport enterprise auth server
func (a *Server) GetLicense(ctx context.Context) (string, error) {
	if modules.GetModules().Features().Cloud {
		return "", trace.AccessDenied("license cannot be downloaded on Cloud")
	}
	if a.license == nil {
		return "", trace.NotFound("license not found")
	}
	return fmt.Sprintf("%s%s", a.license.CertPEM, a.license.KeyPEM), nil
}

// GetHeadlessAuthenticationFromWatcher gets a headless authentication from the headless
// authentication watcher.
func (a *Server) GetHeadlessAuthenticationFromWatcher(ctx context.Context, username, name string) (*types.HeadlessAuthentication, error) {
	sub, err := a.headlessAuthenticationWatcher.Subscribe(ctx, username, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer sub.Close()

	// Wait for the login process to insert the headless authentication resource into the backend.
	// If it already exists and passes the condition, WaitForUpdate will return it immediately.
	headlessAuthn, err := sub.WaitForUpdate(ctx, func(ha *types.HeadlessAuthentication) (bool, error) {
		return services.ValidateHeadlessAuthentication(ha) == nil, nil
	})
	return headlessAuthn, trace.Wrap(err)
}

// UpsertHeadlessAuthenticationStub creates a headless authentication stub for the user
// that will expire after the standard callback timeout.
func (a *Server) UpsertHeadlessAuthenticationStub(ctx context.Context, username string) error {
	// Create the stub. If it already exists, update its expiration.
	expires := a.clock.Now().Add(defaults.HeadlessLoginTimeout)
	stub, err := types.NewHeadlessAuthentication(username, services.HeadlessAuthenticationUserStubID, expires)
	if err != nil {
		return trace.Wrap(err)
	}

	err = a.Services.UpsertHeadlessAuthentication(ctx, stub)
	return trace.Wrap(err)
}

// CompareAndSwapHeadlessAuthentication performs a compare
// and swap replacement on a headless authentication resource.
func (a *Server) CompareAndSwapHeadlessAuthentication(ctx context.Context, old, new *types.HeadlessAuthentication) (*types.HeadlessAuthentication, error) {
	headlessAuthn, err := a.Services.CompareAndSwapHeadlessAuthentication(ctx, old, new)
	return headlessAuthn, trace.Wrap(err)
}

// getAccessRequestMonthlyUsage returns the number of access requests that have been created this month.
func (a *Server) getAccessRequestMonthlyUsage(ctx context.Context) (int, error) {
	return resourceusage.GetAccessRequestMonthlyUsage(ctx, a.Services.AuditLogSessionStreamer, a.clock.Now().UTC())
}

// verifyAccessRequestMonthlyLimit checks whether the cluster has exceeded the monthly access request limit.
// If so, it returns an error. This is only applicable on usage-based billing plans.
func (a *Server) verifyAccessRequestMonthlyLimit(ctx context.Context) error {
	f := modules.GetModules().Features()
	accessRequestsEntitlement := f.GetEntitlement(entitlements.AccessRequests)

	if accessRequestsEntitlement.Limit == 0 {
		return nil // unlimited access
	}

	monthlyLimit := accessRequestsEntitlement.Limit

	const limitReachedMessage = "cluster has reached its monthly access request limit, please contact the cluster administrator"

	usage, err := a.getAccessRequestMonthlyUsage(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if usage >= int(monthlyLimit) {
		return trace.AccessDenied(limitReachedMessage)
	}

	return nil
}

// getProxyPublicAddr returns the first valid, non-empty proxy public address it
// finds, or empty otherwise.
func (a *Server) getProxyPublicAddr() string {
	if proxies, err := a.GetProxies(); err == nil {
		for _, p := range proxies {
			addr := p.GetPublicAddr()
			if addr == "" {
				continue
			}
			if _, err := utils.ParseAddr(addr); err != nil {
				log.Warningf("Invalid public address on the proxy %q: %q: %v.", p.GetName(), addr, err)
				continue
			}
			return addr
		}
	}
	return ""
}

// GetNodeStream streams a list of registered servers.
func (a *Server) GetNodeStream(ctx context.Context, namespace string) stream.Stream[types.Server] {
	var done bool
	startKey := ""
	return stream.PageFunc(func() ([]types.Server, error) {
		if done {
			return nil, io.EOF
		}
		resp, err := a.ListResources(ctx, proto.ListResourcesRequest{
			ResourceType: types.KindNode,
			Namespace:    namespace,
			Limit:        apidefaults.DefaultChunkSize,
			StartKey:     startKey,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		startKey = resp.NextKey
		done = startKey == ""
		resources := types.ResourcesWithLabels(resp.Resources)
		servers, err := resources.AsServers()
		return servers, trace.Wrap(err)
	})
}

// authKeepAliver is a keep aliver using auth server directly
type authKeepAliver struct {
	sync.RWMutex
	a           *Server
	ctx         context.Context
	cancel      context.CancelFunc
	keepAlivesC chan types.KeepAlive
	err         error
}

// KeepAlives returns a channel accepting keep alive requests
func (k *authKeepAliver) KeepAlives() chan<- types.KeepAlive {
	return k.keepAlivesC
}

func (k *authKeepAliver) forwardKeepAlives() {
	for {
		select {
		case <-k.a.closeCtx.Done():
			k.Close()
			return
		case <-k.ctx.Done():
			return
		case keepAlive := <-k.keepAlivesC:
			err := k.a.KeepAliveServer(k.ctx, keepAlive)
			if err != nil {
				k.closeWithError(err)
				return
			}
		}
	}
}

func (k *authKeepAliver) closeWithError(err error) {
	k.Close()
	k.Lock()
	defer k.Unlock()
	k.err = err
}

// Error returns the error if keep aliver
// has been closed
func (k *authKeepAliver) Error() error {
	k.RLock()
	defer k.RUnlock()
	return k.err
}

// Done returns channel that is closed whenever
// keep aliver is closed
func (k *authKeepAliver) Done() <-chan struct{} {
	return k.ctx.Done()
}

// Close closes keep aliver and cancels all goroutines
func (k *authKeepAliver) Close() error {
	k.cancel()
	return nil
}

// DefaultDNSNamesForRole returns default DNS names for the specified role.
func DefaultDNSNamesForRole(role types.SystemRole) []string {
	if (types.SystemRoles{role}).IncludeAny(
		types.RoleAuth,
		types.RoleAdmin,
		types.RoleProxy,
		types.RoleKube,
		types.RoleApp,
		types.RoleDatabase,
		types.RoleWindowsDesktop,
		types.RoleOkta,
	) {
		return []string{
			"*." + constants.APIDomain,
			constants.APIDomain,
		}
	}
	return nil
}
