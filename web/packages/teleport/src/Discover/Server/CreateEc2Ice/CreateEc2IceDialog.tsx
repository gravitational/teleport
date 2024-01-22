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

import {
  Ec2InstanceConnectEndpoint,
  integrationService,
} from 'teleport/services/integrations';
import NodeService from 'teleport/services/nodes';
import { TextIcon } from 'teleport/Discover/Shared';
import { NodeMeta, useDiscover } from 'teleport/Discover/useDiscover';
import { usePoll } from 'teleport/Discover/Shared/usePoll';
import {
  DiscoverEvent,
  DiscoverEventStatus,
} from 'teleport/services/userEvent';

export function CreateEc2IceDialog({
  nextStep,
  retry,
  existingEice,
}: {
  nextStep: () => void;
  retry?: () => void;
  existingEice?: Ec2InstanceConnectEndpoint;
}) {
  // If the EICE already exists from the previous step and is create-complete, we don't need to do any polling for the EICE.
  const [isPollingActive, setIsPollingActive] = useState(
    existingEice?.state !== 'create-complete'
  );

  const { emitErrorEvent, updateAgentMeta, agentMeta, emitEvent } =
    useDiscover();
  const typedAgentMeta = agentMeta as NodeMeta;

  const nodeService = new NodeService();

  const { attempt: fetchEc2IceAttempt, setAttempt: setFetchEc2IceAttempt } =
    useAttempt('');
  const { attempt: createNodeAttempt, setAttempt: setCreateNodeAttempt } =
    useAttempt('');

  // When the EICE's state is 'create-complete', create the node.
  useEffect(() => {
    if (typedAgentMeta.ec2Ice?.state === 'create-complete') {
      createNode();
    }
  }, [typedAgentMeta.ec2Ice]);

  let ec2Ice = usePoll<Ec2InstanceConnectEndpoint>(
    () =>
      fetchEc2InstanceConnectEndpoint().then(e => {
        if (e?.state === 'create-complete') {
          setIsPollingActive(false);
          updateAgentMeta({
            ...typedAgentMeta,
            ec2Ice: e,
          });
        }
        return e;
      }),
    isPollingActive,
    10000 // poll every 10 seconds
  );

  // If the EICE already existed from the previous step and was create-complete, we set
  // `ec2Ice` to it.
  if (existingEice?.state === 'create-complete') {
    ec2Ice = existingEice;
  }

  async function fetchEc2InstanceConnectEndpoint() {
    const integration = typedAgentMeta.awsIntegration;

    setFetchEc2IceAttempt({ status: 'processing' });
    try {
      const { endpoints: fetchedEc2Ices } =
        await integrationService.fetchAwsEc2InstanceConnectEndpoints(
          integration.name,
          {
            region: typedAgentMeta.node.awsMetadata.region,
            vpcId: typedAgentMeta.node.awsMetadata.vpcId,
          }
        );

      setFetchEc2IceAttempt({ status: 'success' });

      const createCompleteEice = fetchedEc2Ices.find(
        e => e.state === 'create-complete'
      );
      if (createCompleteEice) {
        return createCompleteEice;
      }

      const createInProgressEice = fetchedEc2Ices.find(
        e => e.state === 'create-in-progress'
      );
      if (createInProgressEice) {
        return createInProgressEice;
      }

      const createFailedEice = fetchedEc2Ices.find(
        e => e.state === 'create-failed'
      );
      if (createFailedEice) {
        return createFailedEice;
      }
    } catch (err) {
      const errMsg = getErrMessage(err);
      setFetchEc2IceAttempt({ status: 'failed', statusText: errMsg });
      setIsPollingActive(false);
      emitErrorEvent(`ec2 instance connect endpoint fetch error: ${errMsg}`);
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
    if (ec2Ice?.state === 'create-failed') {
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
              We couldn't create the EC2 Instance Connect Endpoint.
              <br />
              Please visit your{' '}
              <Link
                color="text.main"
                href={ec2Ice?.dashboardLink}
                target="_blank"
              >
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
    } else if (
      ec2Ice?.state === 'create-complete' &&
      createNodeAttempt.status === 'success'
    ) {
      content = (
        <>
          {/* Don't show this message if the EICE had already been deployed before this step. */}
          {!(existingEice?.state === 'create-complete') && (
            <Text
              mb={2}
              style={{ display: 'flex', textAlign: 'left', width: '100%' }}
            >
              <Icons.Check size="small" ml={1} mr={2} color="success.main" />
              The EC2 Instance Connect Endpoint was successfully deployed.
            </Text>
          )}
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
    } else {
      content = (
        <>
          <AnimatedProgressBar mb={1} />
          <TextIcon
            mt={2}
            mb={3}
            css={`
              white-space: pre;
            `}
          >
            <Icons.Clock size="medium" />
            This may take a few minutes..
          </TextIcon>
          <ButtonPrimary width="100%" disabled>
            Next
          </ButtonPrimary>
        </>
      );
    }
  }

  let title = 'Creating EC2 Instance Connect Endpoint';

  if (ec2Ice?.state === 'create-complete') {
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
