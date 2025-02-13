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

import {
  AnsibleIcon,
  AWSIcon,
  AzureIcon,
  CircleCIIcon,
  GCPIcon,
  GitHubIcon,
  GitLabIcon,
  JenkinsIcon,
  KubernetesIcon,
  ServersIcon,
  SpaceliftIcon,
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
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/github-actions/',
    icon: <GitHubIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDGitHubActions,
  },
  {
    title: 'CircleCI',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/circleci/',
    icon: <CircleCIIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDCircleCI,
  },
  {
    title: 'GitLab CI/CD',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/gitlab/',
    icon: <GitLabIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDGitLab,
  },
  {
    title: 'Jenkins',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/jenkins/',
    icon: <JenkinsIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDJenkins,
  },
  {
    title: 'Ansible',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/access-guides/ansible/',
    icon: <AnsibleIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDAnsible,
  },
  {
    title: 'Spacelift',
    link: 'https://goteleport.com/docs/admin-guides/infrastructure-as-code/terraform-provider/spacelift/',
    icon: <SpaceliftIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDSpacelift,
  },
  {
    title: 'AWS',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/aws/',
    icon: <AWSIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDAWS,
  },
  {
    title: 'GCP',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/gcp/',
    icon: <GCPIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDGCP,
  },
  {
    title: 'Azure',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/azure/',
    icon: <AzureIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDAzure,
  },
  {
    title: 'Kubernetes',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/kubernetes/',
    icon: <KubernetesIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDKubernetes,
  },
  {
    title: 'Generic',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/getting-started/',
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
      {/* TODO(mcbattirola): replace this section with BotTiles and remove Integrations */}
      <Flex mb={2} gap={3} flexWrap="wrap">
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
