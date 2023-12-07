import React from 'react';
import { Box, Flex, Text } from 'design';
import styled, { useTheme } from 'styled-components';

import cfg from 'shared/config';

import Link from 'design/Link';

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';

import { CtaEvent } from 'teleport/services/userEvent';
import useTeleport from 'teleport/useTeleport';

import { getSalesURL } from 'teleport/services/sales';

import { displayUnixShortDate } from 'shared/services/loc/loc';

import { CycleUsage } from 'e-teleport/Billing/types';

import {
  StripeUsage,
  NonBillableSummaryInformation,
} from 'e-teleport/services/cloud';

import { Usage } from '../common/Usage';
import { UpdatedAtDisplay } from '../common/UpdatedAtDisplay';

// todo (michellescripts) pull usage max/included values from the subscription as part of https://github.com/gravitational/cloud/issues/3536
const incTIA = 50000;
const maxTIA = 300000;

const incTPR = 50;
const maxTPR = 1000;

const maxMAU = 30;
const mauRate = 15;

export interface CycleProps {
  currentUsage: StripeUsage;
  productName: string;
  stripeMissingPaymentMethod: boolean;
  stripeTrialEnd: number;
  usageUpdatedAt: number;
  nonBillableUsage: NonBillableSummaryInformation;
}

export const Cycle = ({
  currentUsage: { periodStart, periodEnd, usageMau, usagePr, usageTia },
  productName,
  stripeMissingPaymentMethod,
  stripeTrialEnd,
  usageUpdatedAt,
  nonBillableUsage: { trustedDeviceUsage, accessRequestUsage },
}: CycleProps) => {
  const theme = useTheme();
  const start = displayUnixShortDate(periodStart);
  const end = displayUnixShortDate(periodEnd);

  const mau = usageMau || 0;
  const tia = usageTia || 0;
  const pr = usagePr || 0;
  const mad = trustedDeviceUsage?.devicesInUse || 0;
  const maxMAD = trustedDeviceUsage?.devicesUsageLimit || 5;
  const ar = accessRequestUsage?.monthlyUsed || 0;
  const maxAR = accessRequestUsage?.monthlyLimit || 0;

  const usage: CycleUsage[] = [
    {
      name: 'Active Users',
      total: mau,
      percentage: Math.round((mau / maxMAU) * 100),
      percentageMax: maxMAU,
      hardMax: maxMAU,
      hasFreeTier: false,
      info: 'Any unique human or machine user, local or SSO username or email with recorded activity during a month.',
    },
    {
      name: 'Teleport Identity Authorizations',
      total: tia,
      percentage: Math.round((tia / incTIA) * 100),
      percentageMax: incTIA,
      hardMax: maxTIA,
      hasFreeTier: true,
      info: 'The authentication or authorization by Teleport of a client connection, API request, SSH session or any other activity related to a human user or service interaction.',
    },
    {
      name: 'Teleport Protected Resources',
      total: pr,
      percentage: Math.round((pr / incTPR) * 100),
      percentageMax: incTPR,
      hardMax: maxTPR,
      hasFreeTier: true,
      info: 'Any unique resource such as a Kubernetes cluster, SSH server, database instance or serverless endpoint, that has registered itself with the Teleport cluster and is protected by Teleport.',
    },
  ];

  // Usage info for entities where accounting does not correspond to a billing cycle
  const monthlyUsage: CycleUsage[] = [
    {
      name: 'Access Requests',
      total: ar,
      percentage: Math.round((ar / maxAR) * 100),
      percentageMax: maxAR,
      hardMax: maxAR,
      hasFreeTier: true,
      info: 'Access requests created this month. Upgrade to enterprise plan for more than five access requests per month.',
    },
    {
      name: 'Trusted Devices',
      total: mad,
      percentage: Math.round((mad / maxMAD) * 100),
      percentageMax: maxMAD,
      hardMax: maxMAD,
      hasFreeTier: true,
      info: 'Unique trusted devices enrolled in Teleport. Upgrade to enterprise plan for more than five devices.',
    },
  ];

  const ctx = useTeleport();

  const getSalesLink = () => {
    const version = ctx.storeUser.state.cluster.authVersion;
    const isEnterprise = ctx.isEnterprise;
    const isTeam = cfg.isTeam;
    return getSalesURL(version, isEnterprise, isTeam);
  };

  return (
    <Box>
      <UsageGroup>
        <Flex alignItems="center" justifyContent="space-between" mr={5}>
          <h2>
            Current Cycle: {start} - {end}
          </h2>
          <UpdatedAtDisplay theme={theme} usageUpdatedAt={usageUpdatedAt} />
        </Flex>
        <Text color={theme.colors.text.secondary}>
          {stripeMissingPaymentMethod
            ? `Your trial will expire on ${displayUnixShortDate(
                stripeTrialEnd
              )}. To maintain access to your Teleport cluster, upgrade to the Teleport ${productName} Plan by adding a payment method.`
            : `Your next invoice will occur on ${end} at a rate of ${mauRate} per active monthly user.`}
        </Text>
        <Flex flexWrap="wrap">
          {usage.map(u => (
            <Usage key={u.name} usage={u} />
          ))}
        </Flex>
        <Text color={theme.colors.text.slightlyMuted} mt="12px">
          <i>
            Your team plan includes a limited amount of free usage. <br />
            If your team exceeds the limit for a given category, your team will
            be charged for the extra use.&nbsp;
            <Link
              color="text.secondary"
              href="https://goteleport.com/teleport-pricing/"
              target="_blank"
            >
              Learn More.
            </Link>{' '}
          </i>
        </Text>
        <hr
          style={{
            margin: '16px auto 16px -40px',
            border: `1px solid ${theme.colors.spotBackground[0]}`,
          }}
        />
        <Flex justifyContent="right" alignItems="center">
          <Text mr={3} typography="paragraph">
            Do you have custom needs?
          </Text>
          <ButtonLockedFeature
            width="196px"
            noIcon
            event={CtaEvent.CTA_UNSPECIFIED}
            mr={5}
          >
            Contact Sales
          </ButtonLockedFeature>
        </Flex>
      </UsageGroup>

      <UsageGroup>
        <h2>
          Monthly allocations:{' '}
          {new Date().toLocaleString('default', { month: 'long' })}
        </h2>
        <Flex flexWrap="wrap">
          {monthlyUsage.map(u => (
            <Usage key={u.name} usage={u} />
          ))}
        </Flex>
        <Text color={theme.colors.text.slightlyMuted} mt="12px">
          <i>
            We can not increase monthly allocations in the Team plan.{' '}
            <Link color="text.secondary" href={getSalesLink()} target="_blank">
              Contact sales
            </Link>{' '}
            to unlock unlimited access requests.
          </i>
        </Text>
      </UsageGroup>
    </Box>
  );
};

const UsageGroup = styled(Box)`
  background: ${({ theme }) => theme.colors.levels.surface};
  border-radius: 8px;
  margin: 20px 0 0;
  padding: 20px 0 20px 40px;
`;
