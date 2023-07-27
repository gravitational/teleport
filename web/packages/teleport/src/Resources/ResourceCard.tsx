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
import styled from 'styled-components';

import { Box, ButtonBorder, Flex, Label, Text } from 'design';

import { CheckboxInput } from 'design/Checkbox';
import { ResourceIcon, ResourceIconName } from 'design/ResourceIcon';
import {
  ApplicationsIcon,
  DatabasesIcon,
  DesktopsIcon,
  KubernetesIcon,
  ServersIcon,
} from 'design/SVGIcon';

import { AgentKind, UnifiedResourceKind } from 'teleport/services/agents';

const SingleLineBox = styled(Box)`
  overflow: hidden;
  white-space: nowrap;
`;

type Props = {
  resource: AgentKind;
};
export const ResourceCard = ({ resource }: Props) => {
  const name = agentName(resource);
  const resIcon = agentIconName(resource);
  const ResTypeIcon = agentTypeIcon(resource.kind);
  const description = agentDescription(resource);
  return (
    <CardContainer p={3} alignItems="start">
      <CheckboxInput type="checkbox" mx={0}></CheckboxInput>
      <ResourceIcon
        alignSelf="center"
        name={resIcon}
        width="60px"
        height="60px"
        ml={2}
      />
      {/* MinWidth is important to prevent descriptions from overflowing. */}
      <Flex flexDirection="column" flex="1" minWidth="0" ml={3} gap={1}>
        <Flex flexDirection="row" alignItems="start">
          <SingleLineBox flex="1" title={name}>
            <Text typography="h5">{name}</Text>
          </SingleLineBox>
          <ButtonBorder size="small">Connect</ButtonBorder>
        </Flex>
        <Flex flexDirection="row">
          {/* This box prevents the icon from being squeezed if the flexbox starts shrinking items. */}
          <Box>
            <ResTypeIcon size={18} />
          </Box>
          {description.primary && (
            <SingleLineBox ml={1} title={description.primary}>
              <Text typography="body2" color="text.slightlyMuted">
                {description.primary}
              </Text>
            </SingleLineBox>
          )}
          {description.secondary && (
            <SingleLineBox ml={2} title={description.secondary}>
              <Text typography="body2" color="text.muted">
                {description.secondary}
              </Text>
            </SingleLineBox>
          )}
        </Flex>
        <div>
          {resource.labels.map(({ name, value }) => (
            <Label kind="secondary" mr={1}>{`${name}: ${value}`}</Label>
          ))}
        </div>
      </Flex>
    </CardContainer>
  );
};

function agentName(agent: AgentKind) {
  return agent.kind === 'node' ? agent.hostname : agent.name;
}

function agentDescription(agent: AgentKind) {
  switch (agent.kind) {
    case 'app':
      return { primary: agent.addrWithProtocol, secondary: agent.description };
    case 'db':
      return { primary: agent.type, secondary: agent.description };
    case 'kube_cluster':
      return { primary: 'Kubernetes' };
    case 'node':
      // TODO: Pass the subkind to display as the primary and push addr to
      // secondary.
      return { primary: agent.addr };
    case 'windows_desktop':
      return { primary: 'Windows', secondary: agent.addr };

    default:
      return {};
  }
}

function agentIconName(agent: AgentKind): ResourceIconName {
  switch (agent.kind) {
    case 'app':
      return 'Application';
    case 'db':
      // agent.
      return 'Database';
    case 'kube_cluster':
      return 'Kube';
    case 'node':
      return 'Server';
    case 'windows_desktop':
      return 'Windows';

    default:
      return 'Server';
  }
}

function agentTypeIcon(kind: UnifiedResourceKind) {
  switch (kind) {
    case 'app':
      return ApplicationsIcon;
    case 'db':
      return DatabasesIcon;
    case 'kube_cluster':
      return KubernetesIcon;
    case 'node':
      return ServersIcon;
    case 'windows_desktop':
      return DesktopsIcon;

    default:
      return ServersIcon;
  }
}

export const CardContainer = styled(Flex)`
  border-top: 2px solid ${props => props.theme.colors.spotBackground[0]};

  @media (min-width: ${props => props.theme.breakpoints.tablet}px) {
    border: ${props => props.theme.borders[2]}
      ${props => props.theme.colors.spotBackground[0]};
    border-radius: ${props => props.theme.radii[3]}px;
  }
`;
