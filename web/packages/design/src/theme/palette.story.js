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

/*eslint import/namespace: ['error', { allowComputed: true }]*/

import React from 'react';

import { Flex, Box } from '..';

import * as colors from './palette';
import { getContrastText } from './darkTheme';

export default {
  title: 'Design/Theme/Palette',
};

export const Palette = () => <Color />;

function Color() {
  const $mainGroups = mainColors.map(color => (
    <ColorGroup key={color} color={color} showAltPalette={true} />
  ));

  const $neutralColors = neutralColors.map(color => (
    <ColorGroup key={color} color={color} showAltPalette={false} />
  ));

  return (
    <Flex flexWrap="wrap">
      {$mainGroups}
      {$neutralColors}
    </Flex>
  );
}

const neutralColors = ['brown', 'grey', 'blueGrey'];

const mainPalette = [50, 100, 200, 300, 400, 500, 600, 700, 800, 900];

const altPalette = ['A100', 'A200', 'A400', 'A700'];

const mainColors = [
  'red',
  'pink',
  'purple',
  'deepPurple',
  'indigo',
  'blue',
  'lightBlue',
  'cyan',
  'teal',
  'green',
  'lightGreen',
  'lime',
  'yellow',
  'amber',
  'orange',
  'deepOrange',
];

function ColorBlock(colorName, colorValue, colorTitle) {
  const bgColor = colors[colorName][colorValue];
  const textColor = getContrastText(bgColor);

  let boxProps = {
    bg: bgColor,
    color: textColor,
    p: 15,
  };

  if (colorValue.toString().indexOf('A1') === 0) {
    boxProps = {
      ...boxProps,
      mt: 2,
    };
  }

  return (
    <Box {...boxProps} key={colorValue}>
      {colorTitle && <Box mb={3}>{colorName}</Box>}
      <Flex justifyContent="space-between">
        <span>{colorValue}</span>
        <span>{bgColor}</span>
      </Flex>
    </Box>
  );
}

function ColorGroup(options) {
  const { color, showAltPalette } = options;
  const colorsList = mainPalette.map(mainValue => ColorBlock(color, mainValue));

  if (showAltPalette) {
    altPalette.forEach(altValue => {
      colorsList.push(ColorBlock(color, altValue));
    });
  }

  return (
    <Box key={color} m={3} width="200px">
      {ColorBlock(color, 500, true)}
      {colorsList}
    </Box>
  );
}
