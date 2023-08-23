/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import styled from 'styled-components';
import { Box, Card, Flex, Image, Link, Text } from 'design';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';

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
        <Box width="100%" textAlign="left">
          <Text typography="h4" bold>
            Access Requests Flow
          </Text>
          <Text typography="subtitle1" mb={5}>
            To learn more about access requests, take a look at&nbsp;
            <Link
              color="text.secondary"
              href="https://goteleport.com/features/access-requests/"
              target="_blank"
            >
              Teleport Documentation.
            </Link>
          </Text>
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
              } ;
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
