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
import {
  Text,
  Flex,
  AnimatedProgressBar,
  ButtonPrimary,
  Link,
  Box,
} from 'design';
import * as Icons from 'design/Icon';
import Dialog, { DialogContent } from 'design/DialogConfirmation';

import { getErrMessage } from 'shared/utils/errorType';

import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';

import cfg from 'teleport/config';
import useTeleport from 'teleport/useTeleport';

import {
  Ec2InstanceConnectEndpoint,
  integrationService,
} from 'teleport/services/integrations';
import { Mark, TextIcon } from 'teleport/Discover/Shared';
import { NodeMeta, useDiscover } from 'teleport/Discover/useDiscover';
import { usePoll } from 'teleport/Discover/Shared/usePoll';
import {
  DiscoverEvent,
  DiscoverEventStatus,
} from 'teleport/services/userEvent';

export function CreateEc2IceDialog({
  nextStep,
  retry,
  existingEices = null,
}: {
  nextStep: () => void;
  retry?: () => void;
  // Only supplied if there exists all the required ec2
  // instance connect endpoints, resulting in the user
  // being able to skip the `eice deployment` step.
  // Though the endpoints might all exist they may not all be
  // in the `create-complete` state, which means polling
  // for this endpoint is required until it becomes
  // `create-complete`.
  // If this field is NOT supplied, then new endpoints
  // have been deployed which also needs to be polled
  // until `create-complete`.
  existingEices?: Ec2InstanceConnectEndpoint[];
}) {
  const { nodeService } = useTeleport();

  // If the EICE already exists from the previous step and is
  // create-complete, we don't need to do any polling for the EICE.
  const [isPollingActive, setIsPollingActive] = useState(() =>
    existingEices
      ? existingEices.some(e => e.state !== 'create-complete')
      : true
  );
  const [mainDashboardLink, setMainDashboardLink] = useState('');

  const { emitErrorEvent, updateAgentMeta, agentMeta, emitEvent } =
    useDiscover();
  const typedAgentMeta = agentMeta as NodeMeta;
  const autoDiscoverEnabled = !!typedAgentMeta.autoDiscovery;

  const { attempt: fetchEc2IceAttempt, setAttempt: setFetchEc2IceAttempt } =
    useAttempt('');
  const { attempt: createNodeAttempt, setAttempt: setCreateNodeAttempt } =
    useAttempt('');

  // When the EICE's state is 'create-complete', create the node.
  useEffect(() => {
    // Auto discovery will automatically create the discovered
    // nodes in the backend.
    if (autoDiscoverEnabled) return;

    if (typedAgentMeta.ec2Ices?.every(e => e.state === 'create-complete')) {
      createNode();
    }
  }, [typedAgentMeta.ec2Ices]);

  let ec2Ices = usePoll<Ec2InstanceConnectEndpoint[]>(
    () =>
      fetchEc2InstanceConnectEndpoints().then(endpoints => {
        if (endpoints?.every(e => e.state === 'create-complete')) {
          setIsPollingActive(false);
          updateAgentMeta({
            ...typedAgentMeta,
            ec2Ices: endpoints,
          });
        }
        return endpoints;
      }),
    isPollingActive,
    10000 // poll every 10 seconds
  );

  // If the EICE already existed from the previous step and was create-complete, we set
  // `ec2Ice` to it.
  if (existingEices?.every(e => e.state === 'create-complete')) {
    ec2Ices = existingEices;
  }

  async function fetchEc2InstanceConnectEndpoints() {
    let vpcIds: string[] = [];
    if (autoDiscoverEnabled) {
      const requiredVpcs = Object.keys(
        typedAgentMeta.autoDiscovery.requiredVpcsAndSubnets
      );
      const inprogressExistingEndpoints =
        typedAgentMeta.ec2Ices
          ?.filter(e => e.state === 'create-in-progress')
          .map(e => e.vpcId) ?? [];
      vpcIds = [...requiredVpcs, ...inprogressExistingEndpoints];
    } else {
      vpcIds = [typedAgentMeta.node.awsMetadata.vpcId];
    }

    setFetchEc2IceAttempt({ status: 'processing' });
    try {
      const resp = await integrationService.fetchAwsEc2InstanceConnectEndpoints(
        typedAgentMeta.awsIntegration.name,
        {
          region: typedAgentMeta.awsRegion,
          vpcIds,
        }
      );

      setMainDashboardLink(resp.dashboardLink);
      setFetchEc2IceAttempt({ status: 'success' });

      const endpoints = resp.endpoints.filter(
        e =>
          e.state === 'create-complete' ||
          e.state === 'create-in-progress' ||
          e.state === 'create-failed'
      );

      if (endpoints.length > 0) {
        return endpoints;
      }
    } catch {
      // eslint-disable-next-line no-empty
      // Ignore any errors, as the poller will keep re-trying.
    }
  }

  async function createNode() {
    setCreateNodeAttempt({ status: 'processing' });
    try {
      const node = await nodeService.createNode(cfg.proxyCluster, {
        hostname: typedAgentMeta.node.hostname,
        addr: typedAgentMeta.node.addr,
        labels: typedAgentMeta.node.labels,
        aws: typedAgentMeta.node.awsMetadata,
        name: typedAgentMeta.node.id,
        subKind: 'openssh-ec2-ice',
      });

      updateAgentMeta({
        ...typedAgentMeta,
        node,
        resourceName: node.id,
      });
      setCreateNodeAttempt({ status: 'success' });

      // Capture event for creating the Node.
      emitEvent(
        { stepStatus: DiscoverEventStatus.Success },
        {
          eventName: DiscoverEvent.CreateNode,
        }
      );
    } catch (err) {
      const errMsg = getErrMessage(err);
      setCreateNodeAttempt({ status: 'failed', statusText: errMsg });
      setIsPollingActive(false);
      emitErrorEvent(`error creating teleport node: ${errMsg}`);
    }
  }

  const endpointsCreated = ec2Ices?.every(e => e.state === 'create-complete');

  let content: JSX.Element;
  if (
    fetchEc2IceAttempt.status === 'failed' ||
    createNodeAttempt.status === 'failed'
  ) {
    content = (
      <>
        <Flex mb={5} alignItems="center">
          {' '}
          <Icons.Warning size="large" ml={1} mr={2} color="error.main" />
          <Text>
            {fetchEc2IceAttempt.status === 'failed'
              ? fetchEc2IceAttempt.statusText
              : createNodeAttempt.statusText}
          </Text>
        </Flex>
        <Flex>
          {!!retry && (
            <ButtonPrimary mr={3} width="50%" onClick={retry}>
              Retry
            </ButtonPrimary>
          )}
        </Flex>
      </>
    );
  } else {
    if (ec2Ices?.some(e => e.state === 'create-failed')) {
      content = (
        <>
          <AnimatedProgressBar mb={1} />
          <TextIcon mt={2} mb={3}>
            <Icons.Warning size="large" ml={1} mr={2} color="warning.main" />
            <Box
              css={`
                text-align: center;
              `}
            >
              We couldn't create some EC2 Instance Connect Endpoints.
              <br />
              Please visit your{' '}
              <Link color="text.main" href={mainDashboardLink} target="_blank">
                dashboard{' '}
              </Link>
              to troubleshoot.
              <br />
              We'll keep looking for the endpoint until it becomes available.
            </Box>
          </TextIcon>
          <ButtonPrimary width="100%" disabled>
            Next
          </ButtonPrimary>
        </>
      );
    } else if (createNodeAttempt.status === 'success' && endpointsCreated) {
      content = (
        <>
          <EndpointSuccessfullyDeployed existingEices={existingEices} />
          <Text
            mb={5}
            style={{ display: 'flex', textAlign: 'left', width: '100%' }}
          >
            <Icons.Check size="small" ml={1} mr={2} color="success.main" />
            The EC2 instance [{typedAgentMeta?.node.awsMetadata.instanceId}] has
            been added to Teleport.
          </Text>
          <ButtonPrimary width="100%" onClick={() => nextStep()}>
            Next
          </ButtonPrimary>
        </>
      );
    } else if (autoDiscoverEnabled && endpointsCreated) {
      content = (
        <>
          <EndpointSuccessfullyDeployed existingEices={existingEices} />
          <Flex mb={5}>
            <Icons.Check size="small" ml={1} mr={2} color="success.main" />
            <Text>
              All endpoints required are created. The discovery service can take
              a few minutes to finish auto-enrolling resources found in region{' '}
              <Mark>{typedAgentMeta.awsRegion}</Mark>.
            </Text>
          </Flex>
          <ButtonPrimary width="100%" onClick={() => nextStep()}>
            Next
          </ButtonPrimary>
        </>
      );
    } else {
      content = (
        <>
          <AnimatedProgressBar mb={1} />
          <Flex mb={3} flexDirection="column" alignItems="center">
            <TextIcon
              mt={2}
              css={`
                white-space: pre;
              `}
            >
              <Icons.Clock size="medium" />
              This may take a few minutes..
            </TextIcon>
            {!endpointsCreated && mainDashboardLink && (
              <Text>
                Meanwhile, visit your{' '}
                <Link
                  color="text.main"
                  href={mainDashboardLink}
                  target="_blank"
                >
                  dashboard
                </Link>{' '}
                to view the status of{' '}
                {autoDiscoverEnabled ? 'each endpoint' : 'this endpoint'}
              </Text>
            )}
          </Flex>
          <ButtonPrimary width="100%" disabled>
            Next
          </ButtonPrimary>
        </>
      );
    }
  }

  let title = 'Creating EC2 Instance Connect Endpoints';

  if (!autoDiscoverEnabled && endpointsCreated) {
    if (createNodeAttempt.status === 'success') {
      title = 'Created Teleport Node';
    } else {
      title = 'Creating Teleport Node';
    }
  }

  return (
    <Dialog disableEscapeKeyDown={false} open={true}>
      <DialogContent
        width="460px"
        alignItems="center"
        mb={0}
        textAlign="center"
      >
        <Text bold caps mb={4}>
          {title}
        </Text>
        {content}
      </DialogContent>
    </Dialog>
  );
}

export type CreateEc2IceDialogProps = {
  ec2Ice: Ec2InstanceConnectEndpoint;
  fetchEc2IceAttempt: Attempt;
  createNodeAttempt: Attempt;
  retry: () => void;
  next: () => void;
};

function EndpointSuccessfullyDeployed({
  existingEices,
}: {
  existingEices: Ec2InstanceConnectEndpoint[];
}) {
  // Don't show this message if the EICE had already been deployed before this step.
  if (!existingEices?.every(e => e.state === 'create-complete')) {
    return (
      <Text
        mb={2}
        style={{ display: 'flex', textAlign: 'left', width: '100%' }}
      >
        <Icons.Check size="small" ml={1} mr={2} color="success.main" />
        The EC2 Instance Connect Endpoints are successfully deployed.
      </Text>
    );
  }
}
