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

import { ToastNotificationProvider } from 'shared/components/ToastNotification';

import cfg from 'teleport/config';
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

export function Default() {
  return (
    <MemoryRouter initialEntries={initialEntries}>
      <ToastNotificationProvider>
        <Route path={path}>
          <IaCIntegrationOverview />
        </Route>
      </ToastNotificationProvider>
    </MemoryRouter>
  );
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
    ],
  },
};

export function Healthy() {
  return (
    <MemoryRouter initialEntries={initialEntries}>
      <ToastNotificationProvider>
        <Route path={path}>
          <IaCIntegrationOverview />
        </Route>
      </ToastNotificationProvider>
    </MemoryRouter>
  );
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
  return (
    <MemoryRouter initialEntries={initialEntries}>
      <ToastNotificationProvider>
        <Route path={path}>
          <IaCIntegrationOverview />
        </Route>
      </ToastNotificationProvider>
    </MemoryRouter>
  );
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
  return (
    <MemoryRouter initialEntries={initialEntries}>
      <ToastNotificationProvider>
        <Route path={path}>
          <IaCIntegrationOverview />
        </Route>
      </ToastNotificationProvider>
    </MemoryRouter>
  );
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
