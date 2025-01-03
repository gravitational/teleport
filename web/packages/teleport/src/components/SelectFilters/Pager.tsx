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

import styled from 'styled-components';

import { Flex, Text } from 'design';
import { CircleArrowLeft, CircleArrowRight } from 'design/Icon';

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
        <Text typography="body3">
          Showing <strong>{initialStartFrom}</strong> - <strong>{endAt}</strong>{' '}
          of <strong>{totalRows}</strong>
        </Text>
      </Flex>
      <StyledButtons>
        <button
          onClick={onPrev}
          title="Previous Page"
          disabled={isPrevDisabled}
        >
          <CircleArrowLeft />
        </button>
        <button onClick={onNext} title="Next Page" disabled={isNextDisabled}>
          <CircleArrowRight />
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

    svg {
      opacity: 0.8;
      height: 20px;
      width: 20px;
      transition: all 0.3s;
      color: #4b4b4b;
    }

    &:hover:not(:disabled) {
      svg {
        opacity: 1;
        color: #000;
      }
    }

    &:disabled {
      svg {
        opacity: 0.35;
      }
    }
  }
`;
