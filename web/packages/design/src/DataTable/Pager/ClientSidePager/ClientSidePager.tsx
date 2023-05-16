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
          <CircleArrowLeft fontSize="3" />
        </StyledArrowBtn>
        <StyledArrowBtn
          ml={0}
          onClick={nextPage}
          title="Next page"
          disabled={isNextDisabled}
        >
          <CircleArrowRight fontSize="3" />
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
