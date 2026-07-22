/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
import api from 'teleport/services/api';

import { AwsMatcher, DiscoveryConfig } from './types';

// the backend expected hardcoded value for field `discoveryGroup`
// when creating a discovery config.
export const DISCOVERY_GROUP_CLOUD = 'cloud-discovery-group';

export const DEFAULT_DISCOVERY_GROUP_NON_CLOUD = 'aws-prod';

export function createDiscoveryConfig(
  clusterId: string,
  req: DiscoveryConfig
): Promise<DiscoveryConfig> {
  return api
    .post(cfg.getDiscoveryConfigUrl(clusterId), {
      name: req.name,
      discoveryGroup: req.discoveryGroup,
      aws: makeAwsMatchersReq(req.aws),
    })
    .then(makeDiscoveryConfig);
}

export function makeDiscoveryConfig(rawResp: DiscoveryConfig): DiscoveryConfig {
  const { name, discoveryGroup, aws } = rawResp;

  return {
    name,
    discoveryGroup,
    aws: makeAws(aws),
  };
}

function makeAws(rawAwsMatchers): AwsMatcher[] {
  if (!rawAwsMatchers) {
    return [];
  }

  return rawAwsMatchers.map(a => ({
    types: a.types || [],
    regions: a.regions || [],
    tags: a.tags || {},
    integration: a.integration,
    kubeAppDiscovery: !!a.kube_app_discovery,
  }));
}

function makeAwsMatchersReq(inputMatchers: AwsMatcher[]) {
  if (!inputMatchers) {
    return [];
  }

  return inputMatchers.map(a => ({
    types: a.types || [],
    regions: a.regions || [],
    tags: a.tags || {},
    integration: a.integration,
    kube_app_discovery: !!a.kubeAppDiscovery,
    ssm: a.ssm ? { document_name: a.ssm.documentName } : undefined,
    install: a.install
      ? {
          enroll_mode: a.install.enrollMode,
          install_teleport: a.install.installTeleport,
          join_token: a.install.joinToken,
        }
      : undefined,
  }));
}
