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

import { setupServer } from 'msw/node';
import { ComponentPropsWithoutRef, PropsWithChildren } from 'react';

import {
  Providers,
  render,
  screen,
  testQueryClient,
  userEvent,
  waitFor,
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
          expires: '2023-12-31T23:59:59Z',
          targets: {
            user: 'test-user',
          },
          createdAt: '2023-01-01T00:00:00Z',
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
});

function renderComponent(
  params?: Pick<
    Partial<ComponentPropsWithoutRef<typeof ResourceUnlockDialog>>,
    'onCancel' | 'onComplete'
  >
) {
  const { onCancel = jest.fn(), onComplete = jest.fn() } = params ?? {};
  const user = userEvent.setup();
  return {
    ...render(
      <ResourceUnlockDialog
        targetKind="user"
        targetName="test-user"
        onCancel={onCancel}
        onComplete={onComplete}
      />,
      { wrapper: makeWrapper() }
    ),
    user,
  };
}

function makeWrapper() {
  const ctx = createTeleportContext();
  return (props: PropsWithChildren) => (
    <Providers>
      <TeleportProviderBasic teleportCtx={ctx}>
        {props.children}
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
