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

import {
  arrayOf,
  requiredField,
  RuleSetValidationResult,
  runRules,
  ValidationResult,
} from 'shared/components/Validation/rules';

import { Option } from 'shared/components/Select';

import { KubernetesResourceKind } from 'teleport/services/resources';

import { nonEmptyLabels } from 'teleport/components/LabelsInput/LabelsInput';

import {
  ResourceAccess,
  KubernetesResourceModel,
  MetadataModel,
  RoleEditorModel,
  RuleModel,
} from './standardmodel';

const kubernetesClusterWideResourceKinds: KubernetesResourceKind[] = [
  'namespace',
  'kube_node',
  'persistentvolume',
  'clusterrole',
  'clusterrolebinding',
  'certificatesigningrequest',
];

export function validateRoleEditorModel({
  metadata,
  resources,
  rules,
}: RoleEditorModel) {
  return {
    metadata: validateMetadata(metadata),
    resources: resources.map(validateResourceAccess),
    rules: rules.map(validateAccessRule),
  };
}

function validateMetadata(model: MetadataModel): MetadataValidationResult {
  return runRules(model, metadataRules);
}

const metadataRules = {
  name: requiredField('Role name is required'),
  labels: nonEmptyLabels,
};
export type MetadataValidationResult = RuleSetValidationResult<
  typeof metadataRules
>;

export function validateResourceAccess(
  res: ResourceAccess
): ResourceAccessValidationResult {
  const { kind } = res;
  switch (kind) {
    case 'kube_cluster':
      return runRules(res, kubernetesAccessValidationRules);
    case 'node':
      return runRules(res, serverAccessValidationRules);
    case 'app':
      return runRules(res, appAccessValidationRules);
    case 'db':
      return runRules(res, databaseAccessValidationRules);
    case 'windows_desktop':
      return runRules(res, windowsDesktopAccessValidationRules);
    default:
      kind satisfies never;
  }
}

export type ResourceAccessValidationResult =
  | ServerAccessValidationResult
  | KubernetesAccessValidationResult
  | AppAccessValidationResult
  | DatabaseAccessValidationResult
  | WindowsDesktopAccessValidationResult;

const validKubernetesResource = (res: KubernetesResourceModel) => () => {
  const name = requiredField(
    'Resource name is required, use "*" for any resource'
  )(res.name)();
  const namespace = kubernetesClusterWideResourceKinds.includes(res.kind.value)
    ? { valid: true }
    : requiredField('Namespace is required for resources of this kind')(
        res.namespace
      )();
  return {
    valid: name.valid && namespace.valid,
    name,
    namespace,
  };
};
export type KubernetesResourceValidationResult = {
  name: ValidationResult;
  namespace: ValidationResult;
};

const kubernetesAccessValidationRules = {
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

const serverAccessValidationRules = {
  labels: nonEmptyLabels,
  logins: noWildcardOptions('Wildcard is not allowed in logins'),
};
export type ServerAccessValidationResult = RuleSetValidationResult<
  typeof serverAccessValidationRules
>;

const appAccessValidationRules = {
  labels: nonEmptyLabels,
  awsRoleARNs: arrayOf(noWildcard('Wildcard is not allowed in AWS role ARNs')),
  azureIdentities: arrayOf(
    noWildcard('Wildcard is not allowed in Azure identities')
  ),
  gcpServiceAccounts: arrayOf(
    noWildcard('Wildcard is not allowed in GCP service accounts')
  ),
};
export type AppAccessValidationResult = RuleSetValidationResult<
  typeof appAccessValidationRules
>;

const databaseAccessValidationRules = {
  labels: nonEmptyLabels,
  roles: noWildcardOptions('Wildcard is not allowed in database roles'),
};
export type DatabaseAccessValidationResult = RuleSetValidationResult<
  typeof databaseAccessValidationRules
>;

const windowsDesktopAccessValidationRules = {
  labels: nonEmptyLabels,
};
export type WindowsDesktopAccessValidationResult = RuleSetValidationResult<
  typeof windowsDesktopAccessValidationRules
>;

export const validateAccessRule = (accessRule: RuleModel) =>
  runRules(accessRule, accessRuleValidationRules);

const accessRuleValidationRules = {
  resources: requiredField('At least one resource kind is required'),
  verbs: requiredField('At least one permission is required'),
};
export type AccessRuleValidationResult = RuleSetValidationResult<
  typeof accessRuleValidationRules
>;
