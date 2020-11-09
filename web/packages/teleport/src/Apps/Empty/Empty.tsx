/*
Copyright 2020 Gravitational, Inc.

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
import { Text, Box, Flex } from 'design';
import Card from 'design/Card';
import Image from 'design/Image';
import ButtonAdd from './../ButtonAdd';
import { emptyPng } from './assets';

export default function Empty(props: Props) {
  return (
    <Card maxWidth="700px" mx="auto" py={4} as={Flex} alignItems="center">
      <Box mx="4">
        <Image width="180px" src={emptyPng} />
      </Box>
      <Box>
        <Box pr={4} mb={6}>
          <Text typography="h6" mb={3}>
            SECURE YOUR FIRST APPLICATION
          </Text>
          <Text mb={3}>
            Teleport Application Access provides secure access to internal
            applications without the need for a VPN and with the auditability
            and control of Teleport.
          </Text>
        </Box>
        <ButtonAdd
          isEnterprise={props.isEnterprise}
          canCreate={props.canCreate}
          onClick={props.onCreate}
          mb="2"
          mx="auto"
          width="240px"
          kind="primary"
        />
      </Box>
    </Card>
  );
}

type Props = {
  isEnterprise: boolean;
  canCreate: boolean;
  onCreate(): void;
};
