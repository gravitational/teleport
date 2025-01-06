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
import styled from 'styled-components';

import { Box, Flex } from 'design';
import { ShimmerBox } from 'design/ShimmerBox';

export function LoadingListItem() {
  const [randomizedSize] = useState(() => ({
    name: randomNum(95, 40),
    description: randomNum(65, 25),
    type: randomNum(80, 60),
    address: randomNum(90, 50),
  }));

  return (
    <LoadingListItemWrapper>
      {/* Image */}
      <ShimmerBox
        height="32px"
        width="32px"
        css={`
          grid-area: image;
        `}
      />
      {/* Name and description */}
      <Flex
        flexDirection="column"
        gap={1}
        css={`
          grid-area: name;
        `}
      >
        <ShimmerBox height="14px" width={`${randomizedSize.name}%`} />
        <ShimmerBox height="10px" width={`${randomizedSize.description}%`} />
      </Flex>
      <ShimmerBox
        css={`
          grid-area: type;
        `}
        height="18px"
        width={`${randomizedSize.type}%`}
      />

      <ShimmerBox
        css={`
          grid-area: address;
        `}
        height="18px"
        width={`${randomizedSize.address}%`}
      />

      <ShimmerBox
        css={`
          grid-area: button;
        `}
        height="24px"
        width="90px"
      />
    </LoadingListItemWrapper>
  );
}

function randomNum(min: number, max: number) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

const LoadingListItemWrapper = styled(Box)`
  height: 58px;
  min-width: 100%;

  display: grid;
  align-items: center;
  column-gap: ${props => props.theme.space[3]}px;
  grid-template-columns: 36px 2fr 1fr 1fr 90px;
  grid-template-areas: 'image name type address button';
  padding-right: ${props => props.theme.space[3]}px;
  padding-left: ${props => props.theme.space[3]}px;
`;
