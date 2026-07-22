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
import { ContextProvider } from 'teleport/index';
import { Access } from 'teleport/Integrations/Enroll/AwsConsole/Access/Access';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { IntegrationKind } from 'teleport/services/integrations';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';

export default {
  title: 'Teleport/Integrations/Enroll/AwsConsole/ConfigureAccess',
};

const raName = 'test-ra';
const ctx = createTeleportContext();

const initialEntries = [
  {
    pathname: cfg.getIntegrationEnrollRoute(IntegrationKind.AwsRa, 'access'),
    state: {
      syncProfileArn: 'arn:aws:sync-profile',
      syncRoleArn: 'arn:aws:sync-profile',
      integrationName: raName,
      trustAnchorArn:
        'arn:aws:rolesanywhere:us-east-2:012345678901:trust-anchor/00000000-0000-0000-0000-000000000000',
    },
  },
];

export const NoProfiles = () => (
  <ContextProvider ctx={ctx}>
    <InfoGuidePanelProvider>
      <MemoryRouter initialEntries={initialEntries}>
        <Access />
      </MemoryRouter>
    </InfoGuidePanelProvider>
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

export const WithProfiles = () => (
  <ContextProvider ctx={ctx}>
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
              acceptRoleSessionName: false,
              tags: {
                'teleport.dev/cluster': 'foo',
                'teleport.dev/integration': 'bar',
                'teleport.dev/origin': 'baz',
              },
              roles: ['RoleA', 'RoleC'],
            },
            {
              arn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/bar',
              enabled: true,
              name: raName,
              acceptRoleSessionName: false,
              tags: {
                'teleport.dev/cluster': 'foo',
                'teleport.dev/integration': 'bar',
                'teleport.dev/origin': 'baz',
              },
              roles: ['RoleB', 'RoleB'],
            },
            {
              arn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/baz',
              enabled: true,
              name: raName,
              acceptRoleSessionName: false,
              tags: {
                'teleport.dev/cluster': 'foo',
                'teleport.dev/integration': 'bar',
                'teleport.dev/origin': 'baz',
              },
            },
            {
              arn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/qux',
              enabled: true,
              name: raName,
              acceptRoleSessionName: false,
              roles: ['RoleB', 'RoleB'],
            },
            {
              arn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/foo',
              enabled: true,
              name: raName,
              acceptRoleSessionName: false,
              tags: {
                'teleport.dev/cluster': 'foo',
                'teleport.dev/integration': 'bar',
                'teleport.dev/origin': 'baz',
              },
              roles: ['RoleA', 'RoleC'],
            },
            {
              arn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/bar',
              enabled: true,
              name: raName,
              acceptRoleSessionName: false,
              tags: {
                'teleport.dev/cluster': 'foo',
                'teleport.dev/integration': 'bar',
                'teleport.dev/origin': 'baz',
              },
              roles: ['RoleB', 'RoleB'],
            },
            {
              arn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/baz',
              enabled: true,
              name: raName,
              acceptRoleSessionName: false,
              tags: {
                'teleport.dev/cluster': 'foo',
                'teleport.dev/integration': 'bar',
                'teleport.dev/origin': 'baz',
              },
            },
            {
              arn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/qux',
              enabled: true,
              name: raName,
              acceptRoleSessionName: false,
              roles: ['RoleB', 'RoleB'],
            },
          ],
        });
      }),
      http.put(cfg.getIntegrationsUrl(raName), () => {
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

export const NotFoundEnroll = () => (
  <ContextProvider ctx={ctx}>
    <InfoGuidePanelProvider>
      <MemoryRouter initialEntries={initialEntries}>
        <CollapsibleInfoSectionComponent openLabel="Devs Instructions">
          <Info
            kind="info"
            details="(Devs) During enrollment, a 404 can occur if the integration created on the previous page is not yet ready. Open the network tab, this component should be retrying /listprofiles every 2 seconds."
          >
            Enrolling: 404 Error
          </Info>
        </CollapsibleInfoSectionComponent>
        <Access />
      </MemoryRouter>
    </InfoGuidePanelProvider>
  </ContextProvider>
);
NotFoundEnroll.parameters = {
  msw: {
    handlers: [
      http.post(cfg.getAwsRolesAnywhereProfilesUrl(raName), () => {
        return HttpResponse.json(
          {
            error: { message: 'Hidden 404 message' },
          },
          { status: 404 }
        );
      }),
    ],
  },
};

export const Edit = () => (
  <ContextProvider ctx={ctx}>
    <InfoGuidePanelProvider>
      <MemoryRouter
        initialEntries={[
          {
            pathname: cfg.getIntegrationEnrollRoute(
              IntegrationKind.AwsRa,
              'access'
            ),
            state: {
              edit: true,
              integrationName: raName,
              trustAnchorArn:
                'arn:aws:rolesanywhere:us-east-2:012345678901:trust-anchor/00000000-0000-0000-0000-000000000000',
              syncProfileArn: 'arn:aws:sync-profile',
              syncRoleArn: 'arn:aws:sync-profile',
            },
          },
        ]}
      >
        <Access />
      </MemoryRouter>
    </InfoGuidePanelProvider>
  </ContextProvider>
);
Edit.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getIntegrationsUrl(raName), () => {
        return HttpResponse.json({
          name: raName,
          kind: IntegrationKind,
          subKind: IntegrationKind.AwsRa,
          awsra: {
            trustAnchorArn: 'foo',
            profileSyncConfig: {
              profileNameFilters: ['test-*', 'dev-*', 'staging-*'],
            },
          },
        });
      }),
      http.post(cfg.getAwsRolesAnywhereProfilesUrl(raName), () => {
        return HttpResponse.json({
          profiles: [
            {
              arn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/foo',
              enabled: true,
              name: raName,
              acceptRoleSessionName: false,
              tags: {
                'teleport.dev/cluster': 'foo',
                'teleport.dev/integration': 'bar',
                'teleport.dev/origin': 'baz',
              },
              roles: ['RoleA', 'RoleC'],
            },
            {
              arn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/bar',
              enabled: true,
              name: raName,
              acceptRoleSessionName: false,
              tags: {
                'teleport.dev/cluster': 'foo',
                'teleport.dev/integration': 'bar',
                'teleport.dev/origin': 'baz',
              },
              roles: ['RoleB', 'RoleB'],
            },
          ],
        });
      }),
    ],
  },
};

