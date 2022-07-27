/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { ButtonPrimary, ButtonSecondary, Text, Box } from 'design';

import { LoginTrait } from './LoginTrait';

import { DownloadScript } from './DownloadScript';

import type { AgentStepProps, AgentStepComponent } from './types';
import type { AgentKind } from './useDiscover';

// agentStepTitles defines the titles per steps defined by enum `AgentStep`.
//
// We use the enum `AgentStep` numerical values to access the list's value,
// so this list's order and length must be equal to the enum.
export const agentStepTitles: string[] = [
  'Select Resource Type',
  'Configure Resource',
  'Configure Role',
  'Test Connection',
];

// agentViews defines the list of components required per AgentKind per steps
// defined by enum `AgentStep`.
//
// We use the enum `AgentStep` numerical values to access the list's value,
// so the list's order and length must be equal to the enum.
export const agentViews: Record<AgentKind, AgentStepComponent[]> = {
  app: [GatherReqs, InstallTeleport, LoginTrait, RoleConfig],
  db: [],
  desktop: [],
  kube: [],
  node: [GatherReqsNode, DownloadScript, LoginTrait, InstallTeleport],
};

function GatherReqs(props: AgentStepProps) {
  return (
    <Box mt={5}>
      <Text mb={2} bold>
        General Requirement Gathering
      </Text>
      <Box width={500}>
        Lorem ipsum dolor sit amet, consectetur adipiscing elit. Curabitur
        lacinia tellus non velit porttitor, congue pretium quam luctus. Quisque
        varius mi non purus consectetur, et dignissim odio cursus. Cras
        tristique quis urna eget vehicula. Sed vehicula aliquam magna, in rutrum
        nisl. Maecenas velit nisi, aliquam nec felis sed, ultricies euismod
        tellus. Ut vel sem eget nisi tristique ullamcorper vel eget augue. Sed
        imperdiet volutpat nisi vel mollis. Duis pulvinar, mauris sit amet
        euismod dapibus, nibh felis dapibus ipsum, sit amet facilisis velit
        nulla non risus. Aenean varius quam nulla. Sed a sem sagittis, gravida
        ipsum quis, fringilla turpis.
      </Box>
      <ButtonPrimary width="182px" onClick={props.nextStep} mt={3}>
        Proceed
      </ButtonPrimary>
    </Box>
  );
}

function GatherReqsNode(props: AgentStepProps) {
  return (
    <Box mt={5}>
      <Text mb={2} bold>
        General Requirement Gathering Specific to Nodes
      </Text>
      <Box width={500}>Node stuff</Box>
      <ButtonPrimary width="182px" onClick={props.nextStep} mt={3}>
        Proceed
      </ButtonPrimary>
    </Box>
  );
}

function InstallTeleport(props: AgentStepProps) {
  return (
    <Box mt={5}>
      <Text mb={2} bold>
        General Teleport Installation
      </Text>
      <Box width={500}>
        Lorem ipsum dolor sit amet, consectetur adipiscing elit. Curabitur
        lacinia tellus non velit porttitor, congue pretium quam luctus.
      </Box>
      <ButtonSecondary mr={3} width="165px" onClick={props.prevStep} mt={3}>
        Back
      </ButtonSecondary>
      <ButtonPrimary width="182px" onClick={props.nextStep} mt={3}>
        Proceed
      </ButtonPrimary>
    </Box>
  );
}

function RoleConfig(props: AgentStepProps) {
  return (
    <Box mt={5}>
      <Text mb={2} bold>
        General Agent Installation
      </Text>
      <Box width={500}>
        Lorem ipsum dolor sit amet, consectetur adipiscing elit. Curabitur
        lacinia tellus non velit porttitor, congue pretium quam luctus. Quisque
        varius mi non purus consectetur, et dignissim odio cursus. Cras
        tristique quis urna eget vehicula. Sed vehicula aliquam magna, in rutrum
        nisl. Maecenas velit nisi, aliquam nec felis sed, ultricies euismod
        tellus. Ut vel sem eget nisi tristique ullamcorper vel eget augue. Sed
        imperdiet volutpat nisi vel mollis. Duis pulvinar, mauris sit amet
        euismod dapibus, nibh felis dapibus ipsum, sit amet facilisis velit
        nulla non risus. Aenean varius quam nulla. Sed a sem sagittis, gravida
        ipsum quis, fringilla turpis.
      </Box>
      <Box width={500}>
        Lorem ipsum dolor sit amet, consectetur adipiscing elit. Curabitur
        lacinia tellus non velit porttitor, congue pretium quam luctus. Quisque
        varius mi non purus consectetur, et dignissim odio cursus.
      </Box>
      <ButtonSecondary mr={3} width="165px" onClick={props.prevStep}>
        Back
      </ButtonSecondary>
    </Box>
  );
}
