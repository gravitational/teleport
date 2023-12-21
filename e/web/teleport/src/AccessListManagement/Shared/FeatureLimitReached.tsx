import React from 'react';
import styled from 'styled-components';
import { Box, Card, Flex, Text } from 'design';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import { CtaEvent } from 'teleport/services/userEvent';
import cfg from 'teleport/config';

export function FeatureLimitReached() {
  let cta = 'Identity Governance & Security';
  if (cfg.isTeam) {
    cta = 'Teleport Enterprise';
  }
  return (
    // to horizontally center an absolutely positioned element
    <Flex
      css={`
        flex-direction: column;
        align-items: center;
      `}
    >
      <FeatureLimitReachedCard>
        <Text mb={3}>
          This cluster has reached its limit for creating access lists. <br />
          Unlock Access List with {cta}
        </Text>
        <ButtonLockedFeature width="200px" event={CtaEvent.CTA_ACCESS_LIST}>
          Contact Sales
        </ButtonLockedFeature>
      </FeatureLimitReachedCard>
    </Flex>
  );
}

const FeatureLimitReachedCard = styled(Card)`
  position: absolute;
  width: 500px;
  padding: ${p => p.theme.space[4]}px;
  z-index: 1;
  text-align: center;
  top: 150px;
`;

export const featureLimitReachedBlurCss = {
  filter: 'blur(2px)',
  pointerEvents: 'none',
  userSelect: 'none',
  position: 'relative',
};

export function FeatureLimitBlurb() {
  return (
    <Box
      mt={4}
      css={`
        text-align: center;
      `}
    >
      <Text color="text.slightlyMuted">
        <i>Your current plan supports 1 free Access List.</i>
      </Text>
      <Text typography="paragraph">
        Want additional Access Lists?{' '}
        <ButtonLockedFeature
          width="176px"
          textLink={true}
          event={CtaEvent.CTA_ACCESS_LIST}
          pl={1}
        >
          Contact Sales
        </ButtonLockedFeature>
      </Text>
    </Box>
  );
}
