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
import { Box, Text } from 'design';
import Table from 'design/DataTable';
import { Danger } from 'design/Alert';
import { FetchStatus } from 'design/DataTable/types';
import { Attempt } from 'shared/hooks/useAttemptNext';

import {
  RadioCell,
  DisableableCell as Cell,
  Labels,
  labelMatcher,
} from 'teleport/Discover/Shared';

import { useDiscover } from 'teleport/Discover/useDiscover';
import { Regions } from 'teleport/services/integrations';
import { isIamPermError } from 'teleport/Discover/Shared/Aws/error';
import { ConfigureIamPerms } from 'teleport/Discover/Shared/Aws/ConfigureIamPerms';

import { CheckedEc2Instance } from './EnrollEc2Instance';

type Props = {
  attempt: Attempt;
  items: CheckedEc2Instance[];
  fetchStatus: FetchStatus;
  fetchNextPage(): void;
  onSelectInstance(item: CheckedEc2Instance): void;
  selectedInstance?: CheckedEc2Instance;
  region: Regions;
};

export const Ec2InstanceList = ({
  attempt,
  items = [],
  fetchStatus = '',
  fetchNextPage,
  onSelectInstance,
  selectedInstance,
  region,
}: Props) => {
  const hasError = attempt.status === 'failed';
  const { agentMeta } = useDiscover();

  const showConfigureScript = isIamPermError(attempt);

  const disabledText = `This EC2 instance is already enrolled and is a part of this cluster`;

  return (
    <>
      {hasError && !showConfigureScript && (
        <Danger>{attempt.statusText}</Danger>
      )}
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
                    disabled={item.ec2InstanceExists}
                    value={item.awsMetadata.instanceId}
                    disabledText={disabledText}
                  />
                );
              },
            },
            {
              altKey: 'name',
              headerText: 'Name',
              render: ({ labels, ec2InstanceExists }) => (
                <Cell disabledText={disabledText} disabled={ec2InstanceExists}>
                  {labels.find(label => label.name === 'Name')?.value}
                </Cell>
              ),
            },
            {
              key: 'hostname',
              headerText: 'Hostname',
              render: ({ hostname, ec2InstanceExists }) => (
                <Cell disabledText={disabledText} disabled={ec2InstanceExists}>
                  {hostname}
                </Cell>
              ),
            },
            {
              key: 'addr',
              headerText: 'Address',
              render: ({ addr, ec2InstanceExists }) => (
                <Cell disabledText={disabledText} disabled={ec2InstanceExists}>
                  {addr}
                </Cell>
              ),
            },
            {
              altKey: 'instanceId',
              headerText: 'AWS Instance ID',
              render: ({ awsMetadata, ec2InstanceExists }) => (
                <Cell disabledText={disabledText} disabled={ec2InstanceExists}>
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
                <Cell disabledText={disabledText} disabled={ec2InstanceExists}>
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
      {showConfigureScript && (
        <Box mt={4}>
          <ConfigureIamPerms
            kind="ec2"
            region={region}
            integrationRoleArn={agentMeta.awsIntegration.spec.roleArn}
          />
        </Box>
      )}
    </>
  );
};
