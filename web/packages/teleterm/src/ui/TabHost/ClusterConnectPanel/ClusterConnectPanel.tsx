import React from 'react';
import { ButtonPrimary, Flex, Text } from 'design';
import clusterPng from './clusters.png';
import Image from 'design/Image';
import { useAppContext } from 'teleterm/ui/appContextProvider';

export function ClusterConnectPanel() {
  const ctx = useAppContext();

  function handleConnect() {
    ctx.commandLauncher.executeCommand('cluster-connect', {});
  }

  return (
    <Flex
      m="auto"
      width="370px"
      pb={6}
      flexDirection="column"
      alignItems="center"
    >
      <Image width="120px" src={clusterPng} mb={3} />
      <Text typography="h3" bold mb={2}>
        Connect a Cluster
      </Text>
      <Text color="text.secondary" mb={3} textAlign="center">
        Connect an existing Teleport cluster <br/> to start using Teleport Connect.
      </Text>
      <ButtonPrimary size="large" onClick={handleConnect}>Connect</ButtonPrimary>
    </Flex>
  );
}
