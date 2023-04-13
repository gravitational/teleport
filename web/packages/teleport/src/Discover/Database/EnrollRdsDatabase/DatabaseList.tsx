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
import { Flex } from 'design';
import Table, { Cell } from 'design/DataTable';
import { FetchStatus } from 'design/DataTable/types';

import {
  AwsDatabase,
  ListAwsDatabaseResponse,
} from 'teleport/services/integrations';

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
      ]}
      emptyText="No Results"
      pagination={{ pageSize: 10 }}
      fetching={{ onFetchMore: fetchNextPage, fetchStatus }}
      isSearchable
    />
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
