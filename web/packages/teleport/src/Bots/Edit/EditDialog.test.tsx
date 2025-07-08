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
    renderComponent();
    await waitForLoading();

    expect(screen.getByText('Error: something went wrong')).toBeInTheDocument();
  });

  it('should show a read unauthorised error state', async () => {
    withFetchBotSuccess();
    renderComponent({
      customAcl: makeAcl({
        bots: {
          ...defaultAccess,
          read: false,
        },
      }),
    });

    expect(
      screen.getByText('You do not have permission to view this bot.')
    ).toBeInTheDocument();
  });

  it('should show an edit unauthorised error state', async () => {
    withFetchBotSuccess();
    renderComponent({
      customAcl: makeAcl({
        bots: {
          ...defaultAccess,
          edit: false,
        },
      }),
    });

    expect(
      screen.getByText('You do not have permission to edit this bot.')
    ).toBeInTheDocument();
  });

  it('should allow roles to be edited', async () => {
    const onSuccess = jest.fn();

    withFetchBotSuccess();
    withFetchRolesSuccess({ items: ['test-role'] });
    renderComponent({ onSuccess });
    await waitForLoading();

    await inputRole('test-role');

    withSaveSuccess();
    const saveButton = screen.getByRole('button', { name: 'Save' });
    expect(saveButton).toBeEnabled();
    fireEvent.click(saveButton);
    await waitForSaveButton();

    expect(onSuccess).toHaveBeenCalledTimes(1);
    expect(onSuccess).toHaveBeenLastCalledWith(
      {
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
      },
      false
    );
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
    expect(onSuccess).toHaveBeenLastCalledWith(
      {
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
      },
      false
    );
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
    expect(onSuccess).toHaveBeenLastCalledWith(
      {
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
      },
      false
    );
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

    expect(screen.getByText('Error: something went wrong')).toBeInTheDocument();

    expect(onSuccess).not.toHaveBeenCalled();
  });

  it('should show a save inconsistency warning', async () => {
    const onSuccess = jest.fn();

    withFetchBotSuccess();
    withFetchRolesSuccess({ items: ['test-role'] });
    renderComponent({ onSuccess });
    await waitForLoading();

    await inputRole('test-role');
    await inputTrait('logins', ['test-value']);
    await inputMaxSessionDuration('12h 30m');

    // Mock the server response to deliberately not match the request, triggering the warning on all fields
    withSaveSuccess({
      roles: ['admin', 'user'],
      traits: [
        {
          name: 'trait-1',
          values: ['value-1', 'value-2', 'value-3'],
        },
      ],
      max_session_ttl: '12h',
    });
    const saveButton = screen.getByRole('button', { name: 'Save' });
    expect(saveButton).toBeEnabled();
    fireEvent.click(saveButton);
    await waitForSaveButton();

    expect(onSuccess).toHaveBeenCalled();

    expect(
      screen.getByText(
        'Warning: Some fields may not have updated correctly; max_session_ttl, roles, traits'
      )
    ).toBeInTheDocument();
  });
});

async function inputRole(role: string) {
  await selectEvent.select(screen.getByLabelText('Roles'), [role]);
}

async function inputTrait(name: string, values: string[]) {
  await selectEvent.select(screen.getAllByLabelText('Key').at(-1)!, [name]);

  const traitValue = screen.getAllByLabelText('Values');

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

function withFetchBotSuccess() {
  server.use(
    getBotSuccess({
      status: 'active',
      kind: 'bot',
      subKind: '',
      version: 'v1',
      metadata: {
        name: 'test-bot-name',
        description: '',
        labels: new Map(),
        namespace: '',
        revision: '',
      },
      spec: {
        roles: ['admin', 'user'],
        traits: [
          {
            name: 'trait-1',
            values: ['value-1', 'value-2', 'value-3'],
          },
        ],
        max_session_ttl: {
          seconds: 43200,
        },
      },
    })
  );
}

function withSaveSuccess(overrides?: Partial<EditBotRequest>) {
  server.use(editBotSuccess(overrides));
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
