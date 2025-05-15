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

import type { JSX } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';

import { Box, Link as ExternalLink, Flex, ResourceIcon, Text } from 'design';
import { Server } from 'design/Icon';
import { P } from 'design/Text/Text';
import { InfoGuideButton } from 'shared/components/SlidingSidePanel/InfoGuide';

import { FeatureHeader, FeatureHeaderTitle } from 'teleport/components/Layout';
import { ToolTipNoPermBadge } from 'teleport/components/ToolTipNoPermBadge';
import cfg from 'teleport/config';
import { IntegrationTile } from 'teleport/Integrations';
import {
  IntegrationEnrollEvent,
  IntegrationEnrollKind,
  userEventService,
} from 'teleport/services/userEvent';
import useTeleport from 'teleport/useTeleport';

import { InfoGuide } from '../InfoGuide';
import { BotFlowType } from '../types';

type BotIntegration = {
  title: string;
  link: string;
  icon: JSX.Element;
  guided: boolean;
  kind: IntegrationEnrollKind;
};

const StyledResourceIcon = styled(ResourceIcon)`
  margin: 0 auto;
  height: 100%;
  min-width: 0;
  max-width: 80px;
`;

const integrations: BotIntegration[] = [
  {
    title: 'GitHub Actions + SSH',
    link: cfg.getBotsNewRoute(BotFlowType.GitHubActions),
    icon: <StyledResourceIcon name="github" />,
    kind: IntegrationEnrollKind.MachineIDGitHubActions,
    guided: true,
  },
  {
    title: 'CircleCI',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/circleci/',
    icon: <StyledResourceIcon name="circleci" />,
    kind: IntegrationEnrollKind.MachineIDCircleCI,
    guided: false,
  },
  {
    title: 'GitLab CI/CD',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/gitlab/',
    icon: <StyledResourceIcon name="gitlab" />,
    kind: IntegrationEnrollKind.MachineIDGitLab,
    guided: false,
  },
  {
    title: 'Jenkins',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/jenkins/',
    icon: <StyledResourceIcon name="jenkins" />,
    kind: IntegrationEnrollKind.MachineIDJenkins,
    guided: false,
  },
  {
    title: 'Ansible',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/access-guides/ansible/',
    icon: <StyledResourceIcon name="ansible" />,
    kind: IntegrationEnrollKind.MachineIDAnsible,
    guided: false,
  },
  {
    title: 'Spacelift',
    link: 'https://goteleport.com/docs/admin-guides/infrastructure-as-code/terraform-provider/spacelift/',
    icon: <StyledResourceIcon name="spacelift" />,
    kind: IntegrationEnrollKind.MachineIDSpacelift,
    guided: false,
  },
  {
    title: 'AWS',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/aws/',
    icon: <StyledResourceIcon name="aws" />,
    kind: IntegrationEnrollKind.MachineIDAWS,
    guided: false,
  },
  {
    title: 'GCP',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/gcp/',
    icon: <StyledResourceIcon name="googlecloud" />,
    kind: IntegrationEnrollKind.MachineIDGCP,
    guided: false,
  },
  {
    title: 'Azure',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/azure/',
    icon: <StyledResourceIcon name="azure" />,
    kind: IntegrationEnrollKind.MachineIDAzure,
    guided: false,
  },
  {
    title: 'Kubernetes',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/deployment/kubernetes/',
    icon: <StyledResourceIcon name="kube" />,
    kind: IntegrationEnrollKind.MachineIDKubernetes,
    guided: false,
  },
  {
    title: 'Generic',
    link: 'https://goteleport.com/docs/enroll-resources/machine-id/getting-started/',
    icon: <Server size={80} />,
    kind: IntegrationEnrollKind.MachineID,
    guided: false,
  },
];

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
    <Flex gap={3} flexWrap="wrap">
      {integrations.map(i => (
        <Box key={i.title}>
          {i.guided ? (
            <GuidedTile
              integration={i}
              hasCreateBotPermission={hasCreateBotPermission}
            />
          ) : (
            <ExternalLinkTile integration={i} />
          )}
        </Box>
      ))}
    </Flex>
  );
}

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

function GuidedTile({
  integration,
  hasCreateBotPermission,
}: {
  integration: BotIntegration;
  hasCreateBotPermission: boolean;
}) {
  return (
    <IntegrationTile
      as={Link}
      to={{
        pathname: hasCreateBotPermission ? integration.link : null,
        state: { previousPathname: location.pathname },
      }}
      onClick={() => {
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
      }}
    >
      {hasCreateBotPermission ? (
        <BadgeGuided>Guided</BadgeGuided>
      ) : (
        <ToolTipNoPermBadge>
          <div>
            You don’t have sufficient permissions to create bots. Reach out to
            your Teleport administrator to request additional permissions.
          </div>
        </ToolTipNoPermBadge>
      )}
      <TileContent icon={integration.icon} title={integration.title} />
    </IntegrationTile>
  );
}

export function DisplayTile({
  icon,
  title,
}: {
  title: string;
  icon: JSX.Element;
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

const HoverIntegrationTile = styled(IntegrationTile)`
  background: none;
  transition: all 0.1s ease-in;
`;
