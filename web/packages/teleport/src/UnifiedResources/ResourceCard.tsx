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

import { UnifiedResource, UnifiedResourceKind } from 'teleport/services/agents';

const SingleLineBox = styled(Box)`
  overflow: hidden;
  white-space: nowrap;
`;

const TruncatingLabel = styled(Label)`
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
`;

/**
 * This box serves twofold purpose: first, it prevents the underlying icon from
 * being squeezed if the parent flexbox starts shrinking items. Second, it
 * prevents the icon from magically occupying too much space, since the SVG
 * element somehow forces the parent to occupy at least full line height.
 */
const ResTypeIconBox = styled(Box)`
  line-height: 0;
`;

type Props = {
  resource: UnifiedResource;
};
export const ResourceCard = ({ resource }: Props) => {
  const name = resourceName(resource);
  const resIcon = resourceIconName(resource);
  const ResTypeIcon = resourceTypeIcon(resource.kind);
  const description = resourceDescription(resource);
  return (
    <CardContainer p={3} alignItems="start">
      <CheckboxInput type="checkbox" mx={0}></CheckboxInput>
      <ResourceIcon
        alignSelf="center"
        name={resIcon}
        width="45px"
        height="45px"
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
        <Flex flexDirection="row" alignItems="center">
          <ResTypeIconBox>
            <ResTypeIcon size={18} />
          </ResTypeIconBox>
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
        <Flex gap={1}>
          {resource.labels.map(({ name, value }) => {
            const label = `${name}: ${value}`;
            return (
              <TruncatingLabel key={label} title={label} kind="secondary">
                {label}
              </TruncatingLabel>
            );
          })}
        </Flex>
      </Flex>
    </CardContainer>
  );
};

function resourceName(resource: UnifiedResource) {
  return resource.kind === 'node' ? resource.hostname : resource.name;
}

function resourceDescription(resource: UnifiedResource) {
  switch (resource.kind) {
    case 'app':
      return {
        primary: resource.addrWithProtocol,
        secondary: resource.description,
      };
    case 'db':
      return { primary: resource.type, secondary: resource.description };
    case 'kube_cluster':
      return { primary: 'Kubernetes' };
    case 'node':
      // TODO(bl-nero): Pass the subkind to display as the primary and push addr
      // to secondary.
      return { primary: resource.addr };
    case 'windows_desktop':
      return { primary: 'Windows', secondary: resource.addr };

    default:
      return {};
  }
}

function resourceIconName(resource: UnifiedResource): ResourceIconName {
  switch (resource.kind) {
    case 'app':
      return 'Application';
    case 'db':
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

function resourceTypeIcon(kind: UnifiedResourceKind) {
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
