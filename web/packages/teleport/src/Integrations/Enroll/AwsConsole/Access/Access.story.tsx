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

import { Info } from 'design/Alert';
import { CollapsibleInfoSection as CollapsibleInfoSectionComponent } from 'design/CollapsibleInfoSection';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import cfg from 'teleport/config';
import { Access } from 'teleport/Integrations/Enroll/AwsConsole/Access/Access';
import { makeAwsOidcStatusContextState } from 'teleport/Integrations/status/AwsOidc/testHelpers/makeAwsOidcStatusContextState';
import { MockAwsOidcStatusProvider } from 'teleport/Integrations/status/AwsOidc/testHelpers/mockAwsOidcStatusProvider';
import { IntegrationKind } from 'teleport/services/integrations';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';

export default {
  title: 'Teleport/Integrations/Enroll/AwsConsole/ConfigureAccess',
};

const raName = 'test-ra';

const initialEntries = [
  {
    pathname: cfg.getIntegrationEnrollRoute(IntegrationKind.AWSRa, 'access'),
    state: {
      integrationName: raName,
      trustAnchorArn: 'trust-anchor-arn',
      syncRoleArn: 'sync-role-arn',
      syncProfileArn: 'sync-profile-arn',
    },
  },
];

export const NoProfiles = () => (
  <MockAwsOidcStatusProvider value={makeAwsOidcStatusContextState()} path="">
    <InfoGuidePanelProvider>
      <MemoryRouter initialEntries={initialEntries}>
        <Access />
      </MemoryRouter>
    </InfoGuidePanelProvider>
  </MockAwsOidcStatusProvider>
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

export const WithProfiles = () => (
  <MockAwsOidcStatusProvider value={makeAwsOidcStatusContextState()} path="">
    <InfoGuidePanelProvider>
      <MemoryRouter initialEntries={initialEntries}>
        <CollapsibleInfoSectionComponent openLabel="Devs Instructions">
          <Info
            kind="info"
            details="(Devs) Success is a redirect that is not yet implemented, that is not mocked here"
          >
            Case: Success
          </Info>
          <Info
            kind="danger"
            details="(Devs) Add a label with `baz` and submit to see filter error formatting"
          >
            Case: Test error
          </Info>
        </CollapsibleInfoSectionComponent>
        <Access />
      </MemoryRouter>
    </InfoGuidePanelProvider>
  </MockAwsOidcStatusProvider>
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
              acceptRoleSessionName: false,
              tags: [{ foo: 'bar' }, { baz: 'qux' }, { TagA: 1 }],
              roles: ['RoleA', 'RoleC'],
            },
            {
              arn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/bar',
              enabled: true,
              name: raName,
              acceptRoleSessionName: false,
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

export const WithoutAccess = () => {
  const acl = makeAcl({
    integrations: {
      ...defaultAccess,
    },
  });

  return (
    <MockAwsOidcStatusProvider
      value={makeAwsOidcStatusContextState()}
      path=""
      customAcl={acl}
    >
      <InfoGuidePanelProvider>
        <MemoryRouter initialEntries={initialEntries}>
          <Access />
        </MemoryRouter>
      </InfoGuidePanelProvider>
    </MockAwsOidcStatusProvider>
  );
};

const missingState = [
  {
    pathname: cfg.getIntegrationEnrollRoute(IntegrationKind.AWSRa, 'access'),
    state: {
      integrationName: raName,
      syncRoleArn: 'sync-role-arn',
      syncProfileArn: 'sync-profile-arn',
    },
  },
];

export const MissingState = () => (
  <MockAwsOidcStatusProvider value={makeAwsOidcStatusContextState()} path="">
    <InfoGuidePanelProvider>
      <MemoryRouter initialEntries={missingState}>
        <Access />
      </MemoryRouter>
    </InfoGuidePanelProvider>
  </MockAwsOidcStatusProvider>
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
