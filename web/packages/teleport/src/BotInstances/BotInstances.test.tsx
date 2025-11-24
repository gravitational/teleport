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
import { setupServer } from 'msw/node';
import { PropsWithChildren } from 'react';
import { MemoryRouter, Route, Router } from 'react-router';

import { darkTheme } from 'design/theme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import {
  render,
  screen,
  testQueryClient,
  userEvent,
  waitFor,
  waitForElementToBeRemoved,
  within,
} from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { listBotInstances } from 'teleport/services/bot/bot';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import {
  getBotInstanceMetricsSuccess,
  getBotInstanceSuccess,
  listBotInstancesError,
  listBotInstancesSuccess,
} from 'teleport/test/helpers/botInstances';

import 'shared/components/TextEditor/TextEditor.mock';

import { ContextProvider } from '..';
import { BotInstances } from './BotInstances';

jest.mock('teleport/services/bot/bot', () => {
  const actual = jest.requireActual('teleport/services/bot/bot');
  return {
    listBotInstances: jest.fn((...all) => {
      return actual.listBotInstances(...all);
    }),
    getBotInstance: jest.fn((...all) => {
      return actual.getBotInstance(...all);
    }),
    getBotInstanceMetrics: jest.fn((...all) => {
      return actual.getBotInstanceMetrics(...all);
    }),
  };
});

const server = setupServer();

beforeAll(() => {
  server.listen();
});

afterEach(async () => {
  server.resetHandlers();
  await testQueryClient.resetQueries();

  jest.useRealTimers();
  jest.clearAllMocks();
});

afterAll(() => server.close());

