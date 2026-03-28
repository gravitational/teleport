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

import { hcl, TFObject } from '../terraform';
import { AwsLabel, AwsMatcher } from './types';

export type AwsDiscoverTerraformModuleConfig = {
  integrationName: string;
  matchers: AwsMatcher[];
  version: string;
};

const TF_MODULE = '/teleport/discovery/aws';

const isStaging = (version: string): boolean => {
  const parsed = parseVersion(version);
  if (!parsed) return false;

  return parsed.prerelease.length > 0;
};

const buildTagMap = (tags: AwsLabel[]): Record<string, string[]> | null => {
  const filtered = tags.filter(o => o.value && o.name);
  if (filtered.length === 0) return null;

  const tagMap: Record<string, string[]> = {};
  filtered.forEach(tag => {
    if (!tagMap[tag.name]) {
      tagMap[tag.name] = [];
    }
    tagMap[tag.name].push(tag.value);
  });

  if (tagMap['*']?.includes('*')) return null;

  return tagMap;
};

const buildTfMatcher = (matcher: AwsMatcher): TFObject => {
  const obj: TFObject = { types: [matcher.type] };

  if (matcher.regions.length > 0) {
    obj.regions = [...matcher.regions].sort();
  }

  const tags = buildTagMap(matcher.tags);
  if (tags) {
    obj.tags = tags;
  }

  return obj;
};

export const buildTerraformConfig = ({
  integrationName,
  matchers,
  version,
}: AwsDiscoverTerraformModuleConfig): string => {
  const tfRegistry = isStaging(version)
    ? cfg.terraform.stagingRegistry
    : cfg.terraform.registry;

  const moduleSrc = `${tfRegistry}${TF_MODULE}`;

  const integrationNameOrNull = integrationName.trim() || null;

  const awsMatchers = matchers.length > 0 ? matchers.map(buildTfMatcher) : null;

  const tfModule = hcl`# Terraform Module
module "aws_discovery" {
  source  = ${moduleSrc}
  version = ${version}

  teleport_integration_use_name_prefix = ${integrationNameOrNull ? false : null}

  teleport_proxy_public_addr    = ${cfg.proxyCluster + ':443'}
  teleport_discovery_group_name = "cloud-discovery-group"
  teleport_integration_name	    = ${integrationNameOrNull}

  aws_matchers = ${awsMatchers}
}
`;

  return tfModule;
};
