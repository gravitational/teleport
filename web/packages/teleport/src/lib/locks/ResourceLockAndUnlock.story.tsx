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
        createLockSuccess({
          name: '76bc5cc7-b9bf-4a03-935f-8018c0a2bc05',
          message: 'This is a test message',
          expires: '2023-12-31T23:59:59Z',
          targets: {
            user: 'example-user',
          },
          createdAt: '2023-01-01T00:00:00Z',
          createdBy: 'admin',
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
