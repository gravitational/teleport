/**
 * Copyright 2023 Gravitational, Inc.
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
import {Box, Flex, Link as ExternalLink, Text} from 'design';
import styled from 'styled-components';
import {
  AnsibleIcon,
  AWSIcon, CircleCIIcon,
  GitHubIcon, GitLabIcon, JenkinsIcon,
  ServerIcon,
  ServersIcon
} from "design/SVGIcon";

export const IntegrationTile = styled(Flex)`
  color: inherit;
  text-decoration: none;
  flex-direction: column;
  align-items: center;
  position: relative;
  border-radius: 4px;
  height: 170px;
  width: 170px;
  background-color: ${({theme}) => theme.colors.buttons.secondary.default};
  text-align: center;
  cursor: pointer;

  ${props => {
  const pointerEvents = props.disabled ? 'none' : null;
  if (props.$exists) {
    return {pointerEvents};
  }

  return `
    opacity: ${props.disabled ? '0.45' : '1'};
    &:hover {
      background-color: ${props.theme.colors.buttons.secondary.hover};
    }
    `;
}}
`;

export const NoCodeIntegrationDescription = () => (
  <Box mb={3}>
    <Text fontWeight="bold" typography="h4">
      No-Code Integrations
    </Text>
    <Text typography="body1">
      Set up Teleport to post notifications to messaging apps, discover and
      import resources from cloud providers and other services.
    </Text>
  </Box>
);

export const MachineIDIntegrationSection = () => {
  interface tile {
    title: string
    link: string
    icon: JSX.Element
  }
  const tiles: tile[] = [
    // TODO: Emit an event or pass these through a /r/ url to redirect to a
    // URL with tracking.
    // Can we emit events in OSS and have them go nowhere?? Is it best to use
    // a /r/ url and have support on both Enterprise and OSS.
    {
      title: 'GitHub Actions',
      link: 'https://goteleport.com/docs/machine-id/guides/github-actions/',
      icon: <GitHubIcon size={80}/>,
    },
    {
      title: 'CircleCI',
      link: 'https://goteleport.com/docs/machine-id/guides/circleci/',
      icon: <CircleCIIcon size={80}/>,
    },
    {
      title: 'GitLab CI',
      link: 'https://goteleport.com/docs/machine-id/guides/gitlab/',
      icon: <GitLabIcon size={80}/>,
    },
    {
      title: 'Jenkins',
      link: 'https://goteleport.com/docs/machine-id/guides/jenkins/',
      icon: <JenkinsIcon size={80}/>,
    },
    {
      title: 'Ansible',
      link: 'https://goteleport.com/docs/machine-id/guides/ansible/',
      icon: <AnsibleIcon size={80}/>,
    },
    {
      title: 'Generic',
      link: 'https://goteleport.com/docs/machine-id/getting-started/',
      icon: <ServersIcon size={80}/>,
    }
  ]

  const propsForTile = (t: tile) => {
    return {
      as: ExternalLink,
      href: t.link,
      target: '_blank',
    }
  }

  return (<>
  <Box mb={3}>
    <Text fontWeight="bold" typography="h4">
      Machine ID
    </Text>
    <Text typography="body1">
      Set up Teleport Machine ID to allow CI/CD workflows and other machines to access resources protected by Teleport.
    </Text>
  </Box>
    <Flex mb={2} gap={3}>
    {
      tiles.map((t: tile) => {
        return (<>
          <IntegrationTile
            {...propsForTile(t)}
          >
            <Box mt={3} mb={2}>
              {t.icon}
            </Box>
            <Text>
              {t.title}
            </Text>
          </IntegrationTile>
        </>)
      })
    }
    </Flex>
</>)
}