/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
import styled from 'styled-components';

import { Flex } from 'design';
import { ShimmerBox } from 'design/ShimmerBox';
import { LoadingSkeleton } from 'shared/components/UnifiedResources/shared/LoadingSkeleton';

export function EventListSkeleton() {
  return <LoadingSkeleton count={18} Element={<LoadingEventRow />} />;
}

function LoadingEventRow() {
  const [randomizedSize] = useState(() => ({
    type: randomNum(60, 40),
    description: randomNum(80, 50),
    time: randomNum(100, 80),
  }));

  return (
    <LoadingRow alignItems="center" height="46px" px={3}>
      {/* Type column */}
      <Flex flex="0 0 120px" pr={3}>
        <ShimmerBox
          height="12px"
          css={`
            width: ${randomizedSize.type}%;
          `}
        />
      </Flex>

      {/* Description column */}
      <Flex flex="1" pr={3}>
        <ShimmerBox
          height="12px"
          css={`
            width: ${randomizedSize.description}%;
          `}
        />
      </Flex>

      {/* Created time column */}
      <Flex flex="0 0 180px" pr={3}>
        <ShimmerBox
          height="12px"
          css={`
            width: ${randomizedSize.time}%;
          `}
        />
      </Flex>

      {/* Action buttons column */}
      <Flex flex="0 0 120px" justifyContent="flex-end">
        <ShimmerBox height="12px" width="87px" />
      </Flex>
    </LoadingRow>
  );
}

function randomNum(max: number, min: number) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

const LoadingRow = styled(Flex)`
  border-bottom: 1px solid ${props => props.theme.colors.spotBackground[0]};
`;
