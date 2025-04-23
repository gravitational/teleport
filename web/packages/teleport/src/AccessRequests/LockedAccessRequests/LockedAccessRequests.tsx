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

import React from 'react';
import styled from 'styled-components';

import { Box, Card, Flex, H2, Image, Link, Subtitle2, Text } from 'design';

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { CtaEvent } from 'teleport/services/userEvent';

import step1 from './assets/step1.png';
import step2 from './assets/step2.png';
import step3 from './assets/step3.png';

export function LockedAccessRequests() {
  const CTAButton = (
    <ButtonLockedFeature height="36px" event={CtaEvent.CTA_ACCESS_REQUESTS}>
      Unlock Access Requests With Teleport Enterprise
    </ButtonLockedFeature>
  );

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <FeatureHeaderTitle>Just In Time Access Requests</FeatureHeaderTitle>
        <Box>{CTAButton}</Box>
      </FeatureHeader>
      <Card
        m="auto"
        p={4}
        pb="44px"
        as={Flex}
        flex="0 0 auto"
        flexDirection="column"
        width="auto"
      >
        <Box as="header" width="100%" textAlign="left">
          <H2 mb={1}>Access Requests Flow</H2>
          <Subtitle2 mb={5}>
            To learn more about access requests, take a look at&nbsp;
            <Link
              color="text.secondary"
              href="https://goteleport.com/features/access-requests/"
              target="_blank"
            >
              Teleport Documentation.
            </Link>
          </Subtitle2>
        </Box>
        <Flex
          gap={4}
          flexWrap="wrap"
          justifyContent="center"
          style={{ position: 'relative' }}
        >
          <StepCard src={step1} step="1">
            Bob can select the resources he needs to access or request the role
            <span style={{ fontWeight: 'bold' }}> dbadmin </span>
            in the Web UI or CLI
          </StepCard>
          <StepCard src={step2} step="2">
            Chatbot will notify both Alice and Ivan
          </StepCard>
          <StepCard src={step3} step="3">
            Alice and Ivan can review and approve request using Web UI or CLI
          </StepCard>
          <Box
            css={`
              position: absolute;
              bottom: 0;
              left: 50%;
              min-width: 600px;
              transform: translate(-50%, 50%);
              @media screen and (max-width: 800px) {
                min-width: 100%;
              }
            `}
          >
            {CTAButton}
          </Box>
        </Flex>
      </Card>
    </FeatureBox>
  );
}

type StepCardProps = {
  src: string;
  step: string;
  children: React.ReactNode;
};

function StepCard({ src, step, children }: StepCardProps) {
  return (
    <StepCardContainer textAlign="left">
      <ImageContainer>
        <Image
          css={`
            display: block;
          `}
          src={src}
        />
      </ImageContainer>
      <Box mt={4} width="100%">
        <Text fontSize="3" bold>
          Step {step}
        </Text>
      </Box>
      <Box width="100%">
        <Text mt="1">{children}</Text>
      </Box>
    </StepCardContainer>
  );
}

const StepCardContainer = styled(Flex)`
  background-color: ${p => p.theme.colors.spotBackground[0]};
  padding: ${p => p.theme.space[4]}px;
  width: 340px;
  height: 420px;
  border-radius: 8px;
  flex-direction: column;
  align-items: center;
`;

const ImageContainer = styled(Flex)`
  width: 293px;
  align-items: center;
  height: 240px;
  border-radius: 8px;
  background-color: ${p => p.theme.colors.spotBackground[1]};
`;
