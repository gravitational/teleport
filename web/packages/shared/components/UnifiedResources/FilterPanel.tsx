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

import { Flex, Text, Toggle } from 'design';
import { ButtonBorder, ButtonSecondary } from 'design/Button';
import { CheckboxInput } from 'design/Checkbox';
import { ArrowsIn, ArrowsOut, ChevronDown, Refresh } from 'design/Icon';
import Menu from 'design/Menu';
import { ViewMode } from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';
import { MultiselectMenu } from 'shared/components/Controls/MultiselectMenu';
import { SortMenu } from 'shared/components/Controls/SortMenu';
import { ViewModeSwitch } from 'shared/components/Controls/ViewModeSwitch';
import { HoverTooltip } from 'shared/components/ToolTip';

import {
  IncludedResourceMode,
  SharedUnifiedResource,
  UnifiedResourcesQueryParams,
} from './types';
import { FilterKind, ResourceAvailabilityFilter } from './UnifiedResources';

const kindToLabel: Record<SharedUnifiedResource['resource']['kind'], string> = {
  app: 'Application',
  db: 'Database',
  windows_desktop: 'Desktop',
  kube_cluster: 'Kubernetes',
  node: 'Server',
  user_group: 'User group',
  git_server: 'Git Server',
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
  onRefresh(): void;
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
  onRefresh,
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
        <MultiselectMenu
          options={availableKinds.map(({ kind, disabled }) => ({
            value: kind,
            label: kindToLabel[kind],
            disabled: disabled,
          }))}
          selected={kinds || []}
          onChange={onKindsChanged}
          label="Types"
          tooltip="Filter by resource type"
          buffered
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
        <HoverTooltip tipContent="Refresh">
          <ButtonBorder
            onClick={onRefresh}
            textTransform="none"
            css={`
              padding: 0 4.5px;
            `}
            size="small"
          >
            <Refresh size={12} />
          </ButtonBorder>
        </HoverTooltip>
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
          current={{
            fieldName: activeSortFieldOption.value,
            dir: sort.dir,
          }}
          fields={sortFieldOptions}
          onChange={newSort => {
            if (newSort.dir !== sort.dir) {
              onSortOrderButtonClicked();
            }
            if (newSort.fieldName !== activeSortFieldOption.value) {
              onSortFieldChange(newSort.fieldName);
            }
          }}
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
          <Text mr={2} mb={1}>
            Show requestable resources
          </Text>
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
  padding-top: 2px;
  padding-left: ${props => props.theme.space[3]}px;
  padding-right: ${props => props.theme.space[3]}px;
  display: flex;
  justify-content: flex-start;
  align-items: center;
  min-width: 140px;
  overflow: hidden;
  text-decoration: none;
  white-space: nowrap;
`;
