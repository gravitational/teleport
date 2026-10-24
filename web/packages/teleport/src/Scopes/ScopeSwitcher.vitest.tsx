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
  describe,
  vi,
} from 'vitest';

import {
  createDeferredResponse,
  Providers,
  server,
  userEvent,
} from 'design/utils/testing';

import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import {
  ScopeSwitcher,
  ScopeSwitcherProps,
} from 'teleport/Scopes/ScopeSwitcher';
import history from 'teleport/services/history/history';
import session from 'teleport/services/websession/websession';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

configMocks({ beforeAll, afterAll, beforeEach, afterEach });
const mio = mockIntersectionObserver();

test('displays a list of scopes', async () => {
  const user = userEvent.setup();
  const { handler, resolve } = createDeferredResponse({
    scopes: ['/bar', '/foo'],
  });
  server.use(http.get(cfg.api.listAssignedScopes, handler));
  // /foo is the current scope...
  renderComponent({ scope: '/foo' });

  // ...so it should be visible on the dropdown button. Note: "scope /foo" is
  // an accessible name that includes the name of the scope icon.
  await user.click(screen.getByRole('button', { name: 'scope /foo' }));
  expect(await screen.findByTestId('indicator')).toBeInTheDocument();

  resolve();
  expect(await screen.findByRole('menuitem', { name: '/bar' })).toBeVisible();
  expect(screen.queryByTestId('indicator')).not.toBeInTheDocument();
  expect(screen.getByRole('menuitem', { name: /\/foo/ })).toBeVisible();
});

test('handles errors', async () => {
  const user = userEvent.setup();
  const handler = vi.fn(() =>
    HttpResponse.json({ error: { message: 'bad request' } }, { status: 400 })
  );
  server.use(http.get(cfg.api.listAssignedScopes, handler));
  renderComponent({ scope: '/foo' });

  await user.click(screen.getByRole('button', { name: 'scope /foo' }));
  expect(screen.getByRole('menuitem', { name: /Teleport Home/ })).toBeVisible();

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
    vi.spyOn(session, 'switchScope').mockImplementation(async () => {});
  });

  test('Teleport Home button switches to an unscoped session', async () => {
    const user = userEvent.setup();
    vi.spyOn(history, 'getEntryRoute').mockReturnValue(cfg.routes.root);
    vi.spyOn(history, 'push').mockImplementation(() => {});
    renderComponent({ scope: '/foo' });
    await user.click(screen.getByRole('button', { name: 'scope /foo' }));
    await user.click(
      await screen.findByRole('menuitem', { name: /Teleport Home/ })
    );
    expect(session.switchScope).toHaveBeenCalledWith('');
  });

  test('scope buttons switch scopes', async () => {
    const user = userEvent.setup();
    vi.spyOn(session, 'switchScope').mockImplementation(async () => {});
    renderComponent({ scope: '' });
    await user.click(
      screen.getByRole('button', { name: 'scope Teleport Home' })
    );
    await user.click(await screen.findByRole('menuitem', { name: /\/foo/ }));
    expect(session.switchScope).toHaveBeenCalledWith('/foo');
    await user.click(screen.getByRole('menuitem', { name: /\/bar/ }));
    expect(session.switchScope).toHaveBeenCalledWith('/bar');
  });
});

test('pagination', async () => {
  const user = userEvent.setup();
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
  renderComponent({ scope: '' });

  await user.click(screen.getByRole('button', { name: 'scope Teleport Home' }));
  expect(
    await screen.findByRole('menuitem', { name: /\/bar-1/ })
  ).toBeVisible();
  expect(screen.getByRole('menuitem', { name: /\/bar-2/ })).toBeVisible();
  expect(
    screen.queryByRole('menuitem', { name: /\/foo-1/ })
  ).not.toBeInTheDocument();

  mio.enterAll();
  expect(
    await screen.findByRole('menuitem', { name: /\/foo-1/ })
  ).toBeVisible();
  expect(screen.getByRole('menuitem', { name: /\/foo-2/ })).toBeVisible();
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
  renderComponent({ scope: '' });
  await user.click(screen.getByRole('button', { name: 'scope Teleport Home' }));

  const fooA = /\/foo\/a/;
  const fooB = /\/foo\/b/;
  const barA = /\/bar\/a/;
  const barB = /\/bar\/b/;

  expect(await screen.findByRole('menuitem', { name: barA })).toBeVisible();
  expect(screen.getByRole('menuitem', { name: barB })).toBeVisible();
  expect(screen.getByRole('menuitem', { name: fooA })).toBeVisible();
  expect(screen.getByRole('menuitem', { name: fooB })).toBeVisible();

  await user.type(screen.getByRole('searchbox'), '/foo');
  await waitForElementToBeRemoved(() =>
    screen.queryByRole('menuitem', { name: barA })
  );
  expect(
    screen.queryByRole('menuitem', { name: barB })
  ).not.toBeInTheDocument();
  expect(await screen.findByRole('menuitem', { name: fooA })).toBeVisible();
  expect(screen.getByRole('menuitem', { name: fooB })).toBeVisible();

  await user.clear(screen.getByRole('searchbox'));
  await user.type(screen.getByRole('searchbox'), '/bar');
  expect(await screen.findByRole('menuitem', { name: barA })).toBeVisible();
  expect(screen.getByRole('menuitem', { name: barB })).toBeVisible();
  expect(
    screen.queryByRole('menuitem', { name: fooA })
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole('menuitem', { name: fooB })
  ).not.toBeInTheDocument();
});

function renderComponent(props: ScopeSwitcherProps) {
  const ctx = createTeleportContext();
  render(
    <TeleportContextProvider ctx={ctx}>
      <Providers>
        <ScopeSwitcher {...props} />
      </Providers>
    </TeleportContextProvider>
  );
}