export const EditError = () => (
  <ContextProvider ctx={ctx}>
    <InfoGuidePanelProvider>
      <MemoryRouter
        initialEntries={[
          {
            pathname: cfg.getIntegrationEnrollRoute(
              IntegrationKind.AwsRa,
              'access'
            ),
            state: {
              edit: true,
              integrationName: raName,
              trustAnchorArn:
                'arn:aws:rolesanywhere:us-east-2:012345678901:trust-anchor/00000000-0000-0000-0000-000000000000',
              syncProfileArn: 'arn:aws:sync-profile',
              syncRoleArn: 'arn:aws:sync-profile',
            },
          },
        ]}
      >
        <Access />
      </MemoryRouter>
    </InfoGuidePanelProvider>
  </ContextProvider>
);
EditError.parameters = {
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
      http.post(cfg.getAwsRolesAnywhereProfilesUrl(raName), () => {
        return HttpResponse.json({
          profiles: [
            {
              arn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/foo',
              enabled: true,
              name: raName,
              acceptRoleSessionName: false,
              tags: {
                'teleport.dev/cluster': 'foo',
                'teleport.dev/integration': 'bar',
                'teleport.dev/origin': 'baz',
              },
              roles: ['RoleA', 'RoleC'],
            },
            {
              arn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/bar',
              enabled: true,
              name: raName,
              acceptRoleSessionName: false,
              tags: {
                'teleport.dev/cluster': 'foo',
                'teleport.dev/integration': 'bar',
                'teleport.dev/origin': 'baz',
              },
              roles: ['RoleB', 'RoleB'],
            },
          ],
        });
      }),
      http.put(cfg.getIntegrationsUrl(raName), () => {
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

export const NotFoundEdit = () => (
  <ContextProvider ctx={ctx}>
    <InfoGuidePanelProvider>
      <MemoryRouter
        initialEntries={[
          {
            pathname: cfg.getIntegrationEnrollRoute(
              IntegrationKind.AwsRa,
              'access'
            ),
            state: {
              edit: true,
              integrationName: raName,
              trustAnchorArn:
                'arn:aws:rolesanywhere:us-east-2:012345678901:trust-anchor/00000000-0000-0000-0000-000000000000',
              syncProfileArn: 'arn:aws:sync-profile',
              syncRoleArn: 'arn:aws:sync-profile',
            },
          },
        ]}
      >
        <CollapsibleInfoSectionComponent openLabel="Devs Instructions">
          <Info
            kind="info"
            details="(Devs) During editing, a 404 will not result in polling as the integration should already exist"
          >
            Editing: 404 Error
          </Info>
        </CollapsibleInfoSectionComponent>
        <Access />
      </MemoryRouter>
    </InfoGuidePanelProvider>
  </ContextProvider>
);
NotFoundEdit.parameters = {
  msw: {
    handlers: [
      http.post(cfg.getAwsRolesAnywhereProfilesUrl(raName), () => {
        return HttpResponse.json(
          {
            error: { message: 'Hidden 404 message' },
          },
          { status: 404 }
        );
      }),
    ],
  },
};

export const BadRequest = () => (
  <ContextProvider ctx={ctx}>
    <InfoGuidePanelProvider>
      <MemoryRouter initialEntries={initialEntries}>
        <CollapsibleInfoSectionComponent openLabel="Devs Instructions">
          <Info
            kind="info"
            details="(Devs) Non-404 errors should not refetch. Open the network tab, you should not see retries to /listprofiles"
          >
            Non-404 Error
          </Info>
        </CollapsibleInfoSectionComponent>
        <Access />
      </MemoryRouter>
    </InfoGuidePanelProvider>
  </ContextProvider>
);
BadRequest.parameters = {
  msw: {
    handlers: [
      http.post(cfg.getAwsRolesAnywhereProfilesUrl(raName), () => {
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

export const WithoutAccess = () => {
  const noCtx = createTeleportContext({
    customAcl: makeAcl({
      integrations: {
        ...defaultAccess,
      },
    }),
  });

  return (
    <ContextProvider ctx={noCtx}>
      <InfoGuidePanelProvider>
        <MemoryRouter initialEntries={initialEntries}>
          <Access />
        </MemoryRouter>
      </InfoGuidePanelProvider>
    </ContextProvider>
  );
};

const missingState = [
  {
    pathname: cfg.getIntegrationEnrollRoute(IntegrationKind.AwsRa, 'access'),
    state: {
      integrationName: undefined,
      trustAnchorArn: undefined,
    },
  },
];

export const MissingState = () => (
  <ContextProvider ctx={ctx}>
    <InfoGuidePanelProvider>
      <MemoryRouter initialEntries={missingState}>
        <Access />
      </MemoryRouter>
    </InfoGuidePanelProvider>
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
