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

import { Flex } from 'design';
import { CircleArrowLeft, CircleArrowRight } from 'design/Icon';

import { StyledArrowBtn } from './StyledPager';

export function ServerSidePager({ nextPage, prevPage }: Props) {
  const isNextDisabled = !nextPage;
  const isPrevDisabled = !prevPage;

  return (
    <Flex justifyContent="flex-end" width="100%">
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

export type Props = {
  nextPage: (() => void) | null;
  prevPage: (() => void) | null;
};
