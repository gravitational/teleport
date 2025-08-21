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

import darkTheme from 'design/theme/themes/darkTheme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import {
  fireEvent,
  render,
  screen,
  testQueryClient,
  waitForElementToBeRemoved,
  within,
} from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import cfg from 'teleport/config';
import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { EditBotRequest } from 'teleport/services/bot/types';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import { listBotInstancesSuccess } from 'teleport/test/helpers/botInstances';
import {
  editBotSuccess,
  getBotError,
  getBotSuccess,
} from 'teleport/test/helpers/bots';
import { successGetRoles } from 'teleport/test/helpers/roles';
import {
  listV2TokensError,
  listV2TokensMfaError,
  listV2TokensSuccess,
} from 'teleport/test/helpers/tokens';

import { BotDetails } from './BotDetails';

const server = setupServer();

beforeAll(() => {
  server.listen();
});

afterEach(async () => {
  server.resetHandlers();
  await testQueryClient.resetQueries();

  jest.clearAllMocks();
});

afterAll(() => server.close());

describe('BotDetails', () => {
  it('should show a page error state', async () => {
    withFetchError();
    renderComponent();
    await waitForLoadingBot();

    expect(screen.getByText('something went wrong')).toBeInTheDocument();
  });

  it('should show a not found error state', async () => {
    withFetchError(404, 'not_found');
    renderComponent();
    await waitForLoadingBot();

    expect(
      screen.getByText('Bot test-bot-name does not exist')
    ).toBeInTheDocument();
  });

  it('should allow back navigation', async () => {
    const history = createMemoryHistory({
      initialEntries: ['/web/bot/test-bot-name'],
    });
    history.goBack = jest.fn();

    withFetchSuccess();
    withFetchJoinTokensSuccess();
    withFetchInstancesSuccess();
    renderComponent({ history });
    await waitForLoadingBot();

    const backButton = screen.getByLabelText('back');
    fireEvent.click(backButton);

    expect(history.goBack).toHaveBeenCalledTimes(1);
  });

  it('should show page title', async () => {
    withFetchSuccess();
    withFetchJoinTokensSuccess();
    withFetchInstancesSuccess();
    renderComponent();
    await waitForLoadingBot();

    const pageHeader = screen.getByTestId('page-header');
    expect(pageHeader).toBeInTheDocument();

    expect(within(pageHeader).getByText('test-bot-name')).toBeInTheDocument();
  });

  it('should show bot metadata', async () => {
    withFetchSuccess();
    withFetchJoinTokensSuccess();
    withFetchInstancesSuccess();
    renderComponent();
    await waitForLoadingBot();

    const panel = screen
      .getByRole('heading', { name: 'Metadata' })
      .closest('section');
    expect(panel).toBeInTheDocument();

    expect(within(panel!).getByText('test-bot-name')).toBeInTheDocument();
    expect(within(panel!).getByText('12h')).toBeInTheDocument();
  });

  it('should show bot roles', async () => {
    withFetchSuccess();
    withFetchJoinTokensSuccess();
    withFetchInstancesSuccess();
    renderComponent();
    await waitForLoadingBot();

    const panel = screen
      .getByRole('heading', { name: 'Roles' })
      .closest('section');
    expect(panel).toBeInTheDocument();

    expect(within(panel!).getByText('admin')).toBeInTheDocument();
    expect(within(panel!).getByText('user')).toBeInTheDocument();
  });

  it('should show bot traits', async () => {
    withFetchSuccess();
    withFetchJoinTokensSuccess();
    withFetchInstancesSuccess();
    renderComponent();
    await waitForLoadingBot();

    const panel = screen
      .getByRole('heading', { name: 'Traits' })
      .closest('section');
    expect(panel).toBeInTheDocument();

    expect(within(panel!).getByText('trait-1')).toBeInTheDocument();
    expect(within(panel!).getByText('value-1')).toBeInTheDocument();
    expect(within(panel!).getByText('value-2')).toBeInTheDocument();
    expect(within(panel!).getByText('value-3')).toBeInTheDocument();
  });

  it('should show bot join tokens', async () => {
    withFetchSuccess();
    withFetchJoinTokensSuccess();
    withFetchInstancesSuccess();
    renderComponent();
    await waitForLoadingBot();
    await waitForLoadingTokens();

    const panel = screen
      .getByRole('heading', { name: 'Join Tokens' })
      .closest('section');
    expect(panel).toBeInTheDocument();

    expect(within(panel!).getByText('github')).toBeInTheDocument();
    expect(within(panel!).getByText('iam')).toBeInTheDocument();
    expect(within(panel!).getByText('oracle')).toBeInTheDocument();
  });

  it('should show bot join tokens outdated proxy warning', async () => {
    withFetchSuccess();
    withFetchJoinTokensOutdatedProxy();
    withFetchInstancesSuccess();
    renderComponent();
    await waitForLoadingBot();
    await waitForLoadingTokens();

    const panel = screen
      .getByRole('heading', { name: 'Join Tokens' })
      .closest('section');
    expect(panel).toBeInTheDocument();

    expect(
      within(panel!).getByText(
        'We could not complete your request. Your proxy (v18.0.0) may be behind the minimum required version (v19.0.0) to support this request. Ensure all proxies are upgraded and try again.'
      )
    ).toBeInTheDocument();
  });

  it('should show bot join tokens mfa message', async () => {
    withFetchSuccess();
    withFetchJoinTokensMfaError();
    withFetchInstancesSuccess();
    renderComponent();
    await waitForLoadingBot();
    await waitForLoadingTokens();

    const panel = screen
      .getByRole('heading', { name: 'Join Tokens' })
      .closest('section');
    expect(panel).toBeInTheDocument();

    expect(
      within(panel!).getByText(
        'Multi-factor authentication is required to view join tokens'
      )
    ).toBeInTheDocument();
  });

  it('should show bot instances', async () => {
    withFetchSuccess();
    withFetchJoinTokensSuccess();
    withFetchInstancesSuccess();
    renderComponent();
    await waitForLoadingBot();
    await waitForLoadingInstances();

    const panel = screen
      .getByRole('heading', { name: 'Active Instances' })
      .closest('section');
    expect(panel).toBeInTheDocument();

    expect(
      within(panel!).getByText('c11250e0-00c2-4f52-bcdf-b367f80b9461')
    ).toBeInTheDocument();
  });

  it('should show an unauthorised error state', async () => {
    renderComponent({
      customAcl: makeAcl({
        bots: {
          ...defaultAccess,
          read: false,
        },
      }),
    });
    expect(
      screen.getByText('You do not have permission to view this bot.', {
        exact: false,
      })
    ).toBeInTheDocument();
  });

  describe('Edit', () => {
    it('should disable edit action if no edit permission', async () => {
      withFetchSuccess();
      withFetchJoinTokensSuccess();
      withFetchInstancesSuccess();
      renderComponent({
        customAcl: makeAcl({
          bots: {
            ...defaultAccess,
            read: true,
          },
        }),
      });
      await waitForLoadingBot();

      expect(screen.getByText('Edit Bot')).toBeDisabled();
      expect(screen.getByText('Edit')).toBeDisabled();
    });

    it('should show edit form on edit action', async () => {
      withFetchSuccess();
      withFetchJoinTokensSuccess();
      withFetchInstancesSuccess();
      renderComponent();
      await waitForLoadingBot();

      withFetchRolesSuccess();
      const editButton = screen.getByRole('button', { name: 'Edit Bot' });
      fireEvent.click(editButton);

      expect(
        screen.getByText('Bot name cannot be changed')
      ).toBeInTheDocument();

      const cancelButton = screen.getByRole('button', { name: 'Cancel' });
      fireEvent.click(cancelButton);

      expect(
        screen.queryByText('Bot name cannot be changed')
      ).not.toBeInTheDocument();
    });

    it("should update the bot's details on edit success", async () => {
      withFetchSuccess();
      withFetchJoinTokensSuccess();
      withFetchInstancesSuccess();
      renderComponent();
      await waitForLoadingBot();

      let configPanel = screen
        .getByRole('heading', { name: 'Metadata' })
        .closest('section');
      expect(configPanel).toBeInTheDocument();
      expect(within(configPanel!).getByText('12h')).toBeInTheDocument();

      let rolesPanel = screen
        .getByRole('heading', { name: 'Roles' })
        .closest('section');
      expect(rolesPanel).toBeInTheDocument();
      expect(within(rolesPanel!).getByText('admin')).toBeInTheDocument();
      expect(within(rolesPanel!).getByText('user')).toBeInTheDocument();

      let traitsPanel = screen
        .getByRole('heading', { name: 'Traits' })
        .closest('section');
      expect(traitsPanel).toBeInTheDocument();
      expect(within(traitsPanel!).getByText('trait-1')).toBeInTheDocument();
      expect(within(traitsPanel!).getByText('value-1')).toBeInTheDocument();
      expect(within(traitsPanel!).getByText('value-2')).toBeInTheDocument();
      expect(within(traitsPanel!).getByText('value-3')).toBeInTheDocument();

      withFetchRolesSuccess();
      const editButton = screen.getByRole('button', { name: 'Edit Bot' });
      fireEvent.click(editButton);

      // Change something to enable the save button
      await inputMaxSessionDuration('12h 30m');

      withSaveSuccess(2, {
        roles: ['role-1'],
        traits: [
          {
            name: 'trait-2',
            values: ['value-3'],
          },
        ],
        max_session_ttl: '12h30m',
      });
      const saveButton = screen.getByRole('button', { name: 'Save' });
      fireEvent.click(saveButton);
      await waitForElementToBeRemoved(() =>
        screen.queryByRole('button', { name: 'Save' })
      );

      configPanel = screen
        .getByRole('heading', { name: 'Metadata' })
        .closest('section');
      expect(configPanel).toBeInTheDocument();
      expect(within(configPanel!).getByText('12h 30m')).toBeInTheDocument();

      rolesPanel = screen
        .getByRole('heading', { name: 'Roles' })
        .closest('section');
      expect(rolesPanel).toBeInTheDocument();
      expect(within(rolesPanel!).getByText('role-1')).toBeInTheDocument();

      traitsPanel = screen
        .getByRole('heading', { name: 'Traits' })
        .closest('section');
      expect(traitsPanel).toBeInTheDocument();
      expect(within(traitsPanel!).getByText('trait-2')).toBeInTheDocument();
      expect(within(traitsPanel!).getByText('value-3')).toBeInTheDocument();
    });
  });
});

