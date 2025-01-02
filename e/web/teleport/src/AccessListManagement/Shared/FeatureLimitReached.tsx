import React from 'react';
import styled from 'styled-components';

import { Box, Card, Flex, P1, P2, Text } from 'design';
import { pluralize } from 'shared/utils/text';

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import { CtaEvent } from 'teleport/services/userEvent';

export function FeatureLimitReached() {
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
          Unlock additional Access Lists with Teleport Identity
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

export const featureLimitReachedBlurCss: React.CSSProperties = {
  filter: 'blur(2px)',
  pointerEvents: 'none',
  userSelect: 'none',
  position: 'relative',
};

type FeatureLimitBlurbProps = {
  limit?: number;
};

export function FeatureLimitBlurb({ limit = 1 }: FeatureLimitBlurbProps) {
  if (limit === 0) {
    // unlimited access
    return null;
  }

  const listText = pluralize(limit, 'List');
  return (
    <Box
      mt={4}
      css={`
        text-align: center;
      `}
    >
      <P2 color="text.slightlyMuted">
        <i>
          Your current plan supports {limit} free Access {listText}.
        </i>
      </P2>
      <P1 mt={1}>
        Want additional Access Lists?{' '}
        <ButtonLockedFeature
          width="176px"
          textLink={true}
          event={CtaEvent.CTA_ACCESS_LIST}
          pl={1}
        >
          Contact Sales
        </ButtonLockedFeature>
      </P1>
    </Box>
  );
}
