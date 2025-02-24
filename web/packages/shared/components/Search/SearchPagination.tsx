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

import { Flex } from 'design';
import { StyledArrowBtn } from 'design/DataTable/Pager/StyledPager';
import { StyledPanel } from 'design/DataTable/StyledTable';
import { CircleArrowLeft, CircleArrowRight } from 'design/Icon';

export function SearchPagination({ prevPage, nextPage }: Props) {
  return (
    <StyledPanel>
      <Flex justifyContent="flex-end" width="100%">
        <Flex alignItems="center" mr={2}></Flex>
        <Flex>
          <StyledArrowBtn
            onClick={prevPage}
            title="Previous page"
            disabled={!prevPage}
            mx={0}
          >
            <CircleArrowLeft />
          </StyledArrowBtn>
          <StyledArrowBtn
            ml={0}
            onClick={nextPage}
            title="Next page"
            disabled={!nextPage}
          >
            <CircleArrowRight />
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
