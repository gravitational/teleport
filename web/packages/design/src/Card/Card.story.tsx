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

import { Flex, Text } from '..';

import Card from '.';

export default {
  title: 'Design/Card',
};

export const Story = () => (
  <Card p={2}>
    <Text as="h1" typography="h1">
      Curabitur ullamcorper diam sed ante gravida imperdiet
    </Text>
    <Text>{loremIpsum}</Text>
  </Card>
);

export const AsFlex = () => (
  <Card p={8} as={Flex} gap={4} flexWrap="wrap">
    {Array(12)
      .fill(undefined)
      .map((_, i) => (
        <Card
          p={2}
          flex={i % 2 === 0 ? 3 : 1}
          css={`
            min-width: 20ch;
          `}
        >
          <Text>{loremIpsum}</Text>
        </Card>
      ))}
  </Card>
);

const loremIpsum =
  'Lorem ipsum dolor sit amet, consectetur adipiscing elit. Vivamus ligula leo, dictum ac bibendum ut, dapibus sed ex. Morbi eget posuere arcu. Duis consectetur felis sed consequat dignissim. In id semper arcu, vitae fringilla lacus. Donec justo justo, feugiat eget lobortis eu, venenatis eu nulla. Morbi scelerisque vitae tortor quis tempus. Integer a nulla pellentesque, pellentesque nisl eleifend, dapibus quam. Donec eget odio vel justo efficitur commodo.';
