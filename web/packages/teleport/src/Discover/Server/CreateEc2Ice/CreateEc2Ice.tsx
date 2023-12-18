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

import React, { useState, useEffect } from 'react';

import { Box, Indicator, Text, Flex } from 'design';
import { Warning } from 'design/Icon';
import { Danger } from 'design/Alert';
import { FetchStatus } from 'design/DataTable/types';

import useAttempt from 'shared/hooks/useAttemptNext';
import { getErrMessage } from 'shared/utils/errorType';

import {
  SecurityGroup,
  integrationService,
} from 'teleport/services/integrations';
import {
  DiscoverEvent,
  DiscoverEventStatus,
} from 'teleport/services/userEvent';
import { NodeMeta, useDiscover } from 'teleport/Discover/useDiscover';
import {
  ActionButtons,
  ButtonBlueText,
  Header,
  SecurityGroupPicker,
} from 'teleport/Discover/Shared';

import { CreateEc2IceDialog } from './CreateEc2IceDialog';

type TableData = {
  items: SecurityGroup[];
  nextToken?: string;
  fetchStatus: FetchStatus;
};

export function CreateEc2Ice() {
  const [showCreatingDialog, setShowCreatingDialog] = useState(false);
  const [selectedSecurityGroups, setSelectedSecurityGroups] = useState<
    string[]
  >([]);
  const [tableData, setTableData] = useState<TableData>({
    items: [],
    nextToken: '',
    fetchStatus: 'disabled',
  });

  function onSelectSecurityGroup(
    sg: SecurityGroup,
    e: React.ChangeEvent<HTMLInputElement>
  ) {
    if (e.target.checked) {
      return setSelectedSecurityGroups([...selectedSecurityGroups, sg.id]);
    } else {
      setSelectedSecurityGroups(
        selectedSecurityGroups.filter(id => id !== sg.id)
      );
    }
  }

  useEffect(() => {
    fetchSecurityGroups();
  }, []);

  const {
    attempt: fetchSecurityGroupsAttempt,
    setAttempt: setFetchSecurityGroupsAttempt,
  } = useAttempt('');

  const { attempt: deployEc2IceAttempt, setAttempt: setDeployEc2IceAttempt } =
    useAttempt('');

  const { emitErrorEvent, agentMeta, prevStep, nextStep, emitEvent } =
    useDiscover();

  async function fetchSecurityGroups() {
    const integration = agentMeta.awsIntegration;

    setFetchSecurityGroupsAttempt({ status: 'processing' });
    try {
      const { securityGroups, nextToken } =
        await integrationService.fetchSecurityGroups(integration.name, {
          vpcId: (agentMeta as NodeMeta).node.awsMetadata.vpcId,
          region: (agentMeta as NodeMeta).node.awsMetadata.region,
          nextToken: tableData.nextToken,
        });

      setFetchSecurityGroupsAttempt({ status: 'success' });
      setTableData({
        nextToken: nextToken,
        fetchStatus: nextToken ? '' : 'disabled',
        items: [...tableData.items, ...securityGroups],
      });
    } catch (err) {
      const errMsg = getErrMessage(err);
      setFetchSecurityGroupsAttempt({ status: 'failed', statusText: errMsg });
      emitErrorEvent(`fetch security groups error: ${errMsg}`);
    }
  }

  async function deployEc2InstanceConnectEndpoint() {
    const integration = agentMeta.awsIntegration;

    setDeployEc2IceAttempt({ status: 'processing' });
    setShowCreatingDialog(true);
    try {
      await integrationService.deployAwsEc2InstanceConnectEndpoint(
        integration.name,
        {
          region: (agentMeta as NodeMeta).node.awsMetadata.region,
          subnetId: (agentMeta as NodeMeta).node.awsMetadata.subnetId,
          ...(selectedSecurityGroups.length && {
            securityGroupIds: selectedSecurityGroups,
          }),
        }
      );
      // Capture event for deploying EICE.
      emitEvent(
        { stepStatus: DiscoverEventStatus.Success },
        {
          eventName: DiscoverEvent.EC2DeployEICE,
        }
      );
    } catch (err) {
      const errMsg = getErrMessage(err);
      setShowCreatingDialog(false);
      setDeployEc2IceAttempt({ status: 'failed', statusText: errMsg });
      // Capture error event for failing to deploy EICE.
      emitEvent(
        { stepStatus: DiscoverEventStatus.Error, stepStatusError: errMsg },
        {
          eventName: DiscoverEvent.EC2DeployEICE,
        }
      );
    }
  }

  function handleOnProceed() {
    deployEc2InstanceConnectEndpoint();
  }

  return (
    <>
      <Box maxWidth="800px">
        <Header>Create an EC2 Instance Connect Endpoint</Header>
        <Box width="800px">
          {deployEc2IceAttempt.status === 'failed' && (
            <Danger>{deployEc2IceAttempt.statusText}</Danger>
          )}
          <Text mb={1} typography="h4">
            Select AWS Security Groups to assign to the new EC2 Instance Connect
            Endpoint:
          </Text>
          <Text mb={2}>
            The security groups you pick should allow outbound connectivity for
            the agent to be able to dial Teleport clusters. If you don't select
            any security groups, the default one for the VPC will be used.
          </Text>
          {fetchSecurityGroupsAttempt.status === 'failed' && (
            <>
              <Flex my={3}>
                <Warning size="medium" ml={1} mr={2} color="error.main" />
                <Text>{fetchSecurityGroupsAttempt.statusText}</Text>
              </Flex>
              <ButtonBlueText ml={1} onClick={fetchSecurityGroups}>
                Retry
              </ButtonBlueText>
            </>
          )}
          {fetchSecurityGroupsAttempt.status === 'processing' && (
            <Flex width="352px" justifyContent="center" mt={3}>
              <Indicator />
            </Flex>
          )}
          {fetchSecurityGroupsAttempt.status === 'success' && (
            <Box width="1000px">
              <SecurityGroupPicker
                items={tableData.items}
                attempt={fetchSecurityGroupsAttempt}
                fetchNextPage={() => fetchSecurityGroups()}
                fetchStatus={tableData.fetchStatus}
                onSelectSecurityGroup={onSelectSecurityGroup}
                selectedSecurityGroups={selectedSecurityGroups}
              />
            </Box>
          )}
        </Box>
        <ActionButtons
          onPrev={prevStep}
          onProceed={() => handleOnProceed()}
          disableProceed={deployEc2IceAttempt.status === 'processing'}
        />
      </Box>
      {showCreatingDialog && (
        <CreateEc2IceDialog
          nextStep={nextStep}
          retry={() => deployEc2InstanceConnectEndpoint()}
        />
      )}
    </>
  );
}
