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

import { Integration, IntegrationStatusCode } from './types';

export const integrationService = {
  fetchIntegration(clusterId: string, name: string): Promise<Integration> {
    return api
      .get(cfg.getIntegrationsUrl(clusterId, name))
      .then(makeIntegration);
  },

  fetchIntegrations(clusterId: string): Promise<Integration[]> {
    return api.get(cfg.getIntegrationsUrl(clusterId)).then(makeIntegrations);
  },
};

export function makeIntegrations(json: any): Integration[] {
  json = json || [];
  return json.map(user => makeIntegration(user));
}

function makeIntegration(json: any): Integration {
  json = json || {};
  const { name, subKind, awsoidc } = json;
  return {
    resourceType: 'integration',
    name,
    kind: subKind,
    spec: {
      roleArn: awsoidc?.roleArn,
    },
    // The integration resource does not have a "status" field, but is
    // a required field for the table that lists both plugin and
    // integration resources together. As discussed, the only
    // supported status for integration is `Running` for now:
    // https://github.com/gravitational/teleport/pull/22556#discussion_r1158674300
    statusCode: IntegrationStatusCode.Running,
  };
}
