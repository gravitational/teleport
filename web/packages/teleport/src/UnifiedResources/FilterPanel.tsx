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

import Box from 'design/Box';
import { ButtonBorder } from 'design/Button';
import Flex from 'design/Flex';
import * as icons from 'design/Icon';
import React from 'react';
import Select from 'shared/components/Select';
import styled from 'styled-components';

import { encodeUrlQueryParams } from 'teleport/components/hooks/useUrlFiltering';
import { ResourceFilter, SortType } from 'teleport/services/agents';

const kindOptions = [
  { label: 'Application', value: 'app' },
  { label: 'Database', value: 'db' },
  { label: 'Desktop', value: 'windows_desktop' },
  { label: 'Kubernetes', value: 'kube_cluster' },
  { label: 'Server', value: 'node' },
];

const sortFieldOptions = [
  { label: 'Name', value: 'name' },
  { label: 'Type', value: 'kind' },
];

export interface FilterPanelProps {
  pathname: string;
  replaceHistory: (path: string) => void;
  params: ResourceFilter;
  setParams: (params: ResourceFilter) => void;
  setSort: (sort: SortType) => void;
}

export function FilterPanel({
  pathname,
  replaceHistory,
  params,
  setParams,
  setSort,
}: FilterPanelProps) {
  const { sort, kinds } = params;

  const activeSortFieldOption = sortFieldOptions.find(
    opt => opt.value === sort.fieldName
  );

  const activeKindOptions = kindOptions.filter(
    opt => kinds && kinds.includes(opt.value)
  );

  const onKindsChanged = (filter: any) => {
    const kinds = (filter ?? []).map(f => f.value);
    setParams({ ...params, kinds });
    // TODO(bl-nero): We really shouldn't have to do it, that's what setParams
    // should be for.
    const isAdvancedSearch = !!params.query;
    replaceHistory(
      encodeUrlQueryParams(
        pathname,
        params.search ?? params.query,
        params.sort,
        kinds,
        isAdvancedSearch
      )
    );
  };

  const onSortFieldChange = (option: any) => {
    setSort({ ...sort, fieldName: option.value });
  };

  const onSortOrderButtonClicked = () => {
    setSort(oppositeSort(sort));
  };

  return (
    <Flex justifyContent="space-between" mb={3}>
      <Box width="300px">
        <FilterSelect
          isMulti={true}
          placeholder="Type"
          options={kindOptions}
          value={activeKindOptions}
          onChange={onKindsChanged}
        />
      </Box>
      <Flex>
        <Box width="100px">
          <SortSelect
            size="small"
            options={sortFieldOptions}
            value={activeSortFieldOption}
            isSearchable={false}
            onChange={onSortFieldChange}
          />
        </Box>
        <SortOrderButton px={3} onClick={onSortOrderButtonClicked}>
          {sort.dir === 'ASC' && <icons.ChevronUp />}
          {sort.dir === 'DESC' && <icons.ChevronDown />}
        </SortOrderButton>
      </Flex>
    </Flex>
  );
  return null;
}

function oppositeSort(sort: SortType): SortType {
  switch (sort.dir) {
    case 'ASC':
      return { ...sort, dir: 'DESC' };
    case 'DESC':
      return { ...sort, dir: 'ASC' };
    default:
      // Will never happen. Of course.
      return sort;
  }
}

const SortOrderButton = styled(ButtonBorder)`
  border-top-left-radius: 0;
  border-bottom-left-radius: 0;
  border-color: ${props => props.theme.colors.spotBackground[1]};
  border-left: none;
`;

const FilterSelect = styled(Select)`
  .react-select__control {
    border: 1px solid ${props => props.theme.colors.spotBackground[1]};
  }
`;

const SortSelect = styled(Select)`
  .react-select__control {
    border-right: none;
    border-top-right-radius: 0;
    border-bottom-right-radius: 0;
    border: 1px solid ${props => props.theme.colors.spotBackground[1]};
  }
  .react-select__dropdown-indicator {
    display: none;
  }
`;
