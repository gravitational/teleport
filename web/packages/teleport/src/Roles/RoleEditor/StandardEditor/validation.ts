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
import {
  arrayOf,
  requiredField,
  RuleSetValidationResult,
  runRules,
  ValidationResult,
} from 'shared/components/Validation/rules';

import { nonEmptyLabels } from 'teleport/components/LabelsInput/LabelsInput';
import { RoleVersion } from 'teleport/services/resources';

import {
  AppAccess,
  DatabaseAccess,
  KubernetesAccess,
  kubernetesResourceKindV7Groups,
  KubernetesResourceModel,
  KubernetesVerbOption,
  MetadataModel,
  ResourceAccess,
  ResourceKindOption,
  resourceKindOptions,
  RoleEditorModel,
  RuleModel,
  ServerAccess,
  supportsKubernetesCustomResources,
  VerbModel,
  WindowsDesktopAccess,
} from './standardmodel';

export const v7kubernetesClusterWideResourceKinds: string[] = [
  'namespace',
  'kube_node',
  'persistentvolume',
  'clusterrole',
  'clusterrolebinding',
  'certificatesigningrequest',
];

export type RoleEditorModelValidationResult = {
  metadata: MetadataValidationResult;
  resources: ResourceAccessValidationResult[];
  rules: AdminRuleValidationResult[];
  /**
   * isValid is true if all the fields in the validation result are valid.
   */
  isValid: boolean;
};

/**
 * Validates the role editor model. In addition to the model itself, this
 * function also takes the previous model and previous validation result. The
 * intention here is to only return a newly created result if the previous
 * model is indeed different. This pattern is then repeated in other validation
 * functions.
 *
 * The purpose of this is less about the performance of the validation process
 * itself, and more about enabling memoization-based rendering optimizations:
 * UI components that take either entire or partial validation results can be
 * cached if the validation results don't change.
 *
 * Note that we can't use `useMemo` here, because `validateRoleEditorModel` is
 * called from the state reducer. Also `highbar.memoize` was not applicable, as
 * it caches an arbitrary amount of previous results.
 */
export function validateRoleEditorModel(
  model: RoleEditorModel,
  previousModel: RoleEditorModel | undefined,
  previousResult: RoleEditorModelValidationResult | undefined
): RoleEditorModelValidationResult {
  const metadataResult = validateMetadata(
    model.metadata,
    previousModel?.metadata,
    previousResult?.metadata
  );

  const resourcesResult = validateResourceAccessList(
    model.resources,
    previousModel?.resources,
    previousResult?.resources
  );

  const rulesResult = validateAdminRuleList(
    model.rules,
    previousModel?.rules,
    previousResult?.rules
  );

  return {
    isValid:
      metadataResult.valid &&
      resourcesResult.every(r => r.valid) &&
      rulesResult.every(r => r.valid),
    metadata: metadataResult,
    resources: resourcesResult,
    rules: rulesResult,
  };
}

function validateMetadata(
  model: MetadataModel,
  previousModel: MetadataModel,
  previousResult: MetadataValidationResult
): MetadataValidationResult {
  if (previousModel === model) {
    return previousResult;
  }
  return runRules(model, metadataRules);
}

const mustBeFalse = (message: string) => (value: boolean) => () => ({
  valid: !value,
  message: value ? message : '',
});

const metadataRules = {
  name: requiredField('Role name is required'),
  nameCollision: mustBeFalse('Role with this name already exists'),
  labels: nonEmptyLabels,
};
export type MetadataValidationResult = RuleSetValidationResult<
  typeof metadataRules
>;

function validateResourceAccessList(
  resources: ResourceAccess[],
  previousResources: ResourceAccess[],
  previousResults: ResourceAccessValidationResult[]
): ResourceAccessValidationResult[] {
  if (previousResources === resources) {
    return previousResults;
  }
  return resources.map((res, i) =>
    validateResourceAccess(res, previousResources?.[i], previousResults?.[i])
  );
}

