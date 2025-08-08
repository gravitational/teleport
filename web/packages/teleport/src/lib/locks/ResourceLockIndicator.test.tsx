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
import { render, screen } from '@testing-library/react';
import { PropsWithChildren } from 'react';

import darkTheme from 'design/theme/themes/darkTheme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import { testQueryClient, userEvent } from 'design/utils/testing';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';

import { ResourceLockIndicator } from './ResourceLockIndicator';
import { useResourceLock } from './useResourceLock';

jest.mock('./useResourceLock', () => ({
  useResourceLock: jest.fn(),
}));

describe('ResourceUnlockDialog', () => {
  it('show a tooltip with details of a single lock', async () => {
    const user = userEvent.setup();

    withMockHook({
      locks: [
        {
          name: '2e76fda0-a698-46c1-977d-cf95ad2df7fc',
          message: 'This is a test message',
          expires: '2023-12-31T23:59:59Z',
          targets: [{ kind: 'user', name: 'test-user' }],
          targetLookup: {
            user: 'test-user',
          },
          createdAt: '2023-01-01T00:00:00Z',
          createdBy: 'admin',
        },
      ],
    });

    render(<ResourceLockIndicator targetKind="user" targetName="test-user" />, {
      wrapper: makeWrapper(),
    });

    expect(screen.getByText('Locked')).toBeInTheDocument();

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

    withMockHook({
      locks: [
        {
          name: '2e76fda0-a698-46c1-977d-cf95ad2df7fc',
          message: 'This is a test message',
          expires: '2023-12-31T23:59:59Z',
          targets: [{ kind: 'user', name: 'test-user' }],
          targetLookup: {
            user: 'test-user',
          },
          createdAt: '2023-01-01T00:00:00Z',
          createdBy: 'admin',
        },
        {
          name: '2e76fda0-a698-46c1-977d-cf95ad2df7fc',
          message: 'This is a another test message',
          expires: '2023-12-31T23:59:59Z',
          targets: [{ kind: 'user', name: 'test-user' }],
          targetLookup: {
            user: 'test-user',
          },
          createdAt: '2023-01-01T00:00:00Z',
          createdBy: 'admin',
        },
      ],
    });

    render(<ResourceLockIndicator targetKind="user" targetName="test-user" />, {
      wrapper: makeWrapper(),
    });

    expect(screen.getByText('Locked')).toBeInTheDocument();

    await user.hover(screen.getByText('Locked'));
    expect(
      screen.getByText('Multiple locks in-force', { exact: false })
    ).toBeInTheDocument();
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

function withMockHook(
  result: Partial<ReturnType<typeof useResourceLock>> = {}
) {
  jest.mocked(useResourceLock).mockReturnValue({
    canLock: false,
    canUnlock: true,
    error: null,
    isLoading: false,
    isLocked: false,
    lock: jest.fn(),
    unlock: jest.fn(),
    lockError: null,
    lockPending: false,
    locks: [],
    unlockError: null,
    unlockPending: false,
    ...result,
  });
}
