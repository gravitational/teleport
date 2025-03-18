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

import { useTheme } from 'styled-components';

import { Box, Flex } from 'design';
import { ButtonSecondary } from 'design/Button';
import { FeatureNames } from 'design/constants';
import { ChevronLeft, ChevronRight } from 'design/Icon';
import Image from 'design/Image';
import { StepComponentProps, StepSlider } from 'design/StepSlider';
import { H1, H3, P, P3 } from 'design/Text/Text';
import type { Theme } from 'design/theme';

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import cfg from 'teleport/config';

import accessGraphPromoDark from './access-graph-promo-dark.png';
import accessGraphPromoLight from './access-graph-promo-light.png';

const promoImageWidth = 782;
const promoImages: Record<Theme['type'], string> = {
  dark: accessGraphPromoDark,
  light: accessGraphPromoLight,
};

const promoFlows = {
  creating: [VisualizeAccessPathsPanel, VisualizeDiffPanel],
  updating: [VisualizeDiffPanel, VisualizeAccessPathsPanel],
};

export function PolicyPlaceholder({
  currentFlow,
}: {
  currentFlow: 'creating' | 'updating';
}) {
  const theme = useTheme();
  return (
    <Box maxWidth={promoImageWidth + 2 * 2} minWidth={300}>
      <H1 mb={2}>{FeatureNames.IdentitySecurity} saves you from mistakes.</H1>
      <Flex mb={4} gap={6} flexWrap="wrap" justifyContent="space-between">
        <Box flex="1" minWidth="30ch">
          <P>
            {FeatureNames.IdentitySecurity} will visualize resource access paths
            as you create and edit roles so you can always see what you are
            granting before you push a role into production.
          </P>
        </Box>
        <Flex flex="0 0 auto" alignItems="start">
          {!cfg.isPolicyEnabled && (
            <>
              <ButtonLockedFeature noIcon py={0} width={undefined}>
                Contact Sales
              </ButtonLockedFeature>
              <ButtonSecondary
                as="a"
                href="https://goteleport.com/platform/policy/"
                target="_blank"
                ml={2}
              >
                Learn More
              </ButtonSecondary>
            </>
          )}
        </Flex>
      </Flex>
      <Flex
        flexDirection="column"
        bg={theme.colors.levels.surface}
        borderRadius={3}
      >
        <Box
          border={2}
          borderRadius={3}
          borderColor={theme.colors.interactive.tonal.neutral[0]}
          overflow="hidden" // Clip those pointy corners!
        >
          <Image
            src={promoImages[theme.type]}
            width="100%"
            alt="Screenshot of a graph that visualizes access to Teleport resources"
          />
        </Box>
        <StepSlider wrapping flows={promoFlows} currFlow={currentFlow} />
      </Flex>
    </Box>
  );
}
function VisualizeAccessPathsPanel(props: StepComponentProps) {
  return (
    <PromoPanel
      {...props}
      heading="Visualize access paths granted by your roles"
      content={
        <>
          See what you’re granting before pushing to prod.{' '}
          {FeatureNames.IdentitySecurity} will show resource access paths
          granted by your role before you save changes.
        </>
      }
    />
  );
}

function VisualizeDiffPanel(props: StepComponentProps) {
  return (
    <PromoPanel
      {...props}
      heading="Visualize the diff in permissions as you edit roles"
      content={
        <>
          Prevent mistakes. {FeatureNames.IdentitySecurity} shows you what
          access is removed and what is added as you make edits to a role—all
          before you save your changes.
        </>
      }
    />
  );
}

function PromoPanel({
  prev,
  next,
  refCallback,
  heading,
  content,
}: StepComponentProps & {
  heading: React.ReactNode;
  content: React.ReactNode;
}) {
  return (
    <Flex m={4} gap={8} ref={refCallback}>
      <Box flex="1">
        <H3>{heading}</H3>
        <Box flex="1">
          <P3>{content}</P3>
        </Box>
      </Box>
      <Flex gap={2} alignItems="center">
        <ButtonSecondary size="medium" width="32px" padding={0} onClick={prev}>
          <ChevronLeft size="small" />
        </ButtonSecondary>
        <ButtonSecondary size="medium" width="32px" padding={0} onClick={next}>
          <ChevronRight size="small" />
        </ButtonSecondary>
      </Flex>
    </Flex>
  );
}
