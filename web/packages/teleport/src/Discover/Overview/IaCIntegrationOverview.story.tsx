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

import { http, HttpResponse } from 'msw';
import { Routes, Route } from 'react-router';

import { ToastNotifications } from 'shared/components/ToastNotification';

import cfg from 'teleport/config';
import { ContentMinWidth } from 'teleport/Main/Main';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { IntegrationKind } from 'teleport/services/integrations';

import { IaCIntegrationOverview } from './IaCIntegrationOverview';

export default {
  title: 'Teleport/IaCIntegrationOverview',
};

const integrationName = 'my-terraform-integration';
const path = cfg.routes.integrationOverview;

function Component({
  kind = IntegrationKind.AwsOidc,
}: { kind?: IntegrationKind } = {}) {
  const initialEntries = [cfg.getIaCIntegrationRoute(kind, integrationName)];
  return (
    <TeleportProviderBasic initialEntries={initialEntries}>
      <ToastNotifications />
      <ContentMinWidth>
        <Routes>
          <Route path={path} element={<IaCIntegrationOverview />} />
        </Routes>
      </ContentMinWidth>
    </TeleportProviderBasic>
  );
}

const statsHandler = (overrides = {}) =>
  http.get(cfg.getIntegrationStatsUrl(integrationName), () => {
    const lastSync = Date.now() - 2 * 60 * 1000;

    return HttpResponse.json({
      name: integrationName,
      subKind: IntegrationKind.AwsOidc,
      unresolvedUserTasks: 2,
      userTasks: [
        {
          name: 'task-1',
          taskType: 'discover-ec2',
          state: 'OPEN',
          issueType: 'ec2-ssm-invocation-failure',
          title: 'EC2 SSM Invocation Failure',
          integration: integrationName,
          lastStateChange: new Date().toISOString(),
        },
        {
          name: 'task-2',
          taskType: 'discover-rds',
          state: 'OPEN',
          issueType: 'rds-agent-not-connecting',
          title: 'RDS Agent Not Connecting',
          integration: integrationName,
          lastStateChange: new Date(
            Date.now() - 24 * 60 * 60 * 1000
          ).toISOString(),
        },
      ],
      awsoidc: {
        roleArn: 'arn:aws:iam::123456789012:role/TeleportRole',
      },
      awsec2: { enrolled: 5, failed: 1, discoverLastSync: lastSync },
      awsrds: { enrolled: 3, failed: 0, discoverLastSync: lastSync },
      awseks: { enrolled: 2, failed: 0, discoverLastSync: lastSync },
      isManagedByTerraform: true,
      ...overrides,
    });
  });

const rulesHandler = http.get(
  `*/integrations/${integrationName}/discoveryrules`,
  ({ request }) => {
    const resourceType = new URL(request.url).searchParams.get('resourceType');
    if (resourceType === 'eks') {
      return HttpResponse.json({
        rules: [
          {
            resourceType: 'eks',
            region: 'us-west-2',
            labelMatcher: [{ name: 'team', value: 'platform' }],
            kubeAppDiscovery: false,
          },
        ],
      });
    }
    return HttpResponse.json({
      rules: [
        {
          resourceType: 'ec2',
          region: 'us-east-1',
          labelMatcher: [{ name: 'env', value: 'prod' }],
        },
      ],
    });
  }
);