async function inputMaxSessionDuration(duration: string) {
  const ttlInput = screen.getByLabelText('Max session duration');
  fireEvent.change(ttlInput, { target: { value: duration } });
}

const renderComponent = (options?: {
  history?: ReturnType<typeof createMemoryHistory>;
  customAcl?: ReturnType<typeof makeAcl>;
}) => {
  return render(<BotDetails />, {
    wrapper: makeWrapper(options),
  });
};

const waitForLoadingBot = async () => {
  await waitForElementToBeRemoved(() => screen.queryByTestId('loading-bot'));
};

const waitForLoadingTokens = async () => {
  await waitForElementToBeRemoved(() => screen.queryByTestId('loading-tokens'));
};

const waitForLoadingInstances = async () => {
  await waitForElementToBeRemoved(() =>
    screen.queryByTestId('loading-instances')
  );
};

const withFetchError = (status = 500, message = 'something went wrong') => {
  server.use(getBotError(status, message));
};

const withFetchSuccess = () => {
  server.use(getBotSuccess());
};

const withFetchJoinTokensSuccess = () => {
  server.use(listV2TokensSuccess());
};

const withFetchJoinTokensMfaError = () => {
  server.use(listV2TokensMfaError());
};

const withFetchJoinTokensOutdatedProxy = () => {
  server.use(
    listV2TokensError(404, 'path not found', {
      proxyVersion: {
        major: 19,
        minor: 0,
        patch: 0,
        preRelease: 'dev',
        string: '18.0.0',
      },
    })
  );
};

