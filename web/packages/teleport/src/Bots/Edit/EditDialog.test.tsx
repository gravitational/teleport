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
import { setupServer } from 'msw/node';
import { PropsWithChildren } from 'react';
import selectEvent from 'react-select-event';

import darkTheme from 'design/theme/themes/darkTheme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import {
  act,
  fireEvent,
  render,
  screen,
  testQueryClient,
  waitFor,
  waitForElementToBeRemoved,
} from 'design/utils/testing';

import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { EditBotRequest, FlatBot } from 'teleport/services/bot/types';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import {
  editBotError,
  editBotSuccess,
  getBotError,
  getBotSuccess,
} from 'teleport/test/helpers/bots';
import { successGetRoles } from 'teleport/test/helpers/roles';

import { EditDialog } from './EditDialog';

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

describe('EditDialog', () => {
  it('should show a fetch error state', async () => {
    withFetchBotError();
    withFetchRolesSuccess({ items: ['test-role'] });
    renderComponent();
    await waitForLoading();

    expect(screen.getByText('something went wrong')).toBeInTheDocument();
  });

  it('should show a read unauthorised error state', async () => {
    withFetchBotError();
    renderComponent({
      customAcl: makeAcl({
        bots: {
          ...defaultAccess,
          read: false,
        },
      }),
    });

    expect(
      screen.getByText('You do not have permission to edit this bot.', {
        exact: false,
      })
    ).toBeInTheDocument();

    const cancelButton = screen.getByRole('button', { name: 'Cancel' });
    expect(cancelButton).toBeEnabled();

    // For some reason, this test fails with "ForwardRef inside a test was not
    // wrapped in act()" errors. Perhaps it's caused by the test finishing before
    // the render has settled. The line below is a hack to get around this issue.
    await act(() => new Promise(resolve => setTimeout(resolve, 0)));
  });

  it('should show an edit unauthorised error state', async () => {
    withFetchBotSuccess();
    renderComponent({
      customAcl: makeAcl({
        bots: {
          ...defaultAccess,
          read: true,
          edit: false,
        },
      }),
    });
    await waitForLoading();

    expect(
      screen.getByText('You do not have permission to edit this bot.', {
        exact: false,
      })
    ).toBeInTheDocument();

    const cancelButton = screen.getByRole('button', { name: 'Cancel' });
    expect(cancelButton).toBeEnabled();
  });

  it('should allow roles to be edited', async () => {
    const onSuccess = jest.fn();

    withFetchBotSuccess();
    withFetchRolesSuccess({ items: ['test-role'] });
    renderComponent({ onSuccess });
    await waitForLoading();

    await inputRole('test-role');

    withSaveSuccess(1);
    const saveButton = screen.getByRole('button', { name: 'Save' });
    expect(saveButton).toBeEnabled();
    fireEvent.click(saveButton);
    await waitForSaveButton();

    expect(onSuccess).toHaveBeenCalledTimes(1);
    expect(onSuccess).toHaveBeenLastCalledWith({
      description: '',
      kind: 'bot',
      labels: new Map(),
      max_session_ttl: {
        seconds: 43200,
      },
      name: 'test-bot-name',
      namespace: '',
      revision: '',
      roles: ['admin', 'user', 'test-role'],
      status: 'active',
      subKind: '',
      traits: [
        {
          name: 'trait-1',
          values: ['value-1', 'value-2', 'value-3'],
        },
      ],
      type: null,
      version: 'v1',
    });
  });

  it('should allow traits to be edited', async () => {
    const onSuccess = jest.fn();

    withFetchBotSuccess();
    withFetchRolesSuccess();
    renderComponent({ onSuccess });
    await waitForLoading();

    const addTraitButton = screen.getByRole('button', {
      name: 'Add another trait',
    });
    fireEvent.click(addTraitButton);

    await inputTrait('logins', ['test-value']);

    withSaveSuccess();
    const saveButton = screen.getByRole('button', { name: 'Save' });
    expect(saveButton).toBeEnabled();
    fireEvent.click(saveButton);
    await waitForSaveButton();

    expect(onSuccess).toHaveBeenCalledTimes(1);
    expect(onSuccess).toHaveBeenLastCalledWith({
      description: '',
      kind: 'bot',
      labels: new Map(),
      max_session_ttl: {
        seconds: 43200,
      },
      name: 'test-bot-name',
      namespace: '',
      revision: '',
      roles: ['admin', 'user'],
      status: 'active',
      subKind: '',
      traits: [
        {
          name: 'trait-1',
          values: ['value-1', 'value-2', 'value-3'],
        },
        {
          name: 'logins',
          values: ['test-value'],
        },
      ],
      type: null,
      version: 'v1',
    });
  });

  it('should allow max session ttl to be edited', async () => {
    const onSuccess = jest.fn();

    withFetchBotSuccess();
    withFetchRolesSuccess();
    renderComponent({ onSuccess });
    await waitForLoading();

    await inputMaxSessionDuration(' 12h 30m ');

    withSaveSuccess();
    const saveButton = screen.getByRole('button', { name: 'Save' });
    expect(saveButton).toBeEnabled();
    fireEvent.click(saveButton);
    await waitForSaveButton();

    expect(onSuccess).toHaveBeenCalledTimes(1);
    expect(onSuccess).toHaveBeenLastCalledWith({
      description: '',
      kind: 'bot',
      labels: new Map(),
      max_session_ttl: {
        seconds: 43200 + 30 * 60,
      },
      name: 'test-bot-name',
      namespace: '',
      revision: '',
      roles: ['admin', 'user'],
      status: 'active',
      subKind: '',
      traits: [
        {
          name: 'trait-1',
          values: ['value-1', 'value-2', 'value-3'],
        },
      ],
      type: null,
      version: 'v1',
    });
  });

  it('should show a save error state', async () => {
    const onSuccess = jest.fn();

    withFetchBotSuccess();
    withFetchRolesSuccess();
    renderComponent({ onSuccess });
    await waitForLoading();

    // Change something to enable the save button
    await inputMaxSessionDuration('12h 30m');

    withSaveError();
    const saveButton = screen.getByRole('button', { name: 'Save' });
    expect(saveButton).toBeEnabled();
    fireEvent.click(saveButton);
    await waitForSaveButton();

    expect(screen.getByText('something went wrong')).toBeInTheDocument();

    expect(onSuccess).not.toHaveBeenCalled();
  });

  it('should show a version mismatch warning', async () => {
    const onSuccess = jest.fn();

    withFetchBotSuccess();
    withFetchRolesSuccess({ items: ['test-role'] });
    renderComponent({ onSuccess });
    await waitForLoading();

    await inputRole('test-role');
    await inputTrait('logins', ['test-value']);
    await inputMaxSessionDuration('12h 30m');

    withSaveVersionMismatch();
    const saveButton = screen.getByRole('button', { name: 'Save' });
    expect(saveButton).toBeEnabled();
    fireEvent.click(saveButton);
    await waitForSaveButton();

    expect(onSuccess).not.toHaveBeenCalled();

    expect(
      screen.getByText(
        'We could not complete your request. Your proxy (v18.0.0) may be behind the minimum required version (v17.6.1) to support this request. Ensure all proxies are upgraded and try again.'
      )
    ).toBeInTheDocument();
  });
});

