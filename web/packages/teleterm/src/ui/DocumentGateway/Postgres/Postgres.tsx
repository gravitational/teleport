/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { Text, Flex, Box, ButtonSecondary } from 'design';
import TextSelectCopy from 'teleport/components/TextSelectCopy';
import { usePostgres, PostgresProps } from './usePostgres';

export function Postgres(props: PostgresProps) {
  const { psqlConnStr, title, disconnect, gateway } = usePostgres(props);
  return (
    <Box maxWidth="1024px" mx="auto" mt="4" px="5">
      <Flex justifyContent="space-between" mb="4">
        <Text typography="h3" color="text.secondary">
          DB Proxy Connection
        </Text>
        <ButtonSecondary size="small" onClick={disconnect}>
          Close Connection
        </ButtonSecondary>
      </Flex>
      <Text bold>Database</Text>
      <Flex
        bg={'primary.dark'}
        p="2"
        alignItems="center"
        justifyContent="space-between"
        borderRadius={2}
        mb={3}
      >
        <Text>{gateway.protocol}</Text>
      </Flex>
      <Text bold>Host Name</Text>
      <Flex
        bg={'primary.dark'}
        p="2"
        alignItems="center"
        justifyContent="space-between"
        borderRadius={2}
        mb={3}
      >
        <Text>{title}</Text>
      </Flex>
      <Text bold>Local Address</Text>
      <TextSelectCopy
        bash={false}
        bg={'primary.dark'}
        mb={4}
        text={`https://${gateway.localAddress}:${gateway.localPort}`}
      />
      <Text bold>Connect with Psql</Text>
      <TextSelectCopy
        bash={false}
        bg={'primary.dark'}
        mb={6}
        text={psqlConnStr}
      />
      <Text typography="h4" bold mb={3}>
        Access Keys
      </Text>
      <Text bold>CA certificate path</Text>
      <TextSelectCopy
        bash={false}
        bg={'primary.dark'}
        mb={3}
        text={gateway.caCertPath}
      />
      <Text bold>Database access certificate path</Text>
      <TextSelectCopy
        bash={false}
        bg={'primary.dark'}
        mb={3}
        text={gateway.certPath}
      />
      <Text bold>Private Key Path</Text>
      <TextSelectCopy
        bash={false}
        bg={'primary.dark'}
        mb={3}
        text={gateway.keyPath}
      />
    </Box>
  );
}
