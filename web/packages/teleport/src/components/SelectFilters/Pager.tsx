/**
 * Copyright 2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import styled from 'styled-components';
import Icon, { CircleArrowLeft, CircleArrowRight } from 'design/Icon';
import { Text, Flex } from 'design';

export default function Pager({
  startFrom = 0,
  endAt = 0,
  totalRows = 0,
  onPrev,
  onNext,
}: Props) {
  const isPrevDisabled = totalRows === 0 || startFrom === 0;
  const isNextDisabled = totalRows === 0 || endAt === totalRows;
  const initialStartFrom = totalRows > 0 ? startFrom + 1 : 0;

  return (
    <Flex m={2} justifyContent="flex-end">
      <Flex alignItems="center" ml={2}>
        <Text typography="body2">
          SHOWING <strong>{initialStartFrom}</strong> - <strong>{endAt}</strong>{' '}
          of <strong>{totalRows}</strong>
        </Text>
      </Flex>
      <StyledButtons>
        <button
          onClick={onPrev}
          title="Previous Page"
          disabled={isPrevDisabled}
        >
          <CircleArrowLeft fontSize="3" />
        </button>
        <button onClick={onNext} title="Next Page" disabled={isNextDisabled}>
          <CircleArrowRight fontSize="3" />
        </button>
      </StyledButtons>
    </Flex>
  );
}

type Props = {
  startFrom: number;
  endAt: number;
  totalRows: number;
  onPrev(): void;
  onNext(): void;
};

export const StyledButtons = styled(Flex)`
  button {
    cursor: pointer;
    padding: 0;
    margin: 0 0 0 8px;
    outline: none;
    transition: all 0.3s;
    text-align: center;
    border-radius: 200px;
    border: none;
    background: #fff;

    ${Icon} {
      opacity: 0.8;
      font-size: 20px;
      transition: all 0.3s;
      color: #4b4b4b;
    }

    &:hover:not(:disabled) {
      ${Icon} {
        opacity: 1;
        color: #000;
      }
    }

    &:disabled {
      ${Icon} {
        opacity: 0.35;
      }
    }
  }
`;
