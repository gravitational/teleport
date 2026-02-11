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

import { Flex, Text } from 'design';
import { ResourceIconName } from 'design/ResourceIcon';

import { ToolTipNoPermBadge } from 'teleport/components/ToolTipNoPermBadge';
import cfg from 'teleport/config';
import { IntegrationTile } from 'teleport/Integrations';
import { IntegrationTag, Tile } from 'teleport/Integrations/Enroll/Shared';
import {
  IntegrationEnrollEvent,
  IntegrationEnrollKind,
  userEventService,
} from 'teleport/services/userEvent';

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
    description:
      'Use Machine & Workload Identity (MWI) to grant GitHub Actions CI/CD access to Teleport resources.',
    link: cfg.getBotsNewRoute(BotFlowType.GitHubActionsSsh),
    icon: 'github',
    kind: IntegrationEnrollKind.MachineIDGitHubActions,
    type: 'bot',
    guided: true,
    tags: ['bot', 'cicd'],
  },
  // Hiding the new guide for now.
  // {
  //   title: 'GitHub Actions + Kubernetes',
  //   description: 'Use Machine & Workload Identity to grant GitHub Actions CI/CD access to Teleport resources.',
  //   link: cfg.getBotsNewRoute(BotFlowType.GitHubActionsK8s),
  //   icon: 'github',
  //   kind: IntegrationEnrollKind.MachineIDGitHubActionsK8s,
  //   type: 'bot',
  //   guided: true,
  //   tags: ['bot', 'cicd'],
  // },
  {
    title: 'GitHub Actions + Kubernetes',
    description:
      'Use Machine & Workload Identity (MWI) to grant GitHub Actions CI/CD access to Kubernetes clusters.',
    link: cfg.getBotsNewRoute(BotFlowType.GitHubActionsK8s),
    icon: 'github',
    kind: IntegrationEnrollKind.MachineIDGitHubActionsKubernetes,
    type: 'bot',
    guided: true,
    tags: ['bot', 'cicd'],
  },
  {
    title: 'CircleCI',
    description:
      'Use Machine & Workload Identity (MWI) to eliminate long-lived credentials in CircleCI CI/CD workflows.',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/circleci/',
    icon: 'circleci',
    kind: IntegrationEnrollKind.MachineIDCircleCI,
    type: 'bot',
    guided: false,
    tags: ['bot', 'cicd'],
  },
  {
    title: 'GitLab CI/CD',
    description:
      'Use Machine & Workload Identity (MWI) to eliminate long-lived credentials in GitLab pipelines.',
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
      'Use Machine & Workload Identity (MWI) to eliminate long-lived credentials in Jenkins.',
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
      'Use Machine & Workload Identity (MWI) to eliminate long-lived credentials from Ansible workflows.',
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
      'Use Machine & Workload Identity (MWI) to authenticate Spacelift runs with Teleport.',
    link: 'https://goteleport.com/docs/admin-guides/infrastructure-as-code/terraform-provider/spacelift/',
    icon: 'spacelift',
    kind: IntegrationEnrollKind.MachineIDSpacelift,
    type: 'bot',
    guided: false,
    tags: ['bot', 'cicd'],
  },
  {
    title: 'AWS',
    description:
      'Use Machine & Workload Identity (MWI) to eliminate long-lived credentials on EC2 VMs.',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/aws/',
    icon: 'aws',
    kind: IntegrationEnrollKind.MachineIDAWS,
    type: 'bot',
    guided: false,
    tags: ['bot', 'resourceaccess'],
  },
  {
    title: 'Google Cloud',
    description:
      'Use Machine & Workload Identity (MWI) to eliminate long-lived credentials on GCE VMs.',
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
      'Use Machine & Workload Identity (MWI) to eliminate long-lived credentials on Azure VMs.',
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
      'Use Machine & Workload Identity (MWI) to eliminate long-lived credentials for Kubernetes workloads.',
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
      'Use Machine & Workload Identity (MWI) to enable Argo CD to connect to external Kubernetes clusters.',
    link: 'https://goteleport.com/docs/machine-workload-identity/machine-id/access-guides/argocd/',
    icon: 'argocd',
    kind: IntegrationEnrollKind.MachineIDArgoCD,
    type: 'bot',
    guided: false,
    tags: ['bot', 'cicd'],
  },
  {
    title: 'Generic Linux',
    description:
      'Use Machine & Workload Identity (MWI) to eliminate long-lived credentials on Linux servers.',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/getting-started/',
    icon: 'server',
    kind: IntegrationEnrollKind.MachineID,
    type: 'bot',
    guided: false,
    tags: ['bot', 'resourceaccess'],
  },
];

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
      title={`MWI: ${integration.title}`}
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
      title={`MWI: ${integration.title}`}
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
