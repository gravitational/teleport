import React from 'react';
import { Text, Flex, Box, Alert, Indicator } from 'design';
import CardEmpty from 'teleport/components/CardEmpty';
import useTeleportE from 'e-teleport/useTeleportE';
import useUsage, { State } from './useUsage';
import UsageSummary from './UsageSummary';

export default function Container() {
  const ctx = useTeleportE();
  const state = useUsage(ctx);
  return <Usage {...state} />;
}

export function Usage({ attempt, productName, cycles, yearlyUsages }: State) {
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
      <UsageSummary items={yearlyUsages[0].items} />
    </Flex>
  );
}
