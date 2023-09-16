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

import React, { useState } from 'react';
import styled from 'styled-components';
import { ButtonBorder, ButtonPrimary, ButtonSecondary } from 'design/Button';
import { SortDir } from 'design/DataTable/types';
import { Text } from 'design';
import Menu, { MenuItem } from 'design/Menu';
import Flex from 'design/Flex';
import { CheckboxInput } from 'design/Checkbox';
import { ArrowUp, ArrowDown, ChevronDown } from 'design/Icon';

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

  const onKindsChanged = (newKinds: string[]) => {
    setParams({ ...params, kinds: newKinds });
    // TODO(bl-nero): We really shouldn't have to do it, that's what setParams
    // should be for.
    const isAdvancedSearch = !!params.query;
    replaceHistory(
      encodeUrlQueryParams(
        pathname,
        params.search ?? params.query,
        params.sort,
        newKinds,
        isAdvancedSearch
      )
    );
  };

  const onSortFieldChange = (value: string) => {
    setSort({ ...sort, fieldName: value });
  };

  const onSortOrderButtonClicked = () => {
    setSort(oppositeSort(sort));
  };

  return (
    <Flex justifyContent="space-between" mb={2}>
      <FilterTypesMenu
        onChange={onKindsChanged}
        kindsFromParams={kinds || []}
      />
      <Flex alignItems="start">
        <SortMenu
          onDirChange={onSortOrderButtonClicked}
          onChange={onSortFieldChange}
          sortType={activeSortFieldOption.label}
          sortDir={sort.dir}
        />
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

type FilterTypesMenuProps = {
  kindsFromParams: string[];
  onChange: (kinds: string[]) => void;
};

const FilterTypesMenu = ({
  onChange,
  kindsFromParams,
}: FilterTypesMenuProps) => {
  const [anchorEl, setAnchorEl] = useState(null);
  // we have a separate state in the filter so we can select a few different things and then click "apply"
  const [kinds, setKinds] = useState<string[]>(kindsFromParams || []);
  const handleOpen = event => {
    setAnchorEl(event.currentTarget);
  };

  const handleClose = () => {
    setAnchorEl(null);
  };

  // if we cancel, we reset the kinds to what is already selected in the params
  const cancelUpdate = () => {
    setKinds(kindsFromParams);
    handleClose();
  };

  const handleSelect = (value: string) => {
    let newKinds = [...kinds];
    if (newKinds.includes(value)) {
      newKinds = newKinds.filter(v => v !== value);
    } else {
      newKinds.push(value);
    }
    setKinds(newKinds);
  };

  const handleSelectAll = () => {
    setKinds(kindOptions.map(k => k.value));
  };

  const handleClearAll = () => {
    setKinds([]);
  };

  const applyFilters = () => {
    onChange(kinds);
    handleClose();
  };

  return (
    <Flex textAlign="center" alignItems="center" mt={1}>
      <ButtonSecondary
        px={2}
        css={`
          border-color: ${props => props.theme.colors.spotBackground[0]};
        `}
        textTransform="none"
        size="small"
        onClick={handleOpen}
      >
        Type
        <ChevronDown ml={2} size="small" color="text.slightlyMuted" />
        {kindsFromParams.length > 0 && <FiltersExistIndicator />}
      </ButtonSecondary>
      <Menu
        popoverCss={() => `margin-top: 36px;`}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'left',
        }}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'left',
        }}
        anchorEl={anchorEl}
        open={Boolean(anchorEl)}
        onClose={cancelUpdate}
      >
        <Flex gap={2} p={2}>
          <ButtonSecondary
            size="small"
            onClick={handleSelectAll}
            css={`
              background-color: transparent;
            `}
            px={2}
          >
            Select All
          </ButtonSecondary>
          <ButtonSecondary
            size="small"
            onClick={handleClearAll}
            css={`
              background-color: transparent;
            `}
            px={2}
          >
            Clear All
          </ButtonSecondary>
        </Flex>
        {kindOptions.map(kind => (
          <MenuItem
            px={2}
            key={kind.value}
            onClick={() => handleSelect(kind.value)}
          >
            <CheckboxInput
              type="checkbox"
              name={kind.label}
              onChange={() => {
                handleSelect(kind.value);
              }}
              id={kind.value}
              checked={kinds.includes(kind.value)}
            />
            <Text fontWeight={300} fontSize={2}>
              {kind.label}
            </Text>
          </MenuItem>
        ))}

        <Flex justifyContent="space-between" p={2} gap={2}>
          <ButtonPrimary
            disabled={kindArraysEqual(kinds, kindsFromParams)}
            size="small"
            onClick={applyFilters}
          >
            Apply Filters
          </ButtonPrimary>
          <ButtonSecondary
            size="small"
            css={`
              background-color: transparent;
            `}
            onClick={cancelUpdate}
          >
            Cancel
          </ButtonSecondary>
        </Flex>
      </Menu>
    </Flex>
  );
};

type SortMenuProps = {
  transformOrigin?: any;
  anchorOrigin?: any;
  sortType: string;
  sortDir: SortDir;
  onChange: (value: string) => void;
  onDirChange: (dir: SortDir) => void;
};

const SortMenu: React.FC<SortMenuProps> = props => {
  const { sortType, onChange, onDirChange, sortDir } = props;
  const [anchorEl, setAnchorEl] = React.useState(null);

  const handleOpen = event => {
    setAnchorEl(event.currentTarget);
  };

  const handleClose = () => {
    setAnchorEl(null);
  };

  const handleSelect = (value: string) => {
    handleClose();
    onChange(value);
  };

  return (
    <Flex textAlign="center" alignItems="center">
      <ButtonBorder
        css={`
          border-right: none;
          border-top-right-radius: 0;
          border-bottom-right-radius: 0;
          border-color: ${props => props.theme.colors.spotBackground[0]};
        `}
        textTransform="none"
        size="small"
        px={2}
        onClick={handleOpen}
      >
        {sortType}
      </ButtonBorder>
      <Menu
        popoverCss={() => `margin-top: 36px;`}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'center',
        }}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'center',
        }}
        anchorEl={anchorEl}
        open={Boolean(anchorEl)}
        onClose={handleClose}
      >
        <MenuItem onClick={() => handleSelect('name')}>Name</MenuItem>
        <MenuItem onClick={() => handleSelect('kind')}>Type</MenuItem>
      </Menu>
      <ButtonBorder
        onClick={onDirChange}
        textTransform="none"
        css={`
          width: 0px; // remove extra width around the button icon
          border-top-left-radius: 0;
          border-bottom-left-radius: 0;
          border-color: ${props => props.theme.colors.spotBackground[0]};
        `}
        size="small"
      >
        {sortDir === 'ASC' ? <ArrowUp size={12} /> : <ArrowDown size={12} />}
      </ButtonBorder>
    </Flex>
  );
};

function kindArraysEqual(arr1: string[], arr2: string[]) {
  if (arr1.length !== arr2.length) {
    return false;
  }

  const sortedArr1 = arr1.slice().sort();
  const sortedArr2 = arr2.slice().sort();

  for (let i = 0; i < sortedArr1.length; i++) {
    if (sortedArr1[i] !== sortedArr2[i]) {
      return false;
    }
  }

  return true;
}

const FiltersExistIndicator = styled.div`
  position: absolute;
  top: -4px;
  right: -4px;
  height: 12px;
  width: 12px;
  background-color: ${props => props.theme.colors.brand};
  border-radius: 50%;
  display: inline-block;
`;
