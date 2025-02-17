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
import { ChevronLeft, ChevronRight } from 'design/Icon';
import Image from 'design/Image';
import { StepComponentProps, StepSlider } from 'design/StepSlider';
import { H1, H3, P, P3 } from 'design/Text/Text';

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import cfg from 'teleport/config';

import tagpromo from './tagpromo.png';

const promoImageWidth = 782;

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
      <H1 mb={2}>Coming soon: Teleport Policy saves you from mistakes</H1>
      <Flex mb={4} gap={4} flexWrap="wrap" justifyContent="space-between">
        <Box flex="1" minWidth="30ch">
          <P>
            Teleport Policy will visualize resource access paths as you create
            and edit roles so you can always see what you are granting before
            you push a role into production.
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
        >
          <Image src={tagpromo} width="100%" />
        </Box>
        <StepSlider flows={promoFlows} currFlow={currentFlow} />
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
          See what you’re granting before pushing to prod. Teleport Policy will
          show resource access paths granted by your role before you save
          changes.
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
          Prevent mistakes. Teleport Policy shows you what access is removed and
          what is added as you make edits to a role—all before you save your
          changes.
        </>
      }
    />
  );
}

function PromoPanel({
  prev,
  next,
  refCallback,
  stepIndex,
  flowLength,
  heading,
  content,
}: StepComponentProps & {
  heading: React.ReactNode;
  content: React.ReactNode;
}) {
  return (
    <Flex m={4} gap={4} ref={refCallback}>
      <Box flex="1">
        <H3>{heading}</H3>
        <Box flex="1">
          <P3>{content}</P3>
        </Box>
      </Box>
      <Flex gap={2} alignItems="center">
        <ButtonSecondary size="small" width="24px" disabled={stepIndex <= 0}>
          <ChevronLeft size="small" onClick={prev} />
        </ButtonSecondary>
        <ButtonSecondary
          size="small"
          width="24px"
          disabled={stepIndex >= flowLength - 1}
        >
          <ChevronRight size="small" onClick={next} />
        </ButtonSecondary>
      </Flex>
    </Flex>
  );
}
