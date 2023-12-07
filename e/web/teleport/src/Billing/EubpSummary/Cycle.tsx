import React from 'react';
import { Box, Flex, Text } from 'design';
import styled, { useTheme } from 'styled-components';

import { displayUnixShortDate } from 'shared/services/loc/loc';

import { UsageQuota } from 'e-teleport/services/cloud/v1/tenants_pb';
import { CycleUsage } from 'e-teleport/Billing/types';
import { StripeUsage } from 'e-teleport/services/cloud';

import { UpdatedAtDisplay } from '../common/UpdatedAtDisplay';
import { Usage } from '../common/Usage';

export interface CycleProps {
  currentUsage: StripeUsage;
  productName: string;
  stripeMissingPaymentMethod: boolean;
  stripeTrialEnd: number;
  usageUpdatedAt: number;
  usageQuota: UsageQuota.AsObject;
}

export const Cycle = ({
  currentUsage: { periodStart, periodEnd, usageMau, usagePr, usageTia },
  usageUpdatedAt,
  usageQuota: { mauMax, tiaMax, tprMax },
}: CycleProps) => {
  const theme = useTheme();
  const start = displayUnixShortDate(periodStart);
  const end = displayUnixShortDate(periodEnd);

  const mau = usageMau || 0;
  const tia = usageTia || 0;
  const pr = usagePr || 0;

  const usage: CycleUsage[] = [
    {
      name: 'Active Users',
      total: mau,
      percentage: Math.round((mau / mauMax) * 100),
      percentageMax: mauMax,
      hardMax: mauMax,
      hasFreeTier: false,
      info: 'Any unique human or machine user, local or SSO username or email with recorded activity during a month.',
    },
    {
      name: 'Teleport Identity Authorizations',
      total: tia,
      percentage: Math.round((tia / tiaMax) * 100),
      percentageMax: tiaMax,
      hardMax: tiaMax,
      hasFreeTier: false,
      info: 'The authentication or authorization by Teleport of a client connection, API request, SSH session or any other activity related to a human user or service interaction.',
    },
    {
      name: 'Teleport Protected Resources',
      total: pr,
      percentage: Math.round((pr / tprMax) * 100),
      percentageMax: tprMax,
      hardMax: tprMax,
      hasFreeTier: false,
      info: 'Any unique resource such as a Kubernetes cluster, SSH server, database instance or serverless endpoint, that has registered itself with the Teleport cluster and is protected by Teleport.',
    },
  ];

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
          Monthly usage will reset at the end of this cycle.
        </Text>
        <Flex flexWrap="wrap">
          {usage.map(u => (
            <Usage key={u.name} usage={u} />
          ))}
        </Flex>
        {/* TODO: show IGS CTA if IGS is not active */}
      </UsageGroup>
    </Box>
  );
};

const UsageGroup = styled(Box)`
  background: ${({ theme }) => theme.colors.levels.surface};
  border-radius: 8px;
  margin: 20px 0 0;
  padding: 20px 0 40px 40px;
`;
