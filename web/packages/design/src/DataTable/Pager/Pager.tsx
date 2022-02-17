import React from 'react';
import { Flex, Text } from 'design';
import { CircleArrowLeft, CircleArrowRight } from 'design/Icon';
import { StyledArrowBtn, StyledFetchMoreBtn } from './StyledPager';
import usePager, { State, Props } from './usePager';

export default function Container(props: Props) {
  const state = usePager(props);
  return <Pager {...state} />;
}

export function Pager({
  nextPage,
  prevPage,
  isNextDisabled,
  isPrevDisabled,
  from,
  to,
  count,
  onFetchMore,
  fetchStatus,
}: State) {
  const isFetchingEnabled = onFetchMore && fetchStatus !== 'disabled';
  return (
    <Flex>
      <Flex alignItems="center" mr={2}>
        <Text typography="body2" color="primary.contrastText" mr={1}>
          SHOWING <strong>{from + 1}</strong> - <strong>{to + 1}</strong> of{' '}
          <strong>{count}</strong>
        </Text>
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
