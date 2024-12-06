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
  AccessSpec,
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
  accessSpecs,
  rules,
}: RoleEditorModel) {
  return {
    metadata: validateMetadata(metadata),
    accessSpecs: accessSpecs.map(validateAccessSpec),
    rules: rules.map(validateAdminRule),
  };
}

function validateMetadata(model: MetadataModel): MetadataValidationResult {
  return runRules(model, metadataRules);
}

const metadataRules = { name: requiredField('Role name is required') };
export type MetadataValidationResult = RuleSetValidationResult<
  typeof metadataRules
>;

export function validateAccessSpec(
  spec: AccessSpec
): AccessSpecValidationResult {
  const { kind } = spec;
  switch (kind) {
    case 'kube_cluster':
      return runRules(spec, kubernetesValidationRules);
    case 'node':
      return runRules(spec, serverValidationRules);
    case 'app':
      return runRules(spec, appSpecValidationRules);
    case 'db':
      return runRules(spec, databaseSpecValidationRules);
    case 'windows_desktop':
      return runRules(spec, windowsDesktopSpecValidationRules);
    default:
      kind satisfies never;
  }
}

export type AccessSpecValidationResult =
  | ServerSpecValidationResult
  | KubernetesSpecValidationResult
  | AppSpecValidationResult
  | DatabaseSpecValidationResult
  | WindowsDesktopSpecValidationResult;

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

const kubernetesValidationRules = {
  labels: nonEmptyLabels,
  resources: arrayOf(validKubernetesResource),
};
export type KubernetesSpecValidationResult = RuleSetValidationResult<
  typeof kubernetesValidationRules
>;

const noWildcard = (message: string) => (value: string) => () => {
  const valid = value !== '*';
  return { valid, message: valid ? '' : message };
};

const noWildcardOptions = (message: string) => (options: Option[]) => () => {
  const valid = options.every(o => o.value !== '*');
  return { valid, message: valid ? '' : message };
};

const serverValidationRules = {
  labels: nonEmptyLabels,
  logins: noWildcardOptions('Wildcard is not allowed in logins'),
};
export type ServerSpecValidationResult = RuleSetValidationResult<
  typeof serverValidationRules
>;

const appSpecValidationRules = {
  labels: nonEmptyLabels,
  awsRoleARNs: arrayOf(noWildcard('Wildcard is not allowed in AWS role ARNs')),
  azureIdentities: arrayOf(
    noWildcard('Wildcard is not allowed in Azure identities')
  ),
  gcpServiceAccounts: arrayOf(
    noWildcard('Wildcard is not allowed in GCP service accounts')
  ),
};
export type AppSpecValidationResult = RuleSetValidationResult<
  typeof appSpecValidationRules
>;

const databaseSpecValidationRules = {
  labels: nonEmptyLabels,
  roles: noWildcardOptions('Wildcard is not allowed in database roles'),
};
export type DatabaseSpecValidationResult = RuleSetValidationResult<
  typeof databaseSpecValidationRules
>;

const windowsDesktopSpecValidationRules = {
  labels: nonEmptyLabels,
};
export type WindowsDesktopSpecValidationResult = RuleSetValidationResult<
  typeof windowsDesktopSpecValidationRules
>;

export const validateAdminRule = (adminRule: RuleModel) =>
  runRules(adminRule, adminRuleValidationRules);

const adminRuleValidationRules = {
  resources: requiredField('At least one resource kind is required'),
  verbs: requiredField('At least one permission is required'),
};
export type AdminRuleValidationResult = RuleSetValidationResult<
  typeof adminRuleValidationRules
>;
