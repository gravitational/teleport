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

import cfg from 'teleport/config';

import { hcl } from '../terraform';
import { AwsConfig, AwsLabel } from './types';

export type AwsDiscoverTerraformModuleConfig = {
  awsConfig: AwsConfig;
};

export const buildTerraformConfig = ({
  awsConfig,
}: AwsDiscoverTerraformModuleConfig): string => {
  const ec2Config = awsConfig.ec2Config;

  const matchAwsTypes = ec2Config.enabled ? ['ec2'] : null;

  const integrationName =
    awsConfig.integration.name.trim() || '<integration_name>';

  const isWildcard =
    awsConfig.regions.length === 1 && awsConfig.regions[0] === '*';

  const regions = isWildcard ? null : awsConfig.regions.sort();

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

  const ec2Matchers = ec2Config.enabled
    ? filteredMatchers(ec2Config.tags)
    : null;

  const tfModule = hcl`# Terraform Module
module "aws_discovery" {
  source = "../.."

  teleport_integration_name     = ${integrationName}
  teleport_cluster_name         = ${cfg.proxyCluster}
  teleport_proxy_public_addr    = ${cfg.proxyCluster + ':443'}
  teleport_discovery_group_name = "cloud-discovery-group"

  match_aws_types  = ${matchAwsTypes}

  match_aws_regions = ${regions}

  match_aws_tags = ${ec2Matchers}
}
`;

  return tfModule;
};
