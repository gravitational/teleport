/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { Option } from 'shared/components/Select';
import { equalsDeep } from 'shared/utils/highbar';

import { Label as UILabel } from 'teleport/components/LabelsInput/LabelsInput';
import {
  KubernetesResource,
  Labels,
  MCPPermissions,
  Role,
  RoleConditions,
} from 'teleport/services/resources';
import {
  CreateDBUserMode,
  CreateHostUserMode,
  GitHubPermission,
  isLegacySamlIdpRbac,
  KubernetesVerb,
  RequireMFAType,
  ResourceKind,
  RoleOptions,
  RoleVersion,
  Rule,
  SessionRecordingMode,
  Verb,
} from 'teleport/services/resources/types';

import {
  ConversionError,
  ConversionErrorGroup,
  ConversionErrorType,
  getOptionOrPushError,
  groupAndSortConversionErrors,
  unsupportedFieldErrorsFromObject,
  unsupportedValueWithReplacement,
} from './errors';
import { RoleEditorModelValidationResult } from './validation';
import { defaultOptions } from './withDefaults';

export enum StandardEditorTab {
  Overview,
  Resources,
  AdminRules,
  Options,
}

export type StandardEditorModel = {
  /**
   * The role model. Can be undefined if there was an unhandled error when
   * converting an existing role.
   */
  roleModel?: RoleEditorModel;
  originalRole: Role;
  /**
   * Will be true if fields have been modified from the original.
   */
  isDirty: boolean;
  /**
   * Indicates if the user interacted with the editor. It's different from
   * {@link StandardEditorModel.isDirty} by not taking into consideration if
   * anything changed from the original.
   */
  isTouched: boolean;
  /**
   * Result of validating {@link StandardEditorModel.roleModel}. Can be
   * undefined if there was an unhandled error when converting an existing
   * role.
   */
  validationResult?: RoleEditorModelValidationResult;
  currentTab: StandardEditorTab;
  disabledTabs: Set<StandardEditorTab>;
};

/**
 * A temporary representation of the role, reflecting the structure of standard
 * editor UI. Since the standard editor UI structure doesn't directly represent
 * the structure of the role resource, we introduce this intermediate model.
 */
export type RoleEditorModel = {
  metadata: MetadataModel;
  resources: ResourceAccess[];
  rules: RuleModel[];
  options: OptionsModel;
  /**
   * Indicates whether the current resource, as described by YAML, is
   * accurately represented by this editor model. If it's not, the user needs
   * to agree to reset it to a compatible resource before editing it in the
   * structured editor.
   */
  requiresReset: boolean;
  conversionErrors: ConversionErrorGroup[];
};

export function requiresReset(rm: RoleEditorModel | undefined): boolean {
  if (rm === undefined) return false;
}

export type MetadataModel = {
  name: string;
  /**
   * Set to `true` when we detect an existing role with the same name. This is
   * for validation purposes only, but it's stored in the model, because our
   * validation framework doesn't currently have a native support for
   * asynchronous validation. This flag is only being set if a new rule is
   * being created.
   */
  nameCollision: boolean;
  description?: string;
  revision?: string;
  labels: UILabel[];
  version: RoleVersionOption;
};

/** A model for resource section. */
export type ResourceAccess =
  | KubernetesAccess
  | ServerAccess
  | AppAccess
  | DatabaseAccess
  | WindowsDesktopAccess
  | GitHubOrganizationAccess;

/**
 * A base for all resource section models. Contains a type discriminator field.
 */
type ResourceAccessBase<T extends ResourceAccessKind> = {
  /**
   * Determines kind of resource that is accessed using this spec. Intended to
   * be mostly consistent with UnifiedResourceKind, but that has no real
   * meaning on the server side; we needed some discriminator, so we picked
   * this one.
   */
  kind: T;
  /**
   * Indicates if the form should make
   */
  hideValidationErrors: boolean;
};

export type ResourceAccessKind =
  | 'node'
  | 'kube_cluster'
  | 'app'
  | 'db'
  | 'windows_desktop'
  | 'git_server';

/** Model for the Kubernetes resource section. */
export type KubernetesAccess = ResourceAccessBase<'kube_cluster'> & {
  groups: readonly Option[];
  labels: UILabel[];
  resources: KubernetesResourceModel[];
  users: readonly Option[];

  /**
   * Version of the role that owns this section. Required to propagate it to
   * {@link KubernetesResourceModel}. It's the responsibility of
   * `useStandardModel` reducer to keep this value consistent with the actual
   * role version.
   */
  roleVersion: RoleVersion;
};

export type KubernetesResourceModel = {
  /** Autogenerated ID to be used with the `key` property. */
  id: string;
  kind: KubernetesResourceKindOption;
  name: string;
  namespace: string;
  verbs: readonly KubernetesVerbOption[];
  apiGroup?: string;
  /**
   * Version of the role that owns this section. Required in order to support
   * version-specific validation rules. It's the responsibility of
   * `useStandardModel` reducer to keep this value consistent with the actual
   * role version.
   */
  roleVersion: RoleVersion;
};

type KubernetesResourceKindOption = Option<string, string>;

/**
 * All possible resource kind drop-down options. This array needs to be kept in
 * sync with `KubernetesResourcesKinds` in `api/types/constants.go.
 *
 * This type list needs to be kept in sync with
 * `KubernetesResourcesKinds` in `api/types/constants.go for roles <=v7.
 */
export const kubernetesResourceKindOptionsV7: KubernetesResourceKindOption[] = [
  // The "any kind" option goes first.
  { value: '*', label: 'Any kind' },

  // The rest is sorted by label.
  ...(
    [
      { value: 'pod', label: 'Pod' },
      { value: 'secret', label: 'Secret' },
      { value: 'configmap', label: 'ConfigMap' },
      { value: 'namespace', label: 'Namespace' },
      { value: 'service', label: 'Service' },
      { value: 'serviceaccount', label: 'ServiceAccount' },
      { value: 'kube_node', label: 'Node' },
      { value: 'persistentvolume', label: 'PersistentVolume' },
      { value: 'persistentvolumeclaim', label: 'PersistentVolumeClaim' },
      { value: 'deployment', label: 'Deployment' },
      { value: 'replicaset', label: 'ReplicaSet' },
      { value: 'statefulset', label: 'Statefulset' },
      { value: 'daemonset', label: 'DaemonSet' },
      { value: 'clusterrole', label: 'ClusterRole' },
      { value: 'kube_role', label: 'Role' },
      { value: 'clusterrolebinding', label: 'ClusterRoleBinding' },
      { value: 'rolebinding', label: 'RoleBinding' },
      { value: 'cronjob', label: 'Cronjob' },
      { value: 'job', label: 'Job' },
      {
        value: 'certificatesigningrequest',
        label: 'CertificateSigningRequest',
      },
      { value: 'ingress', label: 'Ingress' },
    ] as const
  ).toSorted((a, b) => a.label.localeCompare(b.label)),
];

