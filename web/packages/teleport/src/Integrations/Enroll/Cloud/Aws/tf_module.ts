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

import { hcl, teleportProviderConfig, TFObject } from '../terraform';
import {
  AwsLabel,
  AwsMatcher,
  AwsOrganizationalUnits,
  DatabaseServiceConfig,
} from './types';

export type AwsDiscoverTerraformModuleConfig = {
  integrationName: string;
  matchers: AwsMatcher[];
  version: string;
  orgUnits?: AwsOrganizationalUnits | null;
  databaseService?: DatabaseServiceConfig;
};

const TF_DISCOVERY_MODULE = '/teleport/discovery/aws';
const TF_AGENT_MODULE = '/teleport/agent/aws';

const AWS_PROVIDER = `
# The module only creates IAM resources, which are global.
provider "aws" {
  region = "us-east-1"
}

`;

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

  if (matcher.type === 'eks' && matcher.kubeAppDiscovery === true) {
    obj.kube_app_discovery = true;
  }

  return obj;
};

const buildOrgDiscovery = (
  orgUnits?: AwsOrganizationalUnits | null
): TFObject | null => {
  if (!orgUnits) return null;

  const include = orgUnits.include.filter(s => s.trim());
  const exclude = orgUnits.exclude.filter(s => s.trim());

  const organizationalUnits: TFObject = {
    include,
  };

  if (exclude.length > 0) {
    organizationalUnits.exclude = exclude;
  }

  return { organizational_units: organizationalUnits };
};

export const buildTerraformConfig = ({
  integrationName,
  matchers,
  version,
  orgUnits,
  databaseService,
}: AwsDiscoverTerraformModuleConfig): string => {
  const tfRegistry = isStaging(version)
    ? cfg.terraform.stagingRegistry
    : cfg.terraform.registry;

  const moduleSrc = `${tfRegistry}${TF_DISCOVERY_MODULE}`;

  const integrationNameOrNull = integrationName.trim() || null;

  const awsMatchers = matchers.length > 0 ? matchers.map(buildTfMatcher) : null;

  const awsOrgDiscovery = buildOrgDiscovery(orgUnits);

  const orgOutput = awsOrgDiscovery
    ? hcl`
output "aws_child_account_iam_role_template" {
  value = module.aws_discovery.aws_child_account_iam_role_template
}
`
    : '';

  const proxyAddr = cfg.proxyCluster + ':443';

  const tfModule = hcl`# Terraform Module
module "aws_discovery" {
  source  = ${moduleSrc}
  version = ${version}

  teleport_integration_use_name_prefix = ${integrationNameOrNull ? false : null}

  teleport_proxy_public_addr    = ${proxyAddr}
  teleport_discovery_group_name = "cloud-discovery-group"
  teleport_integration_name     = ${integrationNameOrNull}

  # Discover resources across all accounts in the AWS Organization,
  # filtered by Organizational Units.
  aws_organization_discovery = ${awsOrgDiscovery}

  aws_matchers = ${awsMatchers}
}
`;

  let dbAgentModule = '';
  if (databaseService?.deploy) {
    const agentModuleSrc = `${tfRegistry}${TF_AGENT_MODULE}`;

    const agentsByRegionAndVpc: Record<
      string,
      {
        region: string;
        vpc_id: string;
        security_group_ids: string[];
      }
    > = {};
    databaseService.agents.forEach(agent => {
      let newSgis = agent.securityGroupIds.map(id => id.trim());
      const region = agent.region.trim();
      const vpcId = agent.vpcId.trim();
      const key = `${region}/${vpcId}`;
      const existingAgent = agentsByRegionAndVpc[key];
      if (existingAgent) {
        newSgis = [...existingAgent.security_group_ids, ...newSgis];
      }
      agentsByRegionAndVpc[key] = {
        region,
        vpc_id: vpcId,
        security_group_ids: [...new Set(newSgis)],
      };
    });

    dbAgentModule = hcl`
# Added because you selected "Deploy a new Database Service for me".
# Teleport needs network access to reach these databases.
module "db_agent" {
  for_each = ${agentsByRegionAndVpc}

  source  = ${agentModuleSrc}
  version = ${version}

  teleport_proxy_public_addr = ${proxyAddr}

  region = each.value.region
  vpc_id = each.value.vpc_id
  database_service = {
    enabled                           = true
    default_iam_policy_database_types = ["rds"]
    database_security_group_ids       = each.value.security_group_ids
  }
}
`;
  }

  return (
    teleportProviderConfig({ proxyAddr, clusterVersion: version }) +
    AWS_PROVIDER +
    tfModule +
    dbAgentModule +
    orgOutput
  );
};
