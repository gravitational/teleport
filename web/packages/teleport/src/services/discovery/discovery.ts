/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import api from 'teleport/services/api';
import cfg from 'teleport/config';

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