// Map of kinds/groups to detect v7->v8 upgrade issues.
//
// Record<v7kind, { group: ['main group', 'optional additional groups'], v8name: 'v8 plural name' }>
export const kubernetesResourceKindV7Groups: Record<
  string,
  { groups: string[]; v8name: string }
> = {
  pod: { groups: ['core'], v8name: 'pods' },
  secret: { groups: ['core'], v8name: 'secrets' },
  configmap: { groups: ['core'], v8name: 'configmaps' },
  namespace: { groups: ['core'], v8name: 'namespaces' },
  service: { groups: ['core'], v8name: 'services' },
  serviceaccount: { groups: ['core'], v8name: 'serviceaccounts' },
  kube_node: { groups: ['core'], v8name: 'nodes' },
  persistentvolume: { groups: ['core'], v8name: 'persistentvolumes' },
  persistentvolumeclaim: { groups: ['core'], v8name: 'persistentvolumeclaims' },
  deployment: { groups: ['apps', 'extensions'], v8name: 'deployments' },
  replicaset: { groups: ['apps', 'extensions'], v8name: 'replicasets' },
  statefulset: { groups: ['apps'], v8name: 'statefulsets' },
  daemonset: { groups: ['apps', 'extensions'], v8name: 'daemonsets' },
  clusterrole: {
    groups: ['rbac.authorization.k8s.io'],
    v8name: 'clusterroles',
  },
  kube_role: { groups: ['rbac.authorization.k8s.io'], v8name: 'roles' },
  clusterrolebinding: {
    groups: ['rbac.authorization.k8s.io'],
    v8name: 'clusterrolebindings',
  },
  rolebinding: {
    groups: ['rbac.authorization.k8s.io'],
    v8name: 'rolebindings',
  },
  cronjob: { groups: ['batch'], v8name: 'cronjobs' },
  job: { groups: ['batch'], v8name: 'jobs' },
  certificatesigningrequest: {
    groups: ['certificates.k8s.io'],
    v8name: 'certificatesigningrequests',
  },
  ingress: { groups: ['networking.k8s.io', 'extensions'], v8name: 'ingresses' },
} as const;

/**
 * Some of the possible values for the Kubernetes resource kind drop-down.
 * Arbitrary list, only preset for the sake of the UI.
 */
export const kubernetesResourceKindOptionsV8: KubernetesResourceKindOption[] = [
  // The "any kind" option goes first.
  { value: '*', label: 'Any kind' },

  // The rest is sorted by label.
  ...(
    [
      { value: 'pods', label: 'pods' },
      { value: 'secrets', label: 'secrets' },
      { value: 'configmaps', label: 'configmaps' },
      { value: 'namespaces', label: 'namespaces' },
      { value: 'services', label: 'services' },
      { value: 'serviceaccounts', label: 'serviceaccounts' },
      { value: 'nodes', label: 'nodes' },
      { value: 'persistentvolumes', label: 'persistentvolumes' },
      { value: 'persistentvolumeclaims', label: 'persistentvolumeclaims' },
      { value: 'deployments', label: 'deployments' },
      { value: 'replicasets', label: 'replicasets' },
      { value: 'statefulsets', label: 'statefulsets' },
      { value: 'daemonsets', label: 'daemonsets' },
      { value: 'clusterroles', label: 'clusterroles' },
      { value: 'roles', label: 'roles' },
      { value: 'clusterrolebindings', label: 'clusterrolebindings' },
      { value: 'rolebindings', label: 'rolebindings' },
      { value: 'cronjobs', label: 'cronjobs' },
      { value: 'jobs', label: 'jobs' },
      {
        value: 'certificatesigningrequests',
        label: 'certificatesigningrequests',
      },
      { value: 'ingresses', label: 'ingresses' },
    ] as const
  ).toSorted((a, b) => a.label.localeCompare(b.label)),
];

const optionsToMap = <K, V>(opts: Option<K, V>[]) =>
  new Map(opts.map(o => [o.value, o]));

export const kubernetesResourceKindOptionsMapV7 = optionsToMap(
  kubernetesResourceKindOptionsV7
);

export type KubernetesVerbOption = Option<KubernetesVerb, string>;
/**
 * All possible Kubernetes verb drop-down options. This array needs to be kept
 * in sync with `KubernetesVerbs` in `api/types/constants.go.
 */
export const kubernetesVerbOptions: KubernetesVerbOption[] = [
  // The "any kind" option goes first.
  { value: '*', label: 'All verbs' },

  // The rest is sorted.
  ...(
    [
      'get',
      'create',
      'update',
      'patch',
      'delete',
      'list',
      'watch',
      'deletecollection',

      // TODO(bl-nero): These are actually not k8s verbs, but they are allowed
      // in our config. We may want to explain them in the UI somehow.
      'exec',
      'portforward',
    ] as const
  )
    .toSorted((a, b) => a.localeCompare(b))
    .map(stringToOption),
];
export const kubernetesVerbOptionsMap = optionsToMap(kubernetesVerbOptions);

/**
 * An option that denotes a resource kind. The values are mostly {@link
 * ResourceKind}, but we allow unsupported values.
 */
export type ResourceKindOption = Option<string, string>;
export const resourceKindOptions: ResourceKindOption[] = Object.values(
  ResourceKind
)
  .toSorted()
  .map(o => ({ value: o, label: o }));
export const resourceKindOptionsMap = optionsToMap(resourceKindOptions);

type VerbOption = Option<Verb, string>;
export const verbOptions: VerbOption[] = (
  [
    '*',
    'create',
    'create_enroll_token',
    'delete',
    'enroll',
    'list',
    'read',
    'readnosecrets',
    'rotate',
    'update',
    'use',
  ] as const
).map(stringToOption);
export const verbOptionsMap = optionsToMap(verbOptions);

/** Model for the server resource access section. */
export type ServerAccess = ResourceAccessBase<'node'> & {
  labels: UILabel[];
  logins: readonly Option[];
};

export type AppAccess = ResourceAccessBase<'app'> & {
  labels: UILabel[];
  awsRoleARNs: string[];
  azureIdentities: string[];
  gcpServiceAccounts: string[];
  mcpTools: string[];
};

