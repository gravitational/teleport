/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
import { Regions as AwsRegion } from 'teleport/services/integrations';

import { hcl } from '../terraform';
import { AwsLabel, Ec2Config, WildcardRegion } from './types';

export type AwsDiscoverTerraformModuleConfig = {
  integrationName: string;
  regions: WildcardRegion | AwsRegion[];
  ec2Config: Ec2Config;
  version: string;
};

const TF_MODULE = '/teleport/discovery/aws';

const isStaging = (version: string): boolean => {
  const parsed = parseVersion(version);
  if (!parsed) return false;

  return parsed.prerelease.length > 0;
};

export const buildTerraformConfig = ({
  integrationName,
  regions,
  ec2Config,
  version,
}: AwsDiscoverTerraformModuleConfig): string => {
  const tfRegistry = isStaging(version)
    ? cfg.terraform.stagingRegistry
    : cfg.terraform.registry;

  const moduleSrc = `${tfRegistry}${TF_MODULE}`;

  const matchAwsTypes = ec2Config.enabled ? ['ec2'] : null;

  const integrationNameOrNull = integrationName.trim() || null;

  const isWildcardRegion = regions.length === 1 && regions[0] === '*';

  const regionsOrNull = isWildcardRegion ? null : [...regions].sort();

  const filteredMatchers = (tags: AwsLabel[]) => {
    const filtered = tags.filter(o => o.value && o.name);
    if (filtered.length === 0) return null;

    const tagMap: Record<string, string[]> = {};
    filtered.forEach(tag => {
      if (!tagMap[tag.name]) {
        tagMap[tag.name] = [];
      }
      tagMap[tag.name].push(tag.value);
    });
    return tagMap;
  };

  const matchers = ec2Config.enabled ? filteredMatchers(ec2Config.tags) : null;

  const isWildcardMatcher = matchers && matchers['*']?.includes('*');

  const ec2Matchers = ec2Config.enabled && !isWildcardMatcher ? matchers : null;

  const tfModule = hcl`# Terraform Module
module "aws_discovery" {
  source  = ${moduleSrc}
  version = ${version}

  teleport_integration_use_name_prefix = ${integrationNameOrNull ? false : null}

  teleport_proxy_public_addr    = ${cfg.proxyCluster + ':443'}
  teleport_discovery_group_name = "cloud-discovery-group"
  teleport_integration_name	    = ${integrationNameOrNull}

  match_aws_resource_types = ${matchAwsTypes}

  match_aws_regions = ${regionsOrNull}

  match_aws_tags = ${ec2Matchers}
}
`;

  return tfModule;
};
