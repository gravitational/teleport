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
    statusCode: IntegrationStatusCode.Draft,
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
  {
    resourceType: 'plugin',
    name: 'intune',
    details: '',
    kind: 'intune',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'servicenow',
    details: '',
    kind: 'servicenow',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'mattermost',
    details: '',
    kind: 'mattermost',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'jira',
    details: '',
    kind: 'jira',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'discord',
    details: '',
    kind: 'discord',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'entra-id',
    details: '',
    kind: 'entra-id',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'datadog',
    details: '',
    kind: 'datadog',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'pagerduty',
    details: '',
    kind: 'pagerduty',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'email',
    details: '',
    kind: 'email',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
  },
  {
    resourceType: 'plugin',
    name: 'msteams',
    details: '',
    kind: 'msteams',
    statusCode: IntegrationStatusCode.Running,
    spec: {},
  },
];

export const integrations: Integration[] = [
  {
    resourceType: 'integration',
    name: 'aws',
    kind: IntegrationKind.AwsOidc,
    statusCode: IntegrationStatusCode.Running,
    spec: { roleArn: '', issuerS3Prefix: '', issuerS3Bucket: '' },
    summary: {
      resourcesCount: {
        found: 1000,
        enrolled: 999,
        failed: 1,
      },
      unresolvedUserTasks: [
        {
          name: 'ut-ec2-enroll-0001',
          taskType: 'discover-ec2',
          state: 'OPEN',
          issueType: 'ec2-enroll',
          title: 'Enroll discovered EC2 instances',
          integration: 'aws-prod',
          lastStateChange: '2026-01-11T18:42:10Z',
        },
        {
          name: 'ut-eks-enroll-0007',
          taskType: 'discover-eks',
          state: 'OPEN',
          issueType: 'eks-enroll',
          title: 'An extra long title to show text overflow styles',
          integration: 'aws-prod',
          lastStateChange: '2026-01-10T03:15:00Z',
        },
        {
          name: 'ut-rds-failure-0042',
          taskType: 'discover-rds',
          state: 'OPEN',
          issueType: 'rds-discovery-failed',
          title: 'RDS discovery failed',
          integration: 'aws-staging',
          lastStateChange: '2026-01-09T22:01:33Z',
        },
        {
          name: 'aws-oidc:discover-eks:eks-agent-not-connecting:123456789012:us-east-1:example-cluster',
          taskType: 'discover-eks',
          state: 'OPEN',
          issueType: 'eks-agent-not-connecting',
          title:
            'Teleport Kubernetes agent is not connecting for EKS cluster example-cluster',
          integration: 'aws-oidc',
          lastStateChange: '2026-01-10T18:42:00Z',
        },
      ],
    },
  },
  {
    resourceType: 'integration',
    name: 'azure',
    kind: IntegrationKind.AzureOidc,
    statusCode: IntegrationStatusCode.Running,
    summary: {
      unresolvedUserTasks: [
        {
          name: 'ut-rds-failure-0042',
          taskType: 'discover-rds',
          state: 'OPEN',
          issueType: 'rds-discovery-failed',
          title: 'RDS discovery failed',
          integration: 'aws-staging',
          lastStateChange: '2026-01-09T22:01:33Z',
        },
      ],
      resourcesCount: {
        found: 10,
        enrolled: 10,
        failed: 0,
      },
    },
  },
  {
    resourceType: 'integration',
    name: 'github',
    kind: IntegrationKind.GitHub,
    statusCode: IntegrationStatusCode.Running,
    details: 'some-detail',
    spec: { organization: 'lsdf' },
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
