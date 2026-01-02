/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import {
  IntegrationKind,
  IntegrationStatusCode,
  type ExternalAuditStorage,
  type Integration,
  type Plugin,
} from 'teleport/services/integrations';

import { integrationKindToTags, pluginKindToIntegrationTags } from './helpers';

export const plugins: Plugin[] = [
  {
    resourceType: 'plugin',
    name: 'slack-default',
    details: `plugin running status`,
    kind: 'slack',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
    tags: pluginKindToIntegrationTags('slack'),
  },
  {
    resourceType: 'plugin',
    name: 'slack-secondary',
    details: `plugin unknown status`,
    kind: 'slack',
    statusCode: IntegrationStatusCode.Unknown,
    spec: {},
    tags: pluginKindToIntegrationTags('slack'),
  },
  {
    resourceType: 'plugin',
    name: 'acmeco-default',
    details: `plugin unauthorized status`,
    kind: 'acmeco' as any, // unknown plugin, should handle gracefuly
    statusCode: IntegrationStatusCode.Unauthorized,
    spec: {},
    tags: [],
  },
  {
    resourceType: 'plugin',
    name: 'slack',
    details: 'plugin other error status',
    kind: 'slack',
    statusCode: IntegrationStatusCode.OtherError,
    spec: {},
    tags: pluginKindToIntegrationTags('slack'),
  },
  {
    resourceType: 'plugin',
    name: 'slack',
    details: '',
    kind: 'slack',
    statusCode: IntegrationStatusCode.SlackNotInChannel,
    spec: {},
    tags: pluginKindToIntegrationTags('slack'),
  },
  {
    resourceType: 'plugin',
    name: 'openai',
    details: '',
    kind: 'openai',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
    tags: pluginKindToIntegrationTags('openai'),
  },
  {
    resourceType: 'plugin',
    name: 'okta',
    details: '',
    kind: 'okta',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
    tags: pluginKindToIntegrationTags('okta'),
  },
  {
    resourceType: 'plugin',
    name: 'opsgenie',
    details: '',
    kind: 'opsgenie',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
    tags: pluginKindToIntegrationTags('opsgenie'),
  },
  {
    resourceType: 'plugin',
    name: 'jamf',
    details: '',
    kind: 'jamf',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
    tags: pluginKindToIntegrationTags('jamf'),
  },
  {
    resourceType: 'plugin',
    name: 'intune',
    details: '',
    kind: 'intune',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
    tags: pluginKindToIntegrationTags('intune'),
  },
  {
    resourceType: 'plugin',
    name: 'servicenow',
    details: '',
    kind: 'servicenow',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
    tags: pluginKindToIntegrationTags('servicenow'),
  },
  {
    resourceType: 'plugin',
    name: 'mattermost',
    details: '',
    kind: 'mattermost',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
    tags: pluginKindToIntegrationTags('mattermost'),
  },
  {
    resourceType: 'plugin',
    name: 'jira',
    details: '',
    kind: 'jira',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
    tags: pluginKindToIntegrationTags('jira'),
  },
  {
    resourceType: 'plugin',
    name: 'discord',
    details: '',
    kind: 'discord',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
    tags: pluginKindToIntegrationTags('discord'),
  },
  {
    resourceType: 'plugin',
    name: 'entra-id',
    details: '',
    kind: 'entra-id',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
    tags: pluginKindToIntegrationTags('entra-id'),
  },
  {
    resourceType: 'plugin',
    name: 'datadog',
    details: '',
    kind: 'datadog',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
    tags: pluginKindToIntegrationTags('datadog'),
  },
  {
    resourceType: 'plugin',
    name: 'pagerduty',
    details: '',
    kind: 'pagerduty',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
    tags: pluginKindToIntegrationTags('pagerduty'),
  },
  {
    resourceType: 'plugin',
    name: 'email',
    details: '',
    kind: 'email',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
    tags: pluginKindToIntegrationTags('email'),
  },
  {
    resourceType: 'plugin',
    name: 'msteams',
    details: '',
    kind: 'msteams',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
    tags: pluginKindToIntegrationTags('msteams'),
  },
];

export const integrations: Integration[] = [
  {
    resourceType: 'integration',
    name: 'aws',
    kind: IntegrationKind.AwsOidc,
    statusCode: IntegrationStatusCode.Running,
    spec: { roleArn: '', issuerS3Prefix: '', issuerS3Bucket: '' },
    tags: integrationKindToTags(IntegrationKind.AwsOidc),
  },
  {
    resourceType: 'integration',
    name: 'azure',
    kind: IntegrationKind.AzureOidc,
    statusCode: IntegrationStatusCode.Running,
    tags: integrationKindToTags(IntegrationKind.AzureOidc),
  },
  {
    resourceType: 'integration',
    name: 'github',
    kind: IntegrationKind.GitHub,
    statusCode: IntegrationStatusCode.Running,
    details: 'some-detail',
    spec: { organization: 'lsdf' },
    tags: integrationKindToTags(IntegrationKind.GitHub),
  },
  {
    resourceType: 'integration',
    name: 'roles-anywhere',
    kind: IntegrationKind.AwsRa,
    statusCode: IntegrationStatusCode.Running,
    details: 'some-detail',
    spec: {
      trustAnchorARN:
        'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/foo',
      profileSyncConfig: {
        enabled: true,
        profileArn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:profile/bar',
        roleArn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:role/baz',
        filters: ['test-*', 'dev-*'],
      },
    },
    tags: integrationKindToTags(IntegrationKind.AwsRa),
  },
];

export const externalAuditStorage: ExternalAuditStorage = {
  athenaResultsURI: 'athenaResultsURI',
  athenaWorkgroup: 'athenaWorkgroup',
  auditEventsLongTermURI: 'auditEventsLongTermURI',
  glueDatabase: 'glueDatabase',
  glueTable: 'glueTable',
  integrationName: 'integrationName',
  policyName: 'policyName',
  region: 'us-west-2',
  sessionsRecordingsURI: 'sessionsRecordingsURI',
};