describe('BotInstances', () => {
  it('Shows an empty state', async () => {
    server.use(
      listBotInstancesSuccess(
        {
          bot_instances: [],
          next_page_token: '',
        },
        'v1'
      )
    );
    server.use(getBotInstanceMetricsSuccess());

    renderComponent();

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(screen.getByText('No active instances')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Bot instances are ephemeral, and disappear once all issued credentials have expired.'
      )
    ).toBeInTheDocument();
  });

  it('Shows an error state', async () => {
    server.use(listBotInstancesError(500, 'something went wrong'));
    server.use(getBotInstanceMetricsSuccess());

    renderComponent();

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(screen.getByText('something went wrong')).toBeInTheDocument();
  });

  it('Shows an unsupported sort error state', async () => {
    const testErrorMessage =
      'unsupported sort, only bot_name:asc is supported, but got "blah" (desc = true)';
    server.use(listBotInstancesError(400, testErrorMessage));
    server.use(getBotInstanceMetricsSuccess());

    const { user } = renderComponent();

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(screen.getByText(testErrorMessage)).toBeInTheDocument();

    server.use(
      listBotInstancesSuccess(
        {
          bot_instances: [],
          next_page_token: '',
        },
        'v1'
      )
    );

    const resetButton = screen.getByRole('button', { name: 'Reset sort' });
    await user.click(resetButton);

    expect(screen.queryByText(testErrorMessage)).not.toBeInTheDocument();
  });

  it('Shows an unauthorised error state', async () => {
    renderComponent({
      customAcl: makeAcl({
        botInstances: {
          ...defaultAccess,
          list: false,
        },
      }),
    });

    expect(
      screen.getByText(
        'You do not have permission to access Bot instances. Missing role permissions:',
        { exact: false }
      )
    ).toBeInTheDocument();

    expect(screen.getByText('bot_instance.list')).toBeInTheDocument();
  });

  it('Shows a list', async () => {
    jest.useFakeTimers().setSystemTime(new Date('2025-05-19T08:00:00Z'));

    server.use(
      listBotInstancesSuccess(
        {
          bot_instances: [
            {
              bot_name: 'test-bot-1',
              instance_id: '5e885c66-1af3-4a36-987d-a604d8ee49d2',
              active_at_latest: '2025-05-19T07:32:00Z',
              host_name_latest: 'test-hostname',
              join_method_latest: 'github',
              version_latest: '1.0.0-dev-a12b3c',
            },
            {
              bot_name: 'test-bot-2',
              instance_id: '3c3aae3e-de25-4824-a8e9-5a531862f19a',
            },
          ],
          next_page_token: '',
        },
        'v1'
      )
    );
    server.use(getBotInstanceMetricsSuccess());

    renderComponent();

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(screen.getByText('test-bot-1/5e885c6')).toBeInTheDocument();
    expect(screen.getByText('28 minutes ago')).toBeInTheDocument();
    expect(screen.getByText('test-hostname')).toBeInTheDocument();
    expect(screen.getByTestId('res-icon-github')).toBeInTheDocument();
    expect(screen.getByText('v1.0.0-dev-a12b3c')).toBeInTheDocument();
  });

  it('Selects an item', async () => {
    server.use(
      listBotInstancesSuccess(
        {
          bot_instances: [
            {
              bot_name: 'test-bot-1',
              instance_id: '5e885c66-1af3-4a36-987d-a604d8ee49d2',
              active_at_latest: '2025-05-19T07:32:00Z',
              host_name_latest: 'test-hostname',
              join_method_latest: 'github',
              version_latest: '1.0.0-dev-a12b3c',
            },
            {
              bot_name: 'test-bot-2',
              instance_id: '3c3aae3e-de25-4824-a8e9-5a531862f19a',
            },
          ],
          next_page_token: '',
        },
        'v1'
      )
    );
    server.use(getBotInstanceMetricsSuccess());

    server.use(getBotInstanceSuccess());

    const { user } = renderComponent();

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(
      screen.queryByRole('heading', {
        name: 'test-bot-2/3c3aae3e-de25-4824-a8e9-5a531862f19a',
      })
    ).not.toBeInTheDocument();

    const item = screen.getByRole('listitem', {
      name: 'test-bot-2/3c3aae3e-de25-4824-a8e9-5a531862f19a',
    });
    await user.click(item);

    expect(
      screen.getByRole('heading', {
        name: 'test-bot-2/3c3aae3e-de25-4824-a8e9-5a531862f19a',
      })
    ).toBeInTheDocument();

    const summarySection = screen
      .getByRole('heading', {
        name: 'Summary',
      })
      .closest('section');
    expect(
      within(summarySection!).getByText('test-bot-name')
    ).toBeInTheDocument();
  });

  it('Allows paging', async () => {
    server.use(getBotInstanceMetricsSuccess());

    jest.mocked(listBotInstances).mockImplementation(
      ({ pageToken }) =>
        new Promise(resolve => {
          resolve({
            bot_instances: [
              {
                bot_name: `test-bot`,
                instance_id: crypto.randomUUID(),
                active_at_latest: `2025-05-19T07:32:00Z`,
                host_name_latest: 'test-hostname',
                join_method_latest: 'test-join-method',
                version_latest: `1.0.0-dev-a12b3c`,
              },
            ],
            next_page_token: pageToken + '.next',
          });
        })
    );

    expect(listBotInstances).toHaveBeenCalledTimes(0);

    const { user } = renderComponent();

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    const moreAction = screen.getByRole('button', { name: 'Load More' });

    expect(listBotInstances).toHaveBeenCalledTimes(1);
    expect(listBotInstances).toHaveBeenLastCalledWith(
      {
        pageSize: 32,
        pageToken: '',
        searchTerm: '',
        query: undefined,
        sortDir: 'DESC',
        sortField: 'active_at_latest',
      },
      expect.anything()
    );

    await waitFor(() => expect(moreAction).toBeEnabled());
    await user.click(moreAction);

    expect(listBotInstances).toHaveBeenCalledTimes(2);
    expect(listBotInstances).toHaveBeenLastCalledWith(
      {
        pageSize: 32,
        pageToken: '.next',
        searchTerm: '',
        query: undefined,
        sortDir: 'DESC',
        sortField: 'active_at_latest',
      },
      expect.anything()
    );

    await waitFor(() => expect(moreAction).toBeEnabled());
    await user.click(moreAction);

    expect(listBotInstances).toHaveBeenCalledTimes(3);
    expect(listBotInstances).toHaveBeenLastCalledWith(
      {
        pageSize: 32,
        pageToken: '.next.next',
        searchTerm: '',
        query: undefined,
        sortDir: 'DESC',
        sortField: 'active_at_latest',
      },
      expect.anything()
    );
  });

  it('Allows filtering (search)', async () => {
    server.use(getBotInstanceMetricsSuccess());

    jest.mocked(listBotInstances).mockImplementation(
      ({ pageToken }) =>
        new Promise(resolve => {
          resolve({
            bot_instances: [
              {
                bot_name: `test-bot`,
                instance_id: crypto.randomUUID(),
                active_at_latest: `2025-05-19T07:32:00Z`,
                host_name_latest: 'test-hostname',
                join_method_latest: 'test-join-method',
                version_latest: `1.0.0-dev-a12b3c`,
              },
            ],
            next_page_token: pageToken + '.next',
          });
        })
    );

    expect(listBotInstances).toHaveBeenCalledTimes(0);
    const { user, history } = renderComponent();
    jest.spyOn(history, 'push');

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(listBotInstances).toHaveBeenCalledTimes(1);
    expect(listBotInstances).toHaveBeenLastCalledWith(
      {
        pageSize: 32,
        pageToken: '',
        searchTerm: '',
        query: undefined,
        sortDir: 'DESC',
        sortField: 'active_at_latest',
      },
      expect.anything()
    );

    const moreAction = screen.getByRole('button', { name: 'Load More' });
    await waitFor(() => expect(moreAction).toBeEnabled());
    await user.click(moreAction);

    expect(listBotInstances).toHaveBeenCalledTimes(2);
    expect(listBotInstances).toHaveBeenLastCalledWith(
      {
        pageSize: 32,
        pageToken: '.next',
        searchTerm: '',
        query: undefined,
        sortDir: 'DESC',
        sortField: 'active_at_latest',
      },
      expect.anything()
    );

    const search = screen.getByPlaceholderText('Search...');
    await userEvent.type(search, 'test-search-term');
    await userEvent.type(search, '{enter}');

    expect(history.push).toHaveBeenLastCalledWith({
      pathname: '/web/bots/instances',
      search: 'query=test-search-term',
    });
    expect(listBotInstances).toHaveBeenCalledTimes(3);
    expect(listBotInstances).toHaveBeenLastCalledWith(
      {
        pageSize: 32,
        pageToken: '', // Should reset to the first page
        searchTerm: 'test-search-term',
        query: undefined,
        sortDir: 'DESC',
        sortField: 'active_at_latest',
      },
      expect.anything()
    );
  });

  it('Allows filtering (query)', async () => {
    server.use(getBotInstanceMetricsSuccess());

    jest.mocked(listBotInstances).mockImplementation(
      ({ pageToken }) =>
        new Promise(resolve => {
          resolve({
            bot_instances: [
              {
                bot_name: `test-bot`,
                instance_id: crypto.randomUUID(),
                active_at_latest: `2025-05-19T07:32:00Z`,
                host_name_latest: 'test-hostname',
                join_method_latest: 'test-join-method',
                version_latest: `1.0.0-dev-a12b3c`,
              },
            ],
            next_page_token: pageToken + '.next',
          });
        })
    );

    expect(listBotInstances).toHaveBeenCalledTimes(0);
    const { user, history } = renderComponent();
    jest.spyOn(history, 'push');

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(listBotInstances).toHaveBeenCalledTimes(1);
    expect(listBotInstances).toHaveBeenLastCalledWith(
      {
        pageSize: 32,
        pageToken: '',
        searchTerm: '',
        query: undefined,
        sortDir: 'DESC',
        sortField: 'active_at_latest',
      },
      expect.anything()
    );

    const moreAction = screen.getByRole('button', { name: 'Load More' });
    await waitFor(() => expect(moreAction).toBeEnabled());
    await user.click(moreAction);

    expect(listBotInstances).toHaveBeenCalledTimes(2);
    expect(listBotInstances).toHaveBeenLastCalledWith(
      {
        pageSize: 32,
        pageToken: '.next',
        searchTerm: '',
        query: undefined,
        sortDir: 'DESC',
        sortField: 'active_at_latest',
      },
      expect.anything()
    );

    const advancedToggle = screen.getByLabelText('Advanced');
    expect(advancedToggle).not.toBeChecked();
    await userEvent.click(advancedToggle);
    expect(advancedToggle).toBeChecked();

    const search = screen.getByPlaceholderText('Search...');
    await userEvent.type(search, 'test-query');
    await userEvent.type(search, '{enter}');

    expect(history.push).toHaveBeenLastCalledWith({
      pathname: '/web/bots/instances',
      search: 'query=test-query&is_advanced=1',
    });
    expect(listBotInstances).toHaveBeenCalledTimes(3);
    expect(listBotInstances).toHaveBeenLastCalledWith(
      {
        pageSize: 32,
        pageToken: '', // Should reset to the first page
        searchTerm: undefined,
        query: 'test-query',
        sortDir: 'DESC',
        sortField: 'active_at_latest',
      },
      expect.anything()
    );
  });

  it('Allows a filter to be applied from the dashboard', async () => {
    server.use(getBotInstanceMetricsSuccess());

    jest.mocked(listBotInstances).mockImplementation(
      ({ pageToken }) =>
        new Promise(resolve => {
          resolve({
            bot_instances: [],
            next_page_token: pageToken + '.next',
          });
        })
    );

    const { user, history } = renderComponent();
    jest.spyOn(history, 'push');

    await waitForElementToBeRemoved(() =>
      screen.queryByTestId('loading-dashboard')
    );

    expect(listBotInstances).toHaveBeenCalledTimes(1);
    expect(listBotInstances).toHaveBeenLastCalledWith(
      {
        pageSize: 32,
        pageToken: '',
        searchTerm: '',
        query: undefined,
        sortDir: 'DESC',
        sortField: 'active_at_latest',
      },
      expect.anything()
    );

    const item = screen.getByLabelText('Up to date');
    await user.click(item);

    expect(history.push).toHaveBeenLastCalledWith({
      pathname: '/web/bots/instances',
      search: 'query=up+to+date+filter+goes+here&is_advanced=1',
    });
    expect(listBotInstances).toHaveBeenCalledTimes(2);
    expect(listBotInstances).toHaveBeenLastCalledWith(
      {
        pageSize: 32,
        pageToken: '', // Should reset to the first page
        searchTerm: undefined,
        query: 'up to date filter goes here',
        sortDir: 'DESC',
        sortField: 'active_at_latest',
      },
      expect.anything()
    );
  });

  it('Allows sorting', async () => {
    server.use(getBotInstanceMetricsSuccess());

    jest.mocked(listBotInstances).mockImplementation(
      ({ pageToken }) =>
        new Promise(resolve => {
          resolve({
            bot_instances: [
              {
                bot_name: `test-bot`,
                instance_id: `00000000-0000-4000-0000-000000000000`,
                active_at_latest: `2025-05-19T07:32:00Z`,
                host_name_latest: 'test-hostname',
                join_method_latest: 'test-join-method',
                version_latest: `1.0.0-dev-a12b3c`,
              },
            ],
            next_page_token: pageToken + '.next',
          });
        })
    );

    expect(listBotInstances).toHaveBeenCalledTimes(0);

    const { user } = renderComponent();

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(listBotInstances).toHaveBeenCalledTimes(1);
    expect(listBotInstances).toHaveBeenLastCalledWith(
      {
        pageSize: 32,
        pageToken: '',
        searchTerm: '',
        query: undefined,
        sortDir: 'DESC',
        sortField: 'active_at_latest',
      },
      expect.anything()
    );

    const dirAction = screen.getByRole('button', { name: 'Sort direction' });
    await user.click(dirAction);

    expect(listBotInstances).toHaveBeenCalledTimes(2);
    expect(listBotInstances).toHaveBeenLastCalledWith(
      {
        pageSize: 32,
        pageToken: '',
        searchTerm: '',
        query: undefined,
        sortDir: 'ASC',
        sortField: 'active_at_latest',
      },
      expect.anything()
    );

    const sortFieldAction = screen.getByRole('button', { name: 'Sort by' });
    await user.click(sortFieldAction);
    const option = screen.getByRole('menuitem', { name: 'Bot name' });
    await user.click(option);

    expect(listBotInstances).toHaveBeenCalledTimes(3);
    expect(listBotInstances).toHaveBeenLastCalledWith(
      {
        pageSize: 32,
        pageToken: '',
        searchTerm: '',
        query: undefined,
        sortDir: 'ASC',
        sortField: 'bot_name',
      },
      expect.anything()
    );
  });
});

