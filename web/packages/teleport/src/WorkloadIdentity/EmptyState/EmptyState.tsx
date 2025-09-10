/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { ComponentType, useEffect, useRef, useState } from 'react';
import styled, { useTheme } from 'styled-components';

import { Box, ButtonPrimary, H1, Image, Text } from 'design';
import { Theme } from 'design/theme';
import {
  DetailsTab,
  FeatureContainer,
  FeatureSlider,
} from 'shared/components/EmptyState/EmptyState';

import { FeatureBox } from 'teleport/components/Layout';

import feature1dark from './workload_identity_feature_1_dark_mode-min.svg';
import feature1light from './workload_identity_feature_1_light_mode-min.svg';
import feature2dark from './workload_identity_feature_2_dark_mode-min.svg';
import feature2light from './workload_identity_feature_2_light_mode-min.svg';
import feature3dark from './workload_identity_feature_3_dark_mode-min.svg';
import feature3light from './workload_identity_feature_3_light_mode-min.svg';

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
      <Box mt={4} data-testid="workload-identity-empty-state">
        <Box mb={3}>
          <H1 mb={3}>What is Workload Identity</H1>
          <Text css={{ maxWidth }}>
            Based on the open-source SPIFFE standard, Workload Identity replaces
            long-lived API keys and environment secrets that are vulnerable to
            breaches, mistakes and exfiltration with short-lived cryptographic
            identities using x.509 certificates or JWTs.
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
            href="https://goteleport.com/docs/enroll-resources/workload-identity/introduction"
            size="large"
            target="_blank"
          >
            Get Started
          </ButtonPrimary>
        </Box>
      </Box>
    </FeatureBox>
  );
}

const mutualAuthenticationImages: IconSpec = {
  light: feature1light,
  dark: feature1dark,
};

const MutualAuthentication = () => {
  const theme = useTheme();
  return (
    <PreviewBox>
      <Image maxHeight="100%" src={mutualAuthenticationImages[theme.type]} />
    </PreviewBox>
  );
};

const hybridServicesImages: IconSpec = {
  light: feature2light,
  dark: feature2dark,
};

const HybridServices = () => {
  const theme = useTheme();
  return (
    <PreviewBox>
      <Image maxHeight="100%" src={hybridServicesImages[theme.type]} />
    </PreviewBox>
  );
};

const complianceReqsImages: IconSpec = {
  light: feature3light,
  dark: feature3dark,
};

export const ComplianceReqs = () => {
  const theme = useTheme();
  return (
    <PreviewBox>
      <Image maxHeight="100%" src={complianceReqsImages[theme.type]} />
    </PreviewBox>
  );
};

const PreviewBox = styled(Box)`
  margin-left: ${p => p.theme.space[5]}px;
  max-height: 330px;
`;

const tabs: {
  title: string;
  description: string;
  PreviewPanel: ComponentType;
}[] = [
  {
    title: 'Accelerate time to market with mutual authentication',
    description:
      'Enable workloads to access infra and services without relying on API keys. Let the engineers focus on development, not managing secrets.',
    PreviewPanel: MutualAuthentication,
  },
  {
    title: 'Scalable access to cross-cloud and hybrid services',
    description:
      'Use cryptographically-backed identity to authenticate with AWS, GCP, Azure and on-premise infrastructure.',
    PreviewPanel: HybridServices,
  },
  {
    title: 'Meet compliance requirements',
    description:
      'Pass audits by leveraging mTLS with visibility of what identities were issued, when, and where.',
    PreviewPanel: ComplianceReqs,
  },
];

type IconSpec = {
  [K in Theme['type']]: string;
};
