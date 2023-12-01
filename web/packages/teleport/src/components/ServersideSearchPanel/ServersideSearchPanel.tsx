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

import React from 'react';
import { PageIndicatorText } from 'shared/components/Search';
import { Box, Flex } from 'design';
import { StyledPanel } from 'design/DataTable';
import InputSearch from 'design/DataTable/InputSearch';

import { AdvancedSearchToggle } from 'shared/components/AdvancedSearchToggle';

import { PageIndicators } from 'teleport/components/hooks/useServersidePagination';

import useServersideSearchPanel, {
  SearchPanelState,
  HookProps,
} from './useServerSideSearchPanel';

interface ComponentProps {
  pageIndicators: PageIndicators;
  disabled?: boolean;
}

export interface Props extends HookProps, ComponentProps {}

export default function Container(props: Props) {
  const { pageIndicators, disabled, ...hookProps } = props;
  const state = useServersideSearchPanel(hookProps);
  return (
    <ServersideSearchPanel
      {...state}
      pageIndicators={pageIndicators}
      disabled={disabled}
    />
  );
}

interface State extends SearchPanelState, ComponentProps {}

export function ServersideSearchPanel({
  searchString,
  setSearchString,
  isAdvancedSearch,
  setIsAdvancedSearch,
  onSubmitSearch,
  pageIndicators,
  disabled = false,
}: State) {
  function onToggle() {
    setIsAdvancedSearch(!isAdvancedSearch);
  }

  return (
    <StyledPanel
      as="form"
      onSubmit={onSubmitSearch}
      borderTopLeftRadius={3}
      borderTopRightRadius={3}
      style={disabled ? { pointerEvents: 'none', opacity: '0.5' } : {}}
    >
      <Flex justifyContent="space-between" alignItems="center" width="100%">
        <Flex style={{ width: '70%' }} alignItems="center">
          <Box width="100%" mr={3}>
            <InputSearch
              searchValue={searchString}
              setSearchValue={setSearchString}
            >
              <AdvancedSearchToggle
                isToggled={isAdvancedSearch}
                onToggle={onToggle}
                px={4}
              />
            </InputSearch>
          </Box>
        </Flex>
        <Flex>
          <PageIndicatorText
            from={pageIndicators.from}
            to={pageIndicators.to}
            count={pageIndicators.totalCount}
          />
        </Flex>
      </Flex>
    </StyledPanel>
  );
}
