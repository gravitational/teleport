import { differenceInCalendarDays, fromUnixTime } from 'date-fns';
import styled, { useTheme } from 'styled-components';

import {
  Box,
  Flex,
  H2,
  H3,
  Link,
  P2,
  Subtitle2,
  SyncStamp,
  Text,
} from 'design';
import { ArrowSquareOut, Check } from 'design/Icon';

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
  const { aggregateCount, usageHistory, usageUpdatedAt, missingEntitlements } =
    usageResponse;
  const currentCycle = usageHistory[0];
  const {
    usage,
    usageLimits,
    startFormatted,
    endFormatted,
    end,
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

  const features: Feature[] = [
    {
      title: 'Zero Trust Access',
      description:
        'Secure your infrastructure with on-demand, least-privileged access using cryptographic identity and zero trust principles.',
      enabled: true, // always enabled
    },
    {
      title: 'Machine & Workload Identity',
      description:
        'Eliminate static credentials from your CI/CD pipelines and everywhere else that relies on machines and workloads.',
      enabled: true, // always enabled
    },
    {
      title: 'Identity Governance',
      description:
        'Centralize control with policy-based access that keeps teams moving fast, securely.',
      enabled: !missingEntitlements.includes('Identity'),
      disabledLink: 'https://goteleport.com/platform/identity-governance/',
    },
    {
      title: 'Identity Security',
      description:
        'Secure identities and access policies across all of your infrastructure. Eliminate shadow access and blind spots.',
      enabled: !missingEntitlements.includes('Policy'),
      disabledLink: 'https://goteleport.com/platform/identity-security/',
    },
  ];

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
              Current Cycle:
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
        <P2 color={theme.colors.text.slightlyMuted} mb={4}>
          {daysUntil(end)} days left in cycle. The cycle&apos;s usage will be
          calculated at cycle&apos;s close on {endFormatted}.
        </P2>
        <Flex gap={7} width="100%" flexWrap="wrap">
          <UsageBar
            section={section}
            calibrating={calibrationPeriod}
            aggregate={aggregate}
          />
        </Flex>
      </CyclesContainer>
      <FeaturesContainer mt="3" gap={3}>
        {features.map(f => (
          <ClusterFeatureBox key={f.title} {...f} />
        ))}
      </FeaturesContainer>
    </Box>
  );
};

const FeaturesContainer = styled(Flex)`
  flex-direction: column;
  @media screen and (min-width: ${props => props.theme.breakpoints.medium}) {
    flex-direction: row;
  }
`;

type Feature = {
  title: string;
  description: string;
  enabled: boolean;
  disabledLink?: string;
};

const ClusterFeatureBox = ({
  title,
  description,
  enabled,
  disabledLink,
}: Feature) => {
  return (
    <Flex
      flexDirection="column"
      justifyContent="space-between"
      p="4"
      backgroundColor="levels.surface"
      borderWidth="1px"
      borderStyle="solid"
      borderColor="levels.elevated"
      borderRadius="14px"
      style={{ flexGrow: 1, flexBasis: 0 }}
    >
      <Box>
        <H2>{title}</H2>
        <Subtitle2 mt="2" color="text.muted">
          {description}
        </Subtitle2>
      </Box>
      <Box mt={4}>
        <Flex alignItems="center" gap={2}>
          <Flex
            backgroundColor={
              enabled ? 'success.main' : 'buttons.primary.default'
            }
            width="28px"
            height="28px"
            borderRadius="50%"
            justifyContent="center"
          >
            {enabled ? (
              <Check size="small" color="white" />
            ) : (
              <ArrowSquareOut color="white" size="small" />
            )}
          </Flex>
          <Text fontSize={2}>
            {enabled ? (
              'Enabled'
            ) : (
              <Link
                href={disabledLink}
                color="text.main"
                style={{ textDecoration: 'none' }}
              >
                Learn More
              </Link>
            )}
          </Text>
        </Flex>
      </Box>
    </Flex>
  );
};

export function daysUntil(unixTime: number, end: Date = new Date()): number {
  return differenceInCalendarDays(fromUnixTime(unixTime), end);
}
