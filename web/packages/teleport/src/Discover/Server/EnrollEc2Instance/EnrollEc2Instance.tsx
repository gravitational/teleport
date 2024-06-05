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
import { Box, Link as ExternalLink, Text, Toggle } from 'design';
import { Link as InternalLink } from 'react-router-dom';
import styled from 'styled-components';
import { FetchStatus } from 'design/DataTable/types';
import useAttempt from 'shared/hooks/useAttemptNext';
import { Danger } from 'design/Alert';
import { OutlineInfo } from 'design/Alert/Alert';
import { Info } from 'design/Icon';

import { getErrMessage } from 'shared/utils/errorType';
import { ToolTipInfo } from 'shared/components/ToolTip';

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
import {
  DISCOVERY_GROUP_CLOUD,
  DEFAULT_DISCOVERY_GROUP_NON_CLOUD,
  DiscoveryConfig,
  createDiscoveryConfig,
} from 'teleport/services/discovery';
import {
  getAttemptsOneOfErrorMsg,
  isIamPermError,
} from 'teleport/Discover/Shared/Aws/error';
import { ConfigureIamPerms } from 'teleport/Discover/Shared/Aws/ConfigureIamPerms';

import {
  ActionButtons,
  Header,
  SelfHostedAutoDiscoverDirections,
} from '../../Shared';

import { CreateEc2IceDialog } from '../CreateEc2Ice/CreateEc2IceDialog';

import { Ec2InstanceList } from './Ec2InstanceList';
import { NoEc2IceRequiredDialog } from './NoEc2IceRequiredDialog';

// CheckedEc2Instance is a type to describe that an EC2 instance
// has been checked to determine whether or not it is already enrolled in the cluster.
export type CheckedEc2Instance = Node & {
  ec2InstanceExists?: boolean;
};

type TableData = {
  items: CheckedEc2Instance[];
  fetchStatus: FetchStatus;
  nextToken?: string;
};

const emptyTableData: TableData = {
  items: [],
  fetchStatus: 'disabled',
  nextToken: '',
};

