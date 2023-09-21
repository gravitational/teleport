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
import { Danger } from 'design/Alert';
import { Flex, Text, ButtonPrimary } from 'design';

export function Reconnect(props: {
  kubeId: string;
  statusText: string;
  reconnect: () => void;
}) {
  return (
    <Flex gap={4} flexDirection="column" mx="auto" alignItems="center" mt={100}>
      <Text typography="h5" color="text.main">
        A connection to <strong>{props.kubeId}</strong> has failed.
      </Text>
      <Flex flexDirection="column" alignItems="center" mx="auto">
        <Danger mb={3}>
          <Text textAlign="center" css={'white-space: pre-wrap;'}>
            {props.statusText}
          </Text>
        </Danger>
        <ButtonPrimary width="100px" onClick={props.reconnect}>
          Retry
        </ButtonPrimary>
      </Flex>
    </Flex>
  );
}
