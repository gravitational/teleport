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

import React from 'react';

import Box from '../Box';

import Flex from './Flex';

export default {
  title: 'Design/Flex',
};

export const Basic = () => (
  <Flex gap={5}>
    <Box width={1 / 2} bg="pink" p={5}>
      Box one
    </Box>
    <Box width={1 / 2} bg="orange" p={5}>
      Box two
    </Box>
  </Flex>
);

export const Wrapped = () => (
  <Flex flexWrap="wrap" gap={2}>
    <Box width={[1, 1 / 2]} bg="pink" p={5}>
      Box one
    </Box>
    <Box width={[1, 1 / 2]} bg="orange" p={5}>
      Box two
    </Box>
  </Flex>
);

export const Justified = () => (
  <Flex justifyContent="space-around">
    <Box width={1 / 3} bg="pink" p={5}>
      Box one
    </Box>
    <Box width={1 / 3} bg="orange" p={5}>
      Box two
    </Box>
  </Flex>
);
