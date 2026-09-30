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

import { QueryClientProvider } from '@tanstack/react-query';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';
import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import {
  act,
  enableMswServer,
  render,
  screen,
  server,
  testQueryClient,
  userEvent,
  waitFor,
  waitForElementToBeRemoved,
  within,
} from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { botsApiResponseFixture } from 'teleport/Bots/fixtures';
import cfg from 'teleport/config';
import { ContextProvider } from 'teleport/index';
import {
  allAccessAcl,
  createTeleportContext,
  noAccess,
} from 'teleport/mocks/contexts';
import api from 'teleport/services/api';
import TeleportContext from 'teleport/teleportContext';
import {
  deleteBotSuccess,
  listBotsError,
  listBotsSuccess,
} from 'teleport/test/helpers/bots';

import { Bots } from './Bots';

enableMswServer();

const mio = mockIntersectionObserver();

afterEach(async () => {
  await testQueryClient.resetQueries();
  jest.restoreAllMocks();
});

function renderWithContext(
  element,
  {
    ctx = createTeleportContext(),
    initialEntries = ['/'],
  }: { ctx?: TeleportContext; initialEntries?: string[] } = {}
) {
  return render(
    <MemoryRouter initialEntries={initialEntries}>
      <QueryClientProvider client={testQueryClient}>
        <InfoGuidePanelProvider>
          <ContextProvider ctx={ctx}>{element}</ContextProvider>
        </InfoGuidePanelProvider>
      </QueryClientProvider>
    </MemoryRouter>
  );
}

const [firstBot, secondBot] = botsApiResponseFixture.items;

