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
import { Flex, Box } from 'design';
import Table from 'design/DataTable';
import { FetchStatus } from 'design/DataTable/types';

import {
  DisableableCell as Cell,
  RadioCell,
  Labels,
  labelMatcher,
} from 'teleport/Discover/Shared';

import { CheckedAwsRdsDatabase } from './EnrollRdsDatabase';

type Props = {
  items: CheckedAwsRdsDatabase[];
  fetchStatus: FetchStatus;
  fetchNextPage(): void;
  onSelectDatabase(item: CheckedAwsRdsDatabase): void;
  selectedDatabase?: CheckedAwsRdsDatabase;
  wantAutoDiscover: boolean;
};

export const DatabaseList = ({
  items = [],
  fetchStatus = '',
  fetchNextPage,
  onSelectDatabase,
  selectedDatabase,
  wantAutoDiscover,
}: Props) => {
  return (
    <Table
      data={items}
      columns={[
        {
          altKey: 'radio-select',
          headerText: 'Select',
          render: item => {
            const isChecked =
              item.name === selectedDatabase?.name &&
              item.engine === selectedDatabase?.engine;
            return (
              <RadioCell<CheckedAwsRdsDatabase>
                item={item}
                key={`${item.name}${item.resourceId}`}
                isChecked={isChecked}
                onChange={onSelectDatabase}
                value={item.name}
                {...disabledStates(item, wantAutoDiscover)}
              />
            );
          },
        },
        {
          key: 'name',
          headerText: 'Name',
          render: item => (
            <Cell {...disabledStates(item, wantAutoDiscover)}>{item.name}</Cell>
          ),
        },
        {
          key: 'engine',
          headerText: 'Engine',
          render: item => (
            <Cell {...disabledStates(item, wantAutoDiscover)}>
              {item.engine}
            </Cell>
          ),
        },
        {
          key: 'labels',
          headerText: 'Labels',
          render: item => (
            <Cell {...disabledStates(item, wantAutoDiscover)}>
              <Labels labels={item.labels} />
            </Cell>
          ),
        },
        {
          key: 'status',
          headerText: 'Status',
          render: item => (
            <StatusCell item={item} wantAutoDiscover={wantAutoDiscover} />
          ),
        },
      ]}
      emptyText="No Results"
      customSearchMatchers={[labelMatcher]}
      pagination={{ pageSize: 10 }}
      fetching={{ onFetchMore: fetchNextPage, fetchStatus }}
      isSearchable
    />
  );
};

const StatusCell = ({
  item,
  wantAutoDiscover,
}: {
  item: CheckedAwsRdsDatabase;
  wantAutoDiscover: boolean;
}) => {
  const status = getStatus(item);

  return (
    <Cell {...disabledStates(item, wantAutoDiscover)}>
      <Flex alignItems="center">
        <StatusLight status={status} />
        {item.status}
      </Flex>
    </Cell>
  );
};

enum Status {
  Success,
  Warning,
  Error,
}

function getStatus(item: CheckedAwsRdsDatabase) {
  switch (item.status) {
    case 'available':
      return Status.Success;

    case 'failed':
    case 'deleting':
      return Status.Error;
  }
}

// TODO(lisa): copy from IntegrationList.tsx
// move to common file for both files.
const StatusLight = styled(Box)`
  border-radius: 50%;
  margin-right: 6px;
  width: 8px;
  height: 8px;
  background-color: ${({ status, theme }) => {
    if (status === Status.Success) {
      return theme.colors.success.main;
    }
    if (status === Status.Error) {
      return theme.colors.error.main;
    }
    if (status === Status.Warning) {
      return theme.colors.warning;
    }
    return theme.colors.grey[300]; // Unknown
  }};
`;

function disabledStates(
  item: CheckedAwsRdsDatabase,
  wantAutoDiscover: boolean
) {
  const disabled =
    item.status === 'failed' ||
    item.status === 'deleting' ||
    wantAutoDiscover ||
    item.dbServerExists;

  let disabledText = `This RDS database is already enrolled and is a part of this cluster`;
  if (wantAutoDiscover) {
    disabledText = 'All RDS databases will be enrolled automatically';
  } else if (item.status === 'failed') {
    disabledText = 'Not available, try refreshing the list';
  } else if (item.status === 'deleting') {
    disabledText = 'Not available';
  }

  return { disabled, disabledText };
}
