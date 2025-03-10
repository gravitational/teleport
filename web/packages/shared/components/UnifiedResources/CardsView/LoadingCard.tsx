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

import { useState } from 'react';

import { Box, Flex } from 'design';
import { ShimmerBox } from 'design/ShimmerBox';

export function LoadingCard() {
  const [randomizedSize] = useState(() => ({
    name: randomNum(100, 30),
    description: randomNum(90, 40),
    labels: new Array(randomNum(4, 0)),
  }));

  return (
    <Flex gap={2} alignItems="start" height="106px" p={3}>
      {/* Checkbox */}
      <ShimmerBox height="16px" width="16px" />
      {/* Image */}
      <ShimmerBox height="45px" width="45px" />
      <Box flex={1}>
        <Flex gap={2} mb={2} justifyContent="space-between">
          {/* Name */}
          <ShimmerBox
            height="24px"
            css={`
              flex-basis: ${randomizedSize.name}%;
            `}
          />
          {/* Action button */}
          <ShimmerBox height="24px" width="90px" />
        </Flex>
        {/* Description */}
        <ShimmerBox
          height="16px"
          width={`${randomizedSize.description}%`}
          mb={2}
        />
        {/* Labels */}
        <Flex gap={2}>
          {randomizedSize.labels.fill(null).map((_, i) => (
            <ShimmerBox key={i} height="16px" width="60px" />
          ))}
        </Flex>
      </Box>
    </Flex>
  );
}

function randomNum(min: number, max: number) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}
