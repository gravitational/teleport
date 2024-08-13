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

import React, { useState } from 'react';
import styled from 'styled-components';
import { ButtonBorder, ButtonPrimary, ButtonSecondary } from 'design/Button';
import { SortDir } from 'design/DataTable/types';
import { Text, Flex, Toggle } from 'design';
import Menu, { MenuItem } from 'design/Menu';
import { CheckboxInput } from 'design/Checkbox';
import {
  ArrowUp,
  ArrowDown,
  ChevronDown,
  SquaresFour,
  Rows,
  ArrowsIn,
  ArrowsOut,
} from 'design/Icon';

import { ViewMode } from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';

import { HoverTooltip } from 'shared/components/ToolTip';

import { ResourceAvailabilityFilter, FilterKind } from './UnifiedResources';
import {
  IncludedResourceMode,
  SharedUnifiedResource,
  UnifiedResourcesQueryParams,
} from './types';

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
  availableKinds: FilterKind[];
  params: UnifiedResourcesQueryParams;
  setParams: (params: UnifiedResourcesQueryParams) => void;
  selectVisible: () => void;
  selected: boolean;
  BulkActions?: React.ReactElement;
  currentViewMode: ViewMode;
  setCurrentViewMode: (viewMode: ViewMode) => void;
  expandAllLabels: boolean;
  setExpandAllLabels: (expandAllLabels: boolean) => void;
  hideViewModeOptions: boolean;
  /*
   * ClusterDropdown is an optional prop to add a ClusterDropdown to the
   * FilterPanel component. This is useful to turn off in Connect and use on web only
   */
  ClusterDropdown?: JSX.Element;
  availabilityFilter?: ResourceAvailabilityFilter;
  changeAvailableResourceMode(mode: IncludedResourceMode): void;
}

