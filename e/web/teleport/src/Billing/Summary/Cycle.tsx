import React from 'react';
import { Box, Flex, Text } from 'design';
import styled, { useTheme } from 'styled-components';

import { displayUnixShortDate } from 'shared/services/loc/loc';

import Link from 'design/Link';

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';

import { CtaEvent } from 'teleport/services/userEvent';

import { ToolTipInfo } from 'shared/components/ToolTip';

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
  nonBillableUsage: { trustedDeviceUsage },
}: CycleProps) => {
  const theme = useTheme();
  const start = displayUnixShortDate(periodStart);
  const end = displayUnixShortDate(periodEnd);

  const mau = usageMau || 0;
  const tia = usageTia || 0;
  const pr = usagePr || 0;
  const mad = trustedDeviceUsage?.devicesInUse || 0;
  const maxMAD = trustedDeviceUsage?.devicesUsageLimit || 5;

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
    {
      name: 'Trusted Devices',
      total: mad,
      percentage: Math.round((mad / maxMAD) * 100),
      percentageMax: maxMAD,
      hardMax: maxMAD,
      hasFreeTier: true,
      info: 'Unique trusted device enrolled in Teleport. Upgrade to enterprise plan for more than five devices.',
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
      borderRadius="8px"
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
      <Flex flexWrap="wrap">
        {usage.map(u => (
          // todo (michellescripts) add info/hover for description  https://github.com/gravitational/cloud/issues/3536
          <Box key={u.name} width="30%" flex="40%" data-testid={u.name}>
            <Flex flexDirection="row" alignItems="center" gap={2}>
              <h3>{u.name}</h3>
              <ToolTipInfo children={<Text>{u.info}</Text>} />
            </Flex>
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
      <Text color={theme.colors.text.slightlyMuted} mt="12px">
        <i>
          Your team plan includes a limited amount of free usage. <br />
          If your team exceeds the limit for a given category<sup>[1]</sup>,
          your team will be charged for the extra use.&nbsp;
          <Link
            color="text.secondary"
            href="https://goteleport.com/teleport-pricing/"
            target="_blank"
          >
            Learn More.
          </Link>{' '}
          <br />
          <Text color={theme.colors.text.slightlyMuted} fontSize="12px">
            [1] Trusted Device limit cannot be increased in the team plan.
            Contact sales for additional devices.
          </Text>
        </i>
      </Text>
      <footer></footer>
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
