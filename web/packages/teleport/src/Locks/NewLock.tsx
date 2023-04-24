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
  TargetResource,
  DropdownOption,
  OnAdd,
  SelectedLockTarget,
  TargetListProps,
  TargetValue,
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
  const [selectedDropdownOption, setSelectedDropdownOption] =
    useState<DropdownOption>({
      label: 'User',
      value: 'user',
    });
  const [selectedLockTargets, setSelectedLockTargets] = useState<
    SelectedLockTarget[]
  >([]);
  const targetData = useGetTargetData(
    selectedDropdownOption?.value,
    clusterId,
    additionalTargets
  );

  function onAdd(targetValue: TargetValue) {
    selectedLockTargets.push({
      resource: selectedDropdownOption.value,
      targetValue,
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
        <Box width="164px" mb={4} data-testid="resource-selector">
          <Select
            value={selectedDropdownOption}
            options={lockTargets}
            onChange={(o: DropdownOption) => setSelectedDropdownOption(o)}
            label="lock-target-type"
          />
        </Box>
        <QuickAdd
          selectedResource={selectedDropdownOption.value}
          selectedLockTargets={selectedLockTargets}
          onAdd={onAdd}
        />
      </Flex>
      <TargetList
        data={targetData}
        onAdd={onAdd}
        selectedResource={selectedDropdownOption.value}
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
          background: ${({ theme }) => theme.colors.spotBackground[0]};
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
  selectedResource,
  selectedLockTargets,
  onAdd,
}: TargetListProps) {
  if (!data) data = [];

  if (selectedResource === 'device') {
    return <Box>Listing Devices not implemented.</Box>;
  }

  if (selectedResource === 'login') {
    return <Box>Unable to list logins, use quick add box.</Box>;
  }

  const columns: TableColumn<any>[] = data.length
    ? Object.keys(data[0])
        .filter(k => k !== 'targetValue') // don't show targetValue in the table
        .map(c => {
          const col: TableColumn<any> = {
            key: c,
            headerText: c === 'lastUsed' ? 'Last Used' : c,
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
      render: ({ targetValue }) => (
        <Cell align="right">
          <ButtonPrimary
            onClick={onAdd.bind(null, targetValue)}
            data-testid="btn-cell"
            disabled={selectedLockTargets.some(
              target =>
                target.resource === selectedResource &&
                target.targetValue === targetValue
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
  selectedResource,
  selectedLockTargets,
  onAdd,
}: {
  selectedResource: TargetResource;
  selectedLockTargets: SelectedLockTarget[];
  onAdd: OnAdd;
}) {
  const [targetValue, setTargetValue] = useState<TargetValue>('');
  return (
    <Flex
      justifyContent="flex-end"
      alignItems="center"
      css={{ columnGap: '20px' }}
      mb={4}
    >
      <Input
        placeholder={`Quick add ${selectedResource}`}
        width={500}
        value={targetValue}
        onChange={e => setTargetValue(e.currentTarget.value)}
      />
      <ButtonPrimary
        onClick={() => {
          onAdd(targetValue);
          setTargetValue('');
        }}
        disabled={
          !targetValue.length ||
          selectedLockTargets?.some(
            target =>
              target.resource === selectedResource &&
              target.targetValue === targetValue
          )
        }
      >
        + Add
      </ButtonPrimary>
    </Flex>
  );
}
