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

export function createDiscoveryConfig(
  clusterId: string,
  req: DiscoveryConfig
): Promise<DiscoveryConfig> {
  return api
    .post(cfg.getDiscoveryConfigUrl(clusterId), req)
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

function makeAws(rawResp: AwsMatcher[]) {
  if (!rawResp) {
    return [];
  }

  return rawResp.map(a => ({
    types: a.types || [],
    regions: a.regions || [],
    tags: a.tags || {},
    integration: a.integration,
  }));
}
