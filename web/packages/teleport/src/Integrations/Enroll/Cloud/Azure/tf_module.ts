/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { parse as parseVersion } from 'shared/utils/semVer';

import cfg from 'teleport/config';

import { hcl, type TFObject, type TFValue } from '../terraform';
import {
  AzureManagedIdentity,
  AzureTag,
  VmConfig,
  ServiceConfig,
} from './types';

export type AzureDiscoverTerraformModuleConfig = {
  integrationName: string;
  version: string;
  vmConfig: VmConfig;
  managedIdentity: AzureManagedIdentity;
};

const TF_MODULE = '/teleport/discovery/azure';

const isStaging = (version: string): boolean => {
  const parsed = parseVersion(version);
  if (!parsed) return false;

  return parsed.prerelease.length > 0;
};

export const buildTerraformConfig = ({
  integrationName,
  version,
  vmConfig,
  managedIdentity,
}: AzureDiscoverTerraformModuleConfig): string => {
  const tfRegistry = isStaging(version)
    ? cfg.terraform.stagingRegistry
    : cfg.terraform.registry;

  const moduleSrc = `${tfRegistry}${TF_MODULE}`;

  const integrationNameOrNull = integrationName.trim() || null;

  const matcher = buildMatcher(vmConfig);

  const azureMatchers = matcher ? [matcher] : null;

  const resourceGroup = managedIdentity.resourceGroup.trim();

  const managedIdentityRegionOrNull = managedIdentity.region;

  const tfModule = hcl`# Terraform Module
module "azure_discovery" {
  source  = ${moduleSrc}
  version = ${version}

  teleport_integration_use_name_prefix = ${integrationNameOrNull ? false : null}

  teleport_proxy_public_addr    = ${cfg.proxyCluster + ':443'}
  teleport_discovery_group_name = "cloud-discovery-group"
  teleport_integration_name	    = ${integrationNameOrNull}

  # Name of an existing Azure Resource Group where
  # Azure resources will be created.
  azure_resource_group_name = ${resourceGroup}

  # Azure region (location) where the managed identity
  # will be created (e.g., "eastus")
  azure_managed_identity_location = ${managedIdentityRegionOrNull}

  azure_matchers = ${azureMatchers}
}
`;

  return tfModule;
};

const buildMatcher = (config: ServiceConfig): TFObject | null => {
  if (!config.enabled) return null;

  const matcher: { [key: string]: TFValue } = { types: [config.type] };

  // required by azure discovery module
  matcher.subscriptions = [...config.subscriptions].sort();

  if (config.resourceGroups?.length > 0) {
    matcher['resource_groups'] = [...config.resourceGroups].sort();
  }

  if (config.regions.length > 0 && !config.regions.some(r => r === '*')) {
    matcher.regions = [...config.regions].sort();
  }

  const tags = buildTagMap(config.tags);
  if (tags) {
    matcher.tags = tags;
  }

  return matcher;
};

const buildTagMap = (tags: AzureTag[]) => {
  const filtered = tags.filter(t => t.value && t.name);
  if (filtered.length === 0) return null;
  if (filtered.some(t => t.name === '*')) return null;

  const tagMap: Record<string, string[]> = {};
  filtered.forEach(tag => {
    if (!tagMap[tag.name]) {
      tagMap[tag.name] = [];
    }
    tagMap[tag.name].push(tag.value);
  });
  return tagMap;
};
