package resource

import (
	"context"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
	"github.com/gravitational/trace"
)

var resourceHandlers = map[ResourceKind]resource{
	types.KindAccessGraphSettings:                accessGraphSettings,
	types.KindCrownJewel:                         crownJewel,
	types.KindAccessList:                         accessList,
	types.KindAccessMonitoringRule:               accessMonitoringRule,
	types.KindAccessRequest:                      accessRequest,
	types.KindAppServer:                          appServer,
	types.KindApp:                                app,
	types.KindCertAuthority:                      certAuthority,
	types.KindAutoUpdateConfig:                   autoUpdateConfig,
	types.KindAutoUpdateVersion:                  autoUpdateVersion,
	types.KindAutoUpdateAgentReport:              autoUpdateAgentReport,
	types.KindAutoUpdateAgentRollout:             autoUpdateAgentRollout,
	types.KindBot:                                bot,
	types.KindBotInstance:                        botInstance,
	types.KindUIConfig:                           uiConfig,
	types.KindClusterAuthPreference:              clusterAuthPreference,
	types.KindClusterNetworkingConfig:            clusterNetworkingConfig,
	types.KindSessionRecordingConfig:             sessionRecordingConfig,
	types.KindNetworkRestrictions:                networkRestrictions,
	types.KindClusterMaintenanceConfig:           clusterMaintenanceConfig,
	types.KindConnectors:                         connector,
	types.KindOIDC:                               oidcConnector,
	types.KindSAML:                               samlConnector,
	types.KindGithub:                             githubConnector,
	types.KindDatabaseServer:                     databaseServer,
	types.KindDatabase:                           database,
	types.KindDatabaseService:                    databaseService,
	types.KindDatabaseObjectImportRule:           databaseObjectImportRule,
	types.KindDatabaseObject:                     databaseObject,
	types.KindHealthCheckConfig:                  healthCheckConfig,
	types.KindDevice:                             device,
	types.KindDiscoveryConfig:                    discoveryConfig,
	types.KindExternalAuditStorage:               externalAuditStorage,
	types.KindGitServer:                          gitServer,
	types.KindSAMLIdPServiceProvider:             samlIdPServiceProvider,
	types.KindInstaller:                          installer,
	types.KindIntegration:                        integration,
	types.KindKubeServer:                         kubeServer,
	types.KindKubernetesCluster:                  kubeCluster,
	types.KindLock:                               lock,
	types.KindLoginRule:                          loginRule,
	types.KindNamespace:                          namespace,
	types.KindOktaImportRule:                     oktaImportRule,
	types.KindOktaAssignment:                     oktaAssignment,
	types.KindUserGroup:                          userGroup,
	types.KindPlugin:                             plugin,
	types.KindReverseTunnel:                      reverseTunnel,
	types.KindRole:                               role,
	types.KindAuditQuery:                         auditQuery,
	types.KindSecurityReport:                     securityReport,
	types.KindSemaphore:                          semaphore,
	types.KindNode:                               node,
	types.KindAuthServer:                         authServer,
	types.KindProxy:                              proxy,
	types.KindServerInfo:                         serverInfo,
	types.KindStaticHostUser:                     staticHostUser,
	types.KindToken:                              token,
	types.KindTrustedCluster:                     trustedCluster,
	types.KindRemoteCluster:                      remoteCluster,
	types.KindUser:                               user,
	types.KindUserTask:                           userTask,
	types.KindVnetConfig:                         vnetConfig,
	types.KindWindowsDesktopService:              windowsDesktopService,
	types.KindWindowsDesktop:                     windowsDesktop,
	types.KindDynamicWindowsDesktop:              dynamicWindowsDesktop,
	types.KindSPIFFEFederation:                   spiffeFederation,
	types.KindWorkloadIdentity:                   workloadIdentity,
	types.KindWorkloadIdentityX509Revocation:     workloadIdentityX509Revocation,
	types.KindWorkloadIdentityX509IssuerOverride: workloadIdentityX509IssuerOverride,
	types.KindSigstorePolicy:                     sigstorePolicy,
}

type resource struct {
	getHandler    func(context.Context, *authclient.Client, services.Ref, getOpts) (collections.ResourceCollection, error)
	createHandler func(context.Context, *authclient.Client, services.UnknownResource, createOpts) error
	updateHandler func(context.Context, *authclient.Client, services.UnknownResource, createOpts) error
	deleteHandler func(context.Context, *authclient.Client, services.Ref) error
	singleton     bool
	description   string
}

type getOpts struct {
	// withSecrets is true if the user set --with-secrets
	withSecrets bool
}

type createOpts struct {
	// force is true if the user set --force
	force bool
	// confirm is true if the user set --confirm
	confirm bool
}

func (r *resource) get(ctx context.Context, clt *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if r.getHandler == nil {
		return nil, trace.NotImplemented("resource does not support 'tctl get'")
	}
	return r.getHandler(ctx, clt, ref, opts)
}

func (r *resource) create(ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	if r.createHandler == nil {
		return trace.NotImplemented("resource does not support 'tctl create'")
	}
	return r.createHandler(ctx, clt, raw, opts)
}

func (r *resource) update(ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	if r.updateHandler == nil {
		return trace.NotImplemented("resource does not have an update handler")
	}
	return r.updateHandler(ctx, clt, raw, opts)
}

func (r *resource) delete(ctx context.Context, clt *authclient.Client, ref services.Ref) error {
	if ref.Name == "" && !r.singleton {
		return trace.BadParameter("provide a full resource name to delete, for example:\n$ tctl rm cluster/east\n")
	}
	if r.deleteHandler == nil {
		return trace.NotImplemented("resource does not support 'tctl delete'")
	}
	return r.deleteHandler(ctx, clt, ref)
}

func (r *resource) supportedCommands() []string {
	var verbs []string
	if r.getHandler != nil {
		verbs = append(verbs, "get")
	}
	if r.createHandler != nil {
		verbs = append(verbs, "create")
	}
	if r.deleteHandler != nil {
		verbs = append(verbs, "rm")
	}
	return verbs
}
