/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import styled from 'styled-components';

import { Box, Flex } from 'design';
import { ShimmerBox } from 'design/ShimmerBox';

export function LoadingListItem() {
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
        <ShimmerBox height="14px" width={`${randomNum(95, 40)}%`} />
        <ShimmerBox height="10px" width={`${randomNum(65, 25)}%`} />
      </Flex>
      <ShimmerBox
        css={`
          grid-area: type;
        `}
        height="18px"
        width={`${randomNum(80, 60)}%`}
      />

      <ShimmerBox
        css={`
          grid-area: address;
        `}
        height="18px"
        width={`${randomNum(90, 50)}%`}
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
