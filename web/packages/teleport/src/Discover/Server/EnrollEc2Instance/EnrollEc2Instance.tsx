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

import React, { useState } from 'react';
import { Box, Text } from 'design';
import { FetchStatus } from 'design/DataTable/types';
import useAttempt from 'shared/hooks/useAttemptNext';

import { getErrMessage } from 'shared/utils/errorType';

import useTeleport from 'teleport/useTeleport';
import cfg from 'teleport/config';
import { NodeMeta, useDiscover } from 'teleport/Discover/useDiscover';
import {
  Ec2InstanceConnectEndpoint,
  Regions,
  integrationService,
} from 'teleport/services/integrations';
import { AwsRegionSelector } from 'teleport/Discover/Shared/AwsRegionSelector';
import { Node } from 'teleport/services/nodes';

import {
  DiscoverEvent,
  DiscoverEventStatus,
} from 'teleport/services/userEvent';

import { ActionButtons, Header } from '../../Shared';

import { CreateEc2IceDialog } from '../CreateEc2Ice/CreateEc2IceDialog';

import { Ec2InstanceList } from './Ec2InstanceList';

// CheckedEc2Instance is a type to describe that an EC2 instance
// has been checked to determine whether or not it is already enrolled in the cluster.
export type CheckedEc2Instance = Node & {
  ec2InstanceExists?: boolean;
};

type TableData = {
  items: CheckedEc2Instance[];
  fetchStatus: FetchStatus;
  nextToken?: string;
  currRegion?: Regions;
};

const emptyTableData: TableData = {
  items: [],
  fetchStatus: 'disabled',
  nextToken: '',
};

