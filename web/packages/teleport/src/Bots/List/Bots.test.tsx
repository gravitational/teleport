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

import { MemoryRouter } from 'react-router';

import { render, screen, userEvent, waitFor } from 'design/utils/testing';

import { botsApiResponseFixture } from 'teleport/Bots/fixtures';
import { ContextProvider } from 'teleport/index';
import {
  allAccessAcl,
  createTeleportContext,
  noAccess,
} from 'teleport/mocks/contexts';
import api from 'teleport/services/api';
import TeleportContext from 'teleport/teleportContext';

import { Bots } from './Bots';

function renderWithContext(element, ctx?: TeleportContext) {
  if (!ctx) {
    ctx = createTeleportContext();
  }
  return render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>{element}</ContextProvider>
    </MemoryRouter>
  );
}

test('fetches bots on load', async () => {
  jest.spyOn(api, 'get').mockResolvedValueOnce({ ...botsApiResponseFixture });
  renderWithContext(<Bots />);

  await waitFor(() => {
    expect(
      screen.getByText(botsApiResponseFixture.items[0].metadata.name)
    ).toBeInTheDocument();
  });
  expect(api.get).toHaveBeenCalledTimes(1);
});

test('shows empty state when bots are empty', async () => {
  jest.spyOn(api, 'get').mockResolvedValue({ items: [] });
  renderWithContext(<Bots />);

  await waitFor(() => {
    expect(screen.getByTestId('bots-empty-state')).toBeInTheDocument();
  });
});

test('shows missing permissions error if user lacks permissions to list', async () => {
  jest.spyOn(api, 'get').mockResolvedValue({ items: [] });
  const ctx = createTeleportContext();
  ctx.storeUser.setState({ acl: { ...allAccessAcl, bots: noAccess } });
  renderWithContext(<Bots />, ctx);

  await waitFor(() => {
    expect(screen.getByTestId('bots-empty-state')).toBeInTheDocument();
  });
  expect(
    screen.getByText(/You do not have permission to access Bots/i)
  ).toBeInTheDocument();
});

test('calls edit endpoint', async () => {
  jest
    .spyOn(api, 'get')
    .mockResolvedValueOnce({ ...botsApiResponseFixture })
    .mockResolvedValueOnce(['role-1', 'editor']);
  jest.spyOn(api, 'put').mockResolvedValue({});
  renderWithContext(<Bots />);

  await waitFor(() => {
    expect(
      screen.getByText(botsApiResponseFixture.items[0].metadata.name)
    ).toBeInTheDocument();
  });

  const actionCells = screen.queryAllByRole('button', { name: 'Options' });
  expect(actionCells).toHaveLength(botsApiResponseFixture.items.length);
  await userEvent.click(actionCells[0]);

  expect(screen.getByText('Edit...')).toBeInTheDocument();
  await userEvent.click(screen.getByText('Edit...'));

  expect(screen.getByText('Edit Bot')).toBeInTheDocument();
  await userEvent.click(screen.queryByRole('button', { name: 'Save' }));

  expect(screen.queryByText('Edit Bot')).not.toBeInTheDocument();
  expect(api.put).toHaveBeenCalledWith(
    `/v1/webapi/sites/localhost/machine-id/bot/${botsApiResponseFixture.items[0].metadata.name}`,
    { roles: ['bot-bot-role'] }
  );
});

test('calls delete endpoint', async () => {
  jest
    .spyOn(api, 'get')
    .mockResolvedValueOnce({ ...botsApiResponseFixture })
    .mockResolvedValueOnce(['role-1', 'editor']);
  jest.spyOn(api, 'delete').mockResolvedValue({});
  renderWithContext(<Bots />);

  await waitFor(() => {
    expect(
      screen.getByText(botsApiResponseFixture.items[0].metadata.name)
    ).toBeInTheDocument();
  });

  const actionCells = screen.queryAllByRole('button', { name: 'Options' });
  expect(actionCells).toHaveLength(botsApiResponseFixture.items.length);
  await userEvent.click(actionCells[0]);

  expect(screen.getByText('Delete...')).toBeInTheDocument();
  await userEvent.click(screen.getByText('Delete...'));

  expect(screen.getByText('Delete Bot?')).toBeInTheDocument();
  await userEvent.click(
    screen.queryByRole('button', { name: 'Yes, Delete Bot' })
  );

  expect(screen.queryByText('Delete Bot?')).not.toBeInTheDocument();
  expect(api.delete).toHaveBeenCalledWith(
    `/v1/webapi/sites/localhost/machine-id/bot/${botsApiResponseFixture.items[0].metadata.name}`
  );
});
