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

import { Box, Flex } from 'design';

import { ShimmerBox } from './ShimmerBox';

export default {
  title: 'Design/ShimmerBox',
};

export const Cards = () => {
  return (
    <Flex gap={2} flexWrap="wrap">
      {new Array(10).fill(null).map((_, i) => (
        <LoadingCard key={i} />
      ))}
    </Flex>
  );
};

const LoadingCard = () => {
  return (
    <Box p={2} width="300px">
      <Flex gap={2}>
        <ShimmerBox width="45px" height="45px" />
        <Box flex={1}>
          <ShimmerBox height="20px" mb={2} />
          <ShimmerBox height="12px" />
        </Box>
      </Flex>
    </Box>
  );
};
