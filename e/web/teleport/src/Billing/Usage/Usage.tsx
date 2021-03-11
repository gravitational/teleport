import React from 'react';
import { Text, Flex, Box, Alert, Indicator } from 'design';
import CardEmpty from 'teleport/components/CardEmpty';
import useTeleportE from 'e-teleport/useTeleportE';
import BillingCycle from './BillingCycle';
import useUsage, { State } from './useUsage';

export default function Container() {
  const ctx = useTeleportE();
  const state = useUsage(ctx);
  return <Usage {...state} />;
}

export function Usage({ attempt, balance, productName, cycles }: State) {
  if (attempt.status === 'processing') {
    return (
      <Box textAlign="center" m={10}>
        <Indicator />
      </Box>
    );
  }

  if (attempt.status === 'failed') {
    return <Alert kind="danger" children={attempt.statusText} />;
  }

  if (!cycles[0]) {
    return <CardEmpty>You have no usage reports</CardEmpty>;
  }

  return (
    <Flex maxWidth="900px" flexDirection="column">
      <Text typography="h3" mb={3}>
        {productName}
      </Text>
      <Flex
        borderRadius="2"
        px={3}
        mb={3}
        bg="primary.light"
        alignItems="center"
      >
        <Text typography="h4" mr={3}>
          Account Balance:
        </Text>
        <Text typography="h4">{balance}</Text>
      </Flex>
      <BillingCycle balance={balance} cycle={cycles[0]} />
    </Flex>
  );
}
