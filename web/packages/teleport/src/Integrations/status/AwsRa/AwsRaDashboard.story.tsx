/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import cfg from 'teleport/config';
import { ContextProvider } from 'teleport/index';
import { AwsRaDashboard } from 'teleport/Integrations/status/AwsRa/AwsRaDashboard';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { IntegrationKind } from 'teleport/services/integrations';

export default {
  title: 'Teleport/Integrations/AwsRa',
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.getIntegrationsUrl(), () => {
          return HttpResponse.json({
            items: [],
            nextKey: '',
          });
        }),
      ],
    },
  },
};

const raName = 'ra-int';
const trustAnchorArn =
  'arn:aws:rolesanywhere:us-east-2:111:trust-anchor/1420223b-23d8-49f3-bca5-6521042ac283';
const ctx = createTeleportContext();
const initialEntries = [
  cfg.getIntegrationStatusRoute(IntegrationKind.AwsRa, raName),
];
const path = cfg.routes.integrationStatus;

export function Dashboard() {
  return (
    <ContextProvider ctx={ctx}>
      <MemoryRouter initialEntries={initialEntries}>
        <InfoGuidePanelProvider>
          <Route path={path}>
            <AwsRaDashboard />
          </Route>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    </ContextProvider>
  );
}
Dashboard.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getIntegrationsUrl(raName), () => {
        return HttpResponse.json({
          name: raName,
          subKind: 'aws-ra',
          awsra: {
            trustAnchorARN: trustAnchorArn,
            profileSyncConfig: {
              enabled: true,
              profileArn:
                'arn:aws:rolesanywhere:us-east-2:111:profile/31a77874-9f41-4c10-aeae-2aa1140e8ca5',
              roleArn: 'arn:aws:iam::111:role/ra-int',
              filters: ['*'],
            },
          },
        });
      }),
      http.get(cfg.getIntegrationStatsUrl(raName), () => {
        return HttpResponse.json({
          name: raName,
          subKind: 'aws-ra',
          awsra: {
            trustAnchorARN: trustAnchorArn,
            profileSyncConfig: {
              enabled: true,
              profileArn:
                'arn:aws:rolesanywhere:us-east-2:111:profile/31a77874-9f41-4c10-aeae-2aa1140e8ca5',
              roleArn: 'arn:aws:iam::111:role/ra-int',
              filters: ['*'],
            },
          },
          unresolvedUserTasks: 0,
          awsec2: {},
          awsrds: {},
          awseks: {},
          rolesAnywhereProfileSync: {
            enabled: true,
            status: 'SUCCESS',
            syncedProfiles: 3,
            syncStartTime: '2025-08-25T17:57:29.5993537Z',
            syncEndTime: '2025-08-25T17:57:30.538177396Z',
          },
        });
      }),
    ],
  },
};

export function Failed() {
  return (
    <ContextProvider ctx={ctx}>
      <MemoryRouter initialEntries={initialEntries}>
        <InfoGuidePanelProvider>
          <Route path={path}>
            <AwsRaDashboard />
          </Route>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    </ContextProvider>
  );
}
Failed.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getIntegrationsUrl(raName), () => {
        return HttpResponse.json(
          {
            error: { message: 'Generic Bad Request' },
          },
          { status: 400 }
        );
      }),
      http.get(cfg.getIntegrationStatsUrl(raName), () => {
        return HttpResponse.json(
          {
            error: { message: 'Generic Bad Request' },
          },
          { status: 400 }
        );
      }),
    ],
  },
};

export function NoData() {
  return (
    <ContextProvider ctx={ctx}>
      <MemoryRouter initialEntries={initialEntries}>
        <InfoGuidePanelProvider>
          <Route path={path}>
            <AwsRaDashboard />
          </Route>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    </ContextProvider>
  );
}
NoData.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getIntegrationsUrl(raName), () => {
        return HttpResponse.json({
          name: raName,
          subKind: 'aws-ra',
          awsra: {
            trustAnchorARN: trustAnchorArn,
            profileSyncConfig: {
              enabled: true,
              profileArn:
                'arn:aws:rolesanywhere:us-east-2:111:profile/31a77874-9f41-4c10-aeae-2aa1140e8ca5',
              roleArn: 'arn:aws:iam::111:role/ra-int',
              filters: ['*'],
            },
          },
        });
      }),
      http.get(cfg.getIntegrationStatsUrl(raName), () => {
        return HttpResponse.json({
          name: raName,
          subKind: IntegrationKind.AwsRa,
          unresolvedUserTasks: 0,
          awsra: {},
          awsoidc: {},
          awsec2: {},
          awsrds: {},
          awseks: {},
          rolesAnywhereProfileSync: {},
        });
      }),
    ],
  },
};
