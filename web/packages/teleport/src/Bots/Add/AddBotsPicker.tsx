/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { ReactNode } from 'react';
import styled from 'styled-components';

import { Box, Flex, Text } from 'design';
import { ResourceIconName } from 'design/ResourceIcon';
import { P } from 'design/Text/Text';
import { InfoGuideButton } from 'shared/components/SlidingSidePanel/InfoGuide';

import { FeatureHeader, FeatureHeaderTitle } from 'teleport/components/Layout';
import { ToolTipNoPermBadge } from 'teleport/components/ToolTipNoPermBadge';
import cfg from 'teleport/config';
import { IntegrationTile } from 'teleport/Integrations';
import { IntegrationTag, Tile } from 'teleport/Integrations/Enroll/Shared';
import {
  IntegrationEnrollEvent,
  IntegrationEnrollKind,
  userEventService,
} from 'teleport/services/userEvent';
import useTeleport from 'teleport/useTeleport';

import { InfoGuide } from '../InfoGuide';
import { BotFlowType } from '../types';

export type BotIntegration = {
  title: string;
  description: string;
  link: string;
  icon: ResourceIconName;
  guided: boolean;
  type: 'bot';
  kind: IntegrationEnrollKind;
  tags: IntegrationTag[];
};

export const integrations: BotIntegration[] = [
  {
    title: 'GitHub Actions + SSH',
    description: 'Use Machine ID to power GitHub CI/CD workflows.',
    link: cfg.getBotsNewRoute(BotFlowType.GitHubActions),
    icon: 'github',
    kind: IntegrationEnrollKind.MachineIDGitHubActions,
    type: 'bot',
    guided: true,
    tags: ['bot', 'cicd'],
  },
  {
    title: 'CircleCI',
    description: 'Use Machine ID to power CircleCI CI/CD workflows.',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/circleci/',
    icon: 'circleci',
    kind: IntegrationEnrollKind.MachineIDCircleCI,
    type: 'bot',
    guided: false,
    tags: ['bot', 'cicd'],
  },
  {
    title: 'GitLab CI/CD',
    description: 'Use Machine ID to power GitLab CI/CD workflows.',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/gitlab/',
    icon: 'gitlab',
    kind: IntegrationEnrollKind.MachineIDGitLab,
    type: 'bot',
    guided: false,
    tags: ['bot', 'cicd'],
  },
  {
    title: 'Jenkins',
    description:
      'Use Machine ID to eliminate long-lived credentials in Jenkins.',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/jenkins/',
    icon: 'jenkins',
    kind: IntegrationEnrollKind.MachineIDJenkins,
    type: 'bot',
    guided: false,
    tags: ['bot', 'cicd'],
  },
  {
    title: 'Ansible',
    description:
      'Use Machine ID to eliminate long-lived credentials from auth with Linux hosts.',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/access-guides/ansible/',
    icon: 'ansible',
    kind: IntegrationEnrollKind.MachineIDAnsible,
    type: 'bot',
    guided: false,
    tags: ['bot', 'cicd'],
  },
  {
    title: 'Spacelift',
    description:
      'Use Machine ID to authenticate workloads running in Spacelift with Teleport.',
    link: 'https://goteleport.com/docs/admin-guides/infrastructure-as-code/terraform-provider/spacelift/',
    icon: 'spacelift',
    kind: IntegrationEnrollKind.MachineIDSpacelift,
    type: 'bot',
    guided: false,
    tags: ['bot', 'cicd'],
  },
  {
    title: 'AWS',
    description: 'Connect EC2 instances and RDS databases seamlessly.',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/aws/',
    icon: 'aws',
    kind: IntegrationEnrollKind.MachineIDAWS,
    type: 'bot',
    guided: false,
    tags: ['bot', 'resourceaccess'],
  },
  {
    title: 'Google Cloud',
    description: 'Connect GCE instances and CloudSQL databases seamlessly.',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/gcp/',
    icon: 'googlecloud',
    kind: IntegrationEnrollKind.MachineIDGCP,
    type: 'bot',
    guided: false,
    tags: ['bot', 'resourceaccess'],
  },
  {
    title: 'Azure',
    description:
      'Use Machine ID to eliminate long-lived credentials on Azure VMs.',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/azure/',
    icon: 'azure',
    kind: IntegrationEnrollKind.MachineIDAzure,
    type: 'bot',
    guided: false,
    tags: ['bot', 'resourceaccess'],
  },
  {
    title: 'Kubernetes',
    description:
      'Use Machine ID to eliminate long-lived credentials for Kubernetes workloads.',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/kubernetes/',
    icon: 'kube',
    kind: IntegrationEnrollKind.MachineIDKubernetes,
    type: 'bot',
    guided: false,
    tags: ['bot', 'resourceaccess'],
  },
  {
    title: 'Argo CD',
    description:
      'Use Machine ID to enable Argo CD to connect to external Kubernetes clusters.',
    link: 'https://goteleport.com/docs/machine-workload-identity/machine-id/access-guides/argocd/',
    icon: 'argocd',
    kind: IntegrationEnrollKind.MachineIDArgoCD,
    type: 'bot',
    guided: false,
    tags: ['bot', 'cicd'],
  },
  {
    title: 'Generic',
    description: 'Use Machine ID to Integrate generic server with Teleport.',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/getting-started/',
    icon: 'server',
    kind: IntegrationEnrollKind.MachineID,
    type: 'bot',
    guided: false,
    tags: ['bot', 'resourceaccess'],
  },
];

