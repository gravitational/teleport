/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { useState, useEffect } from 'react';
import { useLocation, useHistory } from 'react-router';
import { formatRelative } from 'date-fns';
import { Danger } from 'design/Alert';

import Table, { Cell } from 'design/DataTable';
import { ButtonPrimary, Label as Pill } from 'design';
import useAttempt from 'shared/hooks/useAttemptNext';

import cfg from 'teleport/config';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { NavLink } from 'teleport/components/Router';
import useTeleport from 'teleport/useTeleport';

import { lockService, Lock, LockTarget } from 'teleport/services/locks';

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
          <ButtonPrimary
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
          </ButtonPrimary>
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
                  <TrashButton onClick={() => setLockToDelete(lock)} />
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
  } catch (e) {
    return '';
  }
}

function lockTargetsMatcher(
  targetValue: any,
  searchValue: string,
  propName: keyof Lock & string
) {
  if (propName === 'targets') {
    return targetValue.some(
      ({ name, value }) =>
        name.toLocaleUpperCase().includes(searchValue) ||
        value.toLocaleUpperCase().includes(searchValue) ||
        `${name}: ${value}`.toLocaleUpperCase().includes(searchValue)
    );
  }
}

export function Pills({ targets }: { targets: LockTarget[] }) {
  const pills = targets.map((target, index) => {
    const labelText = `${target.kind}: ${target.name}`;
    return (
      <Pill
        key={`${target.kind}${target.name}${index}`}
        mr="1"
        kind="secondary"
      >
        {labelText}
      </Pill>
    );
  });

  return <span>{pills}</span>;
}
