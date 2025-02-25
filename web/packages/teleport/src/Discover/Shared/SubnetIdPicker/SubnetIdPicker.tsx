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

import { ButtonIcon, Flex, Link } from 'design';
import { Danger } from 'design/Alert';
import { CheckboxInput } from 'design/Checkbox';
import Table, { Cell } from 'design/DataTable';
import { FetchStatus } from 'design/DataTable/types';
import { NewTab } from 'design/Icon';
import { Attempt } from 'shared/hooks/useAttemptNext';

import { Regions, Subnet } from 'teleport/services/integrations';

export function SubnetIdPicker({
  region,
  attempt,
  subnets = [],
  fetchStatus = '',
  fetchNextPage,
  onSelectSubnet,
  selectedSubnets,
}: {
  region: Regions;
  attempt: Attempt;
  subnets: Subnet[];
  fetchStatus: FetchStatus;
  fetchNextPage(): void;
  onSelectSubnet: (
    subnet: Subnet,
    e: React.ChangeEvent<HTMLInputElement>
  ) => void;
  selectedSubnets: string[];
}) {
  if (attempt.status === 'failed') {
    return <Danger>{attempt.statusText}</Danger>;
  }

  return (
    <Table
      data={subnets}
      columns={[
        {
          altKey: 'checkbox-select',
          headerText: 'Select',
          render: item => {
            const isChecked = selectedSubnets.includes(item.id);
            return (
              <CheckboxCell
                item={item}
                key={item.id}
                isChecked={isChecked}
                onChange={onSelectSubnet}
              />
            );
          },
        },
        {
          key: 'name',
          headerText: 'Name',
        },
        {
          key: 'id',
          headerText: 'ID',
        },
        {
          key: 'availabilityZone',
          headerText: 'Availability Zone',
        },
        {
          altKey: 'link-out',
          render: subnet => {
            return (
              <Cell>
                <ButtonIcon
                  as={Link}
                  target="_blank"
                  href={`https://${region}.console.aws.amazon.com/vpcconsole/home?${region}#SubnetDetails:subnetId=${subnet.id}`}
                >
                  <NewTab />
                </ButtonIcon>
              </Cell>
            );
          },
        },
      ]}
      emptyText="No Subnets Found"
      pagination={{ pageSize: 5 }}
      fetching={{ onFetchMore: fetchNextPage, fetchStatus }}
      isSearchable
    />
  );
}

function CheckboxCell({
  item,
  isChecked,
  onChange,
}: {
  item: Subnet;
  isChecked: boolean;
  onChange(selectedItem: Subnet, e: React.ChangeEvent<HTMLInputElement>): void;
}) {
  return (
    <Cell width="20px">
      <Flex alignItems="center" my={2} justifyContent="center">
        <CheckboxInput
          id={item.id}
          onChange={e => {
            onChange(item, e);
          }}
          checked={isChecked}
          data-testid={item.id}
        />
      </Flex>
    </Cell>
  );
}
