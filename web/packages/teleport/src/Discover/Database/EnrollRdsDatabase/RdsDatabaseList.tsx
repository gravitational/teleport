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
import { Flex, Box, Label as Pill } from 'design';
import Table, { Cell } from 'design/DataTable';
import { FetchStatus } from 'design/DataTable/types';

import {
  AwsDatabase,
  ListAwsDatabaseResponse,
} from 'teleport/services/integrations';
import { Label } from 'teleport/types';

type Props = {
  items: ListAwsDatabaseResponse['databases'];
  fetchStatus: FetchStatus;
  fetchNextPage(): void;
  onSelectDatabase(item: AwsDatabase): void;
  selectedDatabase?: AwsDatabase;
};

export const DatabaseList = ({
  items = [],
  fetchStatus = '',
  fetchNextPage,
  onSelectDatabase,
  selectedDatabase,
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
              <RadioCell
                item={item}
                key={`${item.name}${item.engine}`}
                isChecked={isChecked}
                onChange={onSelectDatabase}
              />
            );
          },
        },
        {
          key: 'name',
          headerText: 'Name',
          render: ({ name }) => <Cell>{name}</Cell>,
        },
        {
          key: 'engine',
          headerText: 'Engine',
          render: ({ engine }) => <Cell>{engine}</Cell>,
        },
        {
          key: 'labels',
          headerText: 'Labels',
          render: ({ labels }) => <LabelCell labels={labels} />,
        },
        {
          key: 'status',
          headerText: 'Status',
          render: item => <StatusCell item={item} />,
        },
      ]}
      emptyText="No Results"
      pagination={{ pageSize: 10 }}
      fetching={{ onFetchMore: fetchNextPage, fetchStatus }}
      isSearchable
    />
  );
};

const StatusCell = ({ item }: { item: AwsDatabase }) => {
  const status = getStatus(item);

  return (
    <Cell>
      <Flex alignItems="center">
        <StatusLight status={status} />
        {item.status}
      </Flex>
    </Cell>
  );
};

function RadioCell({
  item,
  isChecked,
  onChange,
}: {
  item: AwsDatabase;
  isChecked: boolean;
  onChange(selectedItem: AwsDatabase): void;
}) {
  return (
    <Cell width="20px">
      <Flex alignItems="center" my={2} justifyContent="center">
        <input
          css={`
            margin: 0 ${props => props.theme.space[2]}px 0 0;
            accent-color: ${props => props.theme.colors.brand.accent};
            cursor: pointer;
          `}
          type="radio"
          name={item.name}
          checked={isChecked}
          onChange={() => onChange(item)}
          value={item.name}
        />
      </Flex>
    </Cell>
  );
}

enum Status {
  Success,
  Warning,
  Error,
}

function getStatus(item: AwsDatabase) {
  switch (item.status) {
    case 'Available':
      return Status.Success;

    case 'Failed':
    case 'Deleting':
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
      return theme.colors.success;
    }
    if (status === Status.Error) {
      return theme.colors.error.light;
    }
    if (status === Status.Warning) {
      return theme.colors.warning;
    }
    return theme.colors.grey[300]; // Unknown
  }};
`;

const LabelCell = ({ labels }: { labels: Label[] }) => {
  const $labels = labels.map((label, index) => {
    const labelText = `${label.name}: ${label.value}`;

    return (
      <Pill key={`${label.name}${label.value}${index}`} mr="1" kind="secondary">
        {labelText}
      </Pill>
    );
  });

  return (
    <Cell>
      <Flex flexWrap="wrap">{$labels}</Flex>
    </Cell>
  );
};
