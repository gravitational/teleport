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

import { Box, ButtonPrimary, ButtonSecondary, Flex, Input, Text } from 'design';
import { Cell, LabelCell } from 'design/DataTable';
import Select from 'shared/components/Select';
import { ArrowBack } from 'design/Icon';

import useStickyClusterId from 'teleport/useStickyClusterId';
import history from 'teleport/services/history';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import cfg from 'teleport/config';

import { CreateLock } from './CreateLock';
import { StyledTable } from './shared';

import { lockTargets, useGetTargetData } from './useGetTargetData';

import type { AdditionalTargets } from './useGetTargetData';
import type {
  LockTarget,
  OnAdd,
  SelectedLockTarget,
  TargetListProps,
} from './types';
import type { TableColumn } from 'design/DataTable/types';
import type { Positions } from 'design/SlidePanel/SlidePanel';

// This is split out like this to allow the router to call 'NewLock'
// but also allow E to use 'NewLockContent' separately.
export default function NewLock() {
  return <NewLockContent />;
}

export function NewLockContent({
  additionalTargets,
}: {
  additionalTargets?: AdditionalTargets;
}) {
  const { clusterId } = useStickyClusterId();
  const [createPanelPosition, setCreatePanelPosition] =
    useState<Positions>('closed');
  const [selectedTargetType, setSelectedTargetType] = useState<LockTarget>({
    label: 'User',
    value: 'user',
  });
  const [selectedLockTargets, setSelectedLockTargets] = useState<
    SelectedLockTarget[]
  >([]);
  const targetData = useGetTargetData(
    selectedTargetType?.value,
    clusterId,
    additionalTargets
  );

  function onAdd(name) {
    selectedLockTargets.push({
      type: selectedTargetType.value,
      name,
    });
    setSelectedLockTargets([...selectedLockTargets]);
  }

  function onClear() {
    setSelectedLockTargets([]);
  }

  const disabledSubmit = !selectedLockTargets.length;

  return (
    <FeatureBox>
      <CreateLock
        panelPosition={createPanelPosition}
        setPanelPosition={setCreatePanelPosition}
        selectedLockTargets={selectedLockTargets}
        setSelectedLockTargets={setSelectedLockTargets}
      />
      <FeatureHeader>
        <FeatureHeaderTitle>
          <Flex alignItems="center">
            <ArrowBack
              fontSize={25}
              mr={3}
              onClick={() => history.push(cfg.getLocksRoute(clusterId))}
              style={{ cursor: 'pointer' }}
            />
            <Box>Create New Lock</Box>
          </Flex>
        </FeatureHeaderTitle>
      </FeatureHeader>
      <Flex justifyContent="space-between">
        <Box width="150px" mb={4} data-testid="resource-selector">
          <Select
            value={selectedTargetType}
            options={lockTargets}
            onChange={(o: LockTarget) => setSelectedTargetType(o)}
            label="lock-target-type"
          />
        </Box>
        <QuickAdd
          selectedTarget={selectedTargetType.value}
          selectedLockTargets={selectedLockTargets}
          onAdd={onAdd}
        />
      </Flex>
      <TargetList
        data={targetData}
        onAdd={onAdd}
        selectedTarget={selectedTargetType.value}
        selectedLockTargets={selectedLockTargets}
      />
      <Flex
        data-testid="selected-locks"
        alignItems="center"
        justifyContent="space-between"
        borderRadius={3}
        p={3}
        mt={4}
        css={`
          background: ${({ theme }) => theme.colors.levels.surfaceSecondary};
        `}
      >
        <Box>
          <Text>Lock targets added ({selectedLockTargets.length})</Text>
        </Box>
        <Box>
          {selectedLockTargets.length > 0 && (
            <ButtonSecondary
              width="165px"
              mr={3}
              onClick={onClear}
              disabled={disabledSubmit}
            >
              Clear Selections
            </ButtonSecondary>
          )}
          <ButtonPrimary
            width="165px"
            onClick={() => setCreatePanelPosition('open')}
            disabled={disabledSubmit}
          >
            Proceed to lock
          </ButtonPrimary>
        </Box>
      </Flex>
    </FeatureBox>
  );
}

function TargetList({
  data,
  selectedTarget,
  selectedLockTargets,
  onAdd,
}: TargetListProps) {
  if (!data) data = [];

  if (selectedTarget === 'device') {
    return <Box>Listing Devices not implemented.</Box>;
  }

  if (selectedTarget === 'login') {
    return <Box>Unable to list logins, use quick add box.</Box>;
  }

  const columns: TableColumn<any>[] = data.length
    ? Object.keys(data[0]).map(c => {
        const col: TableColumn<any> = {
          key: c,
          headerText: c,
          isSortable: true,
        };
        if (c === 'labels') {
          col.render = target => {
            const labels = target.labels || [];
            return (
              <LabelCell data={labels.map(l => `${l.name}: ${l.value}`)} />
            );
          };
        }
        return col;
      })
    : [];

  if (columns.length) {
    columns.push({
      altKey: 'add-btn',
      render: ({ name }) => (
        <Cell align="right">
          <ButtonPrimary
            onClick={onAdd.bind(null, name)}
            data-testid="btn-cell"
            disabled={selectedLockTargets.some(
              target => target.type === selectedTarget && target.name === name
            )}
          >
            + Add
          </ButtonPrimary>
        </Cell>
      ),
    });
  }
  return (
    <StyledTable data={data} columns={columns} emptyText="No Targets Found" />
  );
}

function QuickAdd({
  selectedTarget,
  selectedLockTargets,
  onAdd,
}: {
  selectedTarget: string;
  selectedLockTargets: SelectedLockTarget[];
  onAdd: OnAdd;
}) {
  const [inputValue, setInputValue] = useState<string>('');
  return (
    <Flex
      justifyContent="flex-end"
      alignItems="center"
      css={{ columnGap: '20px' }}
      mb={4}
    >
      <Input
        placeholder={`Quick add ${selectedTarget}`}
        width={500}
        value={inputValue}
        onChange={e => setInputValue(e.currentTarget.value)}
      />
      <ButtonPrimary
        onClick={() => {
          onAdd(inputValue);
          setInputValue('');
        }}
        disabled={
          !inputValue.length ||
          selectedLockTargets?.some(
            target =>
              target.type === selectedTarget && target.name === inputValue
          )
        }
      >
        + Add
      </ButtonPrimary>
    </Flex>
  );
}