export type DatabaseAccess = ResourceAccessBase<'db'> & {
  labels: UILabel[];
  names: readonly Option[];
  users: readonly Option[];
  roles: readonly Option[];
  dbServiceLabels: UILabel[];
};

export type WindowsDesktopAccess = ResourceAccessBase<'windows_desktop'> & {
  labels: UILabel[];
  logins: readonly Option[];
};

export type GitHubOrganizationAccess = ResourceAccessBase<'git_server'> & {
  organizations: readonly Option[];
};

export type RuleModel = {
  /** Autogenerated ID to be used with the `key` property. */
  id: string;

  /**
   * Resource kinds affected by this rule. Note that we allow unknown resource
   * kinds to appear here, since we want to support legacy configurations.
   * (Also: keeping track of supported resource types is hard.)
   */
  resources: readonly ResourceKindOption[];

  /**
   * Indicates whether a wildcard verb is in the list of rule's {@link
   * Rule.verbs}. For the purpose of model conversion, this overrides all verbs
   * in the {@link RuleModel.verbs} list.
   */
  allVerbs: boolean;

  /**
   * Lists all non-wildcard verbs allowed for the currently selected {@link
   * RuleModel.resources}, along with their checked state.
   */
  verbs: VerbModel[];
  where: string;
  hideValidationErrors: boolean;
};

export type VerbModel = { verb: Verb; checked: boolean };

export type OptionsModel = {
  maxSessionTTL: string;
  clientIdleTimeout: string;
  disconnectExpiredCert: boolean;
  requireMFAType: RequireMFATypeOption;
  createHostUserMode: CreateHostUserModeOption;
  createDBUser: boolean;
  createDBUserMode: CreateDBUserModeOption;
  desktopClipboard: boolean;
  createDesktopUser: boolean;
  desktopDirectorySharing: boolean;
  defaultSessionRecordingMode: SessionRecordingModeOption;
  sshSessionRecordingMode: SessionRecordingModeOption;
  recordDesktopSessions: boolean;
  forwardAgent: boolean;
  sshPortForwardingMode: SSHPortForwardingModeOption;
};

type RequireMFATypeOption = Option<RequireMFAType>;
export const requireMFATypeOptions: RequireMFATypeOption[] = [
  { value: false, label: 'No' },
  { value: true, label: 'Yes' },
  { value: 'hardware_key', label: 'Hardware Key' },
  { value: 'hardware_key_touch', label: 'Hardware Key (touch)' },
  {
    value: 'hardware_key_touch_and_pin',
    label: 'Hardware Key (touch and PIN)',
  },
];
export const requireMFATypeOptionsMap = optionsToMap(requireMFATypeOptions);

type CreateHostUserModeOption = Option<CreateHostUserMode>;
export const createHostUserModeOptions: CreateHostUserModeOption[] = [
  { value: '', label: 'Unspecified' },
  { value: 'off', label: 'Off' },
  { value: 'keep', label: 'Keep' },
  { value: 'insecure-drop', label: 'Drop (insecure)' },
];
export const createHostUserModeOptionsMap = optionsToMap(
  createHostUserModeOptions
);

type CreateDBUserModeOption = Option<CreateDBUserMode>;
export const createDBUserModeOptions: CreateDBUserModeOption[] = [
  { value: '', label: 'Unspecified' },
  { value: 'off', label: 'Off' },
  { value: 'keep', label: 'Keep' },
  { value: 'best_effort_drop', label: 'Drop (best effort)' },
];
export const createDBUserModeOptionsMap = optionsToMap(createDBUserModeOptions);

type SessionRecordingModeOption = Option<SessionRecordingMode>;
export const sessionRecordingModeOptions: SessionRecordingModeOption[] = [
  { value: '', label: 'Unspecified' },
  { value: 'best_effort', label: 'Best Effort' },
  { value: 'strict', label: 'Strict' },
];
export const sessionRecordingModeOptionsMap = optionsToMap(
  sessionRecordingModeOptions
);

export type SSHPortForwardingMode =
  | ''
  | 'none'
  | 'local-only'
  | 'remote-only'
  | 'local-and-remote'
  | 'deprecated-on'
  | 'deprecated-off';
export type SSHPortForwardingModeOption = Option<SSHPortForwardingMode> & {
  description?: string;
};
export const sshPortForwardingModeOptions: SSHPortForwardingModeOption[] = [
  { value: '', label: 'Unspecified' },
  { value: 'none', label: 'None' },
  { value: 'local-only', label: 'Local only' },
  { value: 'remote-only', label: 'Remote only' },
  { value: 'local-and-remote', label: 'Local and remote' },
  {
    value: 'deprecated-off',
    label: 'Off (deprecated)',
    description:
      'Changes the implicit default behavior for other roles assigned to a user from "allow all" to "deny all"',
  },
  {
    value: 'deprecated-on',
    label: 'On (deprecated)',
    description: 'Overrides all other roles applied to a user',
  },
];
export const sshPortForwardingModeOptionsMap = optionsToMap(
  sshPortForwardingModeOptions
);

export type RoleVersionOption = Option<RoleVersion>;
export const roleVersionOptions = Object.values(RoleVersion)
  .toSorted()
  .toReversed()
  .map(o => ({ value: o, label: o }));
export const roleVersionOptionsMap = optionsToMap(roleVersionOptions);

export const defaultRoleVersion = RoleVersion.V8;

/**
 * Returns the role object with required fields defined with empty values.
 */
export function newRole(): Role {
  return {
    kind: 'role',
    metadata: {
      name: 'new_role_name',
    },
    spec: {
      allow: {},
      deny: {},
      options: defaultOptions(defaultRoleVersion),
    },
    version: defaultRoleVersion,
  };
}

export function newResourceAccess(
  kind: 'node',
  roleVersion: RoleVersion
): ServerAccess;

export function newResourceAccess(
  kind: 'kube_cluster',
  roleVersion: RoleVersion
): KubernetesAccess;

export function newResourceAccess(
  kind: 'app',
  roleVersion: RoleVersion
): AppAccess;

export function newResourceAccess(
  kind: 'db',
  roleVersion: RoleVersion
): DatabaseAccess;

export function newResourceAccess(
  kind: 'windows_desktop',
  roleVersion: RoleVersion
): WindowsDesktopAccess;

export function newResourceAccess(
  kind: 'git_server',
  roleVersion: RoleVersion
): GitHubOrganizationAccess;

export function newResourceAccess(
  kind: ResourceAccessKind,
  roleVersion: RoleVersion
): AppAccess;

