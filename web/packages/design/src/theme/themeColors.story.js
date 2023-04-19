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

import { Flex, Box, Text } from '..';

import theme from './theme';

export default {
  title: 'Design/Theme/Colors',
};

export const Colors = () => {
  return (
    <Flex flexDirection="column" bg="gray" p="4">
      <Text mb="2" typography="h3" bold>
        Brand colors
      </Text>
      <Flex flexDirection="column">
        <Text mb="2" typography="h4">
          1. Primary
        </Text>
        <ColorBox mb="4" colors={theme.colors.primary} themeType="primary" />
        <Text mb="2" typography="h4">
          2. Secondary
        </Text>
        <ColorBox
          mb="4"
          colors={theme.colors.secondary}
          themeType="secondary"
        />
        <Text mb="2" typography="h4">
          3. Text
        </Text>
        <ColorBox mb="4" colors={theme.colors.text} themeType="text" />
        <Text mb="2" typography="h4">
          4. Error
        </Text>
        <ColorBox mb="4" colors={theme.colors.error} themeType="error" />
        <Text mb="2" typography="h4">
          5. Action
        </Text>
        <ColorBox mb="4" colors={theme.colors.action} themeType="action" />
      </Flex>
      <Text mb="2" typography="h3" bold>
        Other colors
      </Text>
      <ColorBox colors={theme.colors} />
    </Flex>
  );
};

function ColorBox({ colors, themeType = null, ...styles }) {
  const list = Object.keys(colors).map(key => {
    const fullPath = themeType
      ? `theme.colors.${themeType}.${key}`
      : `theme.colors.${key}`;

    return (
      <Flex flexWrap="wrap" key={key} width="260px" mb={3}>
        <Box>{fullPath}</Box>
        <Box width="100%" height="50px" bg={colors[key]} p={3} mr={3} />
      </Flex>
    );
  });

  return (
    <Flex flexWrap="wrap" {...styles}>
      {list}
    </Flex>
  );
}
