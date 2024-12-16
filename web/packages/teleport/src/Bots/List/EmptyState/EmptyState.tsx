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

import React, { useState, useEffect } from 'react';
import styled, { useTheme } from 'styled-components';
import { Link } from 'react-router-dom';
import { GitHubIcon, SpaceliftIcon, AnsibleIcon } from 'design/SVGIcon';
import { ResourceIcon } from 'design/ResourceIcon';
import {
  Description,
  DetailsTab,
  Feature,
  FeatureContainer,
  FeatureProps,
  FeatureSlider,
  Title,
} from 'shared/components/EmptyState/EmptyState';
import { Box, ButtonPrimary, Flex, Image, Text } from 'design';

import cfg from 'teleport/config';
import { DisplayTile } from 'teleport/Bots/Add/AddBotsPicker';

import elimiateSecretsImage from './eliminate-secrets.svg';
import elimiateSecretsLightImage from './eliminate-secrets-light.svg';

import controlWorkflowsImage from './control-workflows.svg';
import controlWorkflowsLightImage from './control-workflows-light.svg';

import argoCD from './argocd.png';

const maxWidth = '1204px';

export function EmptyState() {
  const [currIndex, setCurrIndex] = useState(0);
  const [intervalId, setIntervalId] = useState<any>();

  function handleOnClick(clickedIndex: number) {
    clearInterval(intervalId);
    setCurrIndex(clickedIndex);
    setIntervalId(null);
  }

  useEffect(() => {
    const id = setInterval(() => {
      setCurrIndex(latestIndex => (latestIndex + 1) % 3);
    }, 3000);
    setIntervalId(id);
    return () => clearInterval(id);
  }, []);

  return (
    <Box mt={4} data-testid="bots-empty-state">
      <Box mb={3}>
        <Text typography="h5" mb={3} fontSize={7}>
          What are Bots?
        </Text>
        <Text css={{ maxWidth }}>
          Static keys and API keys in your automated workflows are the target of
          hackers and are one of the primary sources of security breaches.
          <br />
          Teleport Machine ID replaces shared credentials and secrets with
          short-lived x.509 or SSH certificates and gives you a unified plan to
          register, define access policies, and audit all your workflows.
        </Text>
      </Box>
      <FeatureContainer py={2} pr={2}>
        <Box css={{ position: 'relative' }}>
          <FeatureSlider $currIndex={currIndex} />
          <DetailsTab
            active={currIndex === 0}
            isSliding={!!intervalId}
            onClick={() => handleOnClick(0)}
            title="Eliminate secrets and shared credentials from CI/CD workflows"
            description="Teleport Machine ID replaces passwords, API, and static keys with short-lived SSH and x.509 certificates."
          />
          <DetailsTab
            active={currIndex === 1}
            isSliding={!!intervalId}
            onClick={() => handleOnClick(1)}
            title="Control all your workflows from a unified plane."
            description="Unify access policies and get structured audit events for all automatic workflows on your infrastructure. Lock out and terminate connections to potentially compromised workflows."
          />
          <DetailsTab
            active={currIndex === 2}
            isSliding={!!intervalId}
            onClick={() => handleOnClick(2)}
            title="Works with everything you have"
            description="Connect your ArgoCD, Jenkins, Spacelift, GitHub Actions, Ansible, Terraform, and more."
          />
        </Box>
        <Box mt={-2} height={330}>
          {currIndex === 0 && <EliminateSecretsPreview />}
          {currIndex === 1 && <ControlWorkflowsPreview />}
          {currIndex === 2 && <BotTiles />}
        </Box>
      </FeatureContainer>
      {/* setting a max width here to keep it "in the center" with the content above instead of with the screen */}
      <Box width="100%" maxWidth={maxWidth} textAlign="center" mt={6}>
        <ButtonPrimary
          width="280px"
          as={Link}
          to={cfg.getBotsNewRoute()}
          size="large"
        >
          Create Your First Bot
        </ButtonPrimary>
      </Box>
    </Box>
  );
}

export const EliminateSecrets = ({
  active,
  onClick,
  isSliding,
}: FeatureProps) => {
  return (
    <Feature $active={active} onClick={onClick} $isSliding={isSliding}>
      <Title>
        Eliminate secrets and shared credentials from CI/CD workflows.
      </Title>
      <Description>
        Teleport Machine ID replaces passwords, API, and static keys with
        short-lived SSH and x.509 certificates.
      </Description>
    </Feature>
  );
};

const eliminateSecretsImages = {
  light: elimiateSecretsLightImage,
  dark: elimiateSecretsImage,
};

export const EliminateSecretsPreview = () => {
  const theme = useTheme();
  return (
    <PreviewBox includeShadow>
      <Image maxHeight="100%" src={eliminateSecretsImages[theme.type]} />
    </PreviewBox>
  );
};

const controlWorkflowsImages = {
  light: controlWorkflowsLightImage,
  dark: controlWorkflowsImage,
};

export const ControlWorkflowsPreview = () => {
  const theme = useTheme();
  return (
    <PreviewBox includeShadow>
      <Image maxHeight="100%" src={controlWorkflowsImages[theme.type]} />
    </PreviewBox>
  );
};

const TILE_ICON_HEIGHT = 80;

// These logos don't have dark/light mode variants, or are
// not in our system and are exported as PNG by design team
const integrationsTop = [
  {
    title: 'Jenkins',
    icon: <ResourceIcon height={TILE_ICON_HEIGHT} name="jenkins" />,
  },
  {
    title: 'Terraform',
    icon: <ResourceIcon height={TILE_ICON_HEIGHT} name="terraform" />,
  },
  {
    title: 'Argo CD',
    icon: <Image height={TILE_ICON_HEIGHT} src={argoCD} />,
  },
];

const integrationsBottom = [
  {
    title: 'GitHub',
    icon: <GitHubIcon size={TILE_ICON_HEIGHT} />,
  },
  {
    title: 'Ansible',
    icon: <AnsibleIcon size={TILE_ICON_HEIGHT} />,
  },
  {
    title: 'Spacelift',
    icon: <SpaceliftIcon size={TILE_ICON_HEIGHT} />,
  },
];

export const BotTiles = () => {
  return (
    <PreviewBox>
      <Flex>
        {integrationsTop.map(integration => (
          <DisplayTile
            key={integration.title}
            icon={integration.icon}
            title={integration.title}
          />
        ))}
      </Flex>
      <Flex>
        {integrationsBottom.map(integration => (
          <DisplayTile
            key={integration.title}
            icon={integration.icon}
            title={integration.title}
          />
        ))}
      </Flex>
    </PreviewBox>
  );
};

const PreviewBox = styled(Box)<{ includeShadow?: boolean }>`
  margin-left: ${p => p.theme.space[5]}px;
  max-height: 330px;
  box-shadow: ${p => {
    return p.includeShadow ? p.theme.boxShadow[1] : 'none';
  }};
`;
