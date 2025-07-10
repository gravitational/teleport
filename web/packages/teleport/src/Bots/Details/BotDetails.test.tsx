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
import {
  editBotSuccess,
  getBotError,
  getBotSuccess,
} from 'teleport/test/helpers/bots';
import { successGetRoles } from 'teleport/test/helpers/roles';

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
    await waitForLoading();

    expect(screen.getByText('Error: something went wrong')).toBeInTheDocument();
  });

  it('should show a not found error state', async () => {
    withFetchError(404, 'not_found');
    renderComponent();
    await waitForLoading();

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
    renderComponent({ history });
    await waitForLoading();

    const backButton = screen.getByLabelText('back');
    fireEvent.click(backButton);

    expect(history.goBack).toHaveBeenCalledTimes(1);
  });

  it('should show page title', async () => {
    withFetchSuccess();
    renderComponent();
    await waitForLoading();

    const pageHeader = screen.getByTestId('page-header');
    expect(pageHeader).toBeInTheDocument();

    expect(within(pageHeader).getByText('test-bot-name')).toBeInTheDocument();
  });

  it('should show bot metadata', async () => {
    withFetchSuccess();
    renderComponent();
    await waitForLoading();

    const panel = screen
      .getByRole('heading', { name: 'Metadata' })
      .closest('section');
    expect(panel).toBeInTheDocument();

    expect(within(panel!).getByText('test-bot-name')).toBeInTheDocument();
    expect(within(panel!).getByText('12h')).toBeInTheDocument();
  });

  it('should show bot roles', async () => {
    withFetchSuccess();
    renderComponent();
    await waitForLoading();

    const panel = screen
      .getByRole('heading', { name: 'Roles' })
      .closest('section');
    expect(panel).toBeInTheDocument();

    expect(within(panel!).getByText('admin')).toBeInTheDocument();
    expect(within(panel!).getByText('user')).toBeInTheDocument();
  });

  it('should show bot traits', async () => {
    withFetchSuccess();
    renderComponent();
    await waitForLoading();

    const panel = screen
      .getByRole('heading', { name: 'Traits' })
      .closest('section');
    expect(panel).toBeInTheDocument();

    expect(within(panel!).getByText('trait-1')).toBeInTheDocument();
    expect(within(panel!).getByText('value-1')).toBeInTheDocument();
    expect(within(panel!).getByText('value-2')).toBeInTheDocument();
    expect(within(panel!).getByText('value-3')).toBeInTheDocument();
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
      renderComponent({
        customAcl: makeAcl({
          bots: {
            ...defaultAccess,
            read: true,
          },
        }),
      });
      await waitForLoading();

      expect(screen.getByText('Edit Bot')).toBeDisabled();
      expect(screen.getByText('Edit')).toBeDisabled();
    });

    it('should show edit form on edit action', async () => {
      withFetchSuccess();
      renderComponent();
      await waitForLoading();

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
      renderComponent();
      await waitForLoading();

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

const waitForLoading = async () => {
  await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));
};

const withFetchError = (status = 500, message = 'something went wrong') => {
  server.use(getBotError(status, message));
};

const withFetchSuccess = () => {
  server.use(getBotSuccess());
};

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
