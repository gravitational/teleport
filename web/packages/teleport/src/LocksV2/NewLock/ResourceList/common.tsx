/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
import React from 'react';
import styled from 'styled-components';
import { Box, ButtonPrimary, ButtonBorder } from 'design';
import Table, { Cell } from 'design/DataTable';
import {
  CustomSort,
  FetchStatus,
  LabelDescription,
} from 'design/DataTable/types';

import { LockResourceMap, ToggleSelectResourceFn } from '../common';

export type ServerSideListProps = {
  fetchStatus: FetchStatus;
  customSort: CustomSort;
  onLabelClick(label: LabelDescription): void;
  selectedResources: LockResourceMap;
  toggleSelectResource: ToggleSelectResourceFn;
};

export type SimpleListProps = {
  pageSize: number;
  fetchStatus: FetchStatus;
  selectedResources: LockResourceMap;
  toggleSelectResource: ToggleSelectResourceFn;
};

export type LoginsProps = {
  pageSize: number;
  selectedResources: LockResourceMap;
  toggleSelectResource: ToggleSelectResourceFn;
};

export type HybridListProps = {
  pageSize: number;
  selectedResources: LockResourceMap;
  toggleSelectResource: ToggleSelectResourceFn;
  fetchNextPage(): void;
  fetchStatus: FetchStatus;
};

export const StyledTable = styled(Table)`
  & > tbody > tr > td {
    vertical-align: middle;
  }
` as typeof Table;

export const TableWrapper = styled(Box)`
  &.disabled {
    pointer-events: none;
    opacity: 0.5;
  }
`;

export function renderActionCell(
  isResourceSelected: boolean,
  toggleResourceSelect: () => void
) {
  return (
    <Cell align="right">
      {isResourceSelected ? (
        <ButtonPrimary
          width="134px"
          size="small"
          onClick={toggleResourceSelect}
        >
          Remove
        </ButtonPrimary>
      ) : (
        <ButtonBorder width="134px" size="small" onClick={toggleResourceSelect}>
          + Add Target
        </ButtonBorder>
      )}
    </Cell>
  );
}