export function EnrollEc2Instance() {
  const { agentMeta, emitErrorEvent, nextStep, updateAgentMeta, emitEvent } =
    useDiscover();
  const { nodeService } = useTeleport();

  const [currRegion, setCurrRegion] = useState<Regions>();
  const [existingEice, setExistingEice] =
    useState<Ec2InstanceConnectEndpoint>();
  const [selectedInstance, setSelectedInstance] =
    useState<CheckedEc2Instance>();

  const [tableData, setTableData] = useState<TableData>({
    items: [],
    nextToken: '',
    fetchStatus: 'disabled',
  });

  const {
    attempt: fetchEc2InstancesAttempt,
    setAttempt: setFetchEc2InstancesAttempt,
  } = useAttempt('');

  const { attempt: fetchEc2IceAttempt, setAttempt: setFetchEc2IceAttempt } =
    useAttempt('');

  function fetchEc2InstancesWithNewRegion(region: Regions) {
    if (region) {
      setCurrRegion(region);
      fetchEc2Instances({ ...emptyTableData, currRegion: region });
    }
  }

  function fetchNextPage() {
    fetchEc2Instances({ ...tableData });
  }

  function refreshEc2Instances() {
    // When refreshing, start the table back at page 1.
    fetchEc2Instances({ ...tableData, nextToken: '', items: [], currRegion });
  }

  async function fetchEc2Instances(data: TableData) {
    const integrationName = agentMeta.awsIntegration.name;

    setTableData({ ...data, fetchStatus: 'loading' });
    setFetchEc2InstancesAttempt({ status: 'processing' });

    try {
      const { instances: fetchedEc2Instances, nextToken } =
        await integrationService.fetchAwsEc2Instances(integrationName, {
          region: data.currRegion,
          nextToken: data.nextToken,
        });

      // Abort if there were no EC2 instances for the selected region.
      if (fetchedEc2Instances.length <= 0) {
        setFetchEc2InstancesAttempt({ status: 'success' });
        setTableData({ ...data, fetchStatus: 'disabled' });
        return;
      }

      // Check if fetched EC2 instances are already in the cluster
      // so that they can be disabled in the table.

      // Builds the predicate string that will query for
      // all the fetched EC2 instances by searching by the AWS instance ID label.
      const instanceIdPredicateQueries: string[] = fetchedEc2Instances.map(
        d =>
          `labels["teleport.dev/instance-id"] == "${d.awsMetadata.instanceId}"`
      );
      const fullPredicateQuery = instanceIdPredicateQueries.join(' || ');
      const { agents: fetchedNodes } = await nodeService.fetchNodes(
        cfg.proxyCluster,
        {
          query: fullPredicateQuery,
          limit: fetchedEc2Instances.length,
        }
      );

      const ec2InstancesLookupByInstanceId: Record<string, Node> = {};
      fetchedNodes.forEach(d => {
        // Extract the instanceId of the fetched node from its label.
        const instanceId = d.labels.find(
          label => label.name === 'teleport.dev/instance-id'
        )?.value;

        ec2InstancesLookupByInstanceId[instanceId] = d;
      });

      // Check for already existing EC2 instances.
      const checkedEc2Instances: CheckedEc2Instance[] = fetchedEc2Instances.map(
        ec2 => {
          const instance =
            ec2InstancesLookupByInstanceId[ec2.awsMetadata.instanceId];
          if (instance) {
            return {
              ...ec2,
              ec2InstanceExists: true,
            };
          }
          return ec2;
        }
      );

      setFetchEc2InstancesAttempt({ status: 'success' });
      setTableData({
        currRegion,
        nextToken,
        fetchStatus: nextToken ? '' : 'disabled',
        items: [...data.items, ...checkedEc2Instances],
      });
    } catch (err) {
      const errMsg = getErrMessage(err);
      setTableData(data);
      setFetchEc2InstancesAttempt({ status: 'failed', statusText: errMsg });
      emitErrorEvent(`ec2 instance fetch error: ${errMsg}`);
    }
  }

  async function fetchEc2InstanceConnectEndpoints() {
    const integrationName = agentMeta.awsIntegration.name;

    setFetchEc2IceAttempt({ status: 'processing' });
    try {
      const { endpoints: fetchedEc2Ices } =
        await integrationService.fetchAwsEc2InstanceConnectEndpoints(
          integrationName,
          {
            region: selectedInstance.awsMetadata.region,
            vpcId: selectedInstance.awsMetadata.vpcId,
          }
        );
      setFetchEc2IceAttempt({ status: 'success' });
      return fetchedEc2Ices;
    } catch (err) {
      const errMsg = getErrMessage(err);
      setFetchEc2InstancesAttempt({ status: 'failed', statusText: errMsg });
      emitErrorEvent(`ec2 instance connect endpoint fetch error: ${errMsg}`);
    }
  }

  function clear() {
    setFetchEc2InstancesAttempt({ status: '' });
    setTableData(emptyTableData);
    setSelectedInstance(null);
  }

  function handleOnProceed() {
    fetchEc2InstanceConnectEndpoints().then(ec2Ices => {
      const createCompleteEice = ec2Ices.find(
        e => e.state === 'create-complete'
      );
      const createInProgressEice = ec2Ices.find(
        e => e.state === 'create-in-progress'
      );

      // If we find existing EICE's that are either create-complete or create-in-progress, we skip the step where we create the EICE.

      // We first check for any EICE's that are create-complete, if we find one, the dialog will go straight to creating the node.
      // If we don't find any, we check if there are any that are create-in-progress, if we find one, the dialog will wait until
      // it's create-complete and then create the node.
      if (createCompleteEice || createInProgressEice) {
        setExistingEice(createCompleteEice || createInProgressEice);
        // Since the EICE had already been deployed before the flow, emit an event for EC2DeployEICE as `Skipped`.
        emitEvent(
          { stepStatus: DiscoverEventStatus.Skipped },
          {
            eventName: DiscoverEvent.EC2DeployEICE,
          }
        );
        updateAgentMeta({
          ...(agentMeta as NodeMeta),
          node: selectedInstance,
          ec2Ice: createCompleteEice || createInProgressEice,
        });
        // If we find neither, then we go to the next step to create the EICE.
      } else {
        updateAgentMeta({
          ...(agentMeta as NodeMeta),
          node: selectedInstance,
        });
        nextStep();
      }
    });
  }

  return (
    <Box maxWidth="1000px">
      <Header>Enroll an EC2 instance</Header>
      <Text mt={4}>
        Select the AWS Region you would like to see EC2 instances for:
      </Text>
      <AwsRegionSelector
        onFetch={fetchEc2InstancesWithNewRegion}
        onRefresh={refreshEc2Instances}
        clear={clear}
        disableSelector={fetchEc2InstancesAttempt.status === 'processing'}
      />
      {currRegion && (
        <Ec2InstanceList
          attempt={fetchEc2InstancesAttempt}
          items={tableData.items}
          fetchStatus={tableData.fetchStatus}
          selectedInstance={selectedInstance}
          onSelectInstance={setSelectedInstance}
          fetchNextPage={fetchNextPage}
          region={currRegion}
        />
      )}
      {existingEice && (
        <CreateEc2IceDialog
          nextStep={() => nextStep(2)}
          existingEice={existingEice}
        />
      )}
      <ActionButtons
        onProceed={handleOnProceed}
        disableProceed={
          fetchEc2InstancesAttempt.status === 'processing' ||
          fetchEc2IceAttempt.status === 'processing' ||
          !selectedInstance
        }
      />
    </Box>
  );
}
