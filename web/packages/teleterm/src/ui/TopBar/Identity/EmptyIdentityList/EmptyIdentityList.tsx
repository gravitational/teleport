import React from 'react';
import { ButtonPrimary, Flex, Text } from 'design';
import Image from 'design/Image';

import clusterPng from './clusters.png';

interface EmptyIdentityListProps {
  onConnect(): void;
}

export function EmptyIdentityList(props: EmptyIdentityListProps) {
  return (
    <Flex
      m="auto"
      flexDirection="column"
      alignItems="center"
      width="200px"
      p={3}
    >
      <Image width="60px" src={clusterPng} />
      <Text fontSize={1} bold mb={2}>
        No cluster connected
      </Text>
      <ButtonPrimary size="small" onClick={props.onConnect}>
        Connect
      </ButtonPrimary>
    </Flex>
  );
}
