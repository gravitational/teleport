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

import React from 'react';

import { Flex, Text } from 'design';
import { CircleArrowLeft, CircleArrowRight } from 'design/Icon';

import { StyledArrowBtn, StyledFetchMoreBtn } from '../StyledPager';

import { useClientSidePager, Props } from './useClientSidePager';

export function ClientSidePager(props: Props) {
  const { nextPage, prevPage, onFetchMore, fetchStatus } = props;
  const { from, to, count, isNextDisabled, isPrevDisabled } =
    useClientSidePager(props);

  const isFetchingEnabled = onFetchMore && fetchStatus !== 'disabled';
  return (
    <Flex justifyContent="flex-end" width="100%">
      <Flex alignItems="center" mr={2}>
        <PageIndicatorText from={from + 1} to={to + 1} count={count} />
        {isFetchingEnabled && (
          <StyledFetchMoreBtn
            disabled={fetchStatus === 'loading'}
            onClick={onFetchMore}
          >
            Fetch More
          </StyledFetchMoreBtn>
        )}
      </Flex>
      <Flex>
        <StyledArrowBtn
          onClick={prevPage}
          title="Previous page"
          disabled={isPrevDisabled}
          mx={0}
        >
          <CircleArrowLeft />
        </StyledArrowBtn>
        <StyledArrowBtn
          ml={0}
          onClick={nextPage}
          title="Next page"
          disabled={isNextDisabled}
        >
          <CircleArrowRight />
        </StyledArrowBtn>
      </Flex>
    </Flex>
  );
}

export function PageIndicatorText({
  from,
  to,
  count,
}: {
  from: number;
  to: number;
  count: number;
}) {
  return (
    <Text
      typography="body2"
      color="text.main"
      mr={1}
      style={{ whiteSpace: 'nowrap', textTransform: 'uppercase' }}
    >
      Showing <strong>{from}</strong> - <strong>{to}</strong> of{' '}
      <strong>{count}</strong>
    </Text>
  );
}
