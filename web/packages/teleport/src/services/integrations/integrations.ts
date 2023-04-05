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

import { Integration, IntegrationCode } from './types';

export const integrationService = {
  fetchIntegrations(): Promise<Integration[]> {
    return api.get(cfg.getIntegrationsUrl()).then(makeIntegrations);
  },
};

export function makeIntegrations(json: any): Integration[] {
  json = json || [];
  return json.map(user => makeIntegration(user));
}

// TODO(lisa): re-visit after backend implementation.
function makeIntegration(json: any): Integration {
  json = json || {};
  const { name, details, status, type } = json;
  return {
    resourceType: 'integration',
    name,
    details,
    statusDescription: status?.description,
    kind: type,
    statusCode: status?.code,
    statusCodeText: convertIntegrationCodeToText(status?.code),
  };
}

function convertIntegrationCodeToText(code = 0) {
  if (code === IntegrationCode.Running) {
    return 'Running';
  }
  if (code === IntegrationCode.Paused) {
    return 'Paused';
  }
  if (code === IntegrationCode.Error) {
    return 'Error';
  }
  return 'Unspecified';
}