export function validateResourceAccess(
  resource: ResourceAccess,
  previousResource: ResourceAccess,
  previousResult: ResourceAccessValidationResult
): ResourceAccessValidationResult {
  if (resource === previousResource) {
    return previousResult;
  }

  const { kind } = resource;
  switch (kind) {
    case 'kube_cluster':
      return validateKubernetesAccess(resource);
    case 'node':
      return validateServerAccess(resource);
    case 'app':
      return validateAppAccess(resource);
    case 'db':
      return validateDatabaseAccess(resource);
    case 'windows_desktop':
      return validateWindowsDesktopAccess(resource);
    case 'git_server':
      return runRules(resource, gitHubOrganizationAccessValidationRules);
    default:
      kind satisfies never;
  }
}

export type ResourceAccessValidationResult =
  | ServerAccessValidationResult
  | KubernetesAccessValidationResult
  | AppAccessValidationResult
  | DatabaseAccessValidationResult
  | WindowsDesktopAccessValidationResult
  | GitHubOrganizationAccessValidationResult;

const validKubernetesResource = (res: KubernetesResourceModel) => () => {
  const kind = validKubernetesKind(
    res.kind.value,
    res.apiGroup,
    res.roleVersion
  );
  const name = requiredField(
    'Resource name is required, use "*" for any resource'
  )(res.name)();
  const namespace = validKubernetesNamespace(res);
  const verbs = validKubernetesVerbs(res.verbs);
  const apiGroup = validKubernetesGroup(res.apiGroup, res.roleVersion);

  return {
    valid:
      kind.valid &&
      name.valid &&
      namespace.valid &&
      verbs.valid &&
      apiGroup.valid,
    kind,
    name,
    namespace,
    verbs,
    apiGroup,
  };
};

export type KubernetesResourceValidationResult = {
  kind: ValidationResult;
  name: ValidationResult;
  namespace: ValidationResult;
  verbs: ValidationResult;
  apiGroup: ValidationResult;
};

// Best-effort validation for namespace requirement.
// Generated from `kubectl api-resources --namespaced=true -o name --sort-by=name`
const v8kubernetesNamespacedResourceKinds = [
  'bindings',
  'configmaps',
  'controllerrevisions.apps',
  'cronjobs.batch',
  'csistoragecapacities.storage.k8s.io',
  'daemonsets.apps',
  'deployments.apps',
  'endpoints',
  'endpointslices.discovery.k8s.io',
  'events.events.k8s.io',
  'events',
  'horizontalpodautoscalers.autoscaling',
  'ingresses.networking.k8s.io',
  'jobs.batch',
  'leases.coordination.k8s.io',
  'limitranges',
  'localsubjectaccessreviews.authorization.k8s.io',
  'networkpolicies.networking.k8s.io',
  'persistentvolumeclaims',
  'poddisruptionbudgets.policy',
  'pods',
  'podtemplates',
  'replicasets.apps',
  'replicationcontrollers',
  'resourcequotas',
  'rolebindings.rbac.authorization.k8s.io',
  'roles.rbac.authorization.k8s.io',
  'secrets',
  'serviceaccounts',
  'services',
  'statefulsets.apps',
];

const validKubernetesNamespace = (
  res: KubernetesResourceModel
): ValidationResult => {
  const supportsCrds = supportsKubernetesCustomResources(res.roleVersion);

  // If we don't support CRDs we validate against the v7 cluster wide values.
  if (!supportsCrds) {
    return v7kubernetesClusterWideResourceKinds.includes(res.kind.value)
      ? { valid: true }
      : requiredField('Namespace is required for resources of this kind')(
          res.namespace
        )();
  }

  // Otherwise we validate against the v8 namespaced values.
  const kindGroup = !res.apiGroup
    ? res.kind.value
    : `res.kind.value.${res.apiGroup}`;
  if (
    !res.namespace &&
    v8kubernetesNamespacedResourceKinds.includes(kindGroup)
  ) {
    return requiredField('Namespace is required for resources of this kind')(
      res.namespace
    )();
  }

  // If we didn't have a match, it doesn't mean it is valid, but we can't known about
  // it at this point.
  return { valid: true };
};

/**
 * Validates a `kind` field of a `KubernetesResourceModel`. In roles with
 * version v6, the auth server allows only specifying access for `pod`
 * Kubernetes resources.
 */
