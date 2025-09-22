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

import { Meta, StoryObj } from '@storybook/react-vite';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useEffect, useState } from 'react';

import { ButtonPrimary } from 'design/Button';
import Flex from 'design/Flex';
import { LockKey } from 'design/Icon/Icons/LockKey';
import { Unlock } from 'design/Icon/Icons/Unlock';

import { LockResourceKind } from 'teleport/LocksV2/NewLock/common';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import {
  createLockSuccess,
  listV2LocksSuccess,
  removeLockSuccess,
} from 'teleport/test/helpers/locks';

import { ResourceLockDialog } from './ResourceLockDialog';
import { ResourceLockIndicator } from './ResourceLockIndicator';
import { ResourceUnlockDialog } from './ResourceUnlockDialog';
import { useResourceLock } from './useResourceLock';

const meta = {
  title: 'Teleport/lib/Locks',
  component: Wrapper,
  beforeEach: () => {
    queryClient.clear(); // Prevent cached data sharing between stories
  },
} satisfies Meta<typeof Wrapper>;

type Story = StoryObj<typeof meta>;

export default meta;

export const LockingDialogs: Story = {
  args: {
    targetKind: 'user',
    targetName: 'example-user',
  },
  parameters: {
    msw: {
      handlers: [
        listV2LocksSuccess({
          locks: [],
        }),
        removeLockSuccess(),
        createLockSuccess(),
      ],
    },
  },
};

export const UnlockNotSupported: Story = {
  args: {
    targetKind: 'user',
    targetName: 'example-user',
  },
  parameters: {
    msw: {
      handlers: [
        listV2LocksSuccess({
          locks: [
            {
              name: '785d167d-cb2b-4de4-8268-efb6d8932dde',
              message: 'Locking bots until security alert is resolved.',
              expires: '2023-01-01T00:00:00Z',
              createdAt: '2023-01-01T00:00:00Z',
              targets: {
                user: 'example-user',
              },
            },
            {
              name: '0ef9909f-4980-40f7-bc85-ad9ef72bc059',
              expires: '2023-01-01T00:00:00Z',
              targets: {
                user: 'example-user',
              },
            },
            {
              name: '0e07e3f6-55bc-421d-ab24-1b521a0b6452',
              message:
                'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.',
              createdAt: '2023-01-01T00:00:00Z',
              targets: {
                user: 'example-user',
              },
            },
          ],
        }),
      ],
    },
  },
};

type Props = {
  targetKind: LockResourceKind;
  targetName: string;
};

function Wrapper(props: Props) {
  const customAcl = makeAcl({
    lock: {
      ...defaultAccess,
      list: true,
      remove: true,
      create: true,
      edit: true,
    },
  });

  const ctx = createTeleportContext({
    customAcl,
  });
  return (
    <QueryClientProvider client={queryClient}>
      <TeleportProviderBasic teleportCtx={ctx}>
        <Inner {...props} />
      </TeleportProviderBasic>
    </QueryClientProvider>
  );
}

function Inner(props: Props) {
  const { targetKind, targetName } = props;

  const [showDialog, setShowDialog] = useState<'none' | 'lock' | 'unlock'>(
    'none'
  );
  const [message, setMessage] = useState('');

  useEffect(() => {
    const id = setTimeout(() => setMessage(''), 3000);
    return () => clearTimeout(id);
  }, [message]);

  const { isLocked } = useResourceLock({ targetKind, targetName });

  return (
    <Flex flexDirection={'column'} alignItems={'center'} gap={3}>
      <ResourceLockIndicator targetKind={targetKind} targetName={targetName} />
      <ButtonPrimary
        onClick={() => setShowDialog(isLocked ? 'unlock' : 'lock')}
        gap={2}
      >
        {isLocked ? <Unlock size={'medium'} /> : <LockKey size={'medium'} />}
        {isLocked ? 'Unlock' : 'Lock'} {targetName}
      </ButtonPrimary>
      {message}
      {showDialog === 'unlock' ? (
        <ResourceUnlockDialog
          targetKind={targetKind}
          targetName={targetName}
          onCancel={() => {
            setMessage('Unlock was cancelled');
            setShowDialog('none');
          }}
          onComplete={() => {
            setMessage('Unlock was completed');
            setShowDialog('none');
          }}
        />
      ) : undefined}
      {showDialog === 'lock' ? (
        <ResourceLockDialog
          targetKind={targetKind}
          targetName={targetName}
          onCancel={() => {
            setMessage('Lock was cancelled');
            setShowDialog('none');
          }}
          onComplete={() => {
            setMessage('Lock was completed');
            setShowDialog('none');
          }}
        />
      ) : undefined}
    </Flex>
  );
}

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: false,
    },
  },
});
