import { useTheme } from 'styled-components';

import { Box, Flex, H2, H3, P2, SyncStamp } from 'design';

import { GetUsageResponse } from 'e-teleport/services/cloud/v1/tenants_pb';
import { usageUnixInMilliseconds } from 'e-teleport/UsageSummary/helpers';

import { CyclesContainer, getPercentage } from '../shared';
import { UsageBar } from '../UsageBar';

export type Section = {
  name: string;
  info: string;
  blurb?: string;
  enabled: boolean;
  ctaUrl?: string;
  usage: {
    name: string;
    used: number;
    limit: number;
    percentage: number;
    customerUsed: number;
    customerPercentage: number;
  }[];
};

export interface CycleProps {
  usageResponse: GetUsageResponse;
  // the following are used to create the 'underlay' on the tenant view with customer data
  customer: GetUsageResponse;
  aggregate: boolean;
}

export const Cycle = ({ usageResponse, customer, aggregate }: CycleProps) => {
  const theme = useTheme();
  const { aggregateCount, usageHistory, usageUpdatedAt } = usageResponse;
  const currentCycle = usageHistory[0];
  const {
    usage,
    usageLimits,
    startFormatted,
    endFormatted,
    calibratingAccounts,
  } = currentCycle;
  //  if aggregate (customer) only show calibration if all accounts are calibrating
  const calibrationPeriod = aggregate
    ? calibratingAccounts === aggregateCount
    : calibratingAccounts > 0;
  const customerUsage = customer?.usageHistory[0]?.usage || {
    ztamau: 0,
    tpr: 0,
  };

  const section: Section = {
    name: 'Zero Trust Access',
    info: 'A secure, on-demand, least-privileged access to infrastructure using cryptographic identity and Zero Trust principles.',
    enabled: true, // always enabled
    usage: [
      {
        name: 'Monthly Active Users (MAU)',
        used: usage.ztamau,
        percentage: getPercentage({
          usage: usage.ztamau,
          limit: usageLimits.ztamau,
          calibration: calibrationPeriod,
        }),
        limit: usageLimits.ztamau,
        customerUsed: customerUsage.ztamau || 0,
        customerPercentage: getPercentage({
          usage: customerUsage.ztamau,
          limit: usageLimits.ztamau,
          calibration: calibrationPeriod,
        }),
      },
      {
        name: 'Teleport Protected Resources (TPR)',
        used: usage.tpr,
        percentage: getPercentage({
          usage: usage.tpr,
          limit: usageLimits.tpr,
          calibration: calibrationPeriod,
        }),
        limit: usageLimits.tpr,
        customerUsed: customerUsage.tpr || 0,
        customerPercentage: getPercentage({
          usage: customerUsage.tpr,
          limit: usageLimits.tpr,
          calibration: calibrationPeriod,
        }),
      },
    ],
  };

  return (
    <Box>
      <CyclesContainer
        key={section.name}
        data-testid={section.name}
        enabled={section.enabled}
      >
        <Flex justifyContent="space-between" alignItems="start">
          <Flex flexDirection="column">
            <H3 bold color="text.slightlyMuted">
              Current Usage Cycle:
            </H3>
            <Box my={2}>
              <H2>
                {startFormatted} - {endFormatted}
              </H2>
            </Box>
          </Flex>
          <SyncStamp date={new Date(usageUnixInMilliseconds(usageUpdatedAt))}>
            | Updated every 12 hours
          </SyncStamp>
        </Flex>
        <P2 color={theme.colors.text.slightlyMuted}>
          Monthly usage will reset at the end of this cycle
        </P2>
        <Flex gap={3} width="100%" flexWrap="wrap">
          <UsageBar
            section={section}
            calibrating={calibrationPeriod}
            aggregate={aggregate}
          />
        </Flex>
      </CyclesContainer>
    </Box>
  );
};
