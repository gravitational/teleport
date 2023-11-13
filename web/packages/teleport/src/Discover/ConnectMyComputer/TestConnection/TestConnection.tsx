/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useEffect, useCallback, useRef } from 'react';

import { ButtonPrimary, Flex, Indicator, Text } from 'design';
import * as Icons from 'design/Icon';
import { useAsync } from 'shared/hooks/useAsync';

import useTeleport from 'teleport/useTeleport';
import {
  ActionButtons,
  StyledBox,
  Header,
  TextIcon,
} from 'teleport/Discover/Shared';
import { NodeConnect } from 'teleport/UnifiedResources/ResourceActionButton';

import { NodeMeta } from '../../useDiscover';

import type { AgentStepProps } from '../../types';

export const TestConnection = (props: AgentStepProps) => {
  const { userService, nodeService, storeUser } = useTeleport();
  const clusterId = storeUser.getClusterId();
  const meta = props.agentMeta as NodeMeta;

  const abortController = useRef<AbortController>();
  // When the user sets up Connect My Computer in Teleport Connect, a new role gets added to the
  // user. Because of that, we need to reload the current session so that the user is able to
  // connect to the new node, without having to log in to the cluster again.
  //
  // We also need to refetch the node so that it includes any new logins.
  const [refetchNodeAttempt, refetchNode] = useAsync(
    useCallback(
      async (signal: AbortSignal) => {
        await userService.reloadUser(signal);

        const response = await nodeService.fetchNodes(
          clusterId,
          { search: meta.node.id, limit: 1 },
          signal
        );

        if (response.agents.length === 0) {
          throw new Error('Could not find the Connect My Computer node');
        }

        if (response.agents.length > 1) {
          throw new Error(
            'Found multiple nodes matching the ID of the Connect My Computer node'
          );
        }

        return response.agents[0];
      },
      [userService, nodeService, clusterId, meta.node.id]
    )
  );

  useEffect(() => {
    abortController.current = new AbortController();

    refetchNode(abortController.current.signal);

    return () => {
      abortController.current.abort();
    };
  }, []);

  return (
    <Flex flexDirection="column" alignItems="flex-start" mb={2} gap={4}>
      <div>
        <Header>Start a Session</Header>
      </div>

      <StyledBox>
        <Text bold>Step 1: Connect to Your Computer</Text>
        <Text typography="subtitle1" mb={2}>
          Optionally verify that you can connect to &ldquo;
          {meta.resourceName}
          &rdquo; by starting a session.
        </Text>
        {refetchNodeAttempt.status === '' ||
          (refetchNodeAttempt.status === 'processing' && <Indicator />)}

        {refetchNodeAttempt.status === 'error' && (
          <>
            <TextIcon mt={2} mb={3}>
              <Icons.Warning size="medium" ml={1} mr={2} color="error.main" />
              Encountered Error: {refetchNodeAttempt.statusText}
            </TextIcon>

            <ButtonPrimary
              type="button"
              onClick={() => refetchNode(abortController.current.signal)}
            >
              Retry
            </ButtonPrimary>
          </>
        )}

        {refetchNodeAttempt.status === 'success' && (
          <NodeConnect
            node={refetchNodeAttempt.data}
            textTransform="uppercase"
          />
        )}
      </StyledBox>

      <ActionButtons
        onProceed={props.nextStep}
        disableProceed={refetchNodeAttempt.status !== 'success'}
        lastStep={true}
        // onPrev is not passed on purpose to disable the back button. The flow would go back to
        // polling which wouldn't make sense as the user has already connected their computer so the
        // step would poll forever, unless the user removed the agent and configured it again.
      />
    </Flex>
  );
};
