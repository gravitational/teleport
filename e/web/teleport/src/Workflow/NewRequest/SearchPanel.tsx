import React, { useState, useEffect } from 'react';
import styled from 'styled-components';
import { Text, Flex } from 'design';
import { StyledPanel } from 'design/DataTable';
import { PageIndicatorText } from 'design/DataTable/Pager/Pager';
import InputSearch from 'design/DataTable/InputSearch';
import { AgentFilter } from 'teleport/services/agents';
import Toggle from 'teleport/components/Toggle';
import Tooltip from 'teleport/components/ServersideSearchPanel/Tooltip';
import { PredicateDoc } from 'teleport/components/ServersideSearchPanel/ServersideSearchPanel';

export function SearchPanel({
  updateQuery,
  updateSearch,
  pageCount,
  filter,
  showSearchBar,
  disableSearch,
}: {
  updateQuery(s: string): void;
  updateSearch(s: string): void;
  pageCount: { from: number; to: number; total: number };
  filter: AgentFilter;
  showSearchBar: boolean;
  disableSearch: boolean;
}) {
  const [query, setQuery] = useState(filter.search || filter.query || '');
  const [isAdvancedSearch, setIsAdvancedSearch] = useState(!!filter.query);

  useEffect(() => {
    setIsAdvancedSearch(!!filter.query);
    setQuery(filter.search || filter.query || '');
  }, [filter]);

  function onToggle() {
    setQuery('');
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
      as="form"
      onSubmit={handleOnSubmit}
      borderTopLeftRadius={3}
      borderTopRightRadius={3}
    >
      <Flex justifyContent="space-between" alignItems="center" width="100%">
        <Flex style={{ width: '70%' }} alignItems="center">
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
        <PageIndicatorText
          from={pageCount.from}
          to={pageCount.to}
          count={pageCount.total}
        />
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
  &.disabled {
    pointer-events: none;
    opacity: 0.5;
  }
`;
