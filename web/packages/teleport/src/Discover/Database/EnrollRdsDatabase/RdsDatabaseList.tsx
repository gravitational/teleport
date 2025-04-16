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

import Table from 'design/DataTable';
import { FetchStatus } from 'design/DataTable/types';

import {
  DisableableCell as Cell,
  ItemStatus,
  labelMatcher,
  Labels,
  RadioCell,
  StatusCell,
} from 'teleport/Discover/Shared';

import { CheckedAwsRdsDatabase } from './SingleEnrollment';

type Props = {
  items: CheckedAwsRdsDatabase[];
  fetchStatus: FetchStatus;
  fetchNextPage(): void;
  onSelectDatabase?(item: CheckedAwsRdsDatabase): void;
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
        // Hide the selector when choosing to auto enroll
        ...(!wantAutoDiscover
          ? [
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
            ]
          : []),
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
            <StatusCell
              status={getStatus(item)}
              statusText={item.status}
              {...disabledStates(item, wantAutoDiscover)}
            />
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

function getStatus(item: CheckedAwsRdsDatabase) {
  switch (item.status) {
    case 'available':
      return ItemStatus.Success;

    case 'failed':
    case 'deleting':
      return ItemStatus.Error;
  }
}

function disabledStates(
  item: CheckedAwsRdsDatabase,
  wantAutoDiscover: boolean
) {
  const disabled =
    item.status === 'failed' ||
    item.status === 'deleting' ||
    (!wantAutoDiscover && item.dbServerExists);

  let disabledText = `This RDS database is already enrolled and is a part of this cluster`;
  if (item.status === 'failed') {
    disabledText = 'Not available, try refreshing the list';
  } else if (item.status === 'deleting') {
    disabledText = 'Not available';
  }

  return { disabled, disabledText };
}