export function EnrollEc2Instance() {
  const { agentMeta, emitErrorEvent, nextStep, updateAgentMeta, emitEvent } =
    useDiscover();
  const { nodeService, storeUser } = useTeleport();

  const [currRegion, setCurrRegion] = useState<Regions>();
  const [foundAllRequiredEices, setFoundAllRequiredEices] =
    useState<Ec2InstanceConnectEndpoint[]>();
  const [selectedInstance, setSelectedInstance] =
    useState<CheckedEc2Instance>();

  const [tableData, setTableData] = useState<TableData>({
    items: [],
    nextToken: '',
    fetchStatus: 'disabled',
  });

  const [autoDiscoveryCfg, setAutoDiscoveryCfg] = useState<DiscoveryConfig>();
  const [wantAutoDiscover, setWantAutoDiscover] = useState(true);
  const [discoveryGroupName, setDiscoveryGroupName] = useState(() =>
    cfg.isCloud ? '' : DEFAULT_DISCOVERY_GROUP_NON_CLOUD
  );

  const {
    attempt: fetchEc2InstancesAttempt,
    setAttempt: setFetchEc2InstancesAttempt,
  } = useAttempt('');

  const { attempt: fetchEc2IceAttempt, setAttempt: setFetchEc2IceAttempt } =
    useAttempt('');

  function fetchEc2InstancesWithNewRegion(region: Regions) {
    if (region) {
      setCurrRegion(region);
      fetchEc2Instances({ ...emptyTableData }, region);
    }
  }

  function fetchNextPage() {
    fetchEc2Instances({ ...tableData }, currRegion);
  }

  function refreshEc2Instances() {
    setSelectedInstance(null);
    setFetchEc2IceAttempt({ status: '' });
    // When refreshing, start the table back at page 1.
    fetchEc2Instances({ ...tableData, items: [] }, currRegion);
  }

  async function fetchEc2Instances(data: TableData, region: Regions) {
    const integrationName = agentMeta.awsIntegration.name;

    setTableData({ ...data, fetchStatus: 'loading' });
    setFetchEc2InstancesAttempt({ status: 'processing' });

    try {
      let fetchedEc2Instances: Node[] = [];
      let nextPage = '';
      // Requires list of all ec2 instances
      // to formulate map of VPCs and its subnets.
      do {
        const { instances, nextToken } =
          await integrationService.fetchAwsEc2Instances(integrationName, {
            region: region,
            nextToken: nextPage,
          });

        fetchedEc2Instances = [...fetchedEc2Instances, ...instances];
        nextPage = nextToken;
      } while (nextPage);
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
        ...data,
        fetchStatus: 'disabled',
        items: checkedEc2Instances,
      });
    } catch (err) {
      const errMsg = getErrMessage(err);
      setTableData(data);
      setFetchEc2InstancesAttempt({ status: 'failed', statusText: errMsg });
      emitErrorEvent(`ec2 instance fetch error: ${errMsg}`);
    }
  }

  /**
   * @returns
   *    - undefined: if there was an error from request
   *    - array: list of ec2 instance connect endpoints or,
   *      empty list if no endpoints
   */
  async function fetchEc2InstanceConnectEndpointsWithErrorHandling(
    vpcIds: string[]
  ) {
    const integrationName = agentMeta.awsIntegration.name;

    try {
      const { endpoints: fetchedEc2Ices } =
        await integrationService.fetchAwsEc2InstanceConnectEndpoints(
          integrationName,
          {
            region: currRegion,
            vpcIds,
          }
        );
      return fetchedEc2Ices;
    } catch (err) {
      const errMsg = getErrMessage(err);
      setFetchEc2IceAttempt({ status: 'failed', statusText: errMsg });
      emitErrorEvent(`ec2 instance connect endpoint fetch error: ${errMsg}`);
    }
  }

  function clear() {
    setFetchEc2InstancesAttempt({ status: '' });
    setFetchEc2IceAttempt({ status: '' });
    setTableData(emptyTableData);
    setSelectedInstance(null);
    setAutoDiscoveryCfg(null);
    setFoundAllRequiredEices(null);
  }

  /**
   * @returns
   *    - undefined: if there was an error from request or
   *    - object: the created discovery config object
   */
  async function createAutoDiscoveryConfigWithErrorHandling() {
    // We check the agentmeta because a user could've returned
    // to this step from the deploy step (clicking "back" button)
    const alreadyCreatedCfg =
      agentMeta?.autoDiscovery && agentMeta.awsRegion === currRegion;

    if (!autoDiscoveryCfg && !alreadyCreatedCfg) {
      try {
        const discoveryConfig = await createDiscoveryConfig(
          storeUser.getClusterId(),
          {
            name: crypto.randomUUID(),
            discoveryGroup: cfg.isCloud
              ? DISCOVERY_GROUP_CLOUD
              : discoveryGroupName,
            aws: [
              {
                types: ['ec2'],
                regions: [currRegion],
                tags: { '*': ['*'] },
                integration: agentMeta.awsIntegration.name,
              },
            ],
          }
        );
        return discoveryConfig;
      } catch (err) {
        const errMsg = getErrMessage(err);
        setFetchEc2IceAttempt({ status: 'failed', statusText: errMsg });
        emitErrorEvent(`failed to create discovery config:  ${errMsg}`);
      }
    }

    if (agentMeta.autoDiscovery) {
      return agentMeta.autoDiscovery.config;
    }

    return autoDiscoveryCfg;
  }

  /**
   * Note: takes about 1 minute to go from `create-in-progress` to `create-complete`
   * `create-in-progress` can be polled until it reaches `create-complete`
   */
  function getCompleteOrInProgressEndpoints(
    endpoints: Ec2InstanceConnectEndpoint[]
  ) {
    return endpoints.filter(
      e => e.state === 'create-complete' || e.state === 'create-in-progress'
    );
  }

  async function enableAutoDiscovery() {
    // Collect unique vpcIds and its subnet for instances.
    const seenVpcIdAndSubnets: Record<string, string> = {};
    tableData.items.forEach(i => {
      const vpcId = i.awsMetadata.vpcId;
      if (!seenVpcIdAndSubnets[vpcId]) {
        // Instances can have the same vpcId and be assigned
        // different subnetIds, but each subnet belongs to a
        // single VPC, so it does not matter which subnet we
        // assign to this vpc.
        seenVpcIdAndSubnets[vpcId] = i.awsMetadata.subnetId;
      }
    });

    // Check if an instance connect endpoint exist for the collected vpcs.

    // instancesVpcIds can be zero if if no ec2 instances are enrolled.
    const instancesVpcIds = Object.keys(seenVpcIdAndSubnets);
    const gotEc2Ices =
      await fetchEc2InstanceConnectEndpointsWithErrorHandling(instancesVpcIds);
    if (!gotEc2Ices) {
      // errored
      return;
    }

    const listOfExistingEndpoints =
      getCompleteOrInProgressEndpoints(gotEc2Ices);

    // Determine which instance vpc needs a ec2 instance connect endpoint.
    const requiredVpcsAndSubnets: Record<string, string[]> = {};
    if (instancesVpcIds.length != gotEc2Ices.length) {
      instancesVpcIds.forEach(instanceVpcId => {
        const found = gotEc2Ices.some(
          endpoint => endpoint.vpcId == instanceVpcId
        );
        if (!found) {
          requiredVpcsAndSubnets[instanceVpcId] = [
            seenVpcIdAndSubnets[instanceVpcId],
          ];
        }
      });
    }

    const discoveryConfig = await createAutoDiscoveryConfigWithErrorHandling();
    if (!discoveryConfig) {
      // errored
      return;
    }
    setFetchEc2IceAttempt({ status: 'success' });
    setAutoDiscoveryCfg(discoveryConfig);
    updateAgentMeta({
      ...(agentMeta as NodeMeta),
      ec2Ices: listOfExistingEndpoints,
      autoDiscovery: {
        config: discoveryConfig,
        requiredVpcsAndSubnets,
      },
      awsRegion: currRegion,
    });

    // Check if creating endpoints is required.

    const allRequiredEndpointsExists =
      listOfExistingEndpoints.length > 0 &&
      Object.keys(requiredVpcsAndSubnets).length === 0;

    if (allRequiredEndpointsExists || instancesVpcIds.length === 0) {
      setFoundAllRequiredEices(listOfExistingEndpoints);
      emitEvent(
        { stepStatus: DiscoverEventStatus.Skipped },
        {
          eventName: DiscoverEvent.EC2DeployEICE,
        }
      );
    } else {
      nextStep();
    }
  }

  async function handleOnProceed() {
    setFetchEc2IceAttempt({ status: 'processing' });

    if (wantAutoDiscover) {
      enableAutoDiscovery();
    } else {
      const ec2Ices = await fetchEc2InstanceConnectEndpointsWithErrorHandling([
        selectedInstance.awsMetadata.vpcId,
      ]);
      if (!ec2Ices) {
        return;
      }
      setFetchEc2IceAttempt({ status: 'success' });

      const existingEndpoint = getCompleteOrInProgressEndpoints(ec2Ices);

      // If we find existing EICE's that are either create-complete or create-in-progress, we skip the step where we create the EICE.

      // We first check for any EICE's that are create-complete, if we find one, the dialog will go straight to creating the node.
      // If we don't find any, we check if there are any that are create-in-progress, if we find one, the dialog will wait until
      // it's create-complete and then create the node.
      if (existingEndpoint.length > 0) {
        setFoundAllRequiredEices(existingEndpoint);
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
          ec2Ices: existingEndpoint,
          awsRegion: currRegion,
        });
        // If we find neither, then we go to the next step to create the EICE.
      } else {
        updateAgentMeta({
          ...(agentMeta as NodeMeta),
          node: selectedInstance,
          awsRegion: currRegion,
        });
        nextStep();
      }
    }
  }

  // (Temp)
  // Self hosted auto enroll is different from cloud.
  // For cloud, we already run the discovery service for customer.
  // For on-prem, user has to run their own discovery service.
  // We hide the table for on-prem if they are wanting auto discover
  // because it takes up so much space to give them instructions.
  // Future work will simply provide user a script so we can show the table then.
  const showTable = cfg.isCloud || !wantAutoDiscover;

  const errorMsg = getAttemptsOneOfErrorMsg(
    fetchEc2InstancesAttempt,
    fetchEc2IceAttempt
  );

  const hasIamPermError =
    isIamPermError(fetchEc2IceAttempt) ||
    isIamPermError(fetchEc2InstancesAttempt);

  const showContent = !hasIamPermError && currRegion;
  const showAutoEnrollToggle =
    !errorMsg && fetchEc2InstancesAttempt.status === 'success';

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
      {!hasIamPermError && errorMsg && <Danger>{errorMsg}</Danger>}
      {showContent && (
        <>
          {showAutoEnrollToggle && (
            <Box mb={2}>
              <Toggle
                isToggled={wantAutoDiscover}
                onToggle={() => setWantAutoDiscover(b => !b)}
                disabled={tableData.items.length === 0} // necessary?
              >
                <Box ml={2} mr={1}>
                  Auto-enroll all EC2 instances for selected region
                </Box>
                <ToolTipInfo>
                  Auto-enroll will automatically identify all EC2 instances from
                  the selected region and register them as node resources in
                  your infrastructure.
                </ToolTipInfo>
              </Toggle>
              {wantAutoDiscover && (
                <OutlineInfo mt={3} linkColor="buttons.link.default">
                  <Box>
                    <InfoIcon />
                  </Box>
                  <Box>
                    AWS enforces{' '}
                    <ExternalLink
                      target="_blank"
                      href="https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/eice-quotas.html"
                    >
                      strict quotas
                    </ExternalLink>{' '}
                    on auto-enrolled EC2 instances, particularly for the maximum
                    number of allowed concurrent connections per EC2 Instance
                    Connect Endpoint. If these quotas restrict your needs,
                    consider following the{' '}
                    <InternalLink
                      to={{
                        pathname: cfg.routes.discover,
                        state: { searchKeywords: 'linux' },
                      }}
                    >
                      Teleport service installation
                    </InternalLink>{' '}
                    flow instead.
                  </Box>
                </OutlineInfo>
              )}
              {!cfg.isCloud && wantAutoDiscover && (
                <SelfHostedAutoDiscoverDirections
                  clusterPublicUrl={storeUser.state.cluster.publicURL}
                  discoveryGroupName={discoveryGroupName}
                  setDiscoveryGroupName={setDiscoveryGroupName}
                />
              )}
            </Box>
          )}
          {showTable && (
            <Ec2InstanceList
              wantAutoDiscover={wantAutoDiscover}
              attempt={fetchEc2InstancesAttempt}
              items={tableData.items}
              fetchStatus={tableData.fetchStatus}
              selectedInstance={selectedInstance}
              onSelectInstance={setSelectedInstance}
              fetchNextPage={fetchNextPage}
            />
          )}
        </>
      )}
      {foundAllRequiredEices?.length > 0 && (
        <CreateEc2IceDialog
          nextStep={() => nextStep(2)}
          existingEices={foundAllRequiredEices}
        />
      )}
      {foundAllRequiredEices?.length === 0 && (
        <NoEc2IceRequiredDialog nextStep={() => nextStep(2)} />
      )}
      {hasIamPermError && (
        <Box>
          <ConfigureIamPerms
            region={currRegion}
            integrationRoleArn={agentMeta.awsIntegration.spec.roleArn}
            kind="ec2"
          />
        </Box>
      )}
      <ActionButtons
        onProceed={handleOnProceed}
        disableProceed={
          fetchEc2InstancesAttempt.status === 'processing' ||
          fetchEc2IceAttempt.status === 'processing' ||
          !currRegion ||
          (!wantAutoDiscover && !selectedInstance) ||
          (!cfg.isCloud && !discoveryGroupName) ||
          hasIamPermError
        }
      />
    </Box>
  );
}

const InfoIcon = styled(Info)`
  background-color: ${p => p.theme.colors.link};
  border-radius: 100px;
  height: 32px;
  width: 32px;
  color: ${p => p.theme.colors.text.primaryInverse};
  margin-right: ${p => p.theme.space[2]}px;
`;
