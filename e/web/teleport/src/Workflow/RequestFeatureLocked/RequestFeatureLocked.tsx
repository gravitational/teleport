import React from 'react';
import styled from 'styled-components';
import Box from 'design/Box';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import Card from 'design/Card';
import Text from 'design/Text';
import Flex from 'design/Flex';

import Image from 'design/Image';

import step1 from './assets/step1.png';
import step2 from './assets/step2.png';
import step3 from './assets/step3.png';

export function RequestFeatureLocked() {
  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <FeatureHeaderTitle>Just In Time Access Requests</FeatureHeaderTitle>
        <Box>
          <ButtonLockedFeature height="36px">
            Unlock Access Requests With Teleport Enterprise
          </ButtonLockedFeature>
        </Box>
      </FeatureHeader>
      <Card
        p="24px"
        pb="44px"
        as={Flex}
        flex="0 0 auto"
        flexDirection="column"
        width="max-content"
      >
        <Box width="100%" textAlign="left">
          <Text typography="h4" bold>
            Access Requests Flow
          </Text>
          <Text typography="subtitle1" mb={5}>
            To learn more about access requests, take a look at Teleport
            Documentation
          </Text>
        </Box>
        <Flex gap="24px" flexWrap="wrap" style={{ position: 'relative' }}>
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
            `}
          >
            <ButtonLockedFeature>
              Unlock Access Requests with Teleport Enterprise
            </ButtonLockedFeature>
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

const StepCardContainer = styled(Flex)(
  ({ theme }) => `
  background-color: ${theme.colors.spotBackground[0]};
  padding: 24px;
  width: 340px;
  height: 420px;
  border-radius: 8px;
  flex-direction: column;
  align-items: center;
  :last-child: {
    margin: auto;
    background-color: red;
  }
`
);

const ImageContainer = styled(Flex)(
  ({ theme }) => `
  width: 293px;
  align-items: center;
  height: 240px;
  border-radius: 8px;
  background-color: ${theme.colors.spotBackground[1]};
`
);