export function newResourceAccess(
  kind: ResourceAccessKind,
  roleVersion: RoleVersion
): ResourceAccess {
  switch (kind) {
    case 'node':
      return {
        kind: 'node',
        labels: [],
        logins: [stringToOption('{{internal.logins}}')],
        hideValidationErrors: true,
      };
    case 'kube_cluster':
      return {
        kind: 'kube_cluster',
        groups: [stringToOption('{{internal.kubernetes_groups}}')],
        labels: [],
        resources: [],
        users: [],
        roleVersion,
        hideValidationErrors: true,
      };
    case 'app':
      return {
        kind: 'app',
        labels: [],
        awsRoleARNs: ['{{internal.aws_role_arns}}'],
        azureIdentities: ['{{internal.azure_identities}}'],
        gcpServiceAccounts: ['{{internal.gcp_service_accounts}}'],
        mcpTools: ['{{internal.mcp_tools}}'],
        hideValidationErrors: true,
      };
    case 'db':
      return {
        kind: 'db',
        labels: [],
        names: [stringToOption('{{internal.db_names}}')],
        users: [stringToOption('{{internal.db_users}}')],
        roles: [stringToOption('{{internal.db_roles}}')],
        dbServiceLabels: [],
        hideValidationErrors: true,
      };
    case 'windows_desktop':
      return {
        kind: 'windows_desktop',
        labels: [],
        logins: [stringToOption('{{internal.windows_logins}}')],
        hideValidationErrors: true,
      };
    case 'git_server':
      return {
        kind: 'git_server',
        organizations: [stringToOption('{{internal.github_orgs}}')],
        hideValidationErrors: true,
      };
    default:
      kind satisfies never;
  }
}

export function supportsKubernetesCustomResources(roleVersion: RoleVersion) {
  return ![
    RoleVersion.V3,
    RoleVersion.V4,
    RoleVersion.V5,
    RoleVersion.V6,
    RoleVersion.V7,
  ].includes(roleVersion);
}

export function newKubernetesResourceModel(
  roleVersion: RoleVersion
): KubernetesResourceModel {
  return {
    id: crypto.randomUUID(),
    kind: kubernetesResourceKindOptionsV7.find(k => k.value === '*'),
    name: '*',
    namespace: '*',
    verbs: [],
    apiGroup: supportsKubernetesCustomResources(roleVersion) ? '*' : '',
    roleVersion,
  };
}

export function newRuleModel(): RuleModel {
  return {
    id: crypto.randomUUID(),
    resources: [],
    allVerbs: false,
    verbs: newVerbsModel([]),
    where: '',
    hideValidationErrors: true,
  };
}

/**
 * Converts a role to its in-editor UI model representation. The resulting
 * model may be marked as requiring reset if the role contains unsupported
 * features.
 */
export function roleToRoleEditorModel(
  role: Role,
  originalRole?: Role
): RoleEditorModel {
  const conversionErrors: ConversionError[] = [];

  // We use destructuring to strip fields from objects and assert that nothing
  // has been left. Therefore, we don't want Lint to warn us that we didn't use
  // some of the fields.
  // eslint-disable-next-line unused-imports/no-unused-vars
  const { kind, metadata, spec, version, ...unsupported } = role;
  conversionErrors.push(...unsupportedFieldErrorsFromObject('', unsupported));

  const { name, description, revision, labels, ...unsupportedMetadata } =
    metadata;
  conversionErrors.push(
    ...unsupportedFieldErrorsFromObject('metadata', unsupportedMetadata)
  );

  const { allow, deny, options, ...unsupportedSpecs } = spec;
  conversionErrors.push(
    ...unsupportedFieldErrorsFromObject('spec', unsupportedSpecs)
  );
  conversionErrors.push(...unsupportedFieldErrorsFromObject('spec.deny', deny));

  const versionOption = getOptionOrPushError(
    version,
    roleVersionOptionsMap,
    RoleVersion.V8,
    'version',
    conversionErrors
  );

  if (revision !== originalRole?.metadata?.revision) {
    conversionErrors.push({
      type: ConversionErrorType.UnsupportedChange,
      path: 'metadata.revision',
    });
  }

  const {
    resources,
    rules,
    conversionErrors: allowConversionErrors,
  } = roleConditionsToModel(allow, version, 'spec.allow');
  conversionErrors.push(...allowConversionErrors);

  const { model: optionsModel, conversionErrors: optionsConversionErrors } =
    optionsToModel(version, options, 'spec.options');
  conversionErrors.push(...optionsConversionErrors);

  return {
    metadata: {
      name,
      nameCollision: false,
      description,
      revision: originalRole?.metadata?.revision,
      labels: labelsToModel(labels),
      version: versionOption,
    },
    resources,
    rules,
    options: optionsModel,
    requiresReset: conversionErrors.length > 0,
    conversionErrors: groupAndSortConversionErrors(conversionErrors),
  };
}

/**
 * Converts a `RoleConditions` instance (an "allow" or "deny" section, to be
 * specific) to a part of the role editor model.
 */
