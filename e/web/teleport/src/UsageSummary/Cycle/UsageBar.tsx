import styled, { useTheme } from 'styled-components';

import { Box, ButtonSecondary, Flex, Text } from 'design';

import { Section } from 'e-teleport/UsageSummary/Cycle/Cycle';

export function UsageBar({
  section,
  calibrating,
  aggregate,
}: {
  section: Section;
  calibrating: boolean;
  aggregate: boolean;
}) {
  const theme = useTheme();

  const getColor = (
    used: number,
    limit: number,
    isTransparent: boolean
  ): string => {
    if (Number(used) >= Number(limit)) {
      return isTransparent
        ? theme.colors.interactive.tonal.danger[2]
        : theme.colors.interactive.solid.danger.default;
    }
    return isTransparent
      ? theme.colors.interactive.tonal.success[2]
      : theme.colors.interactive.solid.success.default;
  };

  if (!section.enabled) {
    return (
      <Flex
        justifyContent="space-between"
        data-testid={`${section.name.toLowerCase().replace(' ', '_')}-cta`}
      >
        <Text color={theme.colors.text.slightlyMuted} fontWeight="300">
          This feature isn&apos;t part of your current plan.
        </Text>
        <ButtonSecondary as="a" target="blank" href="" disabled>
          Upgrade Now
        </ButtonSecondary>
      </Flex>
    );
  }

  return section.usage.map(
    (
      { percentage, limit, used, name, customerUsed, customerPercentage },
      i
    ) => (
      <BarContainer
        key={name}
        data-testid={name}
        mb={i == section.usage.length - 1 ? '0' : '4'}
      >
        <Flex width="100%" justifyContent="space-between">
          <Text>{name}</Text>
          <Box>
            {calibrating ? (
              <Text style={{ fontStyle: 'italic' }}>Calibrating</Text>
            ) : (
              <>
                {used || 0} of {limit || 0} ({percentage}%)
              </>
            )}
          </Box>
        </Flex>
        <Box mt="3" style={{ position: 'relative' }}>
          {!aggregate && customerUsed > 0 && (
            <StyledBar
              percent={Math.min(customerPercentage, 100)}
              color={getColor(customerUsed, limit, true)}
              calibrating={calibrating}
              style={{
                position: 'absolute',
                width: '100%',
              }}
            />
          )}
          <StyledBar
            percent={Math.min(percentage, 100)}
            color={getColor(used, limit, false)}
            calibrating={calibrating}
            style={{
              width: '100%',
              background: theme.colors.spotBackground[0],
            }}
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
