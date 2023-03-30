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

import React, { useState } from 'react';
import styled from 'styled-components';
import { formatRelative } from 'date-fns';

import Table, { Cell, ClickableLabelCell } from 'design/DataTable';
import { ButtonPrimary } from 'design/Button';
import { Trash } from 'design/Icon';

import api from 'teleport/services/api';
import cfg from 'teleport/config';
import useStickyClusterId from 'teleport/useStickyClusterId';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { NavLink } from 'teleport/components/Router';

import { useLocks } from './useLocks';
import { StyledSpinner } from './shared';

function getFormattedDate(d: string): string {
  try {
    return formatRelative(new Date(d), Date.now());
  } catch (e) {
    return '';
  }
}

export function Locks() {
  const { clusterId } = useStickyClusterId();
  const { locks, fetchLocks } = useLocks(clusterId);
  const [deletePending, setDeletePending] = useState('');

  function onDelete(lockName: string) {
    if (deletePending) return;
    setDeletePending(lockName);
    api.delete(cfg.getLocksUrlWithUuid(clusterId, lockName)).then(() => {
      // It takes longer for the cache to be updated when removing locks so
      // this waits 1s before fetching the list again.
      setTimeout(() => {
        fetchLocks(clusterId);
        setDeletePending('');
      }, 1000);
    });
  }

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Session & Identity Locks</FeatureHeaderTitle>
        <ButtonPrimary
          as={NavLink}
          to={cfg.getNewLocksRoute(clusterId)}
          ml="auto"
        >
          + Add New Lock
        </ButtonPrimary>
      </FeatureHeader>
      <Table
        data={locks}
        columns={[
          {
            key: 'targets',
            headerText: 'Locked Items',
            render: ({ targets }) => (
              <ClickableLabelCell labels={targets} onClick={() => {}} />
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
            render: ({ name }) => (
              <DeleteButton
                onDelete={onDelete.bind(null, name)}
                pending={deletePending === name}
              />
            ),
          },
        ]}
        emptyText="No Locks Found"
        isSearchable
        pagination={{ pageSize: 20 }}
      />
    </FeatureBox>
  );
}

type DeleteButtonProps = {
  onDelete: () => void;
  pending: boolean;
};

const DeleteButton = ({ onDelete, pending }: DeleteButtonProps) => {
  return (
    <Cell align="right">
      <ButtonBG>
        {pending ? (
          <StyledSpinner />
        ) : (
          <Trash
            onClick={onDelete}
            css={`
              padding: 8px 0;
            `}
            data-testid="trash-btn"
          />
        )}
      </ButtonBG>
    </Cell>
  );
};

const ButtonBG = styled.span`
  cursor: pointer;
  font-size: 13px;
  border-radius: 2px;
  padding: 8px;
  background-color: #2e3860;
  border-radius: 2px;
  :hover {
    background-color: #414b70;
  }
`;
