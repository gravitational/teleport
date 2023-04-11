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

import type { Plugin, Integration } from 'teleport/services/integrations';

export const plugins: Plugin[] = [
  {
    resourceType: 'plugin',
    name: 'slack-default',
    details: `plugin running status`,
    kind: 'slack',
    statusCode: 'Running',
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'slack-secondary',
    details: `plugin unknown status`,
    kind: 'slack',
    statusCode: 'Unknown',
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'acmeco-default',
    details: `plugin unauthorized status`,
    kind: 'acmeco' as any, // unknown plugin, should handle gracefuly
    statusCode: 'Unauthorized',
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'slack',
    details: 'plugin unknown error status',
    kind: 'slack',
    statusCode: 'Unknown error',
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'slack',
    details: '',
    kind: 'slack',
    statusCode: 'Bot not invited to channel',
    spec: {},
  },
];

export const integrations: Integration[] = [
  {
    resourceType: 'integration',
    name: 'aws',
    kind: 'aws-oidc',
    statusCode: 'Running',
    spec: { roleArn: '' },
  },
  {
    resourceType: 'integration',
    name: 'some-integration-name',
    kind: '' as any,
    statusCode: 'Running',
    spec: { roleArn: '' },
  },
];