// TODO(alexhemard): delete in a follow up PR
export function AddBotsPicker() {
  const ctx = useTeleport();
  return (
    <>
      <FeatureHeader justifyContent="space-between">
        <FeatureHeaderTitle>Select Bot Type</FeatureHeaderTitle>
        <InfoGuideButton config={{ guide: <InfoGuide /> }} />
      </FeatureHeader>

      <P mb="5">
        Set up Teleport Machine ID to allow CI/CD workflows and other machines
        to access resources protected by Teleport.
      </P>

      <BotTiles hasCreateBotPermission={ctx.getFeatureFlags().addBots} />
    </>
  );
}

export function BotTiles({
  hasCreateBotPermission,
}: {
  hasCreateBotPermission: boolean;
}) {
  return (
    <div
      css={`
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
        gap: 16px;
      `}
    >
      {integrations.map(i => (
        <Box key={i.title}>
          <BotTile
            integration={i}
            hasCreateBotPermission={hasCreateBotPermission}
          />
        </Box>
      ))}
    </div>
  );
}

export function BotTile({
  integration,
  hasCreateBotPermission,
}: {
  integration: BotIntegration;
  hasCreateBotPermission: boolean;
}) {
  if (integration.guided) {
    return (
      <GuidedTile
        integration={integration}
        hasCreateBotPermission={hasCreateBotPermission}
      />
    );
  }
  return <ExternalLinkTile integration={integration} />;
}

function ExternalLinkTile({ integration }: { integration: BotIntegration }) {
  const onBotClick = () => {
    userEventService.captureIntegrationEnrollEvent({
      event: IntegrationEnrollEvent.Started,
      eventData: {
        id: crypto.randomUUID(),
        kind: integration.kind,
      },
    });
  };

  return (
    <Tile
      title={`Machine ID: ${integration.title}`}
      description={integration.description}
      tags={integration.tags}
      link={{ external: true, url: integration.link, onClick: onBotClick }}
      icon={integration.icon}
      hasAccess={true}
    />
  );
}

function GuidedTile({
  integration,
  hasCreateBotPermission,
}: {
  integration: BotIntegration;
  hasCreateBotPermission: boolean;
}) {
  const onBotClick = () => {
    if (!hasCreateBotPermission) {
      return;
    }
    userEventService.captureIntegrationEnrollEvent({
      event: IntegrationEnrollEvent.Started,
      eventData: {
        id: crypto.randomUUID(),
        kind: integration.kind,
      },
    });
  };

  const Badge = hasCreateBotPermission ? undefined : (
    <ToolTipNoPermBadge>
      <div>
        You don’t have sufficient permissions to create bots. Reach out to your
        Teleport administrator to request additional permissions.
      </div>
    </ToolTipNoPermBadge>
  );

  return (
    <Tile
      title={`Machine ID: ${integration.title}`}
      description={integration.description}
      tags={integration.tags}
      hasAccess={hasCreateBotPermission}
      icon={integration.icon}
      link={{ url: integration.link, onClick: onBotClick }}
      Badge={Badge}
    />
  );
}

export function DisplayTile({
  icon,
  title,
}: {
  title: string;
  icon: ReactNode;
}) {
  return (
    <HoverIntegrationTile>
      <TileContent icon={icon} title={title} />
    </HoverIntegrationTile>
  );
}

function TileContent({ icon, title }) {
  return (
    <>
      <Flex flexBasis={100}>{icon}</Flex>
      <Text>{title}</Text>
    </>
  );
}

const HoverIntegrationTile = styled(IntegrationTile)`
  background: none;
  transition: all 0.1s ease-in;
`;
