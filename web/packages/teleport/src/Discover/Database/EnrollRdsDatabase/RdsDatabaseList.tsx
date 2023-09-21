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
import Table, { Cell as TableCell } from 'design/DataTable';
import { FetchStatus } from 'design/DataTable/types';

import { Label } from 'teleport/types';

import { CheckedAwsRdsDatabase } from './EnrollRdsDatabase';

type Props = {
  items: CheckedAwsRdsDatabase[];
  fetchStatus: FetchStatus;
  fetchNextPage(): void;
  onSelectDatabase(item: CheckedAwsRdsDatabase): void;
  selectedDatabase?: CheckedAwsRdsDatabase;
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
                key={`${item.name}${item.resourceId}`}
                isChecked={isChecked}
                onChange={onSelectDatabase}
                disabled={item.dbServerExists}
              />
            );
          },
        },
        {
          key: 'name',
          headerText: 'Name',
          render: ({ name, dbServerExists }) => (
            <Cell disabled={dbServerExists}>{name}</Cell>
          ),
        },
        {
          key: 'engine',
          headerText: 'Engine',
          render: ({ engine, dbServerExists }) => (
            <Cell disabled={dbServerExists}>{engine}</Cell>
          ),
        },
        {
          key: 'labels',
          headerText: 'Labels',
          render: ({ labels, dbServerExists }) => (
            <Cell disabled={dbServerExists}>
              <Labels labels={labels} />
            </Cell>
          ),
        },
        {
          key: 'status',
          headerText: 'Status',
          render: item => <StatusCell item={item} />,
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

const StatusCell = ({ item }: { item: CheckedAwsRdsDatabase }) => {
  const status = getStatus(item);

  return (
    <Cell disabled={item.dbServerExists}>
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
  disabled,
}: {
  item: CheckedAwsRdsDatabase;
  isChecked: boolean;
  onChange(selectedItem: CheckedAwsRdsDatabase): void;
  disabled: boolean;
}) {
  return (
    <Cell width="20px" disabled={disabled}>
      <Flex alignItems="center" my={2} justifyContent="center">
        <input
          css={`
            margin: 0 ${props => props.theme.space[2]}px 0 0;
            accent-color: ${props => props.theme.colors.brand.accent};
            cursor: pointer;

            &:disabled {
              cursor: not-allowed;
            }
          `}
          type="radio"
          name={item.name}
          checked={isChecked}
          onChange={() => onChange(item)}
          value={item.name}
          disabled={disabled}
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
      return theme.colors.success;
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

const Labels = ({ labels }: { labels: Label[] }) => {
  const $labels = labels.map((label, index) => {
    const labelText = `${label.name}: ${label.value}`;

    return (
      <Pill key={`${label.name}${label.value}${index}`} mr="1" kind="secondary">
        {labelText}
      </Pill>
    );
  });

  return <Flex flexWrap="wrap">{$labels}</Flex>;
};

// labelMatcher allows user to client search by labels in the format
//   1) `key: value` or
//   2) `key:value` or
//   3) `key` or `value`
function labelMatcher(
  targetValue: any,
  searchValue: string,
  propName: keyof CheckedAwsRdsDatabase & string
) {
  if (propName === 'labels') {
    return targetValue.some((label: Label) => {
      const convertedKey = label.name.toLocaleUpperCase();
      const convertedVal = label.value.toLocaleUpperCase();
      const formattedWords = [
        `${convertedKey}:${convertedVal}`,
        `${convertedKey}: ${convertedVal}`,
      ];
      return formattedWords.some(w => w.includes(searchValue));
    });
  }
}

const Cell: React.FC<{ disabled: boolean; width?: string }> = ({
  disabled,
  width,
  children,
}) => {
  return (
    <TableCell
      width={width}
      title={
        disabled
          ? 'this RDS database is already enrolled and is a part of this cluster'
          : null
      }
      css={`
        opacity: ${disabled ? '0.5' : '1'};
      `}
    >
      {children}
    </TableCell>
  );
};
