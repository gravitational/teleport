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

import { Flex } from 'design';

import { Header } from 'teleport/Discover/Shared';

import { NodeMeta } from '../../useDiscover';

import type { AgentStepProps } from '../../types';

export const TestConnection = (props: AgentStepProps) => {
  const meta = props.agentMeta as NodeMeta;
  return (
    <Flex flexDirection="column" alignItems="flex-start" mb={2} gap={4}>
      <div>
        <Header>Test Connection to &ldquo;{meta.node.hostname}&rdquo;</Header>
      </div>
    </Flex>
  );
};
