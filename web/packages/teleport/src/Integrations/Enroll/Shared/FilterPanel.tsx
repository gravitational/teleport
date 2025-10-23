/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { type Dispatch, type SetStateAction } from 'react';

import { Box, Flex } from 'design';
import InputSearch from 'design/DataTable/InputSearch';
import { MultiselectMenu } from 'shared/components/Controls/MultiselectMenu';
import { SortMenu } from 'shared/components/Controls/SortMenu';

import {
  integrationTagOptions,
  isIntegrationTag,
  type IntegrationTag,
} from './common';
import {
  IntegrationPickerFilterKey,
  IntegrationPickerSortDirection,
  IntegrationPickerSortKey,
  IntegrationPickerState,
} from './state';

export function FilterPanel({
  state,
  setState,
  tagOptions = integrationTagOptions,
}: {
  state: IntegrationPickerState;
  setState: Dispatch<SetStateAction<IntegrationPickerState>>;
  tagOptions: { value: string; label: string }[];
}) {
  const sortFieldOptions = [{ label: 'Name', value: 'name' }];

  const handleSortChange = (newSort: {
    fieldName: IntegrationPickerSortKey;
    dir: IntegrationPickerSortDirection;
  }) => {
    setState(prev => ({
      ...prev,
      sortKey: newSort.fieldName,
      sortDirection: newSort.dir,
    }));
  };

  const handleFilterChange = (
    key: IntegrationPickerFilterKey,
    value: IntegrationTag[]
  ) => {
    setState(prev => ({
      ...prev,
      filters: {
        ...prev.filters,
        [key]: value,
      },
    }));
  };

  function handleSearchChange(search: string) {
    setState(prev => ({
      ...prev,
      search: search,
    }));
  }

  return (
    <>
      <Box maxWidth="600px" width="100%">
        <InputSearch
          searchValue={state.search}
          setSearchValue={handleSearchChange}
          placeholder="Search for integrations..."
        />
      </Box>
      <Flex justifyContent="space-between">
        <Flex justifyContent="flex-start">
          <MultiselectMenu
            options={tagOptions}
            onChange={tags =>
              handleFilterChange('tags', tags.filter(isIntegrationTag))
            }
            selected={state.filters.tags}
            label="Integration Type"
            tooltip="Filter by integration type"
          />
        </Flex>
        <Flex justifyContent="flex-end">
          <SortMenu
            current={{
              fieldName: state.sortKey || 'name',
              dir: state.sortDirection,
            }}
            fields={sortFieldOptions}
            onChange={handleSortChange}
          />
        </Flex>
      </Flex>
    </>
  );
}
