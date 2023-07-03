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
import { AgentFilter } from 'teleport/services/agents';
import Toggle from 'teleport/components/Toggle';
import Tooltip from 'teleport/components/ServersideSearchPanel/Tooltip';

import { PredicateDoc } from './PredicateDoc';

export function SearchPanel({
  updateQuery,
  updateSearch,
  pageIndicators,
  filter,
  showSearchBar,
  disableSearch,
  extraChildren,
}: {
  updateQuery(s: string): void;
  updateSearch(s: string): void;
  pageIndicators: { from: number; to: number; total: number };
  filter: AgentFilter;
  showSearchBar: boolean;
  disableSearch: boolean;
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
                <ToggleWrapper>
                  <Toggle isToggled={isAdvancedSearch} onToggle={onToggle} />
                  <Text typography="paragraph2">Advanced</Text>
                </ToggleWrapper>
              </InputSearch>
            )}
          </StyledFlex>
          {showSearchBar && (
            <Tooltip>
              <PredicateDoc />
            </Tooltip>
          )}
        </Flex>
        <Flex alignItems="center">
          <PageIndicatorText
            from={pageIndicators.from}
            to={pageIndicators.to}
            count={pageIndicators.total}
          />
          {extraChildren && extraChildren}
        </Flex>
      </Flex>
    </StyledPanel>
  );
}

const ToggleWrapper = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-around;
  padding-right: 16px;
  padding-left: 16px;
  width: 120px;
`;

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
