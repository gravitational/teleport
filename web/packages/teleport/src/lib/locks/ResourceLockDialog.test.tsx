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
import { PropsWithChildren } from 'react';

import {
  fireEvent,
  Providers,
  render,
  screen,
  testQueryClient,
  waitFor,
} from 'design/utils/testing';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import {
  createLockSuccess,
  listV2LocksSuccess,
} from 'teleport/test/helpers/locks';

import { ResourceLockDialog } from './ResourceLockDialog';

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

describe('ResourceLockDialog', () => {
  it('should cancel', async () => {
    withListLocksSuccess();

    const onCancel = jest.fn();

    render(
      <ResourceLockDialog
        targetKind="user"
        targetName="test-user"
        onCancel={onCancel}
        onComplete={jest.fn()}
      />,
      { wrapper: makeWrapper() }
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Cancel' })).toBeEnabled();
    });

    expect(screen.getByText('Lock test-user?')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('should submit', async () => {
    withListLocksSuccess();

    const onComplete = jest.fn();

    render(
      <ResourceLockDialog
        targetKind="user"
        targetName="test-user"
        onCancel={jest.fn()}
        onComplete={onComplete}
      />,
      { wrapper: makeWrapper() }
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Cancel' })).toBeEnabled();
    });

    inputMessage('This is a test message');
    inputTtl('24h');

    withLockSuccess();
    fireEvent.click(screen.getByRole('button', { name: 'Create Lock' }));

    await waitFor(() => {
      expect(onComplete).toHaveBeenCalledTimes(1);
    });

    expect(onComplete).toHaveBeenCalledWith({
      createdAt: expect.anything(),
      createdBy: expect.anything(),
      expires: expect.anything(),
      message: 'This is a test message',
      name: expect.anything(),
      targetLookup: {
        user: 'test-user',
      },
      targets: [
        {
          kind: 'user',
          name: 'test-user',
        },
      ],
    });
  });
});

function makeWrapper(params?: { customAcl?: ReturnType<typeof makeAcl> }) {
  const {
    customAcl = makeAcl({
      lock: {
        ...defaultAccess,
        list: true,
        remove: true,
        create: true,
        edit: true,
      },
    }),
  } = params ?? {};
  const ctx = createTeleportContext({
    customAcl,
  });
  return (props: PropsWithChildren) => (
    <Providers>
      <TeleportProviderBasic teleportCtx={ctx}>
        {props.children}
      </TeleportProviderBasic>
    </Providers>
  );
}

async function inputMessage(value: string) {
  const input = screen.getByLabelText('Reason');
  fireEvent.change(input, { target: { value } });
}

async function inputTtl(value: string) {
  const input = screen.getByLabelText('Expiry');
  fireEvent.change(input, { target: { value } });
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

function withLockSuccess() {
  server.use(createLockSuccess());
}
