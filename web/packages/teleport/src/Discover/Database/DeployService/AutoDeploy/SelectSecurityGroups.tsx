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

import React, { useState, useEffect } from 'react';

import { Text, Flex, Box, Indicator } from 'design';
import * as Icons from 'design/Icon';
import { FetchStatus } from 'design/DataTable/types';

import useAttempt from 'shared/hooks/useAttemptNext';
import { getErrMessage } from 'shared/utils/errorType';

import {
  integrationService,
  SecurityGroup,
} from 'teleport/services/integrations';
import { DbMeta } from 'teleport/Discover/useDiscover';

import { SecurityGroupPicker, ButtonBlueText } from '../../../Shared';

type TableData = {
  items: SecurityGroup[];
  nextToken?: string;
  fetchStatus: FetchStatus;
};

export const SelectSecurityGroups = ({
  selectedSecurityGroups,
  setSelectedSecurityGroups,
  dbMeta,
  emitErrorEvent,
}: {
  selectedSecurityGroups: string[];
  setSelectedSecurityGroups: React.Dispatch<React.SetStateAction<string[]>>;
  dbMeta: DbMeta;
  emitErrorEvent(err: string): void;
}) => {
  const [sgTableData, setSgTableData] = useState<TableData>({
    items: [],
    nextToken: '',
    fetchStatus: 'disabled',
  });

  const { attempt, run } = useAttempt('processing');

  function onSelectSecurityGroup(
    sg: SecurityGroup,
    e: React.ChangeEvent<HTMLInputElement>
  ) {
    if (e.target.checked) {
      return setSelectedSecurityGroups(currentSelectedGroups => [
        ...currentSelectedGroups,
        sg.id,
      ]);
    } else {
      setSelectedSecurityGroups(
        selectedSecurityGroups.filter(id => id !== sg.id)
      );
    }
  }

  async function fetchSecurityGroups() {
    const integration = dbMeta.integration;
    const selectedDb = dbMeta.selectedAwsRdsDb;

    run(() =>
      integrationService
        .fetchSecurityGroups(integration.name, {
          vpcId: selectedDb.vpcId,
          region: selectedDb.region,
          nextToken: sgTableData.nextToken,
        })
        .then(({ securityGroups, nextToken }) => {
          setSgTableData({
            nextToken: nextToken,
            fetchStatus: nextToken ? '' : 'disabled',
            items: [...sgTableData.items, ...securityGroups],
          });
        })
        .catch((err: Error) => {
          const errMsg = getErrMessage(err);
          emitErrorEvent(`fetch security groups error: ${errMsg}`);
          throw err;
        })
    );
  }

  useEffect(() => {
    fetchSecurityGroups();
  }, []);

  return (
    <>
      <Text bold>Step 3 (Optional)</Text>
      <Text bold>Select Security Groups</Text>
      <Text mb={2}>
        Select security groups to assign to the Fargate service that will be
        running the database access agent. The security groups you pick must
        allow outbound connectivity to this Teleport cluster. If you don't
        select any security groups, the default one for the VPC will be used.
      </Text>
      {attempt.status === 'failed' && (
        <>
          <Flex my={3}>
            <Icons.Warning size="medium" ml={1} mr={2} color="error.main" />
            <Text>{attempt.statusText}</Text>
          </Flex>
          <ButtonBlueText ml={1} onClick={fetchSecurityGroups}>
            Retry
          </ButtonBlueText>
        </>
      )}
      {attempt.status === 'processing' && (
        <Flex width="904px" justifyContent="center" mt={3}>
          <Indicator />
        </Flex>
      )}
      {attempt.status === 'success' && (
        <Box mt={3}>
          <SecurityGroupPicker
            items={sgTableData.items}
            attempt={attempt}
            fetchNextPage={fetchSecurityGroups}
            fetchStatus={sgTableData.fetchStatus}
            onSelectSecurityGroup={onSelectSecurityGroup}
            selectedSecurityGroups={selectedSecurityGroups}
          />
        </Box>
      )}
    </>
  );
};