function withFetchInstancesSuccess() {
  server.use(
    listBotInstancesSuccess({
      bot_instances: [
        {
          bot_name: 'ansible-worker',
          instance_id: 'c11250e0-00c2-4f52-bcdf-b367f80b9461',
          active_at_latest: '2025-07-22T10:54:00Z',
          host_name_latest: 'svr-lon-01-ab23cd',
          join_method_latest: 'github',
          os_latest: 'linux',
          version_latest: '4.4.16',
        },
      ],
      next_page_token: '',
    })
  );
}

const withSaveSuccess = (
  version: 1 | 2 = 2,
  overrides?: Partial<EditBotRequest>
) => {
  server.use(editBotSuccess(version, overrides));
};

function withFetchRolesSuccess() {
  server.use(
    successGetRoles({
      items: [],
      startKey: '',
    })
  );
}

function makeWrapper(options?: {
  history?: ReturnType<typeof createMemoryHistory>;
  customAcl?: ReturnType<typeof makeAcl>;
}) {
  const {
    history = createMemoryHistory({
      initialEntries: ['/web/bot/test-bot-name'],
    }),
    customAcl = makeAcl({
      bots: {
        ...defaultAccess,
        read: true,
        edit: true,
      },
      roles: {
        ...defaultAccess,
        list: true,
      },
      tokens: {
        ...defaultAccess,
        list: true,
      },
      botInstances: {
        ...defaultAccess,
        list: true,
      },
    }),
  } = options ?? {};
  return ({ children }: PropsWithChildren) => {
    const ctx = createTeleportContext({
      customAcl,
    });
    return (
      <MemoryRouter>
        <QueryClientProvider client={testQueryClient}>
          <ConfiguredThemeProvider theme={darkTheme}>
            <ContextProvider ctx={ctx}>
              <InfoGuidePanelProvider>
                <Router history={history}>
                  <Route path={cfg.routes.bot}>{children}</Route>
                </Router>
              </InfoGuidePanelProvider>
            </ContextProvider>
          </ConfiguredThemeProvider>
        </QueryClientProvider>
      </MemoryRouter>
    );
  };
}
