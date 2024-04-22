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

import React, { useState, useEffect } from 'react';
import styled from 'styled-components';
import { Text, Flex } from 'design';
import { StyledPanel } from 'design/DataTable';
import InputSearch from 'design/DataTable/InputSearch';
import { ResourceFilter } from 'teleport/services/agents';

import { AdvancedSearchToggle } from 'shared/components/AdvancedSearchToggle';

export function SearchPanel({
  updateQuery,
  updateSearch,
  pageIndicators,
  filter,
  showSearchBar,
  disableSearch,
  hideAdvancedSearch,
  extraChildren,
}: {
  updateQuery(s: string): void;
  updateSearch(s: string): void;
  pageIndicators?: { from: number; to: number; total: number };
  filter: ResourceFilter;
  showSearchBar: boolean;
  disableSearch: boolean;
  hideAdvancedSearch?: boolean;
  extraChildren?: JSX.Element;
}) {
  const [query, setQuery] = useState(filter.search || filter.query || '');
  const [isAdvancedSearch, setIsAdvancedSearch] = useState(!!filter.query);

  useEffect(() => {
    setIsAdvancedSearch(!!filter.query);
    setQuery(filter.search || filter.query || '');
  }, [filter]);

  function onToggle() {
    setIsAdvancedSearch(!isAdvancedSearch);
  }

  function handleOnSubmit(e) {
    e.preventDefault(); // prevent form default

    if (isAdvancedSearch) {
      updateQuery(query);
      return;
    }

    updateSearch(query);
  }

  return (
    <StyledPanel
      onSubmit={handleOnSubmit}
      borderTopLeftRadius={3}
      borderTopRightRadius={3}
    >
      <Flex justifyContent="space-between" alignItems="center" width="100%">
        <Flex as="form" style={{ width: '70%' }} alignItems="center">
          <StyledFlex
            mr={3}
            alignItems="center"
            width="100%"
            className={disableSearch ? 'disabled' : ''}
          >
            {showSearchBar && (
              <InputSearch searchValue={query} setSearchValue={setQuery}>
                {!hideAdvancedSearch && (
                  <AdvancedSearchToggle
                    isToggled={isAdvancedSearch}
                    onToggle={onToggle}
                    px={3}
                  />
                )}
              </InputSearch>
            )}
          </StyledFlex>
        </Flex>
        <Flex alignItems="center">
          {pageIndicators && (
            <PageIndicatorText
              from={pageIndicators.from}
              to={pageIndicators.to}
              count={pageIndicators.total}
            />
          )}
          {extraChildren}
        </Flex>
      </Flex>
    </StyledPanel>
  );
}

const StyledFlex = styled(Flex)`
  // The timing functions of transitions have been chosen so that the element loses opacity slowly
  // when entering the disabled state but gains it quickly when going out of the disabled state.
  transition: opacity 150ms ease-out;
  &.disabled {
    pointer-events: none;
    opacity: 0.7;
    transition: opacity 150ms ease-in;
  }
`;

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
      style={{ textTransform: 'uppercase' }}
      mr={1}
    >
      Showing <strong>{from}</strong> - <strong>{to}</strong> of{' '}
      <strong>{count}</strong>
    </Text>
  );
}
