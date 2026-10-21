/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import {
  act,
  render,
  screen,
  waitForElementToBeRemoved,
} from '@testing-library/react';
import { configMocks, mockIntersectionObserver } from 'jsdom-testing-mocks';
import { http, HttpResponse } from 'msw';
import {
  beforeAll,
  beforeEach,
  afterAll,
  afterEach,
  expect,
  test,
  vi,
  describe,
} from 'vitest';

import {
  createDeferredResponse,
  Providers,
  server,
  userEvent,
} from 'design/utils/testing';

import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import history from 'teleport/services/history/history';
import { storageService } from 'teleport/services/storageService';
import session from 'teleport/services/websession/websession';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

import { LoginScopePicker } from './LoginScopePicker';

configMocks({ beforeAll, afterAll, beforeEach, afterEach });
const mio = mockIntersectionObserver();

afterEach(() => {
  vi.resetAllMocks();
  storageService.clear();
});

test('displays a list of scopes', async () => {
  const { handler, resolve } = createDeferredResponse({
    scopes: ['/bar', '/foo'],
  });
  server.use(http.get(cfg.api.listAssignedScopes, handler));
  renderComponent();

  expect(await screen.findByTestId('indicator')).toBeInTheDocument();
  expect(
    screen.getByRole('button', { name: /Teleport Home Sign in/ })
  ).toBeVisible();

  resolve();
  expect(await screen.findByRole('button', { name: /\/bar/ })).toBeVisible();
  expect(screen.queryByTestId('indicator')).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: /\/foo/ })).toBeVisible();
});

test('handles errors', async () => {
  const handler = vi.fn(() =>
    HttpResponse.json({ error: { message: 'bad request' } }, { status: 400 })
  );
  server.use(http.get(cfg.api.listAssignedScopes, handler));
  renderComponent();

  expect(
    screen.getByRole('button', { name: /Teleport Home Sign in/ })
  ).toBeVisible();

  expect(await screen.findByText('bad request')).toBeVisible();

  // The request must not be restarted even if the error element is visible.
  expect(handler).toHaveBeenCalledTimes(1);
  await act(async () => {
    mio.enterAll();
  });
  expect(handler).toHaveBeenCalledTimes(1);
});

describe('scope buttons', async () => {
  beforeEach(() => {
    server.use(
      http.get(cfg.api.listAssignedScopes, () =>
        HttpResponse.json({ scopes: ['/bar', '/foo'] })
      )
    );
  });

  test('Teleport Home button navigates to the home screen', async () => {
    const user = userEvent.setup();
    vi.spyOn(history, 'getEntryRoute').mockReturnValue(cfg.routes.root);
    vi.spyOn(history, 'push').mockImplementation(() => {});
    renderComponent();
    await user.click(
      await screen.findByRole('button', { name: /Teleport Home/ })
    );
    expect(history.push).toHaveBeenCalledWith(cfg.routes.root, true);
  });

  test('scope buttons switch scopes', async () => {
    const user = userEvent.setup();
    vi.spyOn(session, 'switchScope').mockImplementation(async () => {});
    renderComponent();
    await user.click(await screen.findByRole('button', { name: /\/foo/ }));
    expect(session.switchScope).toHaveBeenCalledWith('/foo');
    await user.click(screen.getByRole('button', { name: /\/bar/ }));
    expect(session.switchScope).toHaveBeenCalledWith('/bar');
  });
});

test('pagination', async () => {
  server.use(
    http.get(cfg.api.listAssignedScopes, ({ request }) => {
      const url = new URL(request.url);
      const pageToken = url.searchParams.get('page_token');
      switch (pageToken) {
        case null:
          return HttpResponse.json({
            scopes: ['/bar-1', '/bar-2'],
            next_page_token: '/foo-1',
          });
        case '/foo-1':
          return HttpResponse.json({ scopes: ['/foo-1', '/foo-2'] });
        default:
          throw new Error(`Unexpected page_token: ${pageToken}`);
      }
    })
  );
  renderComponent();

  expect(await screen.findByRole('button', { name: /\/bar-1/ })).toBeVisible();
  expect(screen.getByRole('button', { name: /\/bar-2/ })).toBeVisible();
  expect(
    screen.queryByRole('button', { name: /\/foo-1/ })
  ).not.toBeInTheDocument();

  mio.enterAll();
  expect(await screen.findByRole('button', { name: /\/foo-1/ })).toBeVisible();
  expect(screen.getByRole('button', { name: /\/foo-2/ })).toBeVisible();
});

test('filtering', async () => {
  const user = userEvent.setup();
  server.use(
    http.get(cfg.api.listAssignedScopes, ({ request }) => {
      const url = new URL(request.url);
      const prefix = url.searchParams.get('prefix');
      switch (prefix) {
        case '/':
          return HttpResponse.json({
            scopes: ['/bar/a', '/bar/b', '/foo/a', '/foo/b'],
          });
        case '/foo':
          return HttpResponse.json({ scopes: ['/foo/a', '/foo/b'] });
        case '/bar':
          return HttpResponse.json({ scopes: ['/bar/a', '/bar/b'] });
        default:
          throw new Error(`Unexpected prefix: ${prefix}`);
      }
    })
  );
  renderComponent();

  const fooA = /\/foo\/a/;
  const fooB = /\/foo\/b/;
  const barA = /\/bar\/a/;
  const barB = /\/bar\/b/;

  expect(await screen.findByRole('button', { name: barA })).toBeVisible();
  expect(screen.getByRole('button', { name: barB })).toBeVisible();
  expect(screen.getByRole('button', { name: fooA })).toBeVisible();
  expect(screen.getByRole('button', { name: fooB })).toBeVisible();

  await user.type(screen.getByRole('searchbox'), '/foo');
  await waitForElementToBeRemoved(() =>
    screen.queryByRole('button', { name: barA })
  );
  expect(screen.queryByRole('button', { name: barB })).not.toBeInTheDocument();
  expect(await screen.findByRole('button', { name: fooA })).toBeVisible();
  expect(screen.getByRole('button', { name: fooB })).toBeVisible();

  await user.clear(screen.getByRole('searchbox'));
  await user.type(screen.getByRole('searchbox'), '/bar');
  expect(await screen.findByRole('button', { name: barA })).toBeVisible();
  expect(screen.getByRole('button', { name: barB })).toBeVisible();
  expect(screen.queryByRole('button', { name: fooA })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: fooB })).not.toBeInTheDocument();
});

function renderComponent() {
  const ctx = createTeleportContext();
  render(
    <TeleportContextProvider ctx={ctx}>
      <Providers>
        <LoginScopePicker />
      </Providers>
    </TeleportContextProvider>
  );
}
