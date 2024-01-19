import React from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';

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

import cfg from 'teleport/config';
import { BotType } from '../types';
import {
  IntegrationEnrollEvent,
  IntegrationEnrollKind,
  userEventService,
} from 'teleport/services/userEvent';
import { IntegrationTile } from 'teleport/Integrations';
import { FeatureHeader, FeatureHeaderTitle } from 'teleport/components/Layout';

type BotIntegration = {
  title: string;
  link: string;
  icon: JSX.Element;
  guided: boolean;
  kind: IntegrationEnrollKind;
}

const integrations: BotIntegration[] = [
  {
    title: 'GitHub Actions',
    link: cfg.getBotsNewRoute(BotType.GitHubActions),
    icon: <GitHubIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDGitHubActions,
    guided: true,
  },
  {
    title: 'CircleCI',
    link: 'https://goteleport.com/docs/machine-id/deployment/circleci/',
    icon: <CircleCIIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDCircleCI,
    guided: false,
  },
  {
    title: 'GitLab CI/CD',
    link: 'https://goteleport.com/docs/machine-id/deployment/gitlab/',
    icon: <GitLabIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDGitLab,
    guided: false,
  },
  {
    title: 'Jenkins',
    link: 'https://goteleport.com/docs/machine-id/deployment/jenkins/',
    icon: <JenkinsIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDJenkins,
    guided: false,
  },
  {
    title: 'Ansible',
    link: 'https://goteleport.com/docs/machine-id/access-guides/ansible/',
    icon: <AnsibleIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDAnsible,
    guided: false,
  },
  {
    title: 'Spacelift',
    link: 'https://goteleport.com/docs/machine-id/deployment/spacelift/',
    icon: <SpaceliftIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDSpacelift,
    guided: false,
  },
  {
    title: 'AWS',
    link: 'https://goteleport.com/docs/machine-id/deployment/aws/',
    icon: <AWSIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDAWS,
    guided: false,
  },
  {
    title: 'GCP',
    link: 'https://goteleport.com/docs/machine-id/deployment/gcp/',
    icon: <GCPIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDGCP,
    guided: false,
  },
  {
    title: 'Azure',
    link: 'https://goteleport.com/docs/machine-id/deployment/azure/',
    icon: <AzureIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDAzure,
    guided: false,
  },
  {
    title: 'Kubernetes',
    link: 'https://goteleport.com/docs/machine-id/deployment/kubernetes/',
    icon: <KubernetesIcon size={80} />,
    kind: IntegrationEnrollKind.MachineIDKubernetes,
    guided: false,
  },
  {
    title: 'Generic',
    link: 'https://goteleport.com/docs/machine-id/getting-started/',
    icon: <ServersIcon size={80} />,
    kind: IntegrationEnrollKind.MachineID,
    guided: false,
  },
];

export function AddBotsPicker() {
  return (
    <>
      <FeatureHeader>
        <FeatureHeaderTitle>Select Bot Type</FeatureHeaderTitle>
      </FeatureHeader>

      <Text typography="body1">
        Set up Teleport Machine ID to allow CI/CD workflows and other machines
        to access resources protected by Teleport.
      </Text>

      <Flex mt={5} gap={3} flexWrap="wrap">
        {integrations.map(i => (
          <Box key={i.title}>
            {i.guided ? (
              <GuidedTile integration={i} />
            ) : (
              <ExternalLinkTile integration={i} />
            )}
          </Box>
        ))}
      </Flex>
    </>
  );
};

function ExternalLinkTile({ integration }: { integration: BotIntegration }) {
  return (
    <IntegrationTile
      as={ExternalLink}
      href={integration.link}
      target="_blank"
      onClick={() => {
        userEventService.captureIntegrationEnrollEvent({
          event: IntegrationEnrollEvent.Started,
          eventData: {
            id: crypto.randomUUID(),
            kind: integration.kind,
          },
        });
      }}
    >
      <TileContent icon={integration.icon} title={integration.title} />
    </IntegrationTile>
  );
}

function GuidedTile({ integration }: { integration: BotIntegration }) {
  return (
    <IntegrationTile as={Link} to={integration.link}
      onClick={() => {
        userEventService.captureIntegrationEnrollEvent({
          event: IntegrationEnrollEvent.Started,
          eventData: {
            id: crypto.randomUUID(),
            kind: integration.kind,
          },
        });
      }}
    >
      <BadgeGuided>Guided</BadgeGuided>
      <TileContent icon={integration.icon} title={integration.title} />
    </IntegrationTile>
  );
}

function TileContent({ icon, title }) {
  return (
    <>
      <Box mt={3} mb={2}>
        {icon}
      </Box>
      <Text>{title}</Text>
    </>
  );
}

const BadgeGuided = styled.div`
  position: absolute;
  background: ${props => props.theme.colors.brand};
  color: ${props => props.theme.colors.text.primaryInverse};
  padding: 0px 6px;
  border-top-right-radius: 8px;
  border-bottom-left-radius: 8px;
  top: 0px;
  right: 0px;
  font-size: 10px;
`;