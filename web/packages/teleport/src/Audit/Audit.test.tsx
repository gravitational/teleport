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
import { mockIntersectionObserver } from 'jsdom-testing-mocks';
import { PropsWithChildren } from 'react';

import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import { act, screen, testQueryClient, theme } from 'design/utils/testing';

import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { makeEvent } from 'teleport/services/audit';
import TeleportContext from 'teleport/teleportContext';
import { renderWithMemoryRouter } from 'teleport/test/helpers/router';

import { ContextProvider } from '..';
import { Audit, AuditContainer } from './Audit';
import type { State } from './useAuditEvents';

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

    const { user, router } = renderComponent(ctx);
    act(mio.enterAll);

    const search = await screen.findByPlaceholderText('Search...');
    await user.type(search, 'test-search');
    await user.type(search, '{enter}');

    expect(router.state.location.pathname).toBe('/web/cluster/root/audit');
    expect(router.state.location.search).toContain('search=test-search');
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

    const { user, router } = renderComponent(ctx);
    act(mio.enterAll);

    const timeHeader = await screen.findByText(/Created \(UTC\)/i);
    await user.click(timeHeader);

    expect(router.state.location.pathname).toBe('/web/cluster/root/audit');
    expect(router.state.location.search).toContain('order=ASC');
  });

  it('does not fetch next page while placeholder data is shown', async () => {
    const ctx = createTeleportContext();
    jest
      .spyOn(ctx.clusterService, 'fetchClusters')
      .mockImplementation(() => new Promise(() => {}));

    const fetchNextPage = jest.fn();

    renderWithMemoryRouter(
      <Audit
        {...makeState(ctx, {
          events: [
            makeEvent({
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
            }),
          ],
          hasNextPage: true,
          isPlaceholderData: true,
          fetchNextPage,
        })}
      />,
      {
        path: cfg.routes.audit,
        initialEntries: ['/web/cluster/root/audit'],
        wrapper: makeWrapper({ ctx }),
      }
    );

    act(mio.enterAll);

    expect(fetchNextPage).not.toHaveBeenCalled();
  });
});

function renderComponent(ctx: TeleportContext) {
  return renderWithMemoryRouter(<AuditContainer />, {
    path: cfg.routes.audit,
    initialEntries: ['/web/cluster/root/audit'],
    wrapper: makeWrapper({ ctx }),
  });
}

function makeWrapper({ ctx }: { ctx: TeleportContext }) {
  return ({ children }: PropsWithChildren) => {
    return (
      <QueryClientProvider client={testQueryClient}>
        <ContextProvider ctx={ctx}>
          <ConfiguredThemeProvider theme={theme}>
            {children}
          </ConfiguredThemeProvider>
        </ContextProvider>
      </QueryClientProvider>
    );
  };
}

function makeState(
  ctx: TeleportContext,
  overrides: Partial<State> = {}
): State {
  return {
    events: [],
    fetchNextPage: jest.fn(),
    hasNextPage: false,
    isPlaceholderData: false,
    isFetchingNextPage: false,
    isLoading: false,
    error: null,
    isSuccess: true,
    refetch: jest.fn(),
    isError: false,
    clusterId: 'root',
    range: undefined,
    setRange: jest.fn(),
    search: '',
    setSearch: jest.fn(),
    sort: { fieldName: 'time', dir: 'DESC' },
    setSort: jest.fn(),
    ctx,
    ...overrides,
  };
}