describe('Bots', () => {
  test('fetches bots on load', async () => {
    server.use(listBotsSuccess(botsApiResponseFixture));
    renderWithContext(<Bots />);

    expect(await screen.findByText(firstBot.metadata.name)).toBeInTheDocument();
  });

  test('loads the next page on scroll', async () => {
    server.use(
      http.get(cfg.api.bot.listV2, ({ request }) => {
        const token = new URL(request.url).searchParams.get('page_token');
        return HttpResponse.json(
          token === 'next'
            ? { items: [secondBot] }
            : { items: [firstBot], next_page_token: 'next' }
        );
      })
    );
    renderWithContext(<Bots />);

    expect(await screen.findByText(firstBot.metadata.name)).toBeInTheDocument();
    expect(screen.queryByText(secondBot.metadata.name)).not.toBeInTheDocument();

    act(mio.enterAll);

    expect(
      await screen.findByText(secondBot.metadata.name)
    ).toBeInTheDocument();
    expect(screen.getByText(firstBot.metadata.name)).toBeInTheDocument();
  });

  test('passes the search from the url', async () => {
    let search: string | null = null;
    server.use(
      http.get(cfg.api.bot.listV2, ({ request }) => {
        search = new URL(request.url).searchParams.get('search');
        return HttpResponse.json({ items: [] });
      })
    );
    renderWithContext(<Bots />, { initialEntries: ['/?search=needle'] });

    expect(await screen.findByText('No Bots Found')).toBeInTheDocument();
    expect(search).toBe('needle');
    expect(screen.queryByTestId('bots-empty-state')).not.toBeInTheDocument();
  });

  test('searches on submit', async () => {
    server.use(
      http.get(cfg.api.bot.listV2, ({ request }) => {
        const search = new URL(request.url).searchParams.get('search');
        return HttpResponse.json(
          search === 'slack' ? { items: [secondBot] } : botsApiResponseFixture
        );
      })
    );
    renderWithContext(<Bots />);

    expect(await screen.findByText(firstBot.metadata.name)).toBeInTheDocument();

    await userEvent.type(screen.getByPlaceholderText('Search...'), 'slack');
    await userEvent.type(screen.getByPlaceholderText('Search...'), '{enter}');

    await waitFor(() => {
      expect(
        screen.queryByText(firstBot.metadata.name)
      ).not.toBeInTheDocument();
    });
    expect(screen.getByText(secondBot.metadata.name)).toBeInTheDocument();
  });

  test('falls back to v1 and filters locally on older proxies', async () => {
    server.use(
      listBotsError(404, 'path not found', {
        proxyVersion: {
          major: 18,
          minor: 0,
          patch: 0,
          preRelease: '',
          string: '18.0.0',
        },
      }),
      listBotsSuccess(botsApiResponseFixture, 'v1')
    );
    renderWithContext(<Bots />, {
      initialEntries: [`/?search=${secondBot.metadata.name}`],
    });

    expect(
      await screen.findByText(secondBot.metadata.name)
    ).toBeInTheDocument();
    expect(screen.queryByText(firstBot.metadata.name)).not.toBeInTheDocument();
  });

  test('pages the v1 fallback locally from one fetch', async () => {
    const bots = Array.from({ length: 60 }, (_, i) => ({
      ...firstBot,
      metadata: { ...firstBot.metadata, name: `bot-${i}` },
    }));
    let v1Calls = 0;
    server.use(
      listBotsError(404, 'path not found', {
        proxyVersion: {
          major: 18,
          minor: 0,
          patch: 0,
          preRelease: '',
          string: '18.0.0',
        },
      }),
      http.get(cfg.api.bot.list, () => {
        v1Calls++;
        return HttpResponse.json({ items: bots });
      })
    );
    renderWithContext(<Bots />);

    expect(await screen.findByText('bot-49')).toBeInTheDocument();
    expect(screen.queryByText('bot-50')).not.toBeInTheDocument();

    act(mio.enterAll);

    expect(await screen.findByText('bot-59')).toBeInTheDocument();
    expect(v1Calls).toBe(1);
  });

  test('shows an error', async () => {
    server.use(listBotsError(500, 'something went wrong'));
    renderWithContext(<Bots />);

    expect(await screen.findByText('something went wrong')).toBeInTheDocument();
  });

  test('shows empty state when bots are empty', async () => {
    server.use(listBotsSuccess({ items: [] }));
    renderWithContext(<Bots />);

    expect(await screen.findByTestId('bots-empty-state')).toBeInTheDocument();
  });

  test('shows missing permissions error if user lacks permissions to list', async () => {
    const ctx = createTeleportContext();
    ctx.storeUser.setState({ acl: { ...allAccessAcl, bots: noAccess } });
    renderWithContext(<Bots />, { ctx });

    expect(await screen.findByTestId('bots-empty-state')).toBeInTheDocument();
    expect(
      screen.getByText(/You do not have permission to access Bots/i)
    ).toBeInTheDocument();
  });

  test('calls delete endpoint', async () => {
    server.use(listBotsSuccess(botsApiResponseFixture), deleteBotSuccess());
    jest.spyOn(api, 'deleteWithOptions');
    renderWithContext(<Bots />);

    expect(await screen.findByText(firstBot.metadata.name)).toBeInTheDocument();

    const actionCells = screen.queryAllByRole('button', { name: 'Options' });
    expect(actionCells).toHaveLength(botsApiResponseFixture.items.length);
    await userEvent.click(actionCells[0]);

    // Wait for the options menu to appear
    const menu = await screen.findByTestId('Modal');
    await userEvent.click(within(menu).getByText('Delete...'));

    expect(
      screen.getByText(`Delete ${firstBot.metadata.name}?`)
    ).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Delete Bot' }));

    await waitForElementToBeRemoved(
      () => screen.queryByText(`Delete ${firstBot.metadata.name}?`),
      { timeout: 5000 }
    );
    expect(api.deleteWithOptions).toHaveBeenCalledWith(
      `/v1/webapi/sites/localhost/machine-id/bot/${firstBot.metadata.name}`,
      { signal: undefined }
    );
    await waitFor(() => {
      expect(
        screen.queryByText(firstBot.metadata.name)
      ).not.toBeInTheDocument();
    });
  });
});
