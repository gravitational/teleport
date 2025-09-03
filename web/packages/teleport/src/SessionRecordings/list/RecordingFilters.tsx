/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { useCallback, useState, type MouseEvent } from 'react';

import Box from 'design/Box';
import { ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import { ChevronDown } from 'design/Icon';
import Menu from 'design/Menu';
import Text from 'design/Text';
import { Toggle } from 'design/Toggle';
import { HoverTooltip } from 'design/Tooltip';
import {
  FiltersExistIndicator,
  MultiselectMenu,
} from 'shared/components/Controls/MultiselectMenu';

import type { RecordingType } from 'teleport/services/recordings';

import type { RecordingsListFilterKey, RecordingsListFilters } from './state';

interface RecordingFilterOption {
  label: string;
  value: string;
}

export type RecordingFilterOptions = {
  [key in Exclude<
    RecordingsListFilterKey,
    'types' | 'hideNonInteractive'
  >]: RecordingFilterOption[];
};

interface RecordingFiltersProps {
  disabled: boolean;
  filters: RecordingsListFilters;
  options: RecordingFilterOptions;
  onFilterChange: (
    key: RecordingsListFilterKey,
    value: string[] | boolean
  ) => void;
}

interface TypeOption {
  label: string;
  value: RecordingType;
}

const typeOptions: TypeOption[] = [
  {
    label: 'SSH',
    value: 'ssh',
  },
  {
    label: 'Kubernetes',
    value: 'k8s',
  },
  {
    label: 'Desktop',
    value: 'desktop',
  },
  {
    label: 'Database',
    value: 'database',
  },
];

export function RecordingFilters({
  disabled,
  onFilterChange,
  options,
  filters,
}: RecordingFiltersProps) {
  const createHandleChange = useCallback(
    (key: RecordingsListFilterKey) => (value: string[]) =>
      onFilterChange(key, value),
    [onFilterChange]
  );

  const handleToggleNonInteractive = useCallback(
    (value: boolean) => onFilterChange('hideNonInteractive', value),
    [onFilterChange]
  );

  return (
    <>
      <MultiselectMenu
        options={typeOptions}
        selected={filters.types}
        onChange={createHandleChange('types')}
        label="Types"
        tooltip="Filter by session recording type"
        buffered
      />

      <MultiselectMenu
        options={options.users}
        selected={filters.users}
        onChange={createHandleChange('users')}
        label="Users"
        tooltip={disabled ? 'No results' : 'Filter by user'}
        buffered
        disabled={disabled}
      />

      <MultiselectMenu
        options={options.resources}
        selected={filters.resources}
        onChange={createHandleChange('resources')}
        label="Resources"
        tooltip={disabled ? 'No results' : 'Filter by resource'}
        buffered
        disabled={disabled}
      />

      <HideNonInteractive
        toggled={filters.hideNonInteractive}
        onToggle={handleToggleNonInteractive}
      />
    </>
  );
}

interface HideNonInteractiveProps {
  toggled: boolean;
  onToggle: (value: boolean) => void;
}

function HideNonInteractive({ toggled, onToggle }: HideNonInteractiveProps) {
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);

  const handleOpen = useCallback((event: MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget);
  }, []);

  const handleClose = useCallback(() => {
    setAnchorEl(null);
  }, []);

  const handleToggle = useCallback(() => {
    onToggle(!toggled);
    handleClose();
  }, [onToggle, toggled, handleClose]);

  return (
    <Flex textAlign="center" alignItems="center">
      <HoverTooltip tipContent={'Filter by interactive sessions'}>
        <ButtonSecondary
          px={2}
          textTransform="none"
          size="small"
          onClick={handleOpen}
        >
          Interactivity
          <ChevronDown ml={2} size="small" color="text.slightlyMuted" />
          {toggled && <FiltersExistIndicator />}
        </ButtonSecondary>
      </HoverTooltip>
      <Menu
        popoverCss={() => `
          // TODO (avatus): fix popover component to calculate correct height/anchor (copied from FilterPanel)
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
        <Box px={3} py={2}>
          <Toggle isToggled={toggled} onToggle={handleToggle}>
            <Text ml={2}>Hide non-interactive sessions</Text>
          </Toggle>
        </Box>
      </Menu>
    </Flex>
  );
}
