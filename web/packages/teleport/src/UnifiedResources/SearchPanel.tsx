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

import { AdvancedSearchToggle } from 'shared/components/AdvancedSearchToggle';

import useServersideSearchPanel, {
  SearchPanelState,
  HookProps,
} from 'teleport/components/ServersideSearchPanel/useServerSideSearchPanel';

import { SearchInput } from './SearchInput';

export default function Container(props: HookProps) {
  const state = useServersideSearchPanel(props);
  return <SearchPanel {...state} />;
}

// Adapted from teleport.components.ServersideSearchPanel
export function SearchPanel({
  searchString,
  setSearchString,
  isAdvancedSearch,
  setIsAdvancedSearch,
  onSubmitSearch,
}: SearchPanelState) {
  function onToggle() {
    setIsAdvancedSearch(wasAdvancedSearch => !wasAdvancedSearch);
  }

  return (
    <Flex as="form" className="SearchPanel" onSubmit={onSubmitSearch} mb={3}>
      <SearchInput searchValue={searchString} setSearchValue={setSearchString}>
        <AdvancedSearchToggle
          isToggled={isAdvancedSearch}
          onToggle={onToggle}
          px={4}
        />
      </SearchInput>
    </Flex>
  );
}
