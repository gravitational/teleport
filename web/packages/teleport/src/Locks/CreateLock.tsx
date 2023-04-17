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

import React, { useRef, useState } from 'react';
import styled from 'styled-components';
import SlidePanel from 'design/SlidePanel';

import { ArrowBack, Trash } from 'design/Icon';
import { Alert, Box, ButtonPrimary, Flex, Input, Text } from 'design';
import Table, { Cell } from 'design/DataTable';

import useStickyClusterId from 'teleport/useStickyClusterId';
import history from 'teleport/services/history';
import cfg from 'teleport/config';

import { useLocks } from './useLocks';
import { StyledSpinner } from './shared';

import type { Positions } from 'design/SlidePanel/SlidePanel';
import type { CreateLockData, SelectedLockTarget } from './types';
import type { ApiError } from 'teleport/services/api/parseError';

type Props = {
  panelPosition: Positions;
  setPanelPosition: (Positions) => void;
  selectedLockTargets: SelectedLockTarget[];
  setSelectedLockTargets: (lockTargets: SelectedLockTarget[]) => void;
};

export function CreateLock({
  panelPosition,
  setPanelPosition,
  selectedLockTargets,
  setSelectedLockTargets,
}: Props) {
  const { clusterId } = useStickyClusterId();
  const { createLock } = useLocks(clusterId);
  const [error, setError] = useState('');
  const [createPending, setCreatePending] = useState(false);

  const messageRef = useRef<HTMLInputElement>(null);
  const ttlRef = useRef<HTMLInputElement>(null);

  function handleCreateLock() {
    setError('');
    setCreatePending(true);
    selectedLockTargets.forEach(async lockTarget => {
      const lockData: CreateLockData = {
        targets: { [lockTarget.type]: lockTarget.name },
      };
      const message = messageRef?.current?.value;
      const ttl = ttlRef?.current?.value;
      if (message) lockData.message = message;
      if (ttl) lockData.ttl = ttl;
      await createLock(clusterId, lockData)
        .then(() => {
          setTimeout(() => {
            // It takes longer for the cache to be updated when adding locks so
            // this waits 1s before redirecting to fetch the list again.
            setCreatePending(false);
            history.push(cfg.getLocksRoute(clusterId));
          }, 1000);
        })
        .catch((e: ApiError) => {
          setCreatePending(false);
          setError(e.message);
        });
    });
  }

  function onRemove(name) {
    const index = selectedLockTargets.findIndex(target => target.name === name);
    selectedLockTargets.splice(index, 1);
    setSelectedLockTargets([...selectedLockTargets]);
  }

  return (
    <SlidePanel
      position={panelPosition}
      closePanel={() => setPanelPosition('closed')}
    >
      <div>
        {error && <Alert kind="danger" children={error} data-testid="alert" />}
        <Flex alignItems="center" mb={3}>
          <ArrowBack
            fontSize={25}
            mr={3}
            onClick={() => setPanelPosition('closed')}
            style={{ cursor: 'pointer' }}
          />
          <Box>
            <Text typography="h4" color="text.primary" bold>
              Create New Lock
            </Text>
          </Box>
        </Flex>

        <StyledTable
          data={selectedLockTargets}
          columns={[
            {
              key: 'type',
              headerText: 'Type',
              isSortable: false,
            },
            {
              key: 'name',
              headerText: 'Name',
              isSortable: false,
            },
            {
              altKey: 'remove-btn',
              render: ({ name }) => (
                <Cell align="right">
                  <Trash
                    fontSize={13}
                    borderRadius={2}
                    p={2}
                    onClick={onRemove.bind(null, name)}
                    css={`
                      cursor: pointer;
                      background-color: ${({ theme }) =>
                        theme.colors.buttons.trashButton.default};
                      border-radius: 2px;
                      :hover {
                        background-color: ${({ theme }) =>
                          theme.colors.buttons.trashButton.hover};
                      }
                    `}
                    data-testid="trash-btn"
                  />
                </Cell>
              ),
            },
          ]}
          emptyText="No Targets Found"
        />
        <Box mt={3}>
          <Text mr={2}>Message: </Text>
          <Input
            placeholder={`Going down for maintenance`}
            ref={messageRef}
            data-testid="description"
          />
        </Box>
        <Box mt={3}>
          <Text mr={2}>TTL: </Text>
          <Input
            placeholder={`2h45m, 5h, empty=never`}
            ref={ttlRef}
            data-testid="ttl"
          />
        </Box>
      </div>
      <Flex mt={5} justifyContent="flex-end">
        <ButtonPrimary
          width="165px"
          onClick={handleCreateLock}
          disabled={!selectedLockTargets.length || createPending}
        >
          {createPending ? <StyledSpinner /> : <>Create locks</>}
        </ButtonPrimary>
      </Flex>
    </SlidePanel>
  );
}

const StyledTable = styled(Table)`
  & > tbody > tr > td {
    vertical-align: middle;
    padding: 8px;
  }
  & > thead > tr > th {
    background: ${props => props.theme.colors.spotBackground[1]};
  }
  box-shadow: ${props => props.theme.boxShadow[0]};
  border-radius: 8px;
  overflow: hidden;
` as typeof Table;
