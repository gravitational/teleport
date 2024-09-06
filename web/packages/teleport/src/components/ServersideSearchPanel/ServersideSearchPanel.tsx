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
import { Flex } from 'design';
import InputSearch from 'design/DataTable/InputSearch';
import { PageIndicatorText } from 'design/DataTable/Pager/PageIndicatorText';

import { AdvancedSearchToggle } from 'shared/components/AdvancedSearchToggle';

import { PageIndicators } from 'teleport/components/hooks/useServersidePagination';

import useServersideSearchPanel, {
  HookProps,
  SearchPanelState,
} from './useServerSideSearchPanel';

interface ComponentProps {
  pageIndicators: PageIndicators;
  disabled?: boolean;
}

export interface Props extends HookProps, ComponentProps {
  bigInputSize?: boolean;
}

export default function Container(props: Props) {
  const {
    pageIndicators,
    disabled,
    bigInputSize = false,
    ...hookProps
  } = props;
  const state = useServersideSearchPanel(hookProps);
  return (
    <ServersideSearchPanel
      {...state}
      pageIndicators={pageIndicators}
      disabled={disabled}
      bigInputSize={bigInputSize}
    />
  );
}

interface State extends SearchPanelState, ComponentProps {
  bigInputSize?: boolean;
}

export function ServersideSearchPanel({
  searchString,
  setSearchString,
  isAdvancedSearch,
  setIsAdvancedSearch,
  onSubmitSearch,
  pageIndicators,
  disabled = false,
  bigInputSize = false,
}: State) {
  function onToggle() {
    setIsAdvancedSearch(!isAdvancedSearch);
  }

  return (
    <Flex
      as="form"
      onSubmit={onSubmitSearch}
      alignItems="center"
      justifyContent="space-between"
      width="100%"
      style={disabled ? { pointerEvents: 'none', opacity: '0.5' } : {}}
    >
      <InputSearch
        searchValue={searchString}
        setSearchValue={setSearchString}
        bigInputSize={bigInputSize}
      >
        <AdvancedSearchToggle
          isToggled={isAdvancedSearch}
          onToggle={onToggle}
          px={4}
        />
      </InputSearch>
      <Flex justifyContent="flex-end" mr={2} mb={1} mt={2}>
        <PageIndicatorText
          from={pageIndicators.from}
          to={pageIndicators.to}
          count={pageIndicators.totalCount}
        />
      </Flex>
    </Flex>
  );
}