function renderComponent(options?: { customAcl?: ReturnType<typeof makeAcl> }) {
  const {
    customAcl = makeAcl({
      botInstances: {
        ...defaultAccess,
        read: true,
        list: true,
      },
    }),
  } = options ?? {};

  const user = userEvent.setup();
  const history = createMemoryHistory({
    initialEntries: ['/web/bots/instances'],
  });
  return {
    ...render(<BotInstances />, {
      wrapper: makeWrapper({ customAcl, history }),
    }),
    user,
    history,
  };
}

function makeWrapper(options: {
  customAcl: ReturnType<typeof makeAcl>;
  history: ReturnType<typeof createMemoryHistory>;
}) {
  const { customAcl, history } = options ?? {};

  return ({ children }: PropsWithChildren) => {
    const ctx = createTeleportContext({
      customAcl,
    });
    return (
      <MemoryRouter>
        <QueryClientProvider client={testQueryClient}>
          <ConfiguredThemeProvider theme={darkTheme}>
            <InfoGuidePanelProvider data-testid="blah">
              <ContextProvider ctx={ctx}>
                <Router history={history}>
                  <Route path={cfg.routes.botInstances}>{children}</Route>
                </Router>
              </ContextProvider>
            </InfoGuidePanelProvider>
          </ConfiguredThemeProvider>
        </QueryClientProvider>
      </MemoryRouter>
    );
  };
}