function roleConditionsToModel(
  conditions: RoleConditions,
  roleVersion: RoleVersion,
  pathPrefix: string
): Pick<RoleEditorModel, 'resources' | 'rules'> & {
  conversionErrors: ConversionError[];
} {
  const conversionErrors: ConversionError[] = [];
  const {
    // Server access
    node_labels,
    logins,

    // Kubernetes access
    kubernetes_groups,
    kubernetes_labels,
    kubernetes_resources,
    kubernetes_users,

    // App access
    app_labels,
    aws_role_arns,
    azure_identities,
    gcp_service_accounts,

    // Database access
    db_labels,
    db_names,
    db_users,
    db_roles,
    db_service_labels,

    // Windows desktop access
    windows_desktop_labels,
    windows_desktop_logins,

    // GitHub organization access
    github_permissions,

    // Admin rules
    rules,

    // MCP permissions
    mcp,

    ...unsupported
  } = conditions;
  conversionErrors.push(
    ...unsupportedFieldErrorsFromObject(pathPrefix, unsupported)
  );

  const resources: ResourceAccess[] = [];

  const nodeLabelsModel = labelsToModel(node_labels);
  const nodeLoginsModel = stringsToOptions(logins ?? []);
  if (someNonEmpty(nodeLabelsModel, nodeLoginsModel)) {
    resources.push({
      kind: 'node',
      labels: nodeLabelsModel,
      logins: nodeLoginsModel,
      hideValidationErrors: false,
    });
  }

  const kubeGroupsModel = stringsToOptions(kubernetes_groups ?? []);
  const kubeLabelsModel = labelsToModel(kubernetes_labels);
  const {
    model: kubeResourcesModel,
    conversionErrors: kubernetesResourceConversionErrors,
  } = kubernetesResourcesToModel(
    kubernetes_resources,
    roleVersion,
    `${pathPrefix}.kubernetes_resources`
  );
  conversionErrors.push(...kubernetesResourceConversionErrors);

  const kubeUsersModel = stringsToOptions(kubernetes_users ?? []);
  if (
    someNonEmpty(
      kubeGroupsModel,
      kubeLabelsModel,
      kubeResourcesModel,
      kubeUsersModel
    )
  ) {
    resources.push({
      kind: 'kube_cluster',
      groups: kubeGroupsModel,
      labels: kubeLabelsModel,
      resources: kubeResourcesModel,
      users: kubeUsersModel,
      roleVersion,
      hideValidationErrors: false,
    });
  }

  const appLabelsModel = labelsToModel(app_labels);
  const awsRoleARNsModel = aws_role_arns ?? [];
  const azureIdentitiesModel = azure_identities ?? [];
  const gcpServiceAccountsModel = gcp_service_accounts ?? [];
  const { model: mcpToolsModel, conversionErrors: mcpToolsConversionErrors } =
    mcpToolsToModel(mcp || {}, `${pathPrefix}.mcp`);
  conversionErrors.push(...mcpToolsConversionErrors);
  if (
    someNonEmpty(
      appLabelsModel,
      awsRoleARNsModel,
      azureIdentitiesModel,
      gcpServiceAccountsModel,
      mcpToolsModel
    )
  ) {
    resources.push({
      kind: 'app',
      labels: appLabelsModel,
      awsRoleARNs: awsRoleARNsModel,
      azureIdentities: azureIdentitiesModel,
      gcpServiceAccounts: gcpServiceAccountsModel,
      mcpTools: mcpToolsModel,
      hideValidationErrors: false,
    });
  }

  const dbLabelsModel = labelsToModel(db_labels);
  const dbNamesModel = db_names ?? [];
  const dbUsersModel = db_users ?? [];
  const dbRolesModel = db_roles ?? [];
  const dbServiceLabelsModel = labelsToModel(db_service_labels);
  if (
    someNonEmpty(
      dbLabelsModel,
      dbNamesModel,
      dbUsersModel,
      dbRolesModel,
      dbServiceLabelsModel
    )
  ) {
    resources.push({
      kind: 'db',
      labels: dbLabelsModel,
      names: stringsToOptions(dbNamesModel),
      users: stringsToOptions(dbUsersModel),
      roles: stringsToOptions(dbRolesModel),
      dbServiceLabels: dbServiceLabelsModel,
      hideValidationErrors: false,
    });
  }

  const windowsDesktopLabelsModel = labelsToModel(windows_desktop_labels);
  const windowsDesktopLoginsModel = stringsToOptions(
    windows_desktop_logins ?? []
  );
  if (someNonEmpty(windowsDesktopLabelsModel, windowsDesktopLoginsModel)) {
    resources.push({
      kind: 'windows_desktop',
      labels: windowsDesktopLabelsModel,
      logins: windowsDesktopLoginsModel,
      hideValidationErrors: false,
    });
  }

  const {
    model: gitHubOrganizationsModel,
    conversionErrors: gitHubOrganizationConversionErrors,
  } = gitHubOrganizationsToModel(
    github_permissions,
    `${pathPrefix}.github_permissions`
  );
  if (someNonEmpty(gitHubOrganizationsModel)) {
    resources.push({
      kind: 'git_server',
      organizations: gitHubOrganizationsModel,
      hideValidationErrors: false,
    });
  }
  conversionErrors.push(...gitHubOrganizationConversionErrors);

  const { model: rulesModel, conversionErrors: ruleConversionErrors } =
    rulesToModel(rules, `${pathPrefix}.rules`);
  conversionErrors.push(...ruleConversionErrors);

  return {
    resources: resources,
    rules: rulesModel,
    conversionErrors,
  };
}

function someNonEmpty(...arr: any[][]): boolean {
  return arr.some(x => x.length > 0);
}

/**
 * Converts a set of labels, as represented in the role resource, to a list of
 * `LabelInput` value models.
 */
export function labelsToModel(labels: Labels | undefined): UILabel[] {
  if (!labels) return [];
  return Object.entries(labels).flatMap(([name, value]) => {
    if (typeof value === 'string') {
      return {
        name,
        value,
      };
    } else {
      return value.map(v => ({ name, value: v }));
    }
  });
}

function stringToOption<T extends string>(s: T): Option<T> {
  return { label: s, value: s };
}

function stringsToOptions<T extends string>(arr: T[]): Option<T>[] {
  return arr.map(stringToOption);
}

function kubernetesResourcesToModel(
  resources: KubernetesResource[] | undefined,
  roleVersion: RoleVersion,
  pathPrefix: string
): {
  model: KubernetesResourceModel[];
  conversionErrors: ConversionError[];
} {
  const result = (resources ?? []).map((res, i) =>
    kubernetesResourceToModel(res, roleVersion, `${pathPrefix}[${i}]`)
  );
  return {
    model: result.map(r => r.model).filter(m => m !== undefined),
    conversionErrors: result.flatMap(r => r.conversionErrors),
  };
}

function kubernetesResourceToModel(
  res: KubernetesResource,
  roleVersion: RoleVersion,
  pathPrefix: string
): {
  model?: KubernetesResourceModel;
  conversionErrors: ConversionError[];
} {
  const {
    kind,
    name,
    namespace = '',
    verbs = [],
    api_group: apiGroup,
    ...unsupported
  } = res;
  const conversionErrors = unsupportedFieldErrorsFromObject(
    pathPrefix,
    unsupported
  );

  const supportsCrds = supportsKubernetesCustomResources(roleVersion);
  const kindOption = supportsCrds
    ? { value: kind, label: kind }
    : kubernetesResourceKindOptionsMapV7.get(kind);

  if (supportsCrds) {
    // If we have an exact match with a v7 entry, it is most likely a mistake.
    const v7groups = kubernetesResourceKindV7Groups[kind];
    if (v7groups && (apiGroup == '*' || v7groups.groups.includes(apiGroup))) {
      kindOption.value = v7groups.v8name;
      kindOption.label = v7groups.v8name;
      conversionErrors.push(
        unsupportedValueWithReplacement(`${pathPrefix}.kind`, v7groups.v8name)
      );
    }
  }

  if (!kindOption) {
    conversionErrors.push({
      type: ConversionErrorType.UnsupportedValue,
      path: `${pathPrefix}.kind`,
    });
  }

  const verbOptions = verbs.map(verb => kubernetesVerbOptionsMap.get(verb));
  const knownVerbOptions: KubernetesVerbOption[] = [];
  verbOptions.forEach((vo, i) => {
    if (vo !== undefined) {
      knownVerbOptions.push(vo);
    } else {
      conversionErrors.push({
        type: ConversionErrorType.UnsupportedValue,
        path: `${pathPrefix}.verbs[${i}]`,
      });
    }
  });

  return {
    model:
      kindOption !== undefined
        ? {
            id: crypto.randomUUID(),
            kind: kindOption,
            name,
            namespace,
            verbs: knownVerbOptions,
            roleVersion,
            apiGroup,
          }
        : undefined,
    conversionErrors,
  };
}