export function FilterPanel({
  availableKinds,
  params,
  setParams,
  selectVisible,
  selected,
  BulkActions,
  currentViewMode,
  setCurrentViewMode,
  availabilityFilter,
  expandAllLabels,
  setExpandAllLabels,
  hideViewModeOptions,
  changeAvailableResourceMode,
  ClusterDropdown = null,
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
          <CheckboxInput
            css={`
              // add extra margin so it aligns with the checkboxes of the resources
              margin-left: 19px;
            `}
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
        {ClusterDropdown}
        {availabilityFilter && (
          <IncludedResourcesSelector
            availabilityFilter={availabilityFilter}
            onChange={changeAvailableResourceMode}
          />
        )}
      </Flex>
      <Flex gap={2} alignItems="center">
        <Flex mr={1}>{BulkActions}</Flex>
        {!hideViewModeOptions && (
          <>
            {currentViewMode === ViewMode.LIST && (
              <ButtonBorder
                size="small"
                css={`
                  border: none;
                  color: ${props => props.theme.colors.text.slightlyMuted};
                  text-transform: none;
                  padding-left: ${props => props.theme.space[2]}px;
                  padding-right: ${props => props.theme.space[2]}px;
                  height: 22px;
                  font-size: 12px;
                `}
                onClick={() => setExpandAllLabels(!expandAllLabels)}
              >
                <Flex alignItems="center" width="100%">
                  {expandAllLabels ? (
                    <ArrowsIn size="small" mr={1} />
                  ) : (
                    <ArrowsOut size="small" mr={1} />
                  )}
                  {expandAllLabels ? 'Collapse ' : 'Expand '} All Labels
                </Flex>
              </ButtonBorder>
            )}
            <ViewModeSwitch
              currentViewMode={currentViewMode}
              setCurrentViewMode={setCurrentViewMode}
            />
          </>
        )}
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
  availableKinds: FilterKind[];
  kindsFromParams: string[];
  onChange: (kinds: string[]) => void;
};

const FilterTypesMenu = ({
  onChange,
  availableKinds,
  kindsFromParams,
}: FilterTypesMenuProps) => {
  const kindOptions = availableKinds.map(({ kind, disabled }) => ({
    value: kind,
    label: kindToLabel[kind],
    disabled: disabled,
  }));

  const [anchorEl, setAnchorEl] = useState(null);
  // we have a separate state in the filter so we can select a few different things and then click "apply"
  const [kinds, setKinds] = useState<string[]>(kindsFromParams || []);
  const handleOpen = event => {
    setKinds(kindsFromParams);
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
    setKinds(kindOptions.filter(k => !k.disabled).map(k => k.value));
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
      <HoverTooltip tipContent={'Filter by resource type'}>
        <ButtonSecondary size="small" onClick={handleOpen}>
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
        {kindOptions.map(kind => {
          const $checkbox = (
            <>
              <CheckboxInput
                type="checkbox"
                name={kind.label}
                disabled={kind.disabled}
                onChange={() => {
                  handleSelect(kind.value);
                }}
                id={kind.value}
                checked={kinds.includes(kind.value)}
              />
              <Text ml={2} fontWeight={300} fontSize={2}>
                {kind.label}
              </Text>
            </>
          );
          return (
            <MenuItem
              disabled={kind.disabled}
              px={2}
              key={kind.value}
              onClick={() => (!kind.disabled ? handleSelect(kind.value) : null)}
            >
              {kind.disabled ? (
                <HoverTooltip
                  tipContent={`You do not have access to ${kind.label} resources.`}
                >
                  {$checkbox}
                </HoverTooltip>
              ) : (
                $checkbox
              )}
            </MenuItem>
          );
        })}
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
  onDirChange: () => void;
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
  setCurrentViewMode,
}: {
  currentViewMode: ViewMode;
  setCurrentViewMode: (viewMode: ViewMode) => void;
}) {
  return (
    <ViewModeSwitchContainer>
      <ViewModeSwitchButton
        className={currentViewMode === ViewMode.CARD ? 'selected' : ''}
        onClick={() => setCurrentViewMode(ViewMode.CARD)}
        css={`
          border-right: 1px solid
            ${props => props.theme.colors.spotBackground[2]};
          border-top-left-radius: 4px;
          border-bottom-left-radius: 4px;
        `}
      >
        <SquaresFour size="small" color="text.main" />
      </ViewModeSwitchButton>
      <ViewModeSwitchButton
        className={currentViewMode === ViewMode.LIST ? 'selected' : ''}
        onClick={() => setCurrentViewMode(ViewMode.LIST)}
        css={`
          border-top-right-radius: 4px;
          border-bottom-right-radius: 4px;
        `}
      >
        <Rows size="small" color="text.main" />
      </ViewModeSwitchButton>
    </ViewModeSwitchContainer>
  );
}

const IncludedResourcesSelector = ({
  onChange,
  availabilityFilter,
}: {
  onChange: (value: IncludedResourceMode) => void;
  availabilityFilter: ResourceAvailabilityFilter;
}) => {
  const [anchorEl, setAnchorEl] = useState(null);

  const handleOpen = event => {
    setAnchorEl(event.currentTarget);
  };

  const handleClose = () => {
    setAnchorEl(null);
  };

  function handleToggle() {
    if (
      availabilityFilter.mode === 'requestable' ||
      availabilityFilter.mode === 'all'
    ) {
      onChange('accessible');
      return;
    }
    onChange(availabilityFilter.canRequestAll ? 'all' : 'requestable');
  }

  return (
    <Flex textAlign="center" alignItems="center">
      <HoverTooltip tipContent={'Filter by resource availability'}>
        <ButtonSecondary
          px={2}
          css={`
            border-color: ${props => props.theme.colors.spotBackground[0]};
          `}
          textTransform="none"
          size="small"
          onClick={handleOpen}
        >
          Access Requests
          <ChevronDown ml={2} size="small" color="text.slightlyMuted" />
          {availabilityFilter.mode === 'accessible' && (
            <FiltersExistIndicator />
          )}
        </ButtonSecondary>
      </HoverTooltip>
      <Menu
        popoverCss={() => `
          // TODO (avatus): fix popover component to calculate correct height/anchor
          margin-top: 36px;
        `}
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
        onClose={handleClose}
      >
        <AccessRequestsToggleItem>
          <Text mr={2}>Show requestable resources</Text>
          <Toggle
            isToggled={
              availabilityFilter.mode === 'requestable' ||
              availabilityFilter.mode === 'all'
            }
            onToggle={handleToggle}
          />
        </AccessRequestsToggleItem>
      </Menu>
    </Flex>
  );
};

const ViewModeSwitchContainer = styled.div`
  height: 22px;
  width: 48px;
  border: 1px solid ${props => props.theme.colors.spotBackground[2]};
  border-radius: 4px;
  display: flex;

  .selected {
    background-color: ${props => props.theme.colors.spotBackground[1]};

    &:hover {
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

  &:hover {
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

const AccessRequestsToggleItem = styled.div`
  min-height: 40px;
  box-sizing: border-box;
  padding-left: ${props => props.theme.space[2]}px;
  padding-right: ${props => props.theme.space[2]}px;
  display: flex;
  justify-content: flex-start;
  align-items: center;
  min-width: 140px;
  overflow: hidden;
  text-decoration: none;
  white-space: nowrap;
`;
