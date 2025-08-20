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

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';

import { fireEvent, render, userEvent } from 'design/utils/testing';

import { ContextProvider } from 'teleport/index';
import { Access } from 'teleport/Integrations/Enroll/AwsConsole/Access/Access';
import { makeAwsOidcStatusContextState } from 'teleport/Integrations/status/AwsOidc/testHelpers/makeAwsOidcStatusContextState';
import { MockAwsOidcStatusProvider } from 'teleport/Integrations/status/AwsOidc/testHelpers/mockAwsOidcStatusProvider';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { integrationService } from 'teleport/services/integrations';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
    },
  },
});

const initialEntries = [
  {
    state: {
      integrationName: 'test',
      trustAnchorArn: 'trust-anchor-arn',
      syncRoleArn: 'sync-role-arn',
      syncProfileArn: 'sync-profile-arn',
    },
  },
];

test('flows through profiles configuration', async () => {
  const user = userEvent.setup();

  jest.spyOn(integrationService, 'awsRolesAnywhereProfiles').mockResolvedValue({
    profiles: [
      {
        arn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/foo',
        enabled: true,
        name: 'test',
        acceptRoleSessionName: false,
        tags: ['foo:bar', 'baz:qux', 'TagA:1'],
        roles: ['RoleA', 'RoleC'],
      },
      {
        arn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/bar',
        enabled: true,
        name: 'test',
        acceptRoleSessionName: false,
        tags: ['foo2:bar2', 'baz2:qux2', 'TagA:2'],
        roles: ['RoleB', 'RoleB'],
      },
    ],
  } as any);
  const createSpy = jest
    .spyOn(integrationService, 'createIntegration')
    .mockResolvedValue({} as any);

  render(
    <ContextProvider ctx={createTeleportContext()}>
      <QueryClientProvider client={queryClient}>
        <MockAwsOidcStatusProvider
          value={makeAwsOidcStatusContextState()}
          path=""
        >
          <MemoryRouter initialEntries={initialEntries}>
            <Access />
          </MemoryRouter>
        </MockAwsOidcStatusProvider>
      </QueryClientProvider>
    </ContextProvider>
  );

  await screen.findByText('Import All Profiles');
  expect(screen.getByText('Configure Access')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Enable Sync' })).toBeEnabled();
  expect(screen.queryByText('Filter by Profile Name')).not.toBeInTheDocument();
  expect(screen.getByTestId('toggle')).toBeEnabled();
  await user.click(screen.getByTestId('toggle'));
  expect(screen.getByText('Filter by Profile Name')).toBeInTheDocument();

  await user.type(screen.getByLabelText('Filter by Profile Name'), 'test-*');
  fireEvent.keyDown(screen.getByLabelText('Filter by Profile Name'), {
    key: 'Enter',
  });
  await user.click(screen.getByRole('button', { name: 'Enable Sync' }));

  expect(createSpy).toHaveBeenCalledTimes(1);
  expect(createSpy).toHaveBeenCalledWith({
    awsRa: {
      profileSyncConfig: {
        enabled: true,
        profileAcceptsRoleSessionName: false,
        profileArn: 'sync-profile-arn',
        profileNameFilters: ['test-*'],
        roleArn: 'sync-role-arn',
      },
      trustAnchorArn: 'trust-anchor-arn',
    },
    name: 'test',
    subKind: 'aws-ra',
  });
});
