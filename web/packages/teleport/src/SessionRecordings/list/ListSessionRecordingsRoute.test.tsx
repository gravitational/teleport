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
import { setupServer } from 'msw/node';
import { generatePath, MemoryRouter } from 'react-router';

import {
  createDeferredResponse,
  render,
  screen,
  testQueryClient,
  userEvent,
  waitFor,
} from 'design/utils/testing';

import cfg from 'teleport/config';
import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';

import { ListSessionRecordingsRoute } from './ListSessionRecordingsRoute';
import { getThumbnail, MOCK_EVENTS, MOCK_THUMBNAIL } from './mock';

const server = setupServer();

beforeAll(() => server.listen());
beforeEach(() => {
  server.use(
    getThumbnail(MOCK_THUMBNAIL),
    http.get(cfg.api.clustersPath, () => {
      return HttpResponse.json([
        {
          name: 'teleport',
          lastConnected: '2025-08-14T14:36:07.976470934Z',
          status: 'online',
          publicURL: '',
          authVersion: '',
          proxyVersion: '',
        },
      ]);
    })
  );
});
afterEach(async () => {
  server.resetHandlers();

  testQueryClient.clear();
});
afterAll(() => server.close());

const listRecordingsUrl = generatePath(cfg.api.clusterEventsRecordingsPath, {
  clusterId: 'localhost',
});

function setupTest() {
  const ctx = createTeleportContext();

  return render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <ListSessionRecordingsRoute />
      </ContextProvider>
    </MemoryRouter>
  );
}

test('displays loading indicator while fetching data', async () => {
  const deferred = createDeferredResponse({
    events: MOCK_EVENTS,
    startKey: '',
  });

  server.use(http.get(listRecordingsUrl, deferred.handler));

  setupTest();

  expect(await screen.findByTestId('indicator')).toBeInTheDocument();

  deferred.resolve();

  await waitFor(() => {
    expect(screen.queryByTestId('indicator')).not.toBeInTheDocument();
  });

  expect(screen.getByText('server-01')).toBeInTheDocument();
});

test('displays error message when request fails', async () => {
  jest.spyOn(console, 'error').mockImplementation();

  const errorMessage = 'Failed to fetch recordings';

  server.use(
    http.get(listRecordingsUrl, () => {
      return HttpResponse.json(
        { error: { message: errorMessage } },
        { status: 400 }
      );
    })
  );

  setupTest();

  expect(
    await screen.findByText(/Failed to fetch recordings/i)
  ).toBeInTheDocument();

  expect(screen.queryByText('server-01')).not.toBeInTheDocument();

  expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
});

test('retries the request on retry button click', async () => {
  jest.spyOn(console, 'error').mockImplementation();

  const errorMessage = 'Failed to fetch recordings';

  server.use(
    http.get(listRecordingsUrl, () => {
      return HttpResponse.json(
        { error: { message: errorMessage } },
        { status: 400 }
      );
    })
  );

  setupTest();

  expect(
    await screen.findByText(/Failed to fetch recordings/i)
  ).toBeInTheDocument();

  expect(screen.queryByText('server-01')).not.toBeInTheDocument();

  server.use(
    http.get(listRecordingsUrl, () => {
      return HttpResponse.json({
        events: MOCK_EVENTS,
      });
    })
  );

  const retryButton = screen.getByRole('button', { name: 'Retry' });

  expect(retryButton).toBeInTheDocument();

  await userEvent.click(retryButton);

  expect(await screen.findByText('server-01')).toBeInTheDocument();

  expect(
    screen.queryByText(/Failed to fetch recordings/i)
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: 'Retry' })
  ).not.toBeInTheDocument();
});

test('displays an error message on connection failure', async () => {
  jest.spyOn(console, 'error').mockImplementation();

  server.use(
    http.get(listRecordingsUrl, () => {
      return HttpResponse.error();
    })
  );

  setupTest();

  expect(await screen.findByText(/Failed to fetch/i)).toBeInTheDocument();
});
