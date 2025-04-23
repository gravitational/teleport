/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

/*eslint import/namespace: ['error', { allowComputed: true }]*/

import { Box, Flex } from '..';
import * as colors from './palette';
import { getContrastText } from './themes/sharedStyles';

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
