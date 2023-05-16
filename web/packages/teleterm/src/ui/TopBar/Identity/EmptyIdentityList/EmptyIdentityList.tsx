/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
