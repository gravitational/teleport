import React from 'react';

import styled, { useTheme } from 'styled-components';

import { Box, Flex, Text } from 'design';
import { ToolTipInfo } from 'shared/components/ToolTip';

import { CycleUsage } from 'e-teleport/Billing/types';

export function Usage({ usage }: { usage: CycleUsage }) {
  const theme = useTheme();
  const getColor = (total, hasFreeTier, freeTierMax, hardMax): string => {
    // if a product has hit its hard max
    if (total >= hardMax) {
      return theme.colors.error.main;
    }

    // if a product does not contain a free tier, or it has exceeded its free tier
    // then they are being charged for usage
    if (!hasFreeTier || total > freeTierMax) {
      return theme.colors.link;
    }

    // the default behavior is for free tier products within their free tier limits
    return theme.colors.success;
  };

  return (
    <Box key={usage.name} width="30%" flex="40% 0" data-testid={usage.name}>
      <Flex flexDirection="row" alignItems="center" gap={2}>
        <h3>{usage.name}</h3>
        <ToolTipInfo children={<Text>{usage.info}</Text>} />
      </Flex>
      {usage.total} of {usage.percentageMax}
      {usage.hasFreeTier && ' Included'} ({usage.percentage}%)
      <StyledBar
        percent={Math.min(usage.percentage, 100)}
        color={getColor(
          usage.total,
          usage.hasFreeTier,
          usage.percentageMax,
          usage.hardMax
        )}
      />
    </Box>
  );
}

const StyledBar = styled.div<{
  percent: number;
  color: string;
}>`
  background: ${props => props.theme.colors.spotBackground[0]};
  border-radius: 13px;
  height: 20px;
  width: 80%;
  padding: 3px;

  &:after {
    content: '';
    display: block;
    background: ${props => props.color};
    width: ${p => p.percent}%;
    height: 100%;
    border-radius: 9px;
  }
`;
