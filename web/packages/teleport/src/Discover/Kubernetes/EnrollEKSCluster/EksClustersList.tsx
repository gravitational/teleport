/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
import Table from 'design/DataTable';
import { FetchStatus } from 'design/DataTable/types';

import {
  DisableableCell as Cell,
  StatusCell,
  ItemStatus,
  RadioCell,
  Labels,
  labelMatcher,
} from 'teleport/Discover/Shared';

import { CheckedEksCluster } from './EnrollEksCluster';

type Props = {
  items: CheckedEksCluster[];
  autoDiscovery: boolean;
  fetchStatus: FetchStatus;
  fetchNextPage(): void;

  onSelectCluster(item: CheckedEksCluster): void;
  selectedCluster?: CheckedEksCluster;
};

export const ClustersList = ({
  items = [],
  autoDiscovery,
  fetchStatus = '',
  fetchNextPage,
  onSelectCluster,
  selectedCluster,
}: Props) => {
  return (
    <Table
      data={items}
      columns={[
        {
          altKey: 'radio-select',
          headerText: 'Select',
          render: item => {
            const isChecked = item.name === selectedCluster?.name;
            return (
              <RadioCell<CheckedEksCluster>
                item={item}
                key={`${item.name}${item.region}`}
                isChecked={isChecked}
                onChange={onSelectCluster}
                value={item.name}
                {...disabledStates(item, autoDiscovery)}
              />
            );
          },
        },
        {
          key: 'name',
          headerText: 'Name',
          render: item => (
            <Cell {...disabledStates(item, autoDiscovery)}>{item.name}</Cell>
          ),
        },
        {
          key: 'labels',
          headerText: 'Labels',
          render: item => (
            <Cell {...disabledStates(item, autoDiscovery)}>
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
              {...disabledStates(item, autoDiscovery)}
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

function getStatus(item: CheckedEksCluster) {
  switch (item.status.toLowerCase()) {
    case 'active':
      return ItemStatus.Success;

    case 'failed':
    case 'deleting':
      return ItemStatus.Error;

    default:
      return ItemStatus.Warning;
  }
}

function disabledStates(item: CheckedEksCluster, autoDiscovery: boolean) {
  const disabled =
    getStatus(item) !== ItemStatus.Success ||
    item.kubeServerExists ||
    autoDiscovery;

  let disabledText = `This EKS cluster is already enrolled and is a part of this cluster`;
  switch (item.status) {
    case 'failed':
    case 'pending':
    case 'creating':
    case 'updating':
      disabledText = 'Not available, try refreshing the list';
      break;
    case 'deleting':
      disabledText = 'Not available';
  }
  if (autoDiscovery) {
    disabledText = 'All eligible EKS clusters will be enrolled automatically';
  }

  return { disabled, disabledText };
}
