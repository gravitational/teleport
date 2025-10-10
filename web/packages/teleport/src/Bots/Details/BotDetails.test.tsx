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

/* eslint-disable testing-library/no-node-access */

import { QueryClientProvider } from '@tanstack/react-query';
import { UserEvent } from '@testing-library/user-event';
import { createMemoryHistory } from 'history';
import { setupServer } from 'msw/node';
import { PropsWithChildren } from 'react';
import { MemoryRouter, Route, Router } from 'react-router';

import darkTheme from 'design/theme/themes/darkTheme';
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
import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { EditBotRequest } from 'teleport/services/bot/types';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import { listBotInstancesSuccess } from 'teleport/test/helpers/botInstances';
import {
  deleteBotSuccess,
  EditBotApiVersion,
  editBotSuccess,
  getBotError,
  getBotSuccess,
} from 'teleport/test/helpers/bots';
import {
  createLockSuccess,
  listV2LocksSuccess,
  removeLockSuccess,
} from 'teleport/test/helpers/locks';
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
    withListLocksSuccess();
    renderComponent();
    await waitForLoadingBot();

    expect(screen.getByText('something went wrong')).toBeInTheDocument();
  });

  it('should show a not found error state', async () => {
    withFetchError(404, 'not_found');
    withListLocksSuccess();
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
    withListLocksSuccess();
    const { user } = renderComponent({ history });
    await waitForLoadingBot();

    const backButton = screen.getByLabelText('back');
    await user.click(backButton);

    expect(history.goBack).toHaveBeenCalledTimes(1);
  });

  it('should show page title', async () => {
    withFetchSuccess();
    withFetchJoinTokensSuccess();
    withFetchInstancesSuccess();
    withListLocksSuccess();
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
    withListLocksSuccess();
    renderComponent();
    await waitForLoadingBot();

    const panel = screen
      .getByRole('heading', { name: 'Metadata' })
      .closest('section');
    expect(panel).toBeInTheDocument();

    expect(within(panel!).getByText('test-bot-name')).toBeInTheDocument();
    expect(
      within(panel!).getByText("This is the bot's description.")
    ).toBeInTheDocument();
    expect(within(panel!).getByText('12h')).toBeInTheDocument();
  });

  it('should show bot roles', async () => {
    withFetchSuccess();
    withFetchJoinTokensSuccess();
    withFetchInstancesSuccess();
    withListLocksSuccess();
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
    withListLocksSuccess();
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
    withListLocksSuccess();
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
    withListLocksSuccess();
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
    withListLocksSuccess();
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
    withListLocksSuccess();
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
      withListLocksSuccess();
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
      withListLocksSuccess();
      const { user } = renderComponent();
      await waitForLoadingBot();

      withFetchRolesSuccess();
      const editButton = screen.getByRole('button', { name: 'Edit Bot' });
      await user.click(editButton);

      expect(
        screen.getByText('Bot name cannot be changed')
      ).toBeInTheDocument();

      const cancelButton = screen.getByRole('button', { name: 'Cancel' });
      await user.click(cancelButton);

      expect(
        screen.queryByText('Bot name cannot be changed')
      ).not.toBeInTheDocument();
    });

    it("should update the bot's details on edit success", async () => {
      withFetchSuccess();
      withFetchJoinTokensSuccess();
      withFetchInstancesSuccess();
      withListLocksSuccess();
      const { user } = renderComponent();
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
      await user.click(editButton);

      // Change something to enable the save button
      await inputMaxSessionDuration(user, '12h 30m');

      withSaveSuccess('v2', {
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
      await user.click(saveButton);
      await waitFor(() => {
        expect(
          screen.queryByRole('button', { name: 'Save' })
        ).not.toBeInTheDocument();
      });

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

  describe('Locks', () => {
    it('should show an overflow option to lock the bot', async () => {
      withFetchSuccess();
      withFetchJoinTokensSuccess();
      withFetchInstancesSuccess();
      withListLocksSuccess({
        locks: [],
      });
      const { user } = renderComponent();
      await waitForLoadingBot();

      expect(screen.queryByText('Locked')).not.toBeInTheDocument();

      const overflowButton = screen.getByTestId('overflow-btn-open');
      await user.click(overflowButton);

      const lockButton = screen.getByText('Lock Bot...');
      expect(lockButton).toBeInTheDocument();
      await user.click(lockButton!);

      expect(screen.getByText('Lock bot-test-bot-name?')).toBeInTheDocument();

      withLockSuccess();
      const submitButton = screen.getByRole('button', { name: 'Create Lock' });
      expect(submitButton).toBeEnabled();
      await user.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText('Locked')).toBeInTheDocument();
      });
    });

    it('should show an overflow option to unlock the bot', async () => {
      withFetchSuccess();
      withFetchJoinTokensSuccess();
      withFetchInstancesSuccess();
      withListLocksSuccess();
      const { user } = renderComponent();
      await waitForLoadingBot();

      expect(screen.getByText('Locked')).toBeInTheDocument();

      const overflowButton = screen.getByTestId('overflow-btn-open');
      await user.click(overflowButton);

      const unlockButton = screen.getByText('Unlock Bot...');
      expect(unlockButton).toBeInTheDocument();
      await user.click(unlockButton!);

      expect(screen.getByText('Unlock bot-test-bot-name?')).toBeInTheDocument();

      withUnlockSuccess();
      const submitButton = screen.getByRole('button', { name: 'Remove Lock' });
      expect(submitButton).toBeEnabled();
      await user.click(submitButton);

      await waitFor(() => {
        expect(screen.queryByText('Locked')).not.toBeInTheDocument();
      });
    });

    it('should disable lock action if no permissions', async () => {
      withFetchSuccess();
      withFetchJoinTokensSuccess();
      withFetchInstancesSuccess();
      withListLocksSuccess({
        locks: [],
      });
      const { user } = renderComponent({
        customAcl: makeAcl({
          bots: {
            ...defaultAccess,
            read: true,
            edit: true,
          },
          lock: {
            ...defaultAccess,
            list: true,
            remove: true,
            create: false,
          },
        }),
      });
      await waitForLoadingBot();

      const overflowButton = screen.getByTestId('overflow-btn-open');
      await user.click(overflowButton);

      const lockButton = screen.getByText('Lock Bot...');
      expect(lockButton).toBeInTheDocument();
      await user.click(lockButton!);

      expect(
        screen.queryByText('Lock bot-test-bot-name?')
      ).not.toBeInTheDocument();
    });

    it('should disable unlock action if no permissions', async () => {
      withFetchSuccess();
      withFetchJoinTokensSuccess();
      withFetchInstancesSuccess();
      withListLocksSuccess();
      const { user } = renderComponent({
        customAcl: makeAcl({
          bots: {
            ...defaultAccess,
            read: true,
            edit: true,
          },
          lock: {
            ...defaultAccess,
            list: true,
            remove: false,
            create: true,
          },
        }),
      });
      await waitForLoadingBot();

      expect(screen.getByText('Locked')).toBeInTheDocument();

      const overflowButton = screen.getByTestId('overflow-btn-open');
      await user.click(overflowButton);

      const unlockButton = screen.getByText('Unlock Bot...');
      expect(unlockButton).toBeInTheDocument();
      await user.click(unlockButton!);

      expect(
        screen.queryByText('Unlock bot-test-bot-name?')
      ).not.toBeInTheDocument();
    });
  });

  describe('Delete', () => {
    it('should show an overflow option to delete the bot', async () => {
      const history = createMemoryHistory({
        initialEntries: ['/web/bot/test-bot-name'],
      });
      history.replace = jest.fn();

      withFetchSuccess();
      withFetchJoinTokensSuccess();
      withFetchInstancesSuccess();
      withListLocksSuccess({
        locks: [],
      });
      withDeleteBotSuccess();
      const { user } = renderComponent({ history });
      await waitForLoadingBot();

      const overflowButton = screen.getByTestId('overflow-btn-open');
      await user.click(overflowButton);

      const deleteButton = screen.getByText('Delete Bot...');
      expect(deleteButton).toBeInTheDocument();
      await user.click(deleteButton!);

      expect(screen.getByText('Delete test-bot-name?')).toBeInTheDocument();
      expect(screen.getByText('Lock Bot')).toBeInTheDocument();

      await user.click(screen.getByText('Delete Bot'));

      // The operation is delayed to account for backend cache lag
      await waitFor(
        () => {
          expect(
            screen.queryByText('Delete test-bot-name?')
          ).not.toBeInTheDocument();
        },
        { timeout: 5000 }
      );

      expect(history.replace).toHaveBeenCalledTimes(1);
      expect(history.replace).toHaveBeenLastCalledWith('/web/bots');
    });

    it('should disable the delete action if no permissions', async () => {
      withFetchSuccess();
      withFetchJoinTokensSuccess();
      withFetchInstancesSuccess();
      withListLocksSuccess();
      withDeleteBotSuccess();
      const { user } = renderComponent({
        customAcl: makeAcl({
          bots: {
            ...defaultAccess,
            read: true,
            edit: true,
            remove: false,
          },
          lock: {
            ...defaultAccess,
            list: true,
            remove: true,
            create: true,
            edit: true,
          },
        }),
      });
      await waitForLoadingBot();

      const overflowButton = screen.getByTestId('overflow-btn-open');
      await user.click(overflowButton);

      const deleteButton = screen.getByText('Delete Bot...');
      expect(deleteButton).toBeInTheDocument();
      await user.click(deleteButton!);

      expect(
        screen.queryByText('Delete test-bot-name?')
      ).not.toBeInTheDocument();
    });

    it('should not allow lock alternative if no permission', async () => {
      withFetchSuccess();
      withFetchJoinTokensSuccess();
      withFetchInstancesSuccess();
      withListLocksSuccess({
        locks: [],
      });
      withDeleteBotSuccess();
      const { user } = renderComponent({
        customAcl: makeAcl({
          bots: {
            ...defaultAccess,
            read: true,
            edit: true,
            remove: true,
          },
          lock: {
            ...defaultAccess,
            list: true,
            remove: true,
            create: false,
            edit: false,
          },
        }),
      });
      await waitForLoadingBot();

      const overflowButton = screen.getByTestId('overflow-btn-open');
      await user.click(overflowButton);

      const deleteButton = screen.getByText('Delete Bot...');
      expect(deleteButton).toBeInTheDocument();
      await user.click(deleteButton!);

      expect(screen.getByText('Delete test-bot-name?')).toBeInTheDocument();

      const lockButton = screen.getByText('Lock Bot');
      expect(lockButton).toBeInTheDocument();
      await user.click(lockButton!);

      expect(
        screen.queryByText('Lock bot-test-bot-name?')
      ).not.toBeInTheDocument();
    });

    it('should not show lock alternative if already locked', async () => {
      withFetchSuccess();
      withFetchJoinTokensSuccess();
      withFetchInstancesSuccess();
      withListLocksSuccess();
      withDeleteBotSuccess();
      const { user } = renderComponent();
      await waitForLoadingBot();

      const overflowButton = screen.getByTestId('overflow-btn-open');
      await user.click(overflowButton);

      const deleteButton = screen.getByText('Delete Bot...');
      expect(deleteButton).toBeInTheDocument();
      await user.click(deleteButton!);

      expect(screen.getByText('Delete test-bot-name?')).toBeInTheDocument();

      expect(screen.queryByText('Lock Bot')).not.toBeInTheDocument();
    });
  });
});

