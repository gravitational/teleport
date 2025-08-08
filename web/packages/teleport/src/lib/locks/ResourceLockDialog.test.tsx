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
import { PropsWithChildren } from 'react';

import darkTheme from 'design/theme/themes/darkTheme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import { testQueryClient } from 'design/utils/testing';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';

import { ResourceLockDialog } from './ResourceLockDialog';
import { useResourceLock } from './useResourceLock';

jest.mock('./useResourceLock', () => ({
  useResourceLock: jest.fn(),
}));

describe('ResourceLockDialog', () => {
  it('should cancel', async () => {
    withMockHook();

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

    expect(screen.getByText('Lock test-user?')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Create Lock' })).toBeEnabled();

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('should submit', async () => {
    const lock = jest.fn().mockResolvedValue({
      name: '2e76fda0-a698-46c1-977d-cf95ad2df7fc',
      message: 'This is a test message',
      expires: '2023-12-31T23:59:59Z',
      targets: {
        user: 'test-user',
      },
      createdAt: '2023-01-01T00:00:00Z',
      createdBy: 'admin',
    });

    withMockHook({
      lock,
    });

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

    inputMessage('This is a test message');
    inputTtl('24h');

    fireEvent.click(screen.getByRole('button', { name: 'Create Lock' }));
    expect(lock).toHaveBeenCalledTimes(1);
    expect(lock).toHaveBeenLastCalledWith('This is a test message', '24h');

    // lock() is async and wont finish immediately, so we need to give it time
    // to call onComplete()
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

function withMockHook(
  result: Partial<ReturnType<typeof useResourceLock>> = {}
) {
  jest.mocked(useResourceLock).mockReturnValue({
    canLock: true,
    canUnlock: false,
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

async function inputMessage(value: string) {
  const input = screen.getByLabelText('Reason');
  fireEvent.change(input, { target: { value } });
}

async function inputTtl(value: string) {
  const input = screen.getByLabelText('Expiry');
  fireEvent.change(input, { target: { value } });
}