const userTaskHandlers = [
  http.get(cfg.api.userTaskPath, ({ params }) => {
    const taskName = params.name as string;
    if (taskName === 'task-1') {
      return HttpResponse.json({
        name: 'task-1',
        taskType: 'discover-ec2',
        state: 'OPEN',
        issueType: 'ec2-ssm-invocation-failure',
        title: 'EC2 SSM Invocation Failure',
        integration: integrationName,
        lastStateChange: new Date().toISOString(),
        description:
          'The SSM agent failed to run the installation script on the following EC2 instances. Please ensure the SSM agent is installed and running on the instances.',
        discoverEc2: {
          region: 'us-east-1',
          account_id: '123456789012',
          instances: {
            'i-1234567890abcdef0': {
              instance_id: 'i-1234567890abcdef0',
              name: 'web-server-1',
              resourceUrl:
                'https://console.aws.amazon.com/ec2/home?region=us-east-1#InstanceDetails:instanceId=i-1234567890abcdef0',
              invocation_url:
                'https://console.aws.amazon.com/systems-manager/run-command/abc123',
            },
            'i-0987654321fedcba0': {
              instance_id: 'i-0987654321fedcba0',
              name: 'web-server-2',
              resourceUrl:
                'https://console.aws.amazon.com/ec2/home?region=us-east-1#InstanceDetails:instanceId=i-0987654321fedcba0',
            },
          },
        },
      });
    }
    if (taskName === 'task-2') {
      return HttpResponse.json({
        name: 'task-2',
        taskType: 'discover-rds',
        state: 'OPEN',
        issueType: 'rds-agent-not-connecting',
        title: 'RDS Agent Not Connecting',
        integration: integrationName,
        lastStateChange: new Date(
          Date.now() - 24 * 60 * 60 * 1000
        ).toISOString(),
        description:
          'The Teleport Database Agent is not connecting to the following RDS databases. Please check network connectivity and security group settings.',
        discoverRds: {
          region: 'us-west-2',
          account_id: '123456789012',
          databases: {
            'my-database-1': {
              name: 'my-database-1',
              resourceUrl:
                'https://console.aws.amazon.com/rds/home?region=us-west-2#database:id=my-database-1',
            },
          },
        },
      });
    }
    return HttpResponse.json(
      { error: { message: 'Task not found' } },
      { status: 404 }
    );
  }),
  http.put(cfg.api.resolveUserTaskPath, ({ params }) => {
    const taskName = params.name as string;
    if (taskName === 'task-1') {
      return HttpResponse.json({ success: true });
    }
    return HttpResponse.json(
      { error: { message: 'Failed to resolve task: permission denied' } },
      { status: 403 }
    );
  }),
];

export function Default() {
  return <Component />;
}
Default.parameters = {
  msw: {
    handlers: [statsHandler(), rulesHandler, ...userTaskHandlers],
  },
};

export function Healthy() {
  return <Component />;
}
Healthy.parameters = {
  msw: {
    handlers: [
      statsHandler({
        unresolvedUserTasks: 0,
        userTasks: [],
        awsec2: {
          enrolled: 10,
          failed: 0,
          discoverLastSync: Date.now() - 2 * 60 * 1000,
        },
        awsrds: {
          enrolled: 5,
          failed: 0,
          discoverLastSync: Date.now() - 2 * 60 * 1000,
        },
        awseks: {
          enrolled: 3,
          failed: 0,
          discoverLastSync: Date.now() - 2 * 60 * 1000,
        },
      }),
      rulesHandler,
    ],
  },
};

export function Loading() {
  return <Component />;
}
Loading.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getIntegrationStatsUrl(integrationName), async () => {
        await new Promise(() => {}); // Never resolves
      }),
    ],
  },
};

export function Error() {
  return <Component />;
}
Error.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getIntegrationStatsUrl(integrationName), () => {
        return HttpResponse.json(
          { error: { message: 'Failed to fetch integration stats' } },
          { status: 500 }
        );
      }),
    ],
  },
};

export function AzureWithWildcardSubscription() {
  return <Component kind={IntegrationKind.AzureOidc} />;
}
AzureWithWildcardSubscription.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getIntegrationStatsUrl(integrationName), () =>
        HttpResponse.json({
          name: integrationName,
          subKind: IntegrationKind.AzureOidc,
          unresolvedUserTasks: 0,
          userTasks: [],
          azurevm: {
            resourcesFound: 3,
            discoverLastSync: Date.now() - 2 * 60 * 1000,
          },
          isManagedByTerraform: true,
        })
      ),
      http.get(`*/integrations/${integrationName}`, () =>
        HttpResponse.json({
          name: integrationName,
          subKind: IntegrationKind.AzureOidc,
          azureoidc: {
            tenantId: 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee',
            clientId: 'ffffffff-0000-1111-2222-333333333333',
            managedIdentity: {
              resourceGroup: 'my-resource-group',
              region: 'eastus',
            },
          },
        })
      ),
      http.get(`*/integrations/${integrationName}/discoveryrules`, () =>
        HttpResponse.json({
          rules: [
            {
              resourceType: 'vm',
              region: 'eastus',
              subscriptions: ['*'],
              resourceGroups: [],
              labelMatcher: [],
            },
          ],
        })
      ),
    ],
  },
};

export function SettingsError() {
  return <Component />;
}
SettingsError.parameters = {
  msw: {
    handlers: [
      statsHandler(),
      http.get(`*/integrations/${integrationName}/discoveryrules`, () => {
        return HttpResponse.json(
          {
            error: {
              message: 'Failed to load integration rules',
            },
          },
          { status: 500 }
        );
      }),
      ...userTaskHandlers,
    ],
  },
};
