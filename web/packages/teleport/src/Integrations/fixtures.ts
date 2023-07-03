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

import { IntegrationStatusCode } from 'teleport/services/integrations';

import type { Plugin, Integration } from 'teleport/services/integrations';

export const plugins: Plugin[] = [
  {
    resourceType: 'plugin',
    name: 'slack-default',
    details: `plugin running status`,
    kind: 'slack',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'slack-secondary',
    details: `plugin unknown status`,
    kind: 'slack',
    statusCode: IntegrationStatusCode.Unknown,
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'acmeco-default',
    details: `plugin unauthorized status`,
    kind: 'acmeco' as any, // unknown plugin, should handle gracefuly
    statusCode: IntegrationStatusCode.Unauthorized,
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'slack',
    details: 'plugin other error status',
    kind: 'slack',
    statusCode: IntegrationStatusCode.OtherError,
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'slack',
    details: '',
    kind: 'slack',
    statusCode: IntegrationStatusCode.SlackNotInChannel,
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'openai',
    details: '',
    kind: 'openai',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'okta',
    details: '',
    kind: 'okta',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'opsgenie',
    details: '',
    kind: 'opsgenie',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'jamf',
    details: '',
    kind: 'jamf',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
  },
];

export const integrations: Integration[] = [
  {
    resourceType: 'integration',
    name: 'aws',
    kind: 'aws-oidc',
    statusCode: IntegrationStatusCode.Running,
    spec: { roleArn: '' },
  },
  {
    resourceType: 'integration',
    name: 'some-integration-name',
    kind: '' as any,
    statusCode: IntegrationStatusCode.Running,
    spec: { roleArn: '' },
  },
];
