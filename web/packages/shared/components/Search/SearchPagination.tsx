import React from 'react';
import { Flex } from 'design';
import { StyledPanel } from 'design/DataTable/StyledTable';
import { StyledArrowBtn } from 'design/DataTable/Pager/StyledPager';
import { CircleArrowLeft, CircleArrowRight } from 'design/Icon';

export function SearchPagination({ prevPage, nextPage }: Props) {
  return (
    <StyledPanel borderBottomLeftRadius={3} borderBottomRightRadius={3}>
      <Flex justifyContent="flex-end" width="100%">
        <Flex alignItems="center" mr={2}></Flex>
        <Flex>
          <StyledArrowBtn
            onClick={prevPage}
            title="Previous page"
            disabled={!prevPage}
            mx={0}
          >
            <CircleArrowLeft fontSize="3" />
          </StyledArrowBtn>
          <StyledArrowBtn
            ml={0}
            onClick={nextPage}
            title="Next page"
            disabled={!nextPage}
          >
            <CircleArrowRight fontSize="3" />
          </StyledArrowBtn>
        </Flex>
      </Flex>
    </StyledPanel>
  );
}

type Props = {
  prevPage?: () => void;
  nextPage?: () => void;
};