export function mcpToolsToModel(
  mcpPermissions: MCPPermissions,
  pathPrefix: string
): {
  model: string[];
  conversionErrors: ConversionError[];
} {
  const { tools, ...unsupported } = mcpPermissions;
  return {
    model: tools || [],
    conversionErrors: unsupportedFieldErrorsFromObject(pathPrefix, unsupported),
  };
}

/**
 * Converts a {@link GitHubPermission} array to a list of organizations.
 * Technically, there can be more than one `GitHubPermission` object, but we
 * simply glue the underlying organization arrays into one. Note that the
 * object's semantics may further be extended to the point where there's a
 * difference between multiple such objects and one object containing all the
 * organizations; however, since this would require adding additional fields to
 * the `GitHubPermission` object (otherwise, such change wouldn't make sense),
 * this function is protected anyway from attempting to interpret such an
 * extended object, and it would return non-empty `conversionErrors` anyway.
 */
export function gitHubOrganizationsToModel(
  gitHubPermissions: GitHubPermission[],
  pathPrefix: string
): {
  model: Option[];
  conversionErrors: ConversionError[];
} {
  const permissions = gitHubPermissions ?? [];
  const model: Option[] = [];
  const conversionErrors: ConversionError[] = [];
  permissions.forEach((permission, i) => {
    const { orgs, ...unsupported } = permission;
    if (orgs) {
      model.push(...stringsToOptions(orgs));
    }
    conversionErrors.push(
      ...unsupportedFieldErrorsFromObject(`${pathPrefix}[${i}]`, unsupported)
    );
  });

  return {
    model,
    conversionErrors,
  };
}

function rulesToModel(
  rules: Rule[] | undefined,
  pathPrefix: string
): {
  model: RuleModel[];
  conversionErrors: ConversionError[];
} {
  const model: RuleModel[] = [];
  const conversionErrors: ConversionError[] = [];
  rules?.forEach?.((rule, i) => {
    const m = ruleToModel(rule, `${pathPrefix}[${i}]`);
    model.push(m.model);
    conversionErrors.push(...m.conversionErrors);
  });
  return {
    model,
    conversionErrors,
  };
}

function ruleToModel(
  rule: Rule,
  pathPrefix: string
): {
  model: RuleModel;
  conversionErrors: ConversionError[];
} {
  const { resources = [], verbs = [], where = '', ...unsupported } = rule;

  const conversionErrors = unsupportedFieldErrorsFromObject(
    pathPrefix,
    unsupported
  );

  const resourcesModel = resources.map(
    // Resource kind can be unknown, so we just create a necessary option on
    // the fly.
    k => resourceKindOptionsMap.get(k) ?? { label: k, value: k }
  );

  const allVerbs = verbs.includes('*');
  const verbsModel = allowedVerbsForResourceKinds(resources).map(verb => ({
    verb,
    checked: allVerbs, // Mark all as checked if there's a wildcard.
  }));

  if (allVerbs) {
    // If there's a wildcard, it needs to be the only verb. Other combinations
    // are not supported because of the editor UI structure.
    // TODO(bl-nero): Consider adding an explanation field to the conversion
    // error type.
    if (verbs.length > 1) {
      conversionErrors.push(
        unsupportedValueWithReplacement(`${pathPrefix}.verbs`, ['*'])
      );
    }
  } else {
    verbs.forEach((v, i) => {
      const vm = verbsModel.find(m => m.verb === v);
      if (vm) {
        vm.checked = true;
      } else {
        conversionErrors.push({
          type: ConversionErrorType.UnsupportedValue,
          path: `${pathPrefix}.verbs[${i}]`,
        });
      }
    });
  }

  return {
    model: {
      id: crypto.randomUUID(),
      resources: resourcesModel,
      allVerbs,
      verbs: verbsModel,
      where,
      hideValidationErrors: false,
    },
    conversionErrors,
  };
}

export function newVerbsModel(
  resKindOptions: readonly ResourceKindOption[]
): VerbModel[] {
  const kinds = resKindOptions.map(rko => rko.value);
  return allowedVerbsForResourceKinds(kinds).map(verb => ({
    verb,
    checked: false,
  }));
}

/**
 * Returns a list of verbs that are known to be supported by a given list of
 * resources. This list excludes the wildcard (*) verb, which gets a special
 * treatment in our model.
 */
export function allowedVerbsForResourceKinds(
  resourceKinds: readonly string[]
): Verb[] {
  const verbs: Verb[] = ['read', 'list', 'create', 'update', 'delete'];
  for (const kind of resourceKinds) {
    if (additionalVerbs.has(kind)) {
      verbs.push(...additionalVerbs.get(kind));
    }
  }
  return verbs;
}

/** A map of known resource-type-specific verbs by resource type. */
const additionalVerbs = new Map<string, Verb[]>([
  [ResourceKind.Plugin, ['readnosecrets']],
  [ResourceKind.CertAuthority, ['readnosecrets', 'rotate']],
  [ResourceKind.WebSession, ['readnosecrets']],
  [ResourceKind.OIDCConnector, ['readnosecrets']],
  [ResourceKind.SAMLConnector, ['readnosecrets']],
  [ResourceKind.GithubConnector, ['readnosecrets']],
  [ResourceKind.Semaphore, ['readnosecrets']],
  [ResourceKind.Device, ['create_enroll_token', 'enroll']],
  [ResourceKind.AuditQuery, ['use']],
  [ResourceKind.SecurityReport, ['use']],
  [ResourceKind.Integration, ['use']],
  [ResourceKind.AccessGraph, ['use']],

  // Currently unsupported, but important for backwards compatibility.
  ['assistant', ['use']],
]);

