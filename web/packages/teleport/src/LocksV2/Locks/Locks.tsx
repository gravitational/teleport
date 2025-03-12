/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { formatRelative } from 'date-fns';
import { Fragment, useEffect, useState } from 'react';
import { useHistory, useLocation } from 'react-router';

import { Button, Label as Pill } from 'design';
import { Danger } from 'design/Alert';
import Table, { Cell } from 'design/DataTable';
import useAttempt from 'shared/hooks/useAttemptNext';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { NavLink } from 'teleport/components/Router';
import cfg from 'teleport/config';
import { Lock, lockService, LockTarget } from 'teleport/services/locks';
import useTeleport from 'teleport/useTeleport';

import { TrashButton } from '../common';
import { DeleteLockDialogue } from './DeleteLockDialogue';

export function Locks() {
  const ctx = useTeleport();
  const history = useHistory();
  const location = useLocation<{ createdLocks: Lock[] }>();
  const { attempt, run } = useAttempt();
  const [locks, setLocks] = useState<Lock[]>([]);
  const [lockToDelete, setLockToDelete] = useState<Lock>();

  useEffect(() => {
    run(() =>
      lockService.fetchLocks().then(res => {
        const updatedLocks = [...res];
        // If location state was set, user is coming back from
        // creating a set of locks. Because of possible cache lagging,
        // we will manually add missing new locks to the fetched list.
        if (location.state?.createdLocks) {
          const seenLock = {};
          updatedLocks.forEach(lock => (seenLock[lock.name] = true));
          location.state.createdLocks.forEach(lock => {
            if (!seenLock[lock.name]) {
              updatedLocks.push(lock);
            }
          });
          history.replace({ state: {} }); // Clear loc state afterwards.
        }
        setLocks(updatedLocks);
      })
    );
  }, []);

  function deleteLock(lockName: string) {
    return lockService.deleteLock(lockName).then(() => {
      // Manually remove from the initial fetched
      // list since there could be cache lagging.
      const updatedLocks = locks.filter(lock => lock.name !== lockName);
      setLocks(updatedLocks);
      setLockToDelete(null);
    });
  }

  const lockAccess = ctx.storeUser.getLockAccess();
  const canCreate = lockAccess.create && lockAccess.edit;

  return (
    <>
      <FeatureBox>
        <FeatureHeader>
          <FeatureHeaderTitle>Session & Identity Locks</FeatureHeaderTitle>
          <Button
            intent="primary"
            fill={
              attempt.status === 'success' && locks.length === 0
                ? 'filled'
                : 'border'
            }
            as={NavLink}
            to={cfg.getNewLocksRoute()}
            ml="auto"
            disabled={!canCreate}
            title={
              canCreate
                ? ''
                : 'You do not have access to create and update locks'
            }
          >
            Add New Lock
          </Button>
        </FeatureHeader>
        {attempt.status === 'failed' && <Danger>{attempt.statusText}</Danger>}
        <Table
          data={locks}
          columns={[
            {
              key: 'targets',
              headerText: 'Locked Items',
              render: ({ targets }) => (
                <Cell>
                  <Pills targets={targets} />
                </Cell>
              ),
            },
            {
              key: 'createdBy',
              headerText: 'Locked By',
              isSortable: true,
            },
            {
              key: 'createdAt',
              headerText: 'Start Date',
              isSortable: true,
              render: ({ createdAt }) => (
                <Cell>{getFormattedDate(createdAt)}</Cell>
              ),
            },
            {
              key: 'expires',
              headerText: 'Expiration',
              isSortable: true,
              render: ({ expires }) => (
                <Cell>{getFormattedDate(expires) || 'Never'}</Cell>
              ),
            },
            {
              key: 'message',
              headerText: 'Message',
              isSortable: true,
              render: ({ message }) => <Cell>{message}</Cell>,
            },
            {
              altKey: 'options-btn',
              render: lock => (
                <Cell align="right">
                  <TrashButton
                    size="medium"
                    onClick={() => setLockToDelete(lock)}
                  />
                </Cell>
              ),
            },
          ]}
          emptyText="No Locks Found"
          isSearchable
          customSearchMatchers={[lockTargetsMatcher]}
          pagination={{ pageSize: 20 }}
          fetching={{
            fetchStatus: attempt.status === 'processing' ? 'loading' : '',
          }}
        />
      </FeatureBox>
      {lockToDelete && (
        <DeleteLockDialogue
          onClose={() => setLockToDelete(null)}
          onDelete={deleteLock}
          lock={lockToDelete}
        />
      )}
    </>
  );
}

function getFormattedDate(d: string): string {
  try {
    return formatRelative(new Date(d), Date.now());
  } catch {
    return '';
  }
}

export function lockTargetsMatcher(
  targetValue: any,
  searchValue: string,
  propName: keyof Lock & string
) {
  if (propName === 'targets') {
    return (targetValue as LockTarget[]).some(
      ({ name, kind }) =>
        name.toLocaleUpperCase().includes(searchValue) ||
        kind.toLocaleUpperCase().includes(searchValue) ||
        `${kind}: ${name}`.toLocaleUpperCase().includes(searchValue)
    );
  }
}

export function Pills({ targets }: { targets: LockTarget[] }) {
  const pills = targets.map((target, index) => {
    const labelText = `${target.kind}: ${target.name}`;
    return (
      <Fragment key={`${target.kind}${target.name}${index}`}>
        {index > 0 && ' '}
        <Pill kind="secondary">{labelText}</Pill>
      </Fragment>
    );
  });

  return <span>{pills}</span>;
}
