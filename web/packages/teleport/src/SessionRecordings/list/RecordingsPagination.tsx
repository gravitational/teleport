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

import { useCallback } from 'react';

import { PageIndicatorText } from 'design/DataTable/Pager/PageIndicatorText';
import {
  StyledArrowBtn,
  StyledFetchMoreBtn,
} from 'design/DataTable/Pager/StyledPager';
import Flex from 'design/Flex';
import { CircleArrowLeft, CircleArrowRight, Warning } from 'design/Icon';
import Text from 'design/Text';

export interface RecordingsPaginationProps {
  count: number;
  fetchMoreAvailable?: boolean;
  fetchMoreDisabled?: boolean;
  fetchMoreError?: boolean;
  from: number;
  onFetchMore?: () => void;
  onPageChange: (page: number) => void;
  page: number;
  pageSize: number;
  to: number;
}

export function RecordingsPagination({
  count,
  fetchMoreAvailable,
  fetchMoreDisabled,
  fetchMoreError,
  from,
  onFetchMore,
  onPageChange,
  page,
  pageSize,
  to,
}: RecordingsPaginationProps) {
  const handlePrevious = useCallback(() => {
    if (page === 0) {
      return;
    }

    onPageChange(page - 1);
  }, [page, onPageChange]);

  const maxPage = Math.ceil(count / pageSize) - 1;

  const handleNext = useCallback(() => {
    if (page >= maxPage) {
      return;
    }

    onPageChange(page + 1);
  }, [page, maxPage, onPageChange]);

  const isPrevDisabled = from === 0;
  const isNextDisabled = to >= count - 1;

  const showFetchMore = Boolean(fetchMoreAvailable && onFetchMore);

  return (
    <>
      <Flex
        alignItems="center"
        ml={3}
        data-testid="recordings-pagination-indicator"
      >
        <PageIndicatorText from={from + 1} to={to + 1} count={count} />

        {fetchMoreError && (
          <Flex alignItems="center" ml={2}>
            <Warning mr={2} color="error.main" size="small" />
            <Text color="error.main">An error occurred</Text>
          </Flex>
        )}

        {showFetchMore && (
          <StyledFetchMoreBtn
            disabled={fetchMoreDisabled}
            onClick={onFetchMore}
          >
            {fetchMoreError ? 'Retry' : 'Fetch More'}
          </StyledFetchMoreBtn>
        )}
      </Flex>
      <Flex>
        <StyledArrowBtn
          onClick={handlePrevious}
          title="Previous page"
          disabled={isPrevDisabled}
          mx={0}
        >
          <CircleArrowLeft />
        </StyledArrowBtn>
        <StyledArrowBtn
          ml={0}
          onClick={handleNext}
          title="Next page"
          disabled={isNextDisabled}
        >
          <CircleArrowRight />
        </StyledArrowBtn>
      </Flex>
    </>
  );
}