// These options must keep their default values, as we don't support them in
// the standard editor, but they are required fields of the RoleOptions type.
const uneditableOptionKeys: (keyof RoleOptions)[] = [
  'cert_format',
  'enhanced_recording',
  'idp',
  'pin_source_ip',
  'ssh_file_copy',
];

function optionsToModel(
  roleVersion: RoleVersion,
  options: RoleOptions,
  pathPrefix: string
): {
  model: OptionsModel;
  conversionErrors: ConversionError[];
} {
  const defaultOpts = defaultOptions(roleVersion);
  const conversionErrors: ConversionError[] = [];
  const {
    // Customizable options.
    max_session_ttl,
    client_idle_timeout = '',
    disconnect_expired_cert = false,
    require_session_mfa = false,
    create_host_user_mode = '',
    create_db_user,
    create_db_user_mode = '',
    desktop_clipboard,
    create_desktop_user,
    desktop_directory_sharing,
    record_session,
    forward_agent,
    port_forwarding,
    ssh_port_forwarding,

    ...unsupported
  } = options;
  for (const key of uneditableOptionKeys) {
    // delete saml idp option for role v8 and above as it
    // is no longer supported.
    if (!isLegacySamlIdpRbac(roleVersion) && key === 'idp') {
      delete unsupported[key];
      continue;
    }
    // Report uneditable options as errors if they diverge from their defaults.
    if (!equalsDeep(options[key], defaultOpts[key])) {
      conversionErrors.push(
        unsupportedValueWithReplacement(
          `${pathPrefix}.${key}`,
          defaultOpts[key]
        )
      );
    }
    // Instead of using destructuring to remove them from our sight, we
    // explicitly delete these here to have these keys declared in a single
    // place (the `uneditableOptionKeys` array) and prevent inconsistency.
    delete unsupported[key];
  }
  conversionErrors.push(
    ...unsupportedFieldErrorsFromObject('spec.options', unsupported)
  );

  const {
    default: defaultRecordingMode = '',
    ssh: sshRecordingMode = '',
    desktop: recordDesktopSessions = true,
    ...unsupportedRecordingOptions
  } = record_session || {};
  conversionErrors.push(
    ...unsupportedFieldErrorsFromObject(
      `${pathPrefix}.record_session`,
      unsupportedRecordingOptions
    )
  );

  const requireMFATypeOption = getOptionOrPushError(
    require_session_mfa,
    requireMFATypeOptionsMap,
    false,
    `${pathPrefix}.require_session_mfa`,
    conversionErrors
  );

  const createHostUserModeOption = getOptionOrPushError(
    create_host_user_mode,
    createHostUserModeOptionsMap,
    '',
    `${pathPrefix}.create_host_user_mode`,
    conversionErrors
  );

  const createDBUserModeOption = getOptionOrPushError(
    create_db_user_mode,
    createDBUserModeOptionsMap,
    '',
    `${pathPrefix}.create_db_user_mode`,
    conversionErrors
  );

  const defaultSessionRecordingModeOption = getOptionOrPushError(
    defaultRecordingMode,
    sessionRecordingModeOptionsMap,
    '',
    `${pathPrefix}.record_session.default`,
    conversionErrors
  );

  const sshSessionRecordingModeOption = getOptionOrPushError(
    sshRecordingMode,
    sessionRecordingModeOptionsMap,
    '',
    `${pathPrefix}.record_session.ssh`,
    conversionErrors
  );

  const {
    model: sshPortForwardingMode,
    conversionErrors: sshPortForwardingConversionErrors,
  } = portForwardingOptionsToModel(
    { ssh_port_forwarding, port_forwarding },
    `${pathPrefix}`
  );
  conversionErrors.push(...sshPortForwardingConversionErrors);

  return {
    model: {
      maxSessionTTL: max_session_ttl,
      clientIdleTimeout: client_idle_timeout,
      disconnectExpiredCert: disconnect_expired_cert,
      requireMFAType: requireMFATypeOption,
      createHostUserMode: createHostUserModeOption,
      createDBUser: create_db_user,
      createDBUserMode: createDBUserModeOption,
      desktopClipboard: desktop_clipboard,
      createDesktopUser: create_desktop_user,
      desktopDirectorySharing: desktop_directory_sharing,
      defaultSessionRecordingMode: defaultSessionRecordingModeOption,
      sshSessionRecordingMode: sshSessionRecordingModeOption,
      recordDesktopSessions,
      forwardAgent: forward_agent,
      sshPortForwardingMode: sshPortForwardingModeOptionsMap.get(
        sshPortForwardingMode
      ),
    },

    conversionErrors,
  };
}

export function portForwardingOptionsToModel(
  {
    ssh_port_forwarding,
    port_forwarding,
  }: Pick<RoleOptions, 'ssh_port_forwarding' | 'port_forwarding'>,
  pathPrefix: string
): {
  model: SSHPortForwardingMode;
  conversionErrors: ConversionError[];
} {
  if (ssh_port_forwarding) {
    const { local, remote, ...unsupported } = ssh_port_forwarding;
    if (!local || !remote) {
      return {
        model: '',
        conversionErrors: [
          {
            type: ConversionErrorType.UnsupportedValue,
            path: `${pathPrefix}.ssh_port_forwarding`,
          },
        ],
      };
    }

    const { enabled: localEnabled, ...localUnsupported } =
      ssh_port_forwarding.local;
    const { enabled: remoteEnabled, ...remoteUnsupported } =
      ssh_port_forwarding.remote;
    if (localEnabled === undefined || remoteEnabled === undefined) {
      return {
        model: '',
        conversionErrors: [
          {
            type: ConversionErrorType.UnsupportedValue,
            path: `${pathPrefix}.ssh_port_forwarding`,
          },
        ],
      };
    }

    const conversionErrors: ConversionError[] = [
      ...unsupportedFieldErrorsFromObject(
        `${pathPrefix}.ssh_port_forwarding`,
        unsupported
      ),
      ...unsupportedFieldErrorsFromObject(
        `${pathPrefix}.ssh_port_forwarding.local`,
        localUnsupported
      ),
      ...unsupportedFieldErrorsFromObject(
        `${pathPrefix}.ssh_port_forwarding.remote`,
        remoteUnsupported
      ),
    ];

    if (!localEnabled && !remoteEnabled) {
      return { model: 'none', conversionErrors };
    }
    if (localEnabled && !remoteEnabled) {
      return { model: 'local-only', conversionErrors };
    }
    if (!localEnabled && remoteEnabled) {
      return { model: 'remote-only', conversionErrors };
    }
    return { model: 'local-and-remote', conversionErrors };
  }
  if (port_forwarding !== undefined) {
    return {
      model: port_forwarding ? 'deprecated-on' : 'deprecated-off',
      conversionErrors: [],
    };
  }
  return { model: '', conversionErrors: [] };
}

