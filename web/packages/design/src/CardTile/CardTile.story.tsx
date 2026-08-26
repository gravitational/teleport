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
import { Link as InternalLink, MemoryRouter } from 'react-router-dom';

import { Text } from '..';
import { CardTile } from './CardTile';

export default {
  title: 'Design/Card/Tile',
};

export const Story = () => (
  <CardTile>
    <Text as="h1" typography="h1">
      Curabitur ullamcorper diam sed ante gravida imperdiet
    </Text>
    <Text>{loremIpsum}</Text>
  </CardTile>
);

export const WithBorder = () => (
  <CardTile withBorder>
    <Text as="h1" typography="h1">
      Curabitur ullamcorper diam sed ante gravida imperdiet
    </Text>
    <Text>{loremIpsum}</Text>
  </CardTile>
);

export const AsLink = () => (
  <MemoryRouter>
    <CardTile as={InternalLink} to="goteleport.com">
      <Text as="h1" typography="h1">
        Curabitur ullamcorper diam sed ante gravida imperdiet
      </Text>
      <Text>{loremIpsum}</Text>
    </CardTile>
  </MemoryRouter>
);

const loremIpsum =
  'Lorem ipsum dolor sit amet, consectetur adipiscing elit. Vivamus ligula leo, dictum ac bibendum ut, dapibus sed ex. Morbi eget posuere arcu. Duis consectetur felis sed consequat dignissim. In id semper arcu, vitae fringilla lacus. Donec justo justo, feugiat eget lobortis eu, venenatis eu nulla. Morbi scelerisque vitae tortor quis tempus. Integer a nulla pellentesque, pellentesque nisl eleifend, dapibus quam. Donec eget odio vel justo efficitur commodo.';
