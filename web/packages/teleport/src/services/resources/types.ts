/**
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

import { AuthType } from 'shared/services';

export type Resource<T extends Kind> = {
  id: string;
  kind: T;
  name: string;
  description?: string;
  // content is config in yaml format.
  content: string;
};

export type KindRole = 'role';
export type KindTrustedCluster = 'trusted_cluster';
export type KindAuthConnectors = 'github' | 'saml' | 'oidc';
export type KindJoinToken = 'join_token';
export type Kind =
  | KindRole
  | KindTrustedCluster
  | KindAuthConnectors
  | KindJoinToken;

/** Teleport role in a resource format. */
export type RoleResource = Resource<KindRole>;

/**
 * Teleport role in full format, as returned from Teleport API.
 * TODO(bl-nero): Add all fields supported on the UI side.
 */
export type Role = {
  kind: KindRole;
  version: RoleVersion;
  metadata: {
    name: string;
    description?: string;
    labels?: Record<string, string>;
    expires?: string;
    revision?: string;
  };
  spec: {
    allow: RoleConditions;
    deny: RoleConditions;
    options: RoleOptions;
  };
};

export enum RoleVersion {
  V3 = 'v3',
  V4 = 'v4',
  V5 = 'v5',
  V6 = 'v6',
  V7 = 'v7',
}

/**
 * A set of conditions that must be matched to allow or deny access. Fields
 * follow the snake case convention to match the wire format.
 */
export type RoleConditions = {
  node_labels?: Labels;
  logins?: string[];

  kubernetes_groups?: string[];
  kubernetes_labels?: Labels;
  kubernetes_resources?: KubernetesResource[];
  kubernetes_users?: string[];

  app_labels?: Labels;
  aws_role_arns?: string[];
  azure_identities?: string[];
  gcp_service_accounts?: string[];

  db_labels?: Labels;
  db_names?: string[];
  db_users?: string[];
  db_roles?: string[];
  db_service_labels?: Labels;

  windows_desktop_labels?: Labels;
  windows_desktop_logins?: string[];

  github_permissions?: GitHubPermission[];

  rules?: Rule[];
};

export type Labels = Record<string, string | string[]>;

export type DefaultAuthConnector = {
  name?: string;
  type: AuthType;
};

export type KubernetesResource = {
  kind?: KubernetesResourceKind;
  name?: string;
  namespace?: string;
  verbs?: KubernetesVerb[];
};

/**
 * Supported Kubernetes resource kinds. This type needs to be kept in sync with
 * `KubernetesResourcesKinds` in `api/types/constants.go, as well as
 * `kubernetesResourceKindOptions` in
 * `web/packages/teleport/src/Roles/RoleEditor/standardmodel.ts`.
 */
export type KubernetesResourceKind =
  | '*'
  | 'pod'
  | 'secret'
  | 'configmap'
  | 'namespace'
  | 'service'
  | 'serviceaccount'
  | 'kube_node'
  | 'persistentvolume'
  | 'persistentvolumeclaim'
  | 'deployment'
  | 'replicaset'
  | 'statefulset'
  | 'daemonset'
  | 'clusterrole'
  | 'kube_role'
  | 'clusterrolebinding'
  | 'rolebinding'
  | 'cronjob'
  | 'job'
  | 'certificatesigningrequest'
  | 'ingress';

/**
 * Supported Kubernetes resource verbs. This type needs to be kept in sync with
 * `KubernetesVerbs` in `api/types/constants.go, as well as
 * `kubernetesVerbOptions` in
 * `web/packages/teleport/src/Roles/RoleEditor/standardmodel.ts`.
 */
export type KubernetesVerb =
  | '*'
  | 'get'
  | 'create'
  | 'update'
  | 'patch'
  | 'delete'
  | 'list'
  | 'watch'
  | 'deletecollection'
  | 'exec'
  | 'portforward';

export type Rule = {
  resources?: ResourceKind[];
  verbs?: Verb[];
  where?: string;
};

export enum ResourceKind {
  Wildcard = '*',

  // This list was taken from all of the `Kind*` constants in
  // `api/types/constants.go`. Please keep these in sync.

