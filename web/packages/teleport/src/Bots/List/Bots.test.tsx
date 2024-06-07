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

import React from 'react';
import { MemoryRouter } from 'react-router';
import { render, screen, userEvent, waitFor } from 'design/utils/testing';

import api from 'teleport/services/api';
import { botsApiResponseFixture } from 'teleport/Bots/fixtures';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { ContextProvider } from 'teleport/index';

import { Bots } from './Bots';

function renderWithContext(element) {
  const ctx = createTeleportContext();
  return render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>{element}</ContextProvider>
    </MemoryRouter>
  );
}

test('fetches bots on load', async () => {
  jest.spyOn(api, 'get').mockResolvedValueOnce({ ...botsApiResponseFixture });
  renderWithContext(<Bots />);

  expect(screen.getByText('Bots')).toBeInTheDocument();
  await waitFor(() => {
    expect(
      screen.getByText(botsApiResponseFixture.items[0].metadata.name)
    ).toBeInTheDocument();
  });
  expect(api.get).toHaveBeenCalledTimes(1);
});

test('calls edit endpoint', async () => {
  jest
    .spyOn(api, 'get')
    .mockResolvedValueOnce({ ...botsApiResponseFixture })
    .mockResolvedValueOnce(['role-1', 'editor']);
  jest.spyOn(api, 'put').mockResolvedValue({});
  renderWithContext(<Bots />);

  expect(screen.getByText('Bots')).toBeInTheDocument();
  await waitFor(() => {
    expect(
      screen.getByText(botsApiResponseFixture.items[0].metadata.name)
    ).toBeInTheDocument();
  });

  const actionCells = screen.queryAllByRole('button', { name: 'OPTIONS' });
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

  expect(screen.getByText('Bots')).toBeInTheDocument();
  await waitFor(() => {
    expect(
      screen.getByText(botsApiResponseFixture.items[0].metadata.name)
    ).toBeInTheDocument();
  });

  const actionCells = screen.queryAllByRole('button', { name: 'OPTIONS' });
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
