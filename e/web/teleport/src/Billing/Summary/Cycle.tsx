import React from 'react';
import { Box, Flex, Text } from 'design';
import styled, { useTheme } from 'styled-components';

import { displayUnixShortDate } from 'shared/services/loc/loc';

import Link from 'design/Link';

import { CycleProps, CycleUsage } from 'e-teleport/Billing/types';

// todo (michellescripts) pull usage max/included values from the subscription as part of https://github.com/gravitational/cloud/issues/3536
const incTIA = 50000;
const maxTIA = 300000;

const incTPR = 50;
const maxTPR = 1000;

const maxMAU = 30;
const mauRate = 15;

export const Cycle = ({
  currentUsage: { periodStart, periodEnd, usageMau, usagePr, usageTia },
  productName,
  stripeMissingPaymentMethod,
  stripeTrialEnd,
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
      percentage: Math.round((mau / maxMAU) * 100),
      percentageMax: maxMAU,
      hardMax: maxMAU,
      hasFreeTier: false,
    },
    {
      name: 'Teleport Identity Authorizations',
      total: tia,
      percentage: Math.round((tia / incTIA) * 100),
      percentageMax: incTIA,
      hardMax: maxTIA,
      hasFreeTier: true,
    },
    {
      name: 'Teleport Protected Resources',
      total: pr,
      percentage: Math.round((pr / incTPR) * 100),
      percentageMax: incTPR,
      hardMax: maxTPR,
      hasFreeTier: true,
    },
  ];

  const getColor = (total, hasFreeTier, freeTierMax, hardMax): string => {
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
    return theme.colors.success;
  };

  return (
    <Box
      bg={theme.colors.levels.surface}
      borderRadius="12px"
      m="20px 0 0 0"
      p="20px 0 20px 40px"
    >
      <h2>
        Current Cycle: {start} - {end}
      </h2>
      <Text color={theme.colors.text.secondary}>
        {stripeMissingPaymentMethod
          ? `Your trial will expire on ${displayUnixShortDate(
              stripeTrialEnd
            )}. To maintain access to your Teleport cluster, upgrade to the Teleport ${productName} Plan by adding a payment method.`
          : `Your next invoice will occur on ${end} at a rate of ${mauRate} per active monthly user.`}
      </Text>
      <Flex>
        {usage.map(u => (
          // todo (michellescripts) add info/hover for description  https://github.com/gravitational/cloud/issues/3536
          <Box key={u.name} width="30%" data-testid={u.name}>
            <h3>{u.name}</h3>
            {u.total} of {u.percentageMax}
            {u.hasFreeTier && ' Included'} ({u.percentage}%)
            <StyledBar
              percent={Math.min(u.percentage, 100)}
              color={getColor(
                u.total,
                u.hasFreeTier,
                u.percentageMax,
                u.hardMax
              )}
            />
          </Box>
        ))}
      </Flex>
      <Text color={theme.colors.text.secondary} mt="12px">
        <i>
          Your team plan includes a limited amount of free usage. <br />
          If your team exceeds the limit for a given category, your team will be
          charged for the extra use.&nbsp;
          <Link
            color="text.secondary"
            href="https://goteleport.com/teleport-pricing/"
            target="_blank"
          >
            Learn More.
          </Link>
        </i>
      </Text>
    </Box>
  );
};

const StyledBar = styled.div<{
  percent: number;
  color: string;
}>`
  background: ${props => props.theme.colors.spotBackground[0]};
  border-radius: 13px;
  height: 20px;
  width: 80%;
  padding: 3px;

  &:after {
    content: '';
    display: block;
    background: ${props => props.color};
    width: ${p => p.percent}%;
    height: 100%;
    border-radius: 9px;
  }
`;
