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
import Table from 'design/DataTable';
import { Box, Indicator, Text, Flex } from 'design';
import { Warning } from 'design/Icon';
import { Danger } from 'design/Alert';
import { FetchStatus } from 'design/DataTable/types';

import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';
import { getErrMessage } from 'shared/utils/errorType';

import {
  AwsOidcDeployEc2InstanceConnectEndpointRequest,
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

  const {
    attempt: fetchSecurityGroupsAttempt,
    setAttempt: setFetchSecurityGroupsAttempt,
  } = useAttempt('');

  const { attempt: deployEc2IceAttempt, setAttempt: setDeployEc2IceAttempt } =
    useAttempt('');

  const { emitErrorEvent, agentMeta, prevStep, nextStep, emitEvent } =
    useDiscover();

  const autoDiscoverEnabled = !!agentMeta.autoDiscovery;

  useEffect(() => {
    // It has been decided for now that with auto discover,
    // default security groups will be used (in the request
    // this is depicted as an empty value)
    if (!autoDiscoverEnabled) {
      fetchSecurityGroups();
    }
  }, []);

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

    let endpoints: AwsOidcDeployEc2InstanceConnectEndpointRequest[] = [];
    if (autoDiscoverEnabled) {
      endpoints = Object.values(
        agentMeta.autoDiscovery.requiredVpcsAndSubnets
      ).map(subnets => ({
        // Being in this step of the flow means
        // the requiredVpcsAndSubnets will always
        // be defined.
        subnetId: subnets[0],
      }));
    } else {
      endpoints = [
        {
          subnetId: (agentMeta as NodeMeta).node.awsMetadata.subnetId,
          ...(selectedSecurityGroups.length && {
            securityGroupIds: selectedSecurityGroups,
          }),
        },
      ];
    }

    setDeployEc2IceAttempt({ status: 'processing' });
    setShowCreatingDialog(true);
    try {
      await integrationService.deployAwsEc2InstanceConnectEndpoints(
        integration.name,
        {
          region: agentMeta.awsRegion,
          endpoints,
        }
      );
      // Capture event for deploying EICE.
      emitEvent(
        { stepStatus: DiscoverEventStatus.Success },
        {
          eventName: DiscoverEvent.EC2DeployEICE,
        }
      );
      setDeployEc2IceAttempt({ status: 'success' });
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
        <Header>
          {autoDiscoverEnabled
            ? 'Create EC2 Instance Connect Endpoints'
            : 'Create an EC2 Instance Connect Endpoint'}
        </Header>
        <Box width="800px">
          {deployEc2IceAttempt.status === 'failed' && (
            <Danger>{deployEc2IceAttempt.statusText}</Danger>
          )}
          {autoDiscoverEnabled ? (
            <CreateEndpointsForAutoDiscover
              requiredVpcIdsAndSubnets={
                agentMeta.autoDiscovery.requiredVpcsAndSubnets
              }
            />
          ) : (
            <SecurityGroups
              fetchSecurityGroupsAttempt={fetchSecurityGroupsAttempt}
              fetchSecurityGroups={fetchSecurityGroups}
              tableData={tableData}
              onSelectSecurityGroup={onSelectSecurityGroup}
              selectedSecurityGroups={selectedSecurityGroups}
            />
          )}
        </Box>
        <ActionButtons
          onPrev={deployEc2IceAttempt.status === 'success' ? null : prevStep}
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

function CreateEndpointsForAutoDiscover({
  requiredVpcIdsAndSubnets,
}: {
  requiredVpcIdsAndSubnets: Record<string, string[]>;
}) {
  const items = Object.keys(requiredVpcIdsAndSubnets).map(key => ({
    vpcId: key,
    subnetId: requiredVpcIdsAndSubnets[key][0],
  }));

  return (
    <Box mt={2}>
      <Text mb={3}>
        EC2 Instance Connect Endpoints will be created for the following VPC
        ID's:
      </Text>
      <Table
        data={items}
        columns={[
          {
            key: 'vpcId',
            headerText: 'VPC ID',
          },
          {
            key: 'subnetId',
            headerText: 'The subnet ID that will be used',
          },
        ]}
        emptyText="No VPC ID's Found"
      />
    </Box>
  );
}

function SecurityGroups({
  fetchSecurityGroupsAttempt,
  fetchSecurityGroups,
  tableData,
  onSelectSecurityGroup,
  selectedSecurityGroups,
}: {
  fetchSecurityGroupsAttempt: Attempt;
  fetchSecurityGroups(): Promise<void>;
  tableData: TableData;
  onSelectSecurityGroup(
    sg: SecurityGroup,
    e: React.ChangeEvent<HTMLInputElement>
  ): void;
  selectedSecurityGroups: string[];
}) {
  return (
    <>
      <Text mb={1} typography="h4">
        Select AWS Security Groups to assign to the new EC2 Instance Connect
        Endpoint:
      </Text>
      <Text mb={2}>
        The security groups you pick should allow outbound connectivity for the
        agent to be able to dial Teleport clusters. If you don't select any
        security groups, the default one for the VPC will be used.
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
    </>
  );
}