const validKubernetesKind = (
  kind: string,
  apiGroup: string | undefined,
  ver: RoleVersion
): ValidationResult => {
  switch (ver) {
    case RoleVersion.V3:
    case RoleVersion.V4:
    case RoleVersion.V5:
    case RoleVersion.V6:
      const v6valid = kind === 'pod';
      return {
        valid: v6valid,
        message: v6valid
          ? undefined
          : `Only pods are allowed for role version ${ver}`,
      };

    case RoleVersion.V7:
      // NOTE: We need to validate in case the user switches between role versions.
      // Valid values in rolev8 could be invalid in older versions.
      const v7valid = resourceKindOptions.some(elem => elem.value === kind);

      return {
        valid: v7valid,
        message: v7valid
          ? undefined
          : `Only core predefined kinds are allowed for role version ${ver}`,
      };

    case RoleVersion.V8:
      const v7groups = kubernetesResourceKindV7Groups[kind];
      const v8valid =
        !v7groups || (apiGroup !== '*' && !v7groups.groups.includes(apiGroup));
      return {
        valid: v8valid,
        message: v8valid
          ? undefined
          : `Kind must use k8s plural name. Did you mean "${v7groups.v8name}"?`,
      };

    default:
      ver satisfies never;
      return { valid: true };
  }
};

/**
 * Validates a `group` field of a `KubernetesResourceModel`. In roles with
 * version prior to v8, the auth server only accepts empty string or "*".
 */
const validKubernetesGroup = (
  group: string,
  ver: RoleVersion
): ValidationResult => {
  switch (ver) {
    case RoleVersion.V3:
    case RoleVersion.V4:
    case RoleVersion.V5:
    case RoleVersion.V6:
    case RoleVersion.V7:
      const v7valid = !group;

      return {
        valid: v7valid,
        message: v7valid
          ? undefined
          : `API Group not supported for role version ${ver}.`,
      };

    case RoleVersion.V8:
      const v8valid = !!group;
      return {
        valid: v8valid,
        message: v8valid
          ? undefined
          : `API Group required. Use "*" for any group.`,
      };

    default:
      ver satisfies never;
      return { valid: true };
  }
};

const validKubernetesVerbs = (
  verbs: readonly KubernetesVerbOption[]
): ValidationResult => {
  // Don't allow mixing '*' and other resource types.
  const valid = verbs.length < 2 || verbs.every(v => v.value !== '*');
  return {
    valid,
    message: valid
      ? undefined
      : 'Mixing "All verbs" with other options is not allowed',
  };
};

const validateKubernetesAccess = (
  a: KubernetesAccess
): KubernetesAccessValidationResult => {
  const result = runRules(a, kubernetesAccessValidationRules);
  if (
    a.groups.length === 0 &&
    a.users.length === 0 &&
    a.labels.length === 0 &&
    a.resources.length === 0
  ) {
    result.valid = false;
    result.message = 'At least one group, user, label, or resource required';
    result.fields.groups.valid = false;
    result.fields.users.valid = false;
    result.fields.labels.valid = false;
    result.fields.resources.valid = false;
  }
  return result;
};

const alwaysValid = () => () => ({ valid: true });

const kubernetesAccessValidationRules = {
  groups: alwaysValid,
  users: alwaysValid,
  labels: nonEmptyLabels,
  resources: arrayOf(validKubernetesResource),
};
export type KubernetesAccessValidationResult = RuleSetValidationResult<
  typeof kubernetesAccessValidationRules
>;

const noWildcard = (message: string) => (value: string) => () => {
  const valid = value !== '*';
  return { valid, message: valid ? '' : message };
};

const noWildcardOptions = (message: string) => (options: Option[]) => () => {
  const valid = options.every(o => o.value !== '*');
  return { valid, message: valid ? '' : message };
};

const validateServerAccess = (
  a: ServerAccess
): ServerAccessValidationResult => {
  const result = runRules(a, serverAccessValidationRules);
  if (a.labels.length === 0 && a.logins.length === 0) {
    result.valid = false;
    result.message = 'At least one label or login required';
    result.fields.labels.valid = false;
    result.fields.logins.valid = false;
  }
  return result;
};

const serverAccessValidationRules = {
  labels: nonEmptyLabels,
  logins: noWildcardOptions('Wildcard is not allowed in logins'),
};
export type ServerAccessValidationResult = RuleSetValidationResult<
  typeof serverAccessValidationRules
>;

