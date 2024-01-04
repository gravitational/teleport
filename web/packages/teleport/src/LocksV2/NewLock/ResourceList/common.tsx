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
