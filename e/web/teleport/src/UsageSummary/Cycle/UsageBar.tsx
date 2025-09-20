import styled, { useTheme } from 'styled-components';

import { Box, ButtonSecondary, Flex, Text } from 'design';

import { ProductUsage } from 'e-teleport/UsageSummary/Cycle/Cycle';
import { getSalesURL } from 'teleport/services/sales';
import { CtaEvent } from 'teleport/services/userEvent';
import useTeleport from 'teleport/useTeleport';

export function UsageBar({
  productUsage,
  calibrating,
}: {
  productUsage: ProductUsage;
  calibrating: boolean;
}) {
  const ctx = useTeleport();
  const version = ctx.storeUser.state.cluster.authVersion;

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

  if (!productUsage.enabled) {
    return (
      <Flex justifyContent="space-between">
        <Text color={theme.colors.text.slightlyMuted} fontWeight="300">
          This feature isn&apos;t part of your current plan.
        </Text>
        <ButtonSecondary
          as="a"
          target="blank"
          href={getSalesURL(
            version,
            true,
            CtaEvent.CTA_UNSPECIFIED,
            productUsage.ctaUrl
          )}
        >
          Upgrade Now
        </ButtonSecondary>
      </Flex>
    );
  }

  return productUsage.usages.map(
    ({ percentageMax, percentage, hardMax, total, name }, i) => (
      <BarContainer
        key={name}
        data-testid={name}
        mb={i == productUsage.usages.length - 1 ? '0' : '4'}
      >
        <Flex width="100%" justifyContent="space-between">
          <Text>{name}</Text>
          <Box>
            {calibrating ? (
              <Text style={{ fontStyle: 'italic' }}>Calibrating</Text>
            ) : (
              <>
                {total || 0} of {percentageMax} ({percentage}%)
              </>
            )}
          </Box>
        </Flex>
        <Box mt="3">
          <StyledBar
            percent={Math.min(percentage, 100)}
            color={getColor(total, false, percentageMax, hardMax)}
            calibrating={calibrating}
          />
        </Box>
      </BarContainer>
    )
  );
}

const BarContainer = styled(Box)`
  weight: 300;
`;

const StyledBar = styled.div<{
  percent: number;
  color: string;
  calibrating: boolean;
}>`
  background: ${props => props.theme.colors.spotBackground[0]};
  border-radius: 13px;
  height: 8px;

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
