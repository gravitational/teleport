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
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { setupServer } from 'msw/node';
import { PropsWithChildren } from 'react';

import darkTheme from 'design/theme/themes/darkTheme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import { testQueryClient } from 'design/utils/testing';

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

    render(
      <ResourceUnlockDialog
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

    expect(screen.getByText('Unlock test-user?')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
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

    render(
      <ResourceUnlockDialog
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

    withUnlockSuccess();
    fireEvent.click(screen.getByRole('button', { name: 'Remove Lock' }));

    await waitFor(() => {
      expect(onComplete).toHaveBeenCalledTimes(1);
    });
  });
});

function makeWrapper() {
  const ctx = createTeleportContext();
  return (props: PropsWithChildren) => (
    <QueryClientProvider client={testQueryClient}>
      <TeleportProviderBasic teleportCtx={ctx}>
        <ConfiguredThemeProvider theme={darkTheme}>
          {props.children}
        </ConfiguredThemeProvider>
      </TeleportProviderBasic>
    </QueryClientProvider>
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
