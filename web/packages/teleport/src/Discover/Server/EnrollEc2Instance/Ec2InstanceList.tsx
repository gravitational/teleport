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
import { Text } from 'design';
import Table from 'design/DataTable';
import { FetchStatus } from 'design/DataTable/types';
import { Attempt } from 'shared/hooks/useAttemptNext';

import {
  RadioCell,
  DisableableCell as Cell,
  Labels,
  labelMatcher,
} from 'teleport/Discover/Shared';

import { CheckedEc2Instance } from './EnrollEc2Instance';

type Props = {
  attempt: Attempt;
  items: CheckedEc2Instance[];
  fetchStatus: FetchStatus;
  fetchNextPage(): void;
  onSelectInstance(item: CheckedEc2Instance): void;
  selectedInstance?: CheckedEc2Instance;
  wantAutoDiscover: boolean;
};

export const Ec2InstanceList = ({
  attempt,
  items = [],
  fetchStatus = '',
  fetchNextPage,
  onSelectInstance,
  selectedInstance,
  wantAutoDiscover,
}: Props) => {
  const hasError = attempt.status === 'failed';

  return (
    <>
      {!hasError && (
        <Table
          data={items}
          columns={[
            {
              altKey: 'radio-select',
              headerText: 'Select',
              render: item => {
                const isChecked =
                  item.awsMetadata.instanceId ===
                  selectedInstance?.awsMetadata.instanceId;
                return (
                  <RadioCell<CheckedEc2Instance>
                    item={item}
                    key={item.awsMetadata.instanceId}
                    isChecked={isChecked}
                    onChange={onSelectInstance}
                    value={item.awsMetadata.instanceId}
                    {...disabledStates(
                      item.ec2InstanceExists,
                      wantAutoDiscover
                    )}
                  />
                );
              },
            },
            {
              altKey: 'name',
              headerText: 'Name',
              render: ({ labels, ec2InstanceExists }) => (
                <Cell {...disabledStates(ec2InstanceExists, wantAutoDiscover)}>
                  {labels.find(label => label.name === 'Name')?.value}
                </Cell>
              ),
            },
            {
              key: 'hostname',
              headerText: 'Hostname',
              render: ({ hostname, ec2InstanceExists }) => (
                <Cell {...disabledStates(ec2InstanceExists, wantAutoDiscover)}>
                  {hostname}
                </Cell>
              ),
            },
            {
              key: 'addr',
              headerText: 'Address',
              render: ({ addr, ec2InstanceExists }) => (
                <Cell {...disabledStates(ec2InstanceExists, wantAutoDiscover)}>
                  {addr}
                </Cell>
              ),
            },
            {
              altKey: 'instanceId',
              headerText: 'AWS Instance ID',
              render: ({ awsMetadata, ec2InstanceExists }) => (
                <Cell {...disabledStates(ec2InstanceExists, wantAutoDiscover)}>
                  <Text
                    css={`
                      text-wrap: nowrap;
                    `}
                  >
                    {awsMetadata.instanceId}
                  </Text>
                </Cell>
              ),
            },
            {
              key: 'labels',
              headerText: 'Labels',
              render: ({ labels, ec2InstanceExists }) => (
                <Cell {...disabledStates(ec2InstanceExists, wantAutoDiscover)}>
                  <Labels labels={labels} />
                </Cell>
              ),
            },
          ]}
          emptyText="No Results"
          pagination={{ pageSize: 10 }}
          customSearchMatchers={[labelMatcher]}
          fetching={{ onFetchMore: fetchNextPage, fetchStatus }}
          isSearchable
        />
      )}
    </>
  );
};

function disabledStates(ec2InstanceExists: boolean, wantAutoDiscover: boolean) {
  const disabled = wantAutoDiscover || ec2InstanceExists;

  let disabledText = `This EC2 instance is already enrolled and is a part of this cluster`;
  if (wantAutoDiscover) {
    disabledText = 'All eligible EC2 instances will be enrolled automatically';
  }

  return { disabled, disabledText };
}
