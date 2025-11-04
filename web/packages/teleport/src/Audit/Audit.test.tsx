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

import { QueryClientProvider } from '@tanstack/react-query';
import { createMemoryHistory } from 'history';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';
import { PropsWithChildren } from 'react';
import { MemoryRouter, Route, Router } from 'react-router';

import { darkTheme } from 'design/theme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import {
  act,
  render,
  screen,
  testQueryClient,
  userEvent,
} from 'design/utils/testing';

import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { makeEvent } from 'teleport/services/audit';
import TeleportContext from 'teleport/teleportContext';

import { ContextProvider } from '..';
import { AuditContainer } from './Audit';

const mio = mockIntersectionObserver();

describe('Audit', () => {
  afterEach(() => {
    testQueryClient.clear();
  });

  it('adds search to URL when searching', async () => {
    const ctx = createTeleportContext();
    jest
      .spyOn(ctx.auditService, 'fetchEventsV2')
      .mockResolvedValue({ events: [], startKey: '' });
    jest.spyOn(ctx.clusterService, 'fetchClusters').mockResolvedValue([]);

    const { history, user } = renderComponent(ctx);
    jest.spyOn(history, 'push');
    act(mio.enterAll);

    const search = await screen.findByPlaceholderText('Search...');
    await user.type(search, 'test-search');
    await user.type(search, '{enter}');

    expect(history.push).toHaveBeenCalledWith({
      pathname: '/web/cluster/root/audit',
      search: expect.stringContaining('search=test-search'),
    });
  });

  it('sets sort direction when clicking table header', async () => {
    const ctx = createTeleportContext();
    const mockEvent = {
      codeDesc: 'Local Login',
      message: 'Local user [root] successfully logged in',
      id: 'user.login:2021-05-25T14:37:27.848Z',
      code: 'T1000I',
      user: 'root',
      time: new Date('2021-05-25T14:37:27.848Z'),
      raw: {
        cluster_name: 'im-a-cluster-name',
        code: 'T1000I',
        ei: 0,
        event: 'user.login',
        method: 'local',
        success: true,
        time: '2021-05-25T14:37:27.848Z',
        user: 'root',
      },
    };

    jest
      .spyOn(ctx.auditService, 'fetchEventsV2')
      .mockResolvedValue({ events: [makeEvent(mockEvent)], startKey: '' });
    jest.spyOn(ctx.clusterService, 'fetchClusters').mockResolvedValue([]);

    const { history, user } = renderComponent(ctx);
    jest.spyOn(history, 'replace');
    act(mio.enterAll);

    const timeHeader = await screen.findByText(/Created \(UTC\)/i);
    await user.click(timeHeader);

    expect(history.replace).toHaveBeenCalledWith({
      pathname: '/web/cluster/root/audit',
      search: expect.stringContaining('order=ASC'),
    });
  });
});

function renderComponent(ctx: TeleportContext) {
  const user = userEvent.setup();
  const history = createMemoryHistory({
    initialEntries: ['/web/cluster/root/audit'],
  });

  return {
    ...render(<AuditContainer />, {
      wrapper: makeWrapper({ history, ctx }),
    }),
    user,
    history,
  };
}

function makeWrapper({
  history,
  ctx,
}: {
  history: ReturnType<typeof createMemoryHistory>;
  ctx: TeleportContext;
}) {
  return ({ children }: PropsWithChildren) => {
    return (
      <MemoryRouter>
        <QueryClientProvider client={testQueryClient}>
          <ContextProvider ctx={ctx}>
            <ConfiguredThemeProvider theme={darkTheme}>
              <Router history={history}>
                <Route path={cfg.routes.audit}>{children}</Route>
              </Router>
            </ConfiguredThemeProvider>
          </ContextProvider>
        </QueryClientProvider>
      </MemoryRouter>
    );
  };
}
