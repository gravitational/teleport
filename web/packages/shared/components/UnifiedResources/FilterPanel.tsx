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
import { Text, Flex, Box } from 'design';
import Menu, { MenuItem } from 'design/Menu';
import { StyledCheckbox } from 'design/Checkbox';
import {
  ArrowUp,
  ArrowDown,
  ChevronDown,
  SquaresFour,
  Rows,
} from 'design/Icon';

import { HoverTooltip } from 'shared/components/ToolTip';

import { SharedUnifiedResource, UnifiedResourcesQueryParams } from './types';

const kindToLabel: Record<SharedUnifiedResource['resource']['kind'], string> = {
  app: 'Application',
  db: 'Database',
  windows_desktop: 'Desktop',
  kube_cluster: 'Kubernetes',
  node: 'Server',
  user_group: 'User group',
};

const sortFieldOptions = [
  { label: 'Name', value: 'name' },
  { label: 'Type', value: 'kind' },
];

interface FilterPanelProps {
  availableKinds: SharedUnifiedResource['resource']['kind'][];
  params: UnifiedResourcesQueryParams;
  setParams: (params: UnifiedResourcesQueryParams) => void;
  selectVisible: () => void;
  selected: boolean;
  BulkActions?: React.ReactElement;
  currentViewMode: ViewMode;
  onSelectViewMode: (viewMode: ViewMode) => void;
}

export function FilterPanel({
  availableKinds,
  params,
  setParams,
  selectVisible,
  selected,
  BulkActions,
  currentViewMode,
  onSelectViewMode,
}: FilterPanelProps) {
  const { sort, kinds } = params;

  const activeSortFieldOption = sortFieldOptions.find(
    opt => opt.value === sort.fieldName
  );

  const onKindsChanged = (newKinds: string[]) => {
    setParams({ ...params, kinds: newKinds });
  };

  const onSortFieldChange = (value: string) => {
    setParams({ ...params, sort: { ...params.sort, fieldName: value } });
  };

  const onSortOrderButtonClicked = () => {
    setParams({ ...params, sort: oppositeSort(sort) });
  };

  return (
    // minHeight is set to 32px so there isn't layout shift when a bulk action button shows up
    <Flex
      mb={2}
      justifyContent="space-between"
      minHeight="32px"
      alignItems="center"
    >
      <Flex gap={2}>
        <HoverTooltip tipContent={selected ? 'Deselect all' : 'Select all'}>
          <StyledCheckbox
            checked={selected}
            onChange={selectVisible}
            data-testid="select_all"
          />
        </HoverTooltip>
        <FilterTypesMenu
          onChange={onKindsChanged}
          availableKinds={availableKinds}
          kindsFromParams={kinds || []}
        />
      </Flex>
      <Flex gap={2} alignItems="center">
        <Box mr={4}>{BulkActions}</Box>
        <ViewModeSwitch
          currentViewMode={currentViewMode}
          onSelectViewMode={onSelectViewMode}
        />
        <SortMenu
          onDirChange={onSortOrderButtonClicked}
          onChange={onSortFieldChange}
          sortType={activeSortFieldOption.label}
          sortDir={sort.dir}
        />
      </Flex>
    </Flex>
  );
}

function oppositeSort(
  sort: UnifiedResourcesQueryParams['sort']
): UnifiedResourcesQueryParams['sort'] {
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
  availableKinds: SharedUnifiedResource['resource']['kind'][];
  kindsFromParams: string[];
  onChange: (kinds: string[]) => void;
};

const FilterTypesMenu = ({
  onChange,
  availableKinds,
  kindsFromParams,
}: FilterTypesMenuProps) => {
  const kindOptions = availableKinds.map(kind => ({
    value: kind,
    label: kindToLabel[kind],
  }));

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
    <Flex textAlign="center" alignItems="center">
      <HoverTooltip tipContent={'Filter types'}>
        <ButtonSecondary
          px={2}
          css={`
            border-color: ${props => props.theme.colors.spotBackground[0]};
          `}
          textTransform="none"
          size="small"
          onClick={handleOpen}
        >
          Types{' '}
          {kindsFromParams.length > 0 ? `(${kindsFromParams.length})` : ''}
          <ChevronDown ml={2} size="small" color="text.slightlyMuted" />
          {kindsFromParams.length > 0 && <FiltersExistIndicator />}
        </ButtonSecondary>
      </HoverTooltip>
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
            textTransform="none"
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
            textTransform="none"
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
            <StyledCheckbox
              type="checkbox"
              name={kind.label}
              onChange={() => {
                handleSelect(kind.value);
              }}
              id={kind.value}
              checked={kinds.includes(kind.value)}
            />
            <Text ml={2} fontWeight={300} fontSize={2}>
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
    <Flex textAlign="center">
      <HoverTooltip tipContent={'Sort by'}>
        <ButtonBorder
          css={`
            border-right: none;
            border-top-right-radius: 0;
            border-bottom-right-radius: 0;
            border-color: ${props => props.theme.colors.spotBackground[2]};
          `}
          textTransform="none"
          size="small"
          px={2}
          onClick={handleOpen}
        >
          {sortType}
        </ButtonBorder>
      </HoverTooltip>
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
      <HoverTooltip tipContent={'Sort direction'}>
        <ButtonBorder
          onClick={onDirChange}
          textTransform="none"
          css={`
            width: 0px; // remove extra width around the button icon
            border-top-left-radius: 0;
            border-bottom-left-radius: 0;
            border-color: ${props => props.theme.colors.spotBackground[2]};
          `}
          size="small"
        >
          {sortDir === 'ASC' ? <ArrowUp size={12} /> : <ArrowDown size={12} />}
        </ButtonBorder>
      </HoverTooltip>
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

function ViewModeSwitch({
  currentViewMode,
  onSelectViewMode,
}: {
  currentViewMode: ViewMode;
  onSelectViewMode: (viewMode: ViewMode) => void;
}) {
  return (
    <ViewModeSwitchContainer>
      <ViewModeSwitchButton
        className={currentViewMode === 'card' ? 'selected' : ''}
        onClick={() => onSelectViewMode('card')}
        css={`
          border-right: 1px solid
            ${props => props.theme.colors.spotBackground[2]};
          border-top-left-radius: 4px;
          border-bottom-left-radius: 4px;
        `}
      >
        <SquaresFour size="small" />
      </ViewModeSwitchButton>
      <ViewModeSwitchButton
        className={currentViewMode === 'list' ? 'selected' : ''}
        onClick={() => onSelectViewMode('list')}
        css={`
          border-top-right-radius: 4px;
          border-bottom-right-radius: 4px;
        `}
      >
        <Rows size="small" />
      </ViewModeSwitchButton>
    </ViewModeSwitchContainer>
  );
}

export type ViewMode = 'card' | 'list';

const ViewModeSwitchContainer = styled.div`
  height: 22px;
  width: 48px;
  border: 1px solid ${props => props.theme.colors.spotBackground[2]};
  border-radius: 4px;
  display: flex;

  .selected {
    background-color: ${props => props.theme.colors.spotBackground[1]};

    :hover {
      background-color: ${props => props.theme.colors.spotBackground[1]};
    }
  }
`;

const ViewModeSwitchButton = styled.button`
  height: 100%;
  width: 50%;
  overflow: hidden;
  border: none;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;

  background-color: transparent;

  :hover {
    background-color: ${props => props.theme.colors.spotBackground[0]};
  }
`;

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
