import React from 'react';
import { Box, Flex, Text } from 'design';
import styled, { useTheme } from 'styled-components';

import { UsageSummary } from 'e-teleport/services/cloud/v1/tenants_pb';
import { CycleUsage } from 'e-teleport/UsageSummary/types';

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
  const mauCalibrationPeriod =
    hasCloudAnonymizationKey &&
    salesforceIdUpdatedAt < cycleEnd &&
    salesforceIdUpdatedAt > cycleStart;

  const usage: CycleUsage[] = [
    {
      name: 'Active Users',
      total: mau.cycleCount,
      percentage: mauCalibrationPeriod
        ? 100
        : ~~Math.round((mau.cycleCount / mau.maximum) * 100),
      percentageMax: mau.maximum,
      hardMax: mau.maximum,
      hasFreeTier: false,
      info: 'Any unique human or machine user, local or SSO username or email with recorded activity during a month.',
      calibrating: mauCalibrationPeriod,
    },
    {
      name: 'Teleport Protected Resources',
      total: tpr.cycleCount,
      percentage: ~~Math.round((tpr.cycleCount / tpr.maximum) * 100),
      percentageMax: tpr.maximum,
      hardMax: tpr.maximum,
      hasFreeTier: false,
      info: 'Any unique resource such as a Kubernetes cluster, SSH server, database instance or serverless endpoint, that has registered itself with the Teleport cluster and is protected by Teleport.',
      calibrating: false, // never in a calibration state
    },
  ];

  return (
    <UsageGroup pb={3}>
      <Flex alignItems="center" justifyContent="space-between" mr={5}>
        <h2>
          Current Cycle: {cycleStartFormatted} - {cycleEndFormatted}
        </h2>
        <UpdatedAtDisplay
          theme={theme}
          usageUpdatedAt={usageUpdatedAt}
          usageUpdatedAtFormatted={usageUpdatedAtFormatted}
        />
      </Flex>
      <Text>Monthly usage will reset at the end of this cycle.</Text>
      <Flex flexWrap="wrap" mb={4}>
        {usage.map(u => (
          <UsageBar key={u.name} usage={u} />
        ))}
      </Flex>
      {mauCalibrationPeriod && (
        <Text
          typography="body2"
          color={theme.colors.text.slightlyMuted}
          style={{ fontStyle: 'italic' }}
        >
          A change to your account requires a calibration period in order to
          accurately count Active Users. This should resolve itself with the
          start of your next billing cycle.
        </Text>
      )}
    </UsageGroup>
  );
};

const UsageGroup = styled(Box)`
  background-color: ${({ theme }) => theme.colors.levels.surface};
  border-radius: 8px;
  margin: 20px 0 0;
  padding: 20px 0 40px 40px;
`;
