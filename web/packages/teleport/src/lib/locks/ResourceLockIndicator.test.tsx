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
  Providers,
  render,
  screen,
  testQueryClient,
  userEvent,
  waitFor,
} from 'design/utils/testing';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { listV2LocksSuccess } from 'teleport/test/helpers/locks';

import { ResourceLockIndicator } from './ResourceLockIndicator';

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
  it('show a tooltip with details of a single lock', async () => {
    const user = userEvent.setup();

    withListLocksSuccess({
      locks: [
        {
          name: '2e76fda0-a698-46c1-977d-cf95ad2df7fc',
          message: 'This is a test message',
          expires: '2023-12-31T23:59:59Z',
          targets: { user: 'test-user' },
          createdAt: '2023-01-01T00:00:00Z',
          createdBy: 'admin',
        },
      ],
    });

    render(<ResourceLockIndicator targetKind="user" targetName="test-user" />, {
      wrapper: makeWrapper(),
    });

    await waitFor(() => {
      expect(screen.getByText('Locked')).toBeInTheDocument();
    });

    await user.hover(screen.getByText('Locked'));
    expect(
      screen.getByText('Message: This is a test message', { exact: false })
    ).toBeInTheDocument();
    expect(
      screen.getByText('Expires: Dec 31, 2023, 11:59 PM GMT+0', {
        exact: false,
      })
    ).toBeInTheDocument();
  });

  it('show a tooltip when multiple locks exist', async () => {
    const user = userEvent.setup();

    withListLocksSuccess({
      locks: [
        {
          name: '2e76fda0-a698-46c1-977d-cf95ad2df7fc',
          message: 'This is a test message',
          expires: '2023-12-31T23:59:59Z',
          targets: { user: 'test-user' },
          createdAt: '2023-01-01T00:00:00Z',
          createdBy: 'admin',
        },
        {
          name: 'b8f312c9-f8b7-4ef2-b3b1-97c07a750bff',
          message: 'This is a another test message',
          expires: '2023-12-31T23:59:59Z',
          targets: { user: 'test-user' },
          createdAt: '2023-01-01T00:00:00Z',
          createdBy: 'admin',
        },
      ],
    });

    render(<ResourceLockIndicator targetKind="user" targetName="test-user" />, {
      wrapper: makeWrapper(),
    });

    await waitFor(() => {
      expect(screen.getByText('Locked')).toBeInTheDocument();
    });

    await user.hover(screen.getByText('Locked'));
    expect(
      screen.getByText('Multiple locks in-force', { exact: false })
    ).toBeInTheDocument();
  });
});

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
