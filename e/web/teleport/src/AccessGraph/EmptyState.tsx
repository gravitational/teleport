import { ComponentType, useEffect, useRef, useState } from 'react';
import styled, { useTheme } from 'styled-components';

import { Box, ButtonPrimary, Flex, H1, Image, Text } from 'design';
import { FeatureName } from 'design/constants';
import { RocketLaunch } from 'design/Icon';
import { ResourceIcon } from 'design/ResourceIcon';
import { Theme } from 'design/theme';
import {
  DetailsTab,
  FeatureContainer,
  FeatureSlider,
} from 'shared/components/EmptyState/EmptyState';

import { DisplayTile } from 'teleport/Bots/Add/AddBotsPicker';
import { FeatureBox } from 'teleport/components/Layout';
import cfg from 'teleport/config';

import crownJewelsImageDark from './crown-jewels-dark.svg';
import crownJewelsImageLight from './crown-jewels-light.svg';
import policyImageDark from './policy-dark.svg';
import policyImageLight from './policy-light.svg';
import riskyImageDark from './risky-access-dark.svg';
import riskyImageLight from './risky-access-light.svg';

const maxWidth = '1204px';

export function EmptyState() {
  const [currIndex, setCurrIndex] = useState(0);

  const PreviewPanel = tabs[currIndex].PreviewPanel;

  const intervalId = useRef<number>(undefined);
  function handleOnClick(clickedIndex: number) {
    if (intervalId.current !== null) {
      clearInterval(intervalId.current); // Clear the interval if it exists
    }
    setCurrIndex(clickedIndex);
  }

  useEffect(() => {
    intervalId.current = window.setInterval(() => {
      setCurrIndex(latestIndex => (latestIndex + 1) % tabs.length);
    }, 5000);

    return () => {
      if (intervalId.current !== null) {
        clearInterval(intervalId.current); // Cleanup on unmount
      }
    };
  }, []);

  return (
    <FeatureBox>
      <Box mt={4} data-testid="tag-empty-state">
        <Box mb={3}>
          <H1 mb={3}>{FeatureName.IdentitySecurity}</H1>
          <Text css={{ maxWidth }}>
            {FeatureName.IdentitySecurity} unifies management of access policies
            across your infrastructure. Eliminate shadow access and blind spots.
          </Text>
        </Box>
        <FeatureContainer py={2} pr={2}>
          <Box css={{ position: 'relative' }}>
            <FeatureSlider $currIndex={currIndex} />
            {tabs.map((tab, index) => (
              <DetailsTab
                key={tab.title}
                active={currIndex === index}
                isSliding={!!intervalId}
                onClick={() => handleOnClick(index)}
                title={tab.title}
                description={tab.description}
              />
            ))}
          </Box>
          <Box mt={-2} height={330}>
            <PreviewPanel />
          </Box>
        </FeatureContainer>
        {/* setting a max width here to keep it "in the center" with the content above instead of with the screen */}
        <Box width="100%" maxWidth={maxWidth} textAlign="center" mt={6}>
          <ButtonPrimary
            as="a"
            href="https://goteleport.com/platform/policy/"
            size="large"
          >
            <RocketLaunch size={20} mr={2} />
            {cfg.isEnterprise
              ? `Try ${FeatureName.IdentitySecurity}`
              : 'Upgrade to Enterprise'}
          </ButtonPrimary>
        </Box>
      </Box>
    </FeatureBox>
  );
}

const standingPrivsImages: IconSpec = {
  light: policyImageLight,
  dark: policyImageDark,
};

const StandingPrivsPreview = () => {
  const theme = useTheme();
  return (
    <PreviewBox>
      <Image maxHeight="100%" src={standingPrivsImages[theme.type]} />
    </PreviewBox>
  );
};

const crownJewelsImages: IconSpec = {
  light: crownJewelsImageLight,
  dark: crownJewelsImageDark,
};

const CrownJewelsPreview = () => {
  const theme = useTheme();
  return (
    <PreviewBox>
      <Image maxHeight="100%" src={crownJewelsImages[theme.type]} />
    </PreviewBox>
  );
};

const riskyAccessImages: IconSpec = {
  light: riskyImageLight,
  dark: riskyImageDark,
};

const RiskyAccessPreview = () => {
  const theme = useTheme();
  return (
    <PreviewBox>
      <Image maxHeight="100%" src={riskyAccessImages[theme.type]} />
    </PreviewBox>
  );
};

const TILE_ICON_HEIGHT = 80;

const PreviewBox = styled(Box)`
  margin-left: ${p => p.theme.space[5]}px;
  max-height: 330px;
`;

const tileTop = [
  {
    title: 'AWS',
    icon: <ResourceIcon height={TILE_ICON_HEIGHT} name="aws" />,
  },
  {
    title: 'Azure',
    icon: <ResourceIcon height={TILE_ICON_HEIGHT} name="azure" />,
  },
  {
    title: 'Google Cloud',
    icon: <ResourceIcon height={TILE_ICON_HEIGHT} name="googlecloud" />,
  },
];

const tilesBottom = [
  {
    title: 'Entra ID',
    icon: <ResourceIcon height={TILE_ICON_HEIGHT} name="entraid" />,
  },
  {
    title: 'Okta',
    icon: <ResourceIcon height={TILE_ICON_HEIGHT} name="okta" />,
  },
  {
    title: 'Gitlab',
    icon: <ResourceIcon height={TILE_ICON_HEIGHT} name="gitlab" />,
  },
];

const Tiles = () => {
  return (
    <PreviewBox>
      <Flex>
        {tileTop.map(integration => (
          <DisplayTile
            key={integration.title}
            icon={integration.icon}
            title={integration.title}
          />
        ))}
      </Flex>
      <Flex>
        {tilesBottom.map(integration => (
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

const tabs: {
  title: string;
  description: string;
  PreviewPanel: ComponentType;
}[] = [
  {
    title: 'Discover and replace long-standing privileges',
    description:
      'Identify accounts with standing privileges or policies across all your infrastructure. Eliminate standing and stale privileges.',
    PreviewPanel: StandingPrivsPreview,
  },
  {
    title: 'Protect your most critical assets',
    description:
      'Tag critical resources as Crown Jewels, for proactive monitoring and auditing of changes in access paths and permissions.',
    PreviewPanel: CrownJewelsPreview,
  },
  {
    title: 'Eliminate shadow and risky access',
    description:
      'Shadow and backdoor access paths put resources and data at risk of a security breach. Identify risks caused by unauthorized or undocumented SSH keys.',
    PreviewPanel: RiskyAccessPreview,
  },
  {
    title: 'Visualize access across your stack',
    description:
      'Connect to your cloud provider, MDM manager, IDP, code repositories and more to identify access paths throughout your tech stack.',
    PreviewPanel: Tiles,
  },
];

type IconSpec = {
  [K in Theme['type']]: string;
};
