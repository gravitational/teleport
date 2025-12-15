/**
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

import Card from '.';
import { Flex, Text } from '..';

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
