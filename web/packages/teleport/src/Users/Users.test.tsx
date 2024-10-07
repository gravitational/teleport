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

import React from 'react';
import { MemoryRouter } from 'react-router';
import {
  render,
  screen,
  testQueryClient,
  userEvent,
} from 'design/utils/testing';

import { fireEvent, within } from '@testing-library/react';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { ContextProvider } from 'teleport';
import { server } from 'teleport/test/handlers/server';
import { errorGetUsers, successGetUsers } from 'teleport/test/handlers/users';
import cfg from 'teleport/config';
import { storageService } from 'teleport/services/storageService';

import { Users } from './Users';

function MockInviteCollaborators() {
  return <div data-testid="invite-collaborators">Invite Collaborators</div>;
}

beforeEach(() => server.listen());
afterEach(() => {
  server.resetHandlers();

  return testQueryClient.resetQueries();
});
afterAll(() => server.close());

test('displays the Create New User button when not configured', async () => {
  server.use(successGetUsers([]));

  const ctx = createTeleportContext();

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <Users />
      </ContextProvider>
    </MemoryRouter>
  );

  await screen.findByPlaceholderText('Search...');

  expect(screen.getByText('Create New User')).toBeInTheDocument();
  expect(screen.queryByText('Enroll Users')).not.toBeInTheDocument();
});

test('displays the Enroll Users button when configured', async () => {
  server.use(successGetUsers([]));

  const ctx = createTeleportContext();

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <Users inviteCollaboratorsComponent={MockInviteCollaborators} />
      </ContextProvider>
    </MemoryRouter>
  );

  await screen.findByPlaceholderText('Search...');

  const enrollButton = screen.getByText('Enroll Users');

  expect(enrollButton).toBeInTheDocument();
  expect(screen.queryByText('Create New User')).not.toBeInTheDocument();

  fireEvent.click(enrollButton);

  expect(screen.getByTestId('invite-collaborators')).toBeVisible();
});

test('Users not equal to MAU Notice', async () => {
  server.use(successGetUsers([]));

  const ctx = createTeleportContext();

  const flags = ctx.getFeatureFlags();

  jest.spyOn(ctx, 'getFeatureFlags').mockImplementation(() => {
    return {
      ...flags,
      billing: true,
    };
  });

  jest
    .spyOn(storageService, 'getUsersMauAcknowledged')
    .mockImplementation(() => false);

  const originalIsUsageBasedBilling = cfg.isUsageBasedBilling;

  cfg.isUsageBasedBilling = true;

  const user = userEvent.setup();

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <Users />
      </ContextProvider>
    </MemoryRouter>
  );

  expect(screen.getByTestId('users-not-mau-alert')).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: 'Dismiss' }));

  expect(screen.queryByTestId('users-not-mau-alert')).not.toBeInTheDocument();

  cfg.isUsageBasedBilling = originalIsUsageBasedBilling;
});

test('displays the traditional reset UI when not configured', async () => {
  server.use(
    successGetUsers([
      {
        authType: 'local',
        name: 'test',
        roles: ['admin'],
        isBot: false,
      },
    ])
  );

  const ctx = createTeleportContext();

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <Users />
      </ContextProvider>
    </MemoryRouter>
  );

  await screen.findByPlaceholderText('Search...');

  const table = screen.queryByRole('table');

  expect(table).toBeInTheDocument();

  const rows = within(table).getAllByRole('row');

  const optionsButton = within(rows[1]).getByRole('button', {
    name: 'Options',
  });

  fireEvent.click(optionsButton);

  const editButton = screen.queryByText('Reset Authentication...');

  fireEvent.click(editButton);

  expect(screen.getByText('Reset User Authentication?')).toBeInTheDocument();
  expect(screen.queryByText('New Reset UI')).not.toBeInTheDocument();
});

test('displays an error if the request fails', async () => {
  server.use(errorGetUsers('An error occurred'));

  const ctx = createTeleportContext();

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <Users />
      </ContextProvider>
    </MemoryRouter>
  );

  expect(await screen.findByText('An error occurred')).toBeInTheDocument();
});
