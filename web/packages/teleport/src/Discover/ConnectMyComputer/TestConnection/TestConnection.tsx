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

import React from 'react';

import { Flex, Text } from 'design';

import { ActionButtons, StyledBox, Header } from 'teleport/Discover/Shared';
import { NodeConnect } from 'teleport/UnifiedResources/ResourceActionButton';

import { NodeMeta } from '../../useDiscover';

import type { AgentStepProps } from '../../types';

export const TestConnection = (props: AgentStepProps) => {
  const meta = props.agentMeta as NodeMeta;

  return (
    <Flex flexDirection="column" alignItems="flex-start" mb={2} gap={4}>
      <div>
        <Header>Start a Session</Header>
      </div>

      <StyledBox>
        <Text bold>Step 1: Connect to Your Computer</Text>
        <Text typography="subtitle1" mb={2}>
          Optionally verify that you can connect to &ldquo;{meta.resourceName}
          &rdquo; by starting a session.
        </Text>
        <NodeConnect node={meta.node} textTransform="uppercase" />
      </StyledBox>

      <ActionButtons
        onProceed={props.nextStep}
        lastStep={true}
        onPrev={props.prevStep}
      />
    </Flex>
  );
};