  // Resources backed by objects in the backend database.
  AccessGraphSecretAuthorizedKey = 'access_graph_authorized_key',
  AccessGraphSecretPrivateKey = 'access_graph_private_key',
  AccessGraphSettings = 'access_graph_settings',
  AccessList = 'access_list',
  AccessListMember = 'access_list_member',
  AccessListReview = 'access_list_review',
  AccessMonitoringRule = 'access_monitoring_rule',
  AccessRequest = 'access_request',
  App = 'app',
  AppOrSAMLIdPServiceProvider = 'app_server_or_saml_idp_sp',
  AppServer = 'app_server',
  AuditQuery = 'audit_query',
  AuthServer = 'auth_server',
  AutoUpdateAgentRollout = 'autoupdate_agent_rollout',
  AutoUpdateConfig = 'autoupdate_config',
  AutoUpdateVersion = 'autoupdate_version',
  Bot = 'bot',
  BotInstance = 'bot_instance',
  CertAuthority = 'cert_authority',
  ClusterAlert = 'cluster_alert',
  ClusterAuditConfig = 'cluster_audit_config',
  ClusterAuthPreference = 'cluster_auth_preference',
  ClusterMaintenanceConfig = 'cluster_maintenance_config',
  ClusterName = 'cluster_name',
  ClusterNetworkingConfig = 'cluster_networking_config',
  ConnectionDiagnostic = 'connection_diagnostic',
  CrownJewel = 'crown_jewel',
  Database = 'db',
  DatabaseObject = 'db_object',
  DatabaseObjectImportRule = 'db_object_import_rule',
  DatabaseServer = 'db_server',
  DatabaseService = 'db_service',
  Device = 'device',
  DiscoveryConfig = 'discovery_config',
  DynamicWindowsDesktop = 'dynamic_windows_desktop',
  ExternalAuditStorage = 'external_audit_storage',
  GitServer = 'git_server',
  // Ignoring duplicate: KindGithub = "github"
  GithubConnector = 'github',
  GlobalNotification = 'global_notification',
  HeadlessAuthentication = 'headless_authentication',
  Identity = 'identity',
  IdentityCenterAccount = 'aws_ic_account',
  IdentityCenterAccountAssignment = 'aws_ic_account_assignment',
  IdentityCenterPermissionSet = 'aws_ic_permission_set',
  IdentityCenterPrincipalAssignment = 'aws_ic_principal_assignment',
  Installer = 'installer',
  Instance = 'instance',
  Integration = 'integration',
  KubeCertificateSigningRequest = 'certificatesigningrequest',
  KubeClusterRole = 'clusterrole',
  KubeClusterRoleBinding = 'clusterrolebinding',
  KubeConfigmap = 'configmap',
  KubeCronjob = 'cronjob',
  KubeDaemonSet = 'daemonset',
  KubeDeployment = 'deployment',
  KubeIngress = 'ingress',
  KubeJob = 'job',
  KubeNamespace = 'namespace',
  KubeNode = 'kube_node',
  KubePersistentVolume = 'persistentvolume',
  KubePersistentVolumeClaim = 'persistentvolumeclaim',
  KubePod = 'pod',
  KubeReplicaSet = 'replicaset',
  KubeRole = 'kube_role',
  KubeRoleBinding = 'rolebinding',
  KubeSecret = 'secret',
  KubeServer = 'kube_server',
  KubeService = 'service',
  KubeServiceAccount = 'serviceaccount',
  KubeStatefulset = 'statefulset',
  KubeWaitingContainer = 'kube_ephemeral_container',
  KubernetesCluster = 'kube_cluster',
  License = 'license',
  Lock = 'lock',
  LoginRule = 'login_rule',
  MFADevice = 'mfa_device',
  // Ignoring duplicate: KindNamespace = "namespace"
  NetworkRestrictions = 'network_restrictions',
  Node = 'node',
  Notification = 'notification',
  // Ignoring duplicate: KindOIDC = "oidc"
  OIDCConnector = 'oidc',
  OktaAssignment = 'okta_assignment',
  OktaImportRule = 'okta_import_rule',
  Plugin = 'plugin',
  PluginData = 'plugin_data',
  PluginStaticCredentials = 'plugin_static_credentials',
  ProvisioningPrincipalState = 'provisioning_principal_state',
  Proxy = 'proxy',
  RecoveryCodes = 'recovery_codes',
  RemoteCluster = 'remote_cluster',
  ReverseTunnel = 'tunnel',
  Role = 'role',
  // Ignoring duplicate: KindSAML = "saml"
  SAMLConnector = 'saml',
  SAMLIdPServiceProvider = 'saml_idp_service_provider',
  SPIFFEFederation = 'spiffe_federation',
  SecurityReport = 'security_report',
  SecurityReportCostLimiter = 'security_report_cost_limiter',
  SecurityReportState = 'security_report_state',
  Semaphore = 'semaphore',
  ServerInfo = 'server_info',
  SessionRecordingConfig = 'session_recording_config',
  SessionTracker = 'session_tracker',
  State = 'state',
  StaticHostUser = 'static_host_user',
  StaticTokens = 'static_tokens',
  Token = 'token',
  TrustedCluster = 'trusted_cluster',
  TunnelConnection = 'tunnel_connection',
  UIConfig = 'ui_config',
  User = 'user',
  UserGroup = 'user_group',
  UserLastSeenNotification = 'user_last_seen_notification',
  UserLoginState = 'user_login_state',
  UserNotificationState = 'user_notification_state',
  UserTask = 'user_task',
  UserToken = 'user_token',
  UserTokenSecrets = 'user_token_secrets',
  VnetConfig = 'vnet_config',
  WatchStatus = 'watch_status',
  WebSession = 'web_session',
  WebToken = 'web_token',
  WindowsDesktop = 'windows_desktop',
  WindowsDesktopService = 'windows_desktop_service',

