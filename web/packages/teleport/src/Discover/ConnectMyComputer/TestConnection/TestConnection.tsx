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
  const { userService } = useTeleport();
  const abortController = useRef<AbortController>();
  const [reloadUserAttempt, reloadUser] = useAsync(
    useCallback(
      (signal: AbortSignal) => userService.reloadUser(signal),
      [userService]
    )
  );

  // When the user sets up Connect My Computer in Teleport Connect, a new role gets added to the
  // user. Because of that, we need to reload the current session so that the user is able to
  // connect to the new node, without having to log in to the cluster again.
  useEffect(() => {
    abortController.current = new AbortController();

    reloadUser(abortController.current.signal);

    return () => {
      abortController.current.abort();
    };
  }, []);

  const meta = props.agentMeta as NodeMeta;

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
        {reloadUserAttempt.status === '' ||
          (reloadUserAttempt.status === 'processing' && <Indicator />)}

        {reloadUserAttempt.status === 'error' && (
          <>
            <TextIcon mt={2} mb={3}>
              <Icons.Warning size="medium" ml={1} mr={2} color="error.main" />
              Encountered Error: {reloadUserAttempt.statusText}
            </TextIcon>

            <ButtonPrimary
              type="button"
              onClick={() => reloadUser(abortController.current.signal)}
            >
              Retry
            </ButtonPrimary>
          </>
        )}

        {reloadUserAttempt.status === 'success' && (
          <NodeConnect node={meta.node} textTransform="uppercase" />
        )}
      </StyledBox>

      <ActionButtons
        onProceed={props.nextStep}
        disableProceed={reloadUserAttempt.status !== 'success'}
        lastStep={true}
        onPrev={props.prevStep}
      />
    </Flex>
  );
};
