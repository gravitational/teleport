import styled, { useTheme } from 'styled-components';

import { Box, Flex, Text } from 'design';

import { UsageSummary } from 'e-teleport/services/cloud/v1/tenants_pb';
import { CycleUsage } from 'e-teleport/UsageSummary/types';

import { isCalibrationPeriod } from './SummaryPage';
import { UpdatedAtDisplay } from './UpdatedAtDisplay';
import { UsageBar } from './UsageBar';

export interface CycleProps {
  summary: UsageSummary;
}

export const Cycle = ({
  summary: {
    cycleEnd,
    cycleEndFormatted,
    cycleStart,
    cycleStartFormatted,
    mau,
    tpr,
    hasCloudAnonymizationKey,
    salesforceIdUpdatedAt,
    usageUpdatedAt,
    usageUpdatedAtFormatted,
  },
}: CycleProps) => {
  const theme = useTheme();
  const calibrationPeriod = isCalibrationPeriod(
    cycleStart,
    cycleEnd,
    hasCloudAnonymizationKey,
    salesforceIdUpdatedAt
  );

  const usage: CycleUsage[] = [
    {
      name: 'Active Users',
      total: mau.cycleCount,
      percentage: calibrationPeriod
        ? 100
        : ~~Math.round((mau.cycleCount / mau.maximum) * 100),
      percentageMax: mau.maximum,
      hardMax: mau.maximum,
      hasFreeTier: false,
      info: 'Any unique human or machine user, local or SSO username or email with recorded activity during a month.',
      calibrating: calibrationPeriod,
    },
    {
      name: 'Teleport Protected Resources',
      total: tpr.cycleCount,
      percentage: calibrationPeriod
        ? 100
        : ~~Math.round((tpr.cycleCount / tpr.maximum) * 100),
      percentageMax: tpr.maximum,
      hardMax: tpr.maximum,
      hasFreeTier: false,
      info: 'Any unique resource such as a Kubernetes cluster, SSH server, database instance or serverless endpoint, that has registered itself with the Teleport cluster and is protected by Teleport.',
      calibrating: calibrationPeriod,
    },
  ];

  return (
    <UsageGroup pb={3}>
      <TitleContainer>
        <h2>
          Current Cycle: {cycleStartFormatted} - {cycleEndFormatted}
        </h2>
        <UpdatedAtDisplay
          theme={theme}
          usageUpdatedAt={usageUpdatedAt}
          usageUpdatedAtFormatted={usageUpdatedAtFormatted}
        />
      </TitleContainer>
      <Text>Monthly usage will reset at the end of this cycle.</Text>
      <CyclesContainer>
        {usage.map(u => (
          <UsageBar key={u.name} usage={u} />
        ))}
      </CyclesContainer>
      {calibrationPeriod && (
        <Text
          typography="body2"
          color={theme.colors.text.slightlyMuted}
          style={{ fontStyle: 'italic' }}
        >
          A change to your account requires a calibration period in order to
          accurately count Active Users and Teleport Protected Resources. This
          should resolve itself with the start of your next billing cycle.
        </Text>
      )}
    </UsageGroup>
  );
};

const TitleContainer = styled(Flex)`
  justify-content: space-between;
  align-items: flex-start;
  flex-direction: column;
  margin-right: ${({ theme }) => theme.space[5]}px;
  @media screen and (min-width: ${p => p.theme.breakpoints.medium}px) {
    align-items: center;
    flex-direction: row;
  }
`;

const UsageGroup = styled(Box)`
  background-color: ${({ theme }) => theme.colors.levels.surface};
  border-radius: 8px;
  padding: 20px 0 40px 40px;
`;

const CyclesContainer = styled(Flex)`
  flex-wrap: wrap;
  margin-bottom: ${({ theme }) => theme.space[4]}px;
  flex-direction: column;
  @media screen and (min-width: ${p => p.theme.breakpoints.medium}px) {
    flex-direction: row;
  }
`;
