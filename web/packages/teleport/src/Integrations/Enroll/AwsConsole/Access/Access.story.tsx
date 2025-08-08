/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';
import { makeErrorAttempt } from 'shared/hooks/useAsync';

import cfg from 'teleport/config';
import { ContextProvider } from 'teleport/index';
import { Access } from 'teleport/Integrations/Enroll/AwsConsole/Access/Access';
import { makeAwsOidcStatusContextState } from 'teleport/Integrations/status/AwsOidc/testHelpers/makeAwsOidcStatusContextState';
import { MockAwsOidcStatusProvider } from 'teleport/Integrations/status/AwsOidc/testHelpers/mockAwsOidcStatusProvider';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { IntegrationKind } from 'teleport/services/integrations';

export default {
  title: 'Teleport/Integrations/Enroll/AwsConsole/ConfigureAccess',
};

const ctx = createTeleportContext();
const raName = 'test-ra';

const initialEntries = [
  {
    pathname: cfg.getIntegrationEnrollChildRoute(
      IntegrationKind.AwsOidc,
      'aws-parent-name',
      IntegrationKind.AWSRa,
      'access'
    ),
    state: {
      integrationName: raName,
      trustAnchorArn: 'trust-anchor-arn',
      syncRoleArn: 'sync-role-arn',
      syncProfileArn: 'sync-profile-arn',
    },
  },
];

export const NoProfiles = () => (
  <ContextProvider ctx={ctx}>
    <MockAwsOidcStatusProvider value={makeAwsOidcStatusContextState()} path="">
      <InfoGuidePanelProvider>
        <MemoryRouter initialEntries={initialEntries}>
          <Access />
        </MemoryRouter>
      </InfoGuidePanelProvider>
    </MockAwsOidcStatusProvider>
  </ContextProvider>
);
NoProfiles.parameters = {
  msw: {
    handlers: [
      http.post(cfg.getAwsRolesAnywhereProfilesUrl(raName), () => {
        return HttpResponse.json({
          profiles: [],
        });
      }),
    ],
  },
};

// Enter a filter of `baz` and click submit to see error formatting
export const WithProfiles = () => (
  <ContextProvider ctx={ctx}>
    <MockAwsOidcStatusProvider value={makeAwsOidcStatusContextState()} path="">
      <InfoGuidePanelProvider>
        <MemoryRouter initialEntries={initialEntries}>
          <Access />
        </MemoryRouter>
      </InfoGuidePanelProvider>
    </MockAwsOidcStatusProvider>
  </ContextProvider>
);
WithProfiles.parameters = {
  msw: {
    handlers: [
      http.post(cfg.getAwsRolesAnywhereProfilesUrl(raName), () => {
        return HttpResponse.json({
          profiles: [
            {
              arn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/foo',
              enabled: true,
              name: raName,
              accept_role_session_name: false,
              tags: [{ foo: 'bar' }, { baz: 'qux' }, { TagA: 1 }],
              roles: ['RoleA', 'RoleC'],
            },
            {
              arn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/bar',
              enabled: true,
              name: raName,
              accept_role_session_name: false,
              tags: [{ foo2: 'bar2' }, { baz2: 'qux2' }, { TagA: 2 }],
              roles: ['RoleB', 'RoleB'],
            },
          ],
        });
      }),
      http.post(cfg.getIntegrationsUrl(), () => {
        return HttpResponse.json(
          {
            error: { message: 'Filter baz invalid.' },
          },
          { status: 400 }
        );
      }),
    ],
  },
};

const missingState = [
  {
    pathname: cfg.getIntegrationEnrollChildRoute(
      IntegrationKind.AwsOidc,
      'aws-parent-name',
      IntegrationKind.AWSRa,
      'access'
    ),
    state: {
      integrationName: raName,
      syncRoleArn: 'sync-role-arn',
      syncProfileArn: 'sync-profile-arn',
    },
  },
];

export const ParentIssue = () => (
  <ContextProvider ctx={ctx}>
    <MockAwsOidcStatusProvider
      value={makeAwsOidcStatusContextState({
        statsAttempt: makeErrorAttempt(
          new Error('Error with aws oidc parent integration.')
        ),
      })}
      path=""
    >
      <InfoGuidePanelProvider>
        <MemoryRouter initialEntries={missingState}>
          <Access />
        </MemoryRouter>
      </InfoGuidePanelProvider>
    </MockAwsOidcStatusProvider>
  </ContextProvider>
);
ParentIssue.parameters = {
  msw: {
    handlers: [
      http.post(cfg.getAwsRolesAnywhereProfilesUrl(raName), () => {
        return HttpResponse.json({
          profiles: [],
        });
      }),
    ],
  },
};

export const MissingState = () => (
  <ContextProvider ctx={ctx}>
    <MockAwsOidcStatusProvider value={makeAwsOidcStatusContextState()} path="">
      <InfoGuidePanelProvider>
        <MemoryRouter initialEntries={missingState}>
          <Access />
        </MemoryRouter>
      </InfoGuidePanelProvider>
    </MockAwsOidcStatusProvider>
  </ContextProvider>
);
MissingState.parameters = {
  msw: {
    handlers: [
      http.post(cfg.getAwsRolesAnywhereProfilesUrl(raName), () => {
        return HttpResponse.json({
          profiles: [],
        });
      }),
    ],
  },
};
