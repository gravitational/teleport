import styled, { useTheme } from 'styled-components';

import { Box, Flex, Text } from 'design';
import { IconTooltip } from 'design/Tooltip';

import { CycleUsage } from 'e-teleport/UsageSummary/types';

export function UsageBar({
  usage: {
    name,
    info,
    percentageMax,
    percentage,
    hardMax,
    hasFreeTier,
    total,
    calibrating,
  },
}: {
  usage: CycleUsage;
}) {
  const theme = useTheme();
  const getColor = (
    total: number,
    hasFreeTier: boolean,
    freeTierMax: number,
    hardMax: number
  ): string => {
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
    return theme.colors.success.main;
  };

  return (
    <Box key={name} width="30%" flex="40% 0" data-testid={name}>
      <Flex flexDirection="row" alignItems="center" gap={2}>
        <h3>{name}</h3>
        <IconTooltip children={<Text>{info}</Text>} />
      </Flex>
      {calibrating ? (
        <Text style={{ fontStyle: 'italic' }}>Calibrating...</Text>
      ) : (
        <>
          {total} of {percentageMax}
          {hasFreeTier && ' Included'} ({percentage}%)
        </>
      )}
      <StyledBar
        percent={Math.min(percentage, 100)}
        color={getColor(total, hasFreeTier, percentageMax, hardMax)}
        calibrating={calibrating}
      />
    </Box>
  );
}

const StyledBar = styled.div<{
  percent: number;
  color: string;
  calibrating: boolean;
}>`
  background: ${props => props.theme.colors.spotBackground[0]};
  border-radius: 13px;
  height: 20px;
  width: 80%;
  padding: 3px;

  &:after {
    content: '';
    display: block;
    background: ${props =>
      props.calibrating
        ? `repeating-linear-gradient(
        120deg,
        ${props.theme.colors.dataVisualisation.primary.purple}, 
        ${props.theme.colors.dataVisualisation.primary.purple} 20px, 
        ${props.theme.colors.dataVisualisation.secondary.purple} 20px, 
        ${props.theme.colors.dataVisualisation.secondary.purple} 40px
        )`
        : props.color};
    width: ${p => p.percent}%;
    height: 100%;
    border-radius: 9px;
  }
`;