const validateAppAccess = (a: AppAccess): AppAccessValidationResult => {
  const result = runRules(a, appAccessValidationRules);
  if (
    a.labels.length === 0 &&
    a.awsRoleARNs.length === 0 &&
    a.azureIdentities.length === 0 &&
    a.gcpServiceAccounts.length === 0 &&
    a.mcpTools.length === 0
  ) {
    result.valid = false;
    result.message =
      'At least one label, AWS role ARN, Azure identity, GCP service account, or MCP tools required';
    result.fields.labels.valid = false;
    result.fields.awsRoleARNs.valid = false;
    result.fields.azureIdentities.valid = false;
    result.fields.gcpServiceAccounts.valid = false;
    result.fields.mcpTools.valid = false;
  }
  return result;
};

const appAccessValidationRules = {
  labels: nonEmptyLabels,
  awsRoleARNs: arrayOf(noWildcard('Wildcard is not allowed in AWS role ARNs')),
  azureIdentities: arrayOf(
    noWildcard('Wildcard is not allowed in Azure identities')
  ),
  gcpServiceAccounts: arrayOf(
    noWildcard('Wildcard is not allowed in GCP service accounts')
  ),
  mcpTools: alwaysValid,
};
export type AppAccessValidationResult = RuleSetValidationResult<
  typeof appAccessValidationRules
>;

const validateDatabaseAccess = (
  a: DatabaseAccess
): DatabaseAccessValidationResult => {
  const result = runRules(a, databaseAccessValidationRules);
  if (
    a.labels.length === 0 &&
    a.names.length === 0 &&
    a.users.length === 0 &&
    a.roles.length === 0 &&
    a.dbServiceLabels.length === 0
  ) {
    result.valid = false;
    result.message =
      'At least one label, database name, user, role, or service label required';
    result.fields.labels.valid = false;
    result.fields.names.valid = false;
    result.fields.users.valid = false;
    result.fields.roles.valid = false;
    result.fields.dbServiceLabels.valid = false;
  }
  return result;
};

const databaseAccessValidationRules = {
  labels: nonEmptyLabels,
  names: alwaysValid,
  users: alwaysValid,
  roles: noWildcardOptions('Wildcard is not allowed in database roles'),
  dbServiceLabels: nonEmptyLabels,
};
export type DatabaseAccessValidationResult = RuleSetValidationResult<
  typeof databaseAccessValidationRules
>;

const validateWindowsDesktopAccess = (
  a: WindowsDesktopAccess
): WindowsDesktopAccessValidationResult => {
  const result = runRules(a, windowsDesktopAccessValidationRules);
  if (a.labels.length === 0 && a.logins.length === 0) {
    result.valid = false;
    result.message = 'At least one label or login required';
    result.fields.labels.valid = false;
    result.fields.logins.valid = false;
  }
  return result;
};

const windowsDesktopAccessValidationRules = {
  labels: nonEmptyLabels,
  logins: alwaysValid,
};
export type WindowsDesktopAccessValidationResult = RuleSetValidationResult<
  typeof windowsDesktopAccessValidationRules
>;

const gitHubOrganizationAccessValidationRules = {
  organizations: requiredField<Option>('At least one organization required'),
};
export type GitHubOrganizationAccessValidationResult = RuleSetValidationResult<
  typeof gitHubOrganizationAccessValidationRules
>;

export function validateAdminRuleList(
  rules: RuleModel[],
  previousRules: RuleModel[],
  previousResults: AdminRuleValidationResult[]
): AdminRuleValidationResult[] {
  if (previousRules === rules) {
    return previousResults;
  }
  return rules.map((rule, i) =>
    validateAdminRule(rule, previousRules?.[i], previousResults?.[i])
  );
}

export const validateAdminRule = (
  rule: RuleModel,
  previousRule: RuleModel,
  previousResult: AdminRuleValidationResult
): AdminRuleValidationResult => {
  if (previousRule === rule) {
    return previousResult;
  }
  return runRules(rule, adminRuleValidationRules);
};

/** Ensures that at least one verb is checked. */
const requiredVerbs = (message: string) => (verbs: VerbModel[]) => () => {
  const valid = verbs.some(v => v.checked);
  return { valid, message: valid ? '' : message };
};

const adminRuleValidationRules = {
  resources: requiredField<ResourceKindOption>(
    'At least one resource kind is required'
  ),
  verbs: requiredVerbs('At least one permission is required'),
};
export type AdminRuleValidationResult = RuleSetValidationResult<
  typeof adminRuleValidationRules
>;
