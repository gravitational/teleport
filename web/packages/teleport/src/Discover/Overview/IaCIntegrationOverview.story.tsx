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
import { MemoryRouter } from 'react-router';
import { Route } from 'react-router-dom';

import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';
import {
  ToastNotificationProvider,
  ToastNotifications,
} from 'shared/components/ToastNotification';

import cfg from 'teleport/config';
import { ContentMinWidth } from 'teleport/Main/Main';
import { IntegrationKind } from 'teleport/services/integrations';

import { IaCIntegrationOverview } from './IaCIntegrationOverview';

export default {
  title: 'Teleport/IaCIntegrationOverview',
};

const integrationName = 'my-terraform-integration';
const initialEntries = [
  cfg.getIaCIntegrationRoute(IntegrationKind.AwsOidc, integrationName),
];
const path = cfg.routes.integrationOverview;

function Component() {
  return (
    <MemoryRouter initialEntries={initialEntries}>
      <ToastNotificationProvider>
        <ToastNotifications />
        <InfoGuidePanelProvider>
          <ContentMinWidth>
            <Route path={path}>
              <IaCIntegrationOverview />
            </Route>
          </ContentMinWidth>
        </InfoGuidePanelProvider>
      </ToastNotificationProvider>
    </MemoryRouter>
  );
}

export function Default() {
  return <Component />;
}
Default.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getIntegrationStatsUrl(integrationName), () => {
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
          awsec2: { enrolled: 5, failed: 1 },
          awsrds: { enrolled: 3, failed: 0 },
          awseks: { enrolled: 2, failed: 0 },
          isManagedByTerraform: true,
        });
      }),
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
    ],
  },
};

export function Healthy() {
  return <Component />;
}
Healthy.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getIntegrationStatsUrl(integrationName), () => {
        return HttpResponse.json({
          name: integrationName,
          subKind: IntegrationKind.AwsOidc,
          unresolvedUserTasks: 0,
          userTasks: [],
          awsoidc: {
            roleArn: 'arn:aws:iam::123456789012:role/TeleportRole',
          },
          awsec2: { enrolled: 10, failed: 0 },
          awsrds: { enrolled: 5, failed: 0 },
          awseks: { enrolled: 3, failed: 0 },
          isManagedByTerraform: true,
        });
      }),
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
