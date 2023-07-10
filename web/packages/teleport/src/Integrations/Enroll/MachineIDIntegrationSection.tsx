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

import {
  AnsibleIcon,
  CircleCIIcon,
  GitHubIcon,
  GitLabIcon,
  JenkinsIcon,
  ServersIcon,
} from 'design/SVGIcon';
import { Box, Flex, Link as ExternalLink, Text } from 'design';
import React from 'react';

import {
  IntegrationEnrollEvent,
  IntegrationEnrollKind,
  userEventService,
} from 'teleport/services/userEvent';

import { IntegrationTile } from './common';

interface Integration {
  title: string;
  link: string;
  icon: JSX.Element;
  kind: IntegrationEnrollKind;
}

const integrations: Integration[] = [
  {
    title: 'GitHub Actions',
    link: 'https://goteleport.com/docs/machine-id/guides/github-actions/',
    icon: <GitHubIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDGitHubActions,
  },
  {
    title: 'CircleCI',
    link: 'https://goteleport.com/docs/machine-id/guides/circleci/',
    icon: <CircleCIIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDCircleCI,
  },
  {
    title: 'GitLab CI/CD',
    link: 'https://goteleport.com/docs/machine-id/guides/gitlab/',
    icon: <GitLabIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDGitLab,
  },
  {
    title: 'Jenkins',
    link: 'https://goteleport.com/docs/machine-id/guides/jenkins/',
    icon: <JenkinsIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDJenkins,
  },
  {
    title: 'Ansible',
    link: 'https://goteleport.com/docs/machine-id/guides/ansible/',
    icon: <AnsibleIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDAnsible,
  },
  {
    title: 'Generic',
    link: 'https://goteleport.com/docs/machine-id/getting-started/',
    icon: <ServersIcon size={80} />,
    kind: IntegrationEnrollKind.MachineID,
  },
];

export const MachineIDIntegrationSection = () => {
  return (
    <>
      <Box mb={3}>
        <Text fontWeight="bold" typography="h4">
          Machine ID
        </Text>
        <Text typography="body1">
          Set up Teleport Machine ID to allow CI/CD workflows and other machines
          to access resources protected by Teleport.
        </Text>
      </Box>
      <Flex mb={2} gap={3}>
        {integrations.map(i => {
          return (
            <IntegrationTile
              key={i.title}
              as={ExternalLink}
              href={i.link}
              target="_blank"
              onClick={() => {
                userEventService.captureIntegrationEnrollEvent({
                  event: IntegrationEnrollEvent.Started,
                  eventData: {
                    id: crypto.randomUUID(),
                    kind: i.kind,
                  },
                });
              }}
            >
              <Box mt={3} mb={2}>
                {i.icon}
              </Box>
              <Text>{i.title}</Text>
            </IntegrationTile>
          );
        })}
      </Flex>
    </>
  );
};
