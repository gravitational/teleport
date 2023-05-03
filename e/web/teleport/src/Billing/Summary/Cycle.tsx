import React from 'react';
import { Box, Flex, Text } from 'design';
import styled, { useTheme } from 'styled-components';

import { displayUnixShortDate } from 'shared/services/loc/loc';

import Link from 'design/Link';

import { CycleProps, CycleUsage } from 'e-teleport/Billing/types';

const MAX_TIA = 12000;
const MAX_PR = 50;
const MAX_MAU = 30;

export const Cycle = ({
  currentUsage: { periodStart, periodEnd, usageMau, usagePr, usageTia },
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
      max: MAX_MAU,
      percentage: Math.round((mau / MAX_MAU) * 100),
    },
    {
      name: 'Teleport Identity Authorizations',
      total: tia,
      max: MAX_TIA,
      percentage: Math.round((tia / MAX_TIA) * 100),
    },
    {
      name: 'Teleport Protected Resources',
      total: pr,
      max: MAX_PR,
      percentage: Math.round((pr / MAX_PR) * 100),
    },
  ];

  return (
    <Box
      bg={theme.colors.spotBackground[0]}
      borderRadius="12px"
      m="20px 0 0 0"
      p="20px 0 20px 40px"
    >
      <h2>
        Current Cycle: {start} - {end}
      </h2>
      <Text color={theme.colors.text.secondary}>
        Your next invoice will occur on {end}
      </Text>
      <Flex>
        {usage.map(u => (
          // todo (michellescripts) add info/hover for description  https://github.com/gravitational/cloud/issues/3536
          <Box key={u.name} width="30%" data-testid={u.name}>
            <h3>{u.name}</h3>
            {u.total} of {u.max} ({u.percentage}%)
            <StyledBar percent={u.percentage} />
          </Box>
        ))}
      </Flex>
      <Text color={theme.colors.text.secondary} mt="12px">
        <i>
          Your team plan includes a limited amount of free usage. If your team
          exceeds the limit for a given category, your team will be charged for
          the extra use. Cluster owners are notified if usage approaches or
          exceeds a limit.&nbsp;
          <Link
            color="text.secondary"
            href="https://goteleport.com/teleport-pricing/"
            target="_blank"
          >
            Learn More
          </Link>
        </i>
      </Text>
    </Box>
  );
};

const StyledBar = styled.div<{ percent: number }>`
  background: ${props => props.theme.colors.spotBackground[1]};
  border-radius: 13px;
  height: 20px;
  width: 80%;
  padding: 3px;

  &:after {
    content: '';
    display: block;
    background: ${props => props.theme.colors.success};
    width: ${p => p.percent}%;
    height: 100%;
    border-radius: 9px;
  }
`;
