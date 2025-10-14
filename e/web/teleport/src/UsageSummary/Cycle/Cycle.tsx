import styled, { useTheme } from 'styled-components';

import { Box, Flex, H2, H3, Subtitle2, SyncStamp, Text } from 'design';

import { GetUsageResponse } from 'e-teleport/services/cloud/v1/tenants_pb';
import { usageUnixInMilliseconds } from 'e-teleport/UsageSummary/helpers';

import { UsageBar } from './UsageBar';

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

// MWI_PER_MAU is how many free MWI customers get for each MAU they acquire.
// This value is used to tell if a customer has bought additional MWI and hence is
// in the new price model, or not.
// TODO(mcbattirola): This is temporary and will be removed in fall 2025.
const MWI_PER_MAU = 0.5;

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

  // hasExtraMwi is used to show or hide MWI's blurb, which contains additional info
  // that only customers in the old price model should see.
  // Ideally, this information should come from the Cloud backend, but since this is
  // temporary and all products use the same MWI per MAU (0.5), we hardcoded it here.
  // TODO(mcbattirola): remove this and MWI blurb completely on v19.
  const hasExtraMwi =
    usageLimits.mwi > Math.ceil(MWI_PER_MAU * usageLimits.ztamau);

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
          name: 'MWI',
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
      blurb: hasExtraMwi
        ? null
        : 'MWIs were previously counted as TPRs, but are now part of a new product. Billing will remain consistent with your current contract.',
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
        Current Billing Cycle: {startFormatted} - {endFormatted}
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
            {section.blurb && (
              <Text color="text.muted" mt="4" fontWeight={400}>
                {section.blurb}
              </Text>
            )}
          </CyclesContainer>
        ))}
      </Flex>
      <SyncStamp date={new Date(usageUnixInMilliseconds(usageUpdatedAt))}>
        | Updated every 12 hours
      </SyncStamp>
    </Box>
  );
};

const CyclesContainer = styled(Flex)<{ enabled?: boolean }>`
  flex: 1 1 33%;
  min-width: 420px;
  background-color: ${({ theme }) => theme.colors.levels.surface};
  border-radius: 8px;
  padding: ${({ theme }) => theme.space[4]}px;
  flex-direction: column;
  justify-content: ${({ enabled }) => (enabled ? 'normal' : 'space-between')};
`;

function getPercentage({
  usage,
  limit,
  calibration,
}: {
  usage: number;
  limit: number;
  calibration: boolean;
}): number {
  if (calibration) {
    return 100;
  }
  return ~~Math.round((usage / limit) * 100);
}
