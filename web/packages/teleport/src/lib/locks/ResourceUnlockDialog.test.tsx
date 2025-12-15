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

import { createMemoryHistory } from 'history';
import { setupServer } from 'msw/node';
import {
  ComponentPropsWithoutRef,
  MouseEventHandler,
  PropsWithChildren,
} from 'react';
import { Router } from 'react-router';

import {
  Providers,
  render,
  screen,
  testQueryClient,
  userEvent,
  waitFor,
  within,
} from 'design/utils/testing';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import {
  listV2LocksSuccess,
  removeLockSuccess,
} from 'teleport/test/helpers/locks';

import { ResourceUnlockDialog } from './ResourceUnlockDialog';

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

describe('ResourceUnlockDialog', () => {
  it('should cancel', async () => {
    withListLocksSuccess({
      locks: [
        {
          name: '76bc5cc7-b9bf-4a03-935f-8018c0a2bc05',
          message: 'This is a test message',
          expires: '2023-01-01T00:00:00Z',
          targets: {
            user: 'test-user',
          },
          createdAt: '2023-12-31T23:59:59Z',
          createdBy: 'admin',
        },
      ],
    });

    const onCancel = jest.fn();

    const { user } = renderComponent({ onCancel });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Cancel' })).toBeEnabled();
    });

    expect(screen.getByText('Unlock test-user?')).toBeInTheDocument();
    expect(
      screen.getByText('This is a test message', { exact: false })
    ).toBeInTheDocument();
    expect(
      within(screen.getByText('Expires').closest('div')!).getByText(
        'Jan 1, 2023, 12:00 AM GMT+0',
        { exact: false }
      )
    ).toBeInTheDocument();
    expect(
      within(screen.getByText('Locked on').closest('div')!).getByText(
        'Dec 31, 2023, 11:59 PM GMT+0',
        { exact: false }
      )
    ).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('should submit', async () => {
    withListLocksSuccess({
      locks: [
        {
          name: '76bc5cc7-b9bf-4a03-935f-8018c0a2bc05',
          message: 'This is a test message',
          expires: '2023-12-31T23:59:59Z',
          targets: {
            user: 'test-user',
          },
          createdAt: '2023-01-01T00:00:00Z',
          createdBy: 'admin',
        },
      ],
    });

    const onComplete = jest.fn();

    const { user } = renderComponent({ onComplete });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Cancel' })).toBeEnabled();
    });

    withUnlockSuccess();
    await user.click(screen.getByRole('button', { name: 'Remove Lock' }));

    await waitFor(() => {
      expect(onComplete).toHaveBeenCalledTimes(1);
    });
  });

  it('should handle multiple locks', async () => {
    withListLocksSuccess({
      locks: [
        {
          name: '76bc5cc7-b9bf-4a03-935f-8018c0a2bc05',
          message: 'message 1',
          expires: '1790-01-01T00:00:01Z',
          targets: {
            user: 'test-user',
          },
          createdAt: '1790-01-01T00:00:02Z',
          createdBy: 'admin',
        },
        {
          name: 'de64fc0c-7169-4bee-95a0-5458bc42447f',
          message: 'message 2',
          expires: '1790-01-01T00:00:03Z',
          targets: {
            user: 'test-user',
          },
          createdAt: '1790-01-01T00:00:04Z',
          createdBy: 'admin',
        },
      ],
    });

    const onCancel = jest.fn();
    const onGoToLocksForTesting = jest.fn((event => {
      // Prevent errors related to window.location not being implemented.
      event.preventDefault();
    }) satisfies MouseEventHandler<HTMLAnchorElement>);

    const { user } = renderComponent({ onCancel, onGoToLocksForTesting });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Cancel' })).toBeEnabled();
    });

    expect(screen.getByText('Unlock test-user?')).toBeInTheDocument();
    expect(
      screen.getByText('Multiple locks exist', { exact: false })
    ).toBeInTheDocument();
    expect(screen.getByText('message 1', { exact: false })).toBeInTheDocument();
    expect(screen.getByText('message 2', { exact: false })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(onCancel).toHaveBeenCalledTimes(1);

    await user.click(
      screen.getByRole('link', { name: 'Go to Session and Identity Locks' })
    );
    expect(onGoToLocksForTesting).toHaveBeenCalledTimes(1);
  });
});

function renderComponent(
  options?: { history?: ReturnType<typeof createMemoryHistory> } & Pick<
    Partial<ComponentPropsWithoutRef<typeof ResourceUnlockDialog>>,
    'onCancel' | 'onComplete' | 'onGoToLocksForTesting'
  >
) {
  const {
    onCancel = jest.fn(),
    onComplete = jest.fn(),
    onGoToLocksForTesting,
  } = options ?? {};
  const user = userEvent.setup();
  return {
    ...render(
      <ResourceUnlockDialog
        targetKind="user"
        targetName="test-user"
        onCancel={onCancel}
        onComplete={onComplete}
        onGoToLocksForTesting={onGoToLocksForTesting}
      />,
      { wrapper: makeWrapper(options) }
    ),
    user,
  };
}

function makeWrapper(options?: {
  history?: ReturnType<typeof createMemoryHistory>;
}) {
  const { history = createMemoryHistory() } = options ?? {};
  const ctx = createTeleportContext();
  return (props: PropsWithChildren) => (
    <Providers>
      <TeleportProviderBasic teleportCtx={ctx}>
        <Router history={history}>{props.children}</Router>
      </TeleportProviderBasic>
    </Providers>
  );
}

function withListLocksSuccess(
  ...params: Parameters<typeof listV2LocksSuccess>
) {
  server.use(
    listV2LocksSuccess({
      locks: params[0]?.locks ?? [],
    })
  );
}

function withUnlockSuccess() {
  server.use(removeLockSuccess());
}
