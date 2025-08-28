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
import { renderHook, waitFor } from '@testing-library/react';
import { setupServer } from 'msw/node';
import { PropsWithChildren } from 'react';

import { testQueryClient } from 'design/utils/testing';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import {
  createLockSuccess,
  listV2LocksError,
  listV2LocksSuccess,
  removeLockSuccess,
} from 'teleport/test/helpers/locks';

import { useResourceLock } from './useResourceLock';

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

describe('useResourceLock', () => {
  it('returns locks (empty)', async () => {
    withListLocksSuccess({ locks: [] });

    const { result } = renderHook(
      () =>
        useResourceLock({
          targetKind: 'user',
          targetName: 'test-user',
        }),
      {
        wrapper: makeWrapper(),
      }
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBeNull();
    expect(result.current.locks).toHaveLength(0);
    expect(result.current.isLocked).toBe(false);
    expect(result.current.canUnlock).toBe(false);
  });

  it('returns locks (single)', async () => {
    withListLocksSuccess();

    const { result } = renderHook(
      () =>
        useResourceLock({
          targetKind: 'user',
          targetName: 'test-user',
        }),
      {
        wrapper: makeWrapper(),
      }
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBeNull();
    expect(result.current.locks).toHaveLength(1);
    expect(result.current.isLocked).toBe(true);
    expect(result.current.canUnlock).toBe(true);
  });

  it('returns locks (multiple)', async () => {
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
        {
          name: '2e76fda0-a698-46c1-977d-cf95ad2df7fc',
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

    const { result } = renderHook(
      () =>
        useResourceLock({
          targetKind: 'user',
          targetName: 'test-user',
        }),
      {
        wrapper: makeWrapper(),
      }
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBeNull();
    expect(result.current.locks).toHaveLength(2);
    expect(result.current.isLocked).toBe(true);
    expect(result.current.canUnlock).toBe(true);
  });

  it('handles lock with other targets', async () => {
    withListLocksSuccess({
      locks: [
        {
          name: '76bc5cc7-b9bf-4a03-935f-8018c0a2bc05',
          message: 'This is a test message',
          expires: '2023-12-31T23:59:59Z',
          targets: {
            user: 'test-user',
            role: 'test-role', // This target means the lock cannot be removed.
          },
          createdAt: '2023-01-01T00:00:00Z',
          createdBy: 'admin',
        },
      ],
    });

    const { result } = renderHook(
      () =>
        useResourceLock({
          targetKind: 'user',
          targetName: 'test-user',
        }),
      {
        wrapper: makeWrapper(),
      }
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBeNull();
    expect(result.current.locks).toHaveLength(1);
    expect(result.current.isLocked).toBe(true);
    expect(result.current.canUnlock).toBe(false);
  });

  it('returns list error', async () => {
    withListLocksError(500, 'an error occurred');

    const { result } = renderHook(
      () =>
        useResourceLock({
          targetKind: 'user',
          targetName: 'test-user',
        }),
      {
        wrapper: makeWrapper(),
      }
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).not.toBeNull();
    expect(result.current.error?.message).toBe('an error occurred');
  });

  it('can lock and unlock the resource', async () => {
    withListLocksSuccess();

    const { result } = renderHook(
      () =>
        useResourceLock({
          targetKind: 'user',
          targetName: 'test-user',
        }),
      {
        wrapper: makeWrapper(),
      }
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    withUnlockSuccess();
    await result.current.unlock();

    await waitFor(() => {
      expect(result.current.unlockPending).toBe(false);
    });

    expect(result.current.locks).toHaveLength(0);

    withLockSuccess();
    await result.current.lock('', '');

    await waitFor(() => {
      expect(result.current.lockPending).toBe(false);
    });

    expect(result.current.locks).toHaveLength(1);
  });

  it('cant unlock without permission', async () => {
    withListLocksSuccess();

    const { result } = renderHook(
      () =>
        useResourceLock({
          targetKind: 'user',
          targetName: 'test-user',
        }),
      {
        wrapper: makeWrapper({
          customAcl: makeAcl({
            lock: {
              ...defaultAccess,
              list: true,
              remove: false, // remove is required to unlock
              create: true,
              edit: true,
            },
          }),
        }),
      }
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.canUnlock).toBe(false);

    await expect(result.current.unlock).rejects.toEqual(
      new Error('missing permission to remove locks')
    );

    expect(result.current.locks).toHaveLength(1);
  });

  it('cant lock without permission', async () => {
    withListLocksSuccess({ locks: [] });

    const { result } = renderHook(
      () =>
        useResourceLock({
          targetKind: 'user',
          targetName: 'test-user',
        }),
      {
        wrapper: makeWrapper({
          customAcl: makeAcl({
            lock: {
              ...defaultAccess,
              list: true,
              remove: true,
              create: false, // both create and edit are required to lock
              edit: false, // both create and edit are required to lock
            },
          }),
        }),
      }
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.canLock).toBe(false);
    await expect(() => result.current.lock('', '')).rejects.toEqual(
      new Error('missing permission to create locks')
    );

    expect(result.current.locks).toHaveLength(0);
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
    <QueryClientProvider client={testQueryClient}>
      <TeleportProviderBasic teleportCtx={ctx}>
        {props.children}
      </TeleportProviderBasic>
    </QueryClientProvider>
  );
}

function withListLocksSuccess(
  ...params: Parameters<typeof listV2LocksSuccess>
) {
  server.use(
    listV2LocksSuccess({
      locks: params[0]?.locks ?? [
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
    })
  );
}

function withListLocksError(...params: Parameters<typeof listV2LocksError>) {
  server.use(listV2LocksError(...params));
}

function withUnlockSuccess() {
  server.use(removeLockSuccess());
}

function withLockSuccess() {
  server.use(createLockSuccess());
}
