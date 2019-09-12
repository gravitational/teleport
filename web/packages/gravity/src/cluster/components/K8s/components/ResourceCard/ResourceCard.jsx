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
import { displayDateTime } from 'gravity/lib/dateUtils';
import { Text, Card, Flex, ButtonPrimary } from 'design';
import * as Icons from 'design/Icon';

export default function ResourceCard({onClick, created, name, Icon=Icons.FileCode, buttonTitle='EDIT' }) {
  const displayCreated = displayDateTime(created);
  return (
    <Card width="260px" bg="primary.light" px="4" py="4" mb={4} mr={4}>
      <Flex width="100%" textAlign="center" flexDirection="column" justifyContent="center">
        <Text typography="h6" mb="1">{name}</Text>
        <Text typography="body2" color="text.primary">CREATED: {displayCreated}</Text>
      </Flex>
      <Flex alignItems="center" justifyContent="center" flexDirection="column">
        <Icon fontSize="50px" my={4} />
        <ButtonPrimary mt={3} size="medium" onClick={onClick} block={true}>
          {buttonTitle}
        </ButtonPrimary>
      </Flex>
    </Card>
  );
}