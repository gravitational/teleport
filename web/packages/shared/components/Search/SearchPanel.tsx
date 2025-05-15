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

import { useEffect, useState, type JSX } from 'react';
import styled from 'styled-components';

import { Flex } from 'design';
import InputSearch from 'design/DataTable/InputSearch';
import { PageIndicatorText } from 'design/DataTable/Pager/PageIndicatorText';
import { AdvancedSearchToggle } from 'shared/components/AdvancedSearchToggle';

import { ResourceFilter } from 'teleport/services/agents';

export function SearchPanel({
  updateQuery,
  updateSearch,
  pageIndicators,
  filter,
  disableSearch,
  hideAdvancedSearch,
  extraChildren,
}: {
  updateQuery(s: string): void;
  updateSearch(s: string): void;
  pageIndicators?: { from: number; to: number; total: number };
  filter: ResourceFilter;
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

  function updateQueryForRefetching(newQuery: string) {
    setQuery(newQuery);

    if (isAdvancedSearch) {
      updateQuery(newQuery);
      return;
    }

    updateSearch(newQuery);
  }

  return (
    <Flex
      justifyContent="space-between"
      alignItems="center"
      width="100%"
      mb={3}
    >
      <Flex style={{ width: '100%' }} alignItems="center">
        <StyledFlex
          mr={3}
          alignItems="center"
          width="100%"
          className={disableSearch ? 'disabled' : ''}
        >
          <InputSearch
            searchValue={query}
            setSearchValue={updateQueryForRefetching}
          >
            {!hideAdvancedSearch && (
              <AdvancedSearchToggle
                isToggled={isAdvancedSearch}
                onToggle={onToggle}
                px={3}
              />
            )}
          </InputSearch>
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