async function inputMaxSessionDuration(user: UserEvent, duration: string) {
  const ttlInput = screen.getByLabelText('Max session duration');
  await user.type(ttlInput, duration);
}

const renderComponent = (options?: {
  history?: ReturnType<typeof createMemoryHistory>;
  customAcl?: ReturnType<typeof makeAcl>;
}) => {
  const user = userEvent.setup();
  return {
    ...render(<BotDetails />, {
      wrapper: makeWrapper(options),
    }),
    user,
  };
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
    listBotInstancesSuccess(
      {
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
      },
      'v1'
    )
  );
}

function withSaveSuccess(
  version: EditBotApiVersion = 'v3',
  overrides?: Partial<EditBotRequest>
) {
  server.use(editBotSuccess(version, overrides));
}

function withFetchRolesSuccess() {
  server.use(
    successGetRoles({
      items: [],
      startKey: '',
    })
  );
}

function withListLocksSuccess(
  ...params: Parameters<typeof listV2LocksSuccess>
) {
  server.use(
    listV2LocksSuccess({
      locks: params[0]?.locks ?? [
        {
          name: '76bc5cc7-b9bf-4a03-935f-8018c0a2bc05',
          message: 'This is a test message',
          expires: '2023-12-31T23:59:59Z',
          targets: {
            user: 'bot-test-bot-name',
          },
          createdAt: '2023-01-01T00:00:00Z',
          createdBy: 'admin',
        },
      ],
    })
  );
}

function withUnlockSuccess() {
  server.use(removeLockSuccess());
}

function withLockSuccess() {
  server.use(createLockSuccess());
}

function withDeleteBotSuccess() {
  server.use(deleteBotSuccess());
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
        remove: true,
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
      lock: {
        ...defaultAccess,
        list: true,
        remove: true,
        create: true,
        edit: true,
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
