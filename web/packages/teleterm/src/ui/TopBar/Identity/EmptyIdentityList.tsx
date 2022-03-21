import React from 'react';
import { ButtonPrimary, Flex, Text } from 'design';
import { useAppContext } from 'teleterm/ui/appContextProvider';

export function EmptyIdentityList() {
  const ctx = useAppContext();

  function handleConnect() {
    ctx.commandLauncher.executeCommand('cluster-connect', {});
  }

  return (
    <Flex m="auto" flexDirection="column" alignItems="center">
      <Text typography="h6" bold mb={2}>
        Connect to a Cluster
      </Text>
      <ButtonPrimary size="small" onClick={handleConnect}>
        Connect
      </ButtonPrimary>
    </Flex>
  );
}