/**
 * Converts a role editor model to a role. This operation is lossless.
 */
export function roleEditorModelToRole(roleModel: RoleEditorModel): Role {
  const { name, description, revision, labels, version, ...mRest } =
    roleModel.metadata;
  // Compile-time assert that protects us from silently losing fields.
  // `nameCollision` is the only field we don't care about, since its only use
  // is validation, and it's not expected to be included in the result.
  mRest satisfies { nameCollision: boolean };

  const role: Role = {
    kind: 'role',
    metadata: {
      name,
      description,
      revision,
      labels:
        labels.length > 0
          ? Object.fromEntries(labels.map(l => [l.name, l.value]))
          : undefined,
    },
    spec: {
      allow: {},
      deny: {},
      options: optionsModelToRoleOptions(version.value, roleModel.options),
    },
    version: version.value,
  };

  for (const res of roleModel.resources) {
    const { kind } = res;
    switch (kind) {
      case 'node':
        role.spec.allow.node_labels = labelsModelToLabels(res.labels);
        role.spec.allow.logins = optionsToStrings(res.logins);
        break;

      case 'kube_cluster':
        role.spec.allow.kubernetes_groups = optionsToStrings(res.groups);
        role.spec.allow.kubernetes_labels = labelsModelToLabels(res.labels);
        role.spec.allow.kubernetes_resources = res.resources.map(
          ({ kind, name, namespace, verbs, apiGroup }) => ({
            kind: kind.value,
            name,
            namespace,
            verbs: optionsToStrings(verbs),
            api_group: apiGroup,
          })
        );
        role.spec.allow.kubernetes_users = optionsToStrings(res.users);
        break;

      case 'app':
        role.spec.allow.app_labels = labelsModelToLabels(res.labels);
        role.spec.allow.aws_role_arns = res.awsRoleARNs;
        role.spec.allow.azure_identities = res.azureIdentities;
        role.spec.allow.gcp_service_accounts = res.gcpServiceAccounts;
        if (res.mcpTools.length > 0) {
          if (role.spec.allow.mcp === undefined) {
            role.spec.allow.mcp = {};
          }
          role.spec.allow.mcp.tools = res.mcpTools;
        }
        break;

      case 'db':
        role.spec.allow.db_labels = labelsModelToLabels(res.labels);
        role.spec.allow.db_names = optionsToStrings(res.names);
        role.spec.allow.db_users = optionsToStrings(res.users);
        role.spec.allow.db_roles = optionsToStrings(res.roles);
        role.spec.allow.db_service_labels = labelsModelToLabels(
          res.dbServiceLabels
        );
        break;

      case 'windows_desktop':
        role.spec.allow.windows_desktop_labels = labelsModelToLabels(
          res.labels
        );
        role.spec.allow.windows_desktop_logins = optionsToStrings(res.logins);
        break;

      case 'git_server':
        const orgs = optionsToStrings(res.organizations);
        if (orgs.length > 0) {
          role.spec.allow.github_permissions = [{ orgs }];
        }
        break;

      default:
        kind satisfies never;
    }
  }

  if (roleModel.rules.length > 0) {
    role.spec.allow.rules = roleModel.rules.map(role => ({
      resources: role.resources.map(r => r.value),
      verbs: role.allVerbs
        ? ['*']
        : role.verbs.filter(v => v.checked).map(v => v.verb),
      where: role.where || undefined,
    }));
  }

  return role;
}

/**
 * Converts a list of `LabelInput` value models to a set of labels, as
 * represented in the role resource.
 */
export function labelsModelToLabels(uiLabels: UILabel[]): Labels {
  const labels = {};
  for (const { name, value } of uiLabels) {
    if (!Object.hasOwn(labels, name)) {
      labels[name] = value;
    } else if (typeof labels[name] === 'string') {
      labels[name] = [labels[name], value];
    } else {
      labels[name].push(value);
    }
  }
  return labels;
}

function optionsModelToRoleOptions(
  roleVersion: RoleVersion,
  model: OptionsModel
): RoleOptions {
  const options = {
    ...defaultOptions(roleVersion),

    // Note: technically, coercing the optional fields to undefined is not
    // necessary, but it's easier to test it this way, since we achieve
    // symmetry between what goes into the model and what goes out of it, even
    // if some fields are optional.
    max_session_ttl: model.maxSessionTTL,
    client_idle_timeout: model.clientIdleTimeout || undefined,
    disconnect_expired_cert: model.disconnectExpiredCert || undefined,
    require_session_mfa: model.requireMFAType.value || undefined,
    create_host_user_mode: model.createHostUserMode.value || undefined,
    create_db_user: model.createDBUser,
    create_db_user_mode: model.createDBUserMode.value || undefined,
    desktop_clipboard: model.desktopClipboard,
    create_desktop_user: model.createDesktopUser,
    desktop_directory_sharing: model.desktopDirectorySharing,
    record_session: {
      default: model.defaultSessionRecordingMode.value || undefined,
      ssh: model.sshSessionRecordingMode.value || undefined,
      desktop: model.recordDesktopSessions,
    },
    forward_agent: model.forwardAgent,
  };

  const mode = model.sshPortForwardingMode.value;
  switch (mode) {
    case 'none':
      options.ssh_port_forwarding = {
        local: { enabled: false },
        remote: { enabled: false },
      };
      break;

    case 'local-only':
      options.ssh_port_forwarding = {
        local: { enabled: true },
        remote: { enabled: false },
      };
      break;

    case 'remote-only':
      options.ssh_port_forwarding = {
        local: { enabled: false },
        remote: { enabled: true },
      };
      break;

    case 'local-and-remote':
      options.ssh_port_forwarding = {
        local: { enabled: true },
        remote: { enabled: true },
      };
      break;

    case 'deprecated-off':
      options.port_forwarding = false;
      break;

    case 'deprecated-on':
      options.port_forwarding = true;
      break;

    default:
      mode satisfies '';
  }

  return options;
}

function optionsToStrings<T = string>(opts: readonly Option<T>[]): T[] {
  return opts.map(opt => opt.value);
}

/** Detects if fields were modified by comparing against the original role. */
export function hasModifiedFields(
  updated: RoleEditorModel,
  originalRole: Role
) {
  return !equalsDeep(roleEditorModelToRole(updated), originalRole, {
    ignoreUndefined: true,
  });
}
