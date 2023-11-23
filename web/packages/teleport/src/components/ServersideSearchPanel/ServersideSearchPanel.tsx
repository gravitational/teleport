/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