async function inputRole(role: string) {
  await selectEvent.select(screen.getByLabelText('Roles'), [role]);
}

async function inputTrait(name: string, values: string[]) {
  await selectEvent.select(screen.getAllByLabelText('trait-key').at(-1)!, [
    name,
  ]);

  const traitValue = screen.getAllByLabelText('trait-values');

  for (const value of values) {
    fireEvent.change(traitValue.at(-1)!, {
      target: { value: value },
    });
    fireEvent.keyDown(traitValue.at(-1)!, { key: 'Enter' });
  }
}

async function inputMaxSessionDuration(duration: string) {
  const ttlInput = screen.getByLabelText('Max session duration');
  fireEvent.change(ttlInput, { target: { value: duration } });
}

function withFetchBotError(status = 500, message = 'something went wrong') {
  server.use(getBotError(status, message));
}

function withSaveError(status = 500, message = 'something went wrong') {
  server.use(editBotError(status, message));
}

function withSaveVersionMismatch() {
  server.use(
    editBotError(404, 'path not found', {
      proxyVersion: {
        major: 19,
        minor: 0,
        patch: 0,
        preRelease: 'dev',
        string: '18.0.0',
      },
    })
  );
}

function withFetchBotSuccess() {
  server.use(getBotSuccess());
}

function withSaveSuccess(
  version: 1 | 2 = 2,
  overrides?: Partial<EditBotRequest>
) {
  server.use(editBotSuccess(version, overrides));
}

function withFetchRolesSuccess(options?: { items: string[] }) {
  const { items = [] } = options ?? {};
  server.use(
    successGetRoles({
      items: items.map(r => ({
        name: r,
        id: r,
        kind: 'role',
        content: '',
      })),
      startKey: '',
    })
  );
}

function renderComponent(options?: {
  onCancel?: () => void;
  onSuccess?: (bot: FlatBot) => void;
  customAcl?: ReturnType<typeof makeAcl>;
}) {
  const {
    onCancel = jest.fn(),
    onSuccess = jest.fn(),
    customAcl,
  } = options ?? {};
  return render(
    <EditDialog
      botName="test-bot-name"
      onCancel={onCancel}
      onSuccess={onSuccess}
    />,
    { wrapper: makeWrapper({ customAcl }) }
  );
}

async function waitForLoading() {
  return waitForElementToBeRemoved(() => screen.queryByTestId('loading'));
}

async function waitForSaveButton() {
  return waitFor(() => {
    const saveButton = screen.getByRole('button', { name: 'Save' });
    expect(saveButton).toBeEnabled();
  });
}

function makeWrapper(params?: { customAcl?: ReturnType<typeof makeAcl> }) {
  const {
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
  } = params ?? {};
  return ({ children }: PropsWithChildren) => {
    const ctx = createTeleportContext({
      customAcl,
    });
    return (
      <QueryClientProvider client={testQueryClient}>
        <ConfiguredThemeProvider theme={darkTheme}>
          <ContextProvider ctx={ctx}>{children}</ContextProvider>
        </ConfiguredThemeProvider>
      </QueryClientProvider>
    );
  };
}