  // Resources that have no actual data representation, but serve for checking
  // access to various features.
  AccessGraph = 'access_graph',
  AccessPluginData = 'access_plugin_data',
  AuthConnector = 'auth_connector',
  Billing = 'billing',
  ClusterConfig = 'cluster_config',
  Connectors = 'connectors',
  DatabaseCertificate = 'database_certificate',
  Download = 'download',
  Event = 'event',
  GithubRequest = 'github_request',
  HostCert = 'host_cert',
  IdentityCenter = 'aws_identity_center',
  JWT = 'jwt',
  OIDCRequest = 'oidc_request',
  SAMLRequest = 'saml_request',
  SSHSession = 'ssh_session',
  Session = 'session',
  UnifiedResource = 'unified_resource',
  UsageEvent = 'usage_event',

  // For completeness: these kind constants were not included here, as they
  // refer to resource subkind names that are not used for access control.
  //
  // KindAppSession = "app_session"
  // KindSAMLIdPSession = "saml_idp_session"
  // KindSnowflakeSession = "snowflake_session"
}

export type Verb =
  | '*'
  | 'create'
  | 'create_enroll_token'
  | 'delete'
  | 'enroll'
  | 'list'
  | 'read'
  | 'readnosecrets'
  | 'rotate'
  | 'update'
  | 'use';

export type GitHubPermission = {
  orgs?: string[];
};

/**
 * Teleport role options in full format, as returned from Teleport API. Note
 * that its fields follow the snake case convention to match the wire format.
 */
export type RoleOptions = {
  cert_format: string;
  create_db_user: boolean;
  create_desktop_user: boolean;
  desktop_clipboard: boolean;
  desktop_directory_sharing: boolean;
  enhanced_recording: string[];
  forward_agent: boolean;
  idp: {
    // There's a subtle quirk in `Rolev6.CheckAndSetDefaults`: if you ask
    // Teleport to create a resource with `idp` field set to null, it's instead
    // going to create the entire idp->saml->enabled structure. However, it's
    // possible to create a role with idp set to an empty object, and the
    // server will retain this state. This makes the `saml` field optional.
    saml?: {
      enabled: boolean;
    };
  };
  max_session_ttl: string;
  pin_source_ip: boolean;
  ssh_port_forwarding?: SSHPortForwarding;
  port_forwarding?: boolean;
  record_session: {
    default?: SessionRecordingMode;
    ssh?: SessionRecordingMode;
    desktop: boolean;
  };
  ssh_file_copy: boolean;
  client_idle_timeout?: string;
  disconnect_expired_cert?: boolean;
  require_session_mfa?: RequireMFAType;
  create_host_user_mode?: CreateHostUserMode;
  create_db_user_mode?: CreateDBUserMode;
};

export type SSHPortForwarding = {
  local?: {
    enabled?: boolean;
  };
  remote?: {
    enabled?: boolean;
  };
};

export type RequireMFAType =
  | boolean
  | 'hardware_key'
  | 'hardware_key_touch'
  | 'hardware_key_pin'
  | 'hardware_key_touch_and_pin';

export type CreateHostUserMode = '' | 'off' | 'keep' | 'insecure-drop';

export type CreateDBUserMode = '' | 'off' | 'keep' | 'best_effort_drop';

export type SessionRecordingMode = '' | 'strict' | 'best_effort';

export type RoleWithYaml = {
  object: Role;
  /**
   * yaml string used with yaml editors.
   */
  yaml: string;
};
