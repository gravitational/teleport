import { useTheme } from 'styled-components';

import { Box, Flex, H2, H3, Subtitle2, SyncStamp, Text } from 'design';

import { GetUsageResponse } from 'e-teleport/services/cloud/v1/tenants_pb';
import { usageUnixInMilliseconds } from 'e-teleport/UsageSummary/helpers';

import { CyclesContainer, getPercentage } from '../shared';
import { UsageBar } from '../UsageBar';

export type Section = {
  name: string;
  info: string;
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
  const { aggregateCount, usageHistory, missingEntitlements, usageUpdatedAt } =
    usageResponse;
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
    igmau: 0,
    mwi: 0,
  };

  const sections: Section[] = [
    {
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
    },
    {
      name: 'Machine and Workload Identities',
      info: 'Improve infrastructure resiliency by securing access to systems  and data between machines & workloads.',
      enabled: true, // always enabled
      usage: [
        {
          name: 'Machine & Workload Identities',
          used: usage.mwi,
          percentage: getPercentage({
            usage: usage.mwi,
            limit: usageLimits.mwi,
            calibration: calibrationPeriod,
          }),
          limit: usageLimits.mwi,
          customerUsed: customerUsage.mwi || 0,
          customerPercentage: getPercentage({
            usage: customerUsage.mwi,
            limit: usageLimits.mwi,
            calibration: calibrationPeriod,
          }),
        },
      ],
    },
    {
      name: 'Identity Governance',
      info: 'Harden your infrastructure with identity governance and security.',
      enabled: !missingEntitlements.includes('Identity'),
      ctaUrl: '',
      usage: [
        {
          name: 'Monthly Active Users (MAU)',
          used: usage.igmau,
          percentage: getPercentage({
            usage: usage.igmau,
            limit: usageLimits.igmau,
            calibration: calibrationPeriod,
          }),
          limit: usageLimits.igmau,
          customerUsed: customerUsage.igmau || 0,
          customerPercentage: getPercentage({
            usage: customerUsage.igmau,
            limit: usageLimits.igmau,
            calibration: calibrationPeriod,
          }),
        },
      ],
    },
    {
      name: 'Identity Security',
      info: 'Secure identities and access policies across all of your infrastructure. Eliminate shadow access and blind spots.',
      enabled: !missingEntitlements.includes('Policy'),
      ctaUrl: '',
      usage: [
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
    },
  ];

  return (
    <Box>
      <H2>
        Current Cycle: {startFormatted} - {endFormatted}
      </H2>
      <Subtitle2 color={theme.colors.text.slightlyMuted} mt="2">
        Monthly usage will reset at the end of this cycle
      </Subtitle2>
      <Flex gap="3" flexWrap="wrap" my="3">
        {sections.map(section => (
          <CyclesContainer
            key={section.name}
            data-testid={section.name}
            enabled={section.enabled}
          >
            <H3>{section.name}</H3>
            <Text color="text.slightlyMuted" mt="2" fontWeight={300}>
              {section.info}
            </Text>
            <Box mt="4">
              <UsageBar
                section={section}
                calibrating={calibrationPeriod}
                aggregate={aggregate}
              />
            </Box>
          </CyclesContainer>
        ))}
      </Flex>
      <SyncStamp date={new Date(usageUnixInMilliseconds(usageUpdatedAt))}>
        | Updated every 12 hours
      </SyncStamp>
    </Box>
  );
};
