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
      <Text fontSize={1} bold>
        No root cluster connected
      </Text>
      <Text
        typography="subtitle2"
        fontWeight="regular"
        color="text.secondary"
        mb={3}
        textAlign="center"
        fontSize={1}
      >
        Lorem ipsum dolor sit amet, consectetur adipiscing elit
      </Text>
      <ButtonPrimary size="small" onClick={props.onConnect}>
        Connect
      </ButtonPrimary>
    </Flex>
  );
}
