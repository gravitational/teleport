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

import React, { useState } from 'react';
import styled, { useTheme } from 'styled-components';

import {
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Input,
  Menu,
  MenuItem,
  Text,
} from 'design';
import { Check, ChevronDown } from 'design/Icon';
import * as Icons from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';
import { FiltersExistIndicator } from 'shared/components/Controls/MultiselectMenu';
import Select from 'shared/components/Select';

export type FilterOption =
  | 'up-to-date'
  | 'patch'
  | 'upgrade'
  | 'unsupported'
  | 'custom';

export type CustomOperator =
  | 'equals'
  | 'less-than'
  | 'greater-than'
  | 'between';

interface VersionsFilterPanelProps {
  currentVersion: string;
  onApply: (filter: {
    selectedOption: FilterOption;
    operator: CustomOperator;
    value1: string;
    value2: string;
  }) => void;
  tooltip?: string;
  disabled?: boolean;
  filter?: FilterOption;
  operator?: CustomOperator;
  value1?: string;
  value2?: string;
}

export function VersionsFilterPanel({
  currentVersion,
  onApply,
  tooltip = 'Filter by version',
  disabled = false,
  filter,
  operator = 'equals',
  value1 = '',
  value2 = '',
}: VersionsFilterPanelProps) {
  const theme = useTheme();
  const [anchorEl, setAnchorEl] = useState<HTMLElement>(null);
  const [selectedOption, setSelectedOption] = useState<FilterOption | null>(
    null
  );
  const [customOperator, setCustomOperator] =
    useState<CustomOperator>('equals');
  const [customValue1, setCustomValue1] = useState('');
  const [customValue2, setCustomValue2] = useState('');

  const handleOpen = (event: React.MouseEvent<HTMLButtonElement>) => {
    setSelectedOption(filter || null);
    setCustomOperator(operator || 'equals');
    setCustomValue1(value1);
    setCustomValue2(value2);
    setAnchorEl(event.currentTarget);
  };

  const handleClose = () => {
    setAnchorEl(null);
  };

  const handleApply = () => {
    onApply({
      selectedOption: selectedOption,
      operator: customOperator,
      value1: customValue1,
      value2: customValue2,
    });
    handleClose();
  };

  const handleOptionSelect = (option: FilterOption) => {
    if (selectedOption === option) {
      setSelectedOption(null);
      setCustomValue1('');
      setCustomValue2('');
    } else {
      setSelectedOption(option);
      if (option !== 'custom') {
        setCustomValue1('');
        setCustomValue2('');
      }
    }
  };

  const handleClearCustom = () => {
    setCustomValue1('');
    setCustomValue2('');
    setSelectedOption(null);
  };

  const operatorOptions = [
    { value: 'equals' as const, label: 'Equals' },
    { value: 'less-than' as const, label: 'Less than' },
    { value: 'greater-than' as const, label: 'Greater than' },
    { value: 'between' as const, label: 'Between' },
  ];

  const presetOptions: Array<{
    value: FilterOption;
    label: string;
  }> = [
    { value: 'up-to-date', label: 'Up-to-date' },
    { value: 'patch', label: 'Patch available' },
    { value: 'upgrade', label: 'Upgrade available' },
    { value: 'unsupported', label: 'Unsupported' },
  ];

  return (
    <Flex textAlign="center" alignItems="center">
      <HoverTooltip tipContent={tooltip}>
        <ButtonSecondary
          size="small"
          onClick={handleOpen}
          aria-haspopup="true"
          aria-expanded={!!anchorEl}
          disabled={disabled}
        >
          Version
          {filter && ' (1)'}
          <ChevronDown
            ml={2}
            size="small"
            color={disabled ? 'text.disabled' : 'text.slightlyMuted'}
          />
          {filter && <FiltersExistIndicator />}
        </ButtonSecondary>
      </HoverTooltip>
      <Menu
        popoverCss={() => `margin-top: 36px; width: 360px;`}
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
        {presetOptions.map(opt => (
          <FilterMenuItem
            key={opt.value}
            px={2}
            onClick={() => handleOptionSelect(opt.value)}
            width="100%"
          >
            <Flex alignItems="center" gap={2} width="100%">
              <CheckIconWrapper>
                {selectedOption === opt.value && <Check size="small" />}
              </CheckIconWrapper>
              <Flex width="100%" justifyContent="space-between">
                <Text fontWeight={300} fontSize={2}>
                  {opt.label}
                </Text>
                <Text
                  fontSize={1}
                  color="text.muted"
                  css={`
                    white-space: nowrap;
                  `}
                >
                  {getFilterDescription(opt.value, currentVersion)}
                </Text>
              </Flex>
            </Flex>
          </FilterMenuItem>
        ))}

        <FilterMenuItem
          px={2}
          py={2}
          width="100%"
          css={`
            cursor: pointer;
          `}
          onClick={() => handleOptionSelect('custom')}
        >
          <Flex alignItems="flex-start" gap={2}>
            <CheckIconWrapper css="margin-top: 4px; cursor: pointer;">
              {selectedOption === 'custom' && <Check size="small" />}
            </CheckIconWrapper>
            <Box flex="1">
              <Text fontWeight={300} fontSize={2} mb={2}>
                Custom condition
              </Text>
              <Flex
                alignItems="center"
                flexWrap="nowrap"
                onClick={e => {
                  if (selectedOption === 'custom') {
                    e.stopPropagation();
                  }
                }}
              >
                <Select
                  size="small"
                  value={operatorOptions.find(
                    opt => opt.value === customOperator
                  )}
                  options={operatorOptions}
                  onChange={(option: { value: CustomOperator }) => {
                    setCustomOperator(option.value);
                    if (selectedOption !== 'custom') {
                      handleOptionSelect('custom');
                    }
                  }}
                  isDisabled={selectedOption !== 'custom'}
                  isSearchable={false}
                  menuPosition="fixed"
                  stylesConfig={{
                    control: base => ({ ...base, width: '120px' }),
                    menuPortal: base => ({
                      ...base,
                      zIndex: 100,
                      backgroundColor: theme.colors.levels.elevated,
                    }),
                    menu: base => ({
                      ...base,
                      backgroundColor: theme.colors.levels.elevated,
                      width: '120px',
                    }),
                    option: (base, state) => ({
                      ...base,
                      backgroundColor: state.isFocused
                        ? theme.colors.interactive.tonal.neutral[0]
                        : 'transparent',
                      color: theme.colors.text.main,
                      cursor: 'pointer',
                      '&:active': {
                        backgroundColor:
                          theme.colors.interactive.tonal.neutral[1],
                      },
                    }),
                  }}
                />

                <Flex alignItems="center">
                  {customOperator === 'between' ? (
                    <>
                      <Input
                        ml={1}
                        size="small"
                        placeholder={currentVersion}
                        value={customValue1}
                        onChange={e => setCustomValue1(e.target.value)}
                        disabled={selectedOption !== 'custom'}
                        onFocus={() => {
                          if (selectedOption !== 'custom') {
                            handleOptionSelect('custom');
                          }
                        }}
                        width="70px"
                      />
                      <Text color="text.muted" mx={1}>
                        &
                      </Text>
                      <Input
                        size="small"
                        placeholder={currentVersion}
                        value={customValue2}
                        onChange={e => setCustomValue2(e.target.value)}
                        disabled={selectedOption !== 'custom'}
                        onFocus={() => {
                          if (selectedOption !== 'custom') {
                            handleOptionSelect('custom');
                          }
                        }}
                        width="70px"
                      />
                    </>
                  ) : (
                    <Input
                      ml={1}
                      size="small"
                      placeholder={currentVersion}
                      value={customValue1}
                      onChange={e => setCustomValue1(e.target.value)}
                      disabled={selectedOption !== 'custom'}
                      onFocus={() => {
                        if (selectedOption !== 'custom') {
                          handleOptionSelect('custom');
                        }
                      }}
                    />
                  )}
                </Flex>
                <ClearButton
                  onClick={handleClearCustom}
                  disabled={selectedOption !== 'custom'}
                >
                  <Icons.Cross size="small" />
                </ClearButton>
              </Flex>
            </Box>
          </Flex>
        </FilterMenuItem>
        <Divider />

        <ActionButtonsContainer justifyContent="flex-start" p={3} gap={2}>
          <ButtonPrimary size="small" onClick={handleApply}>
            Apply Filters
          </ButtonPrimary>
          <ButtonSecondary
            size="small"
            css={`
              background-color: transparent;
            `}
            onClick={handleClose}
          >
            Cancel
          </ButtonSecondary>
        </ActionButtonsContainer>
      </Menu>
    </Flex>
  );
}

export function getMajorVersion(version: string): string {
  const parts = version.split('.');
  return `${parts[0]}.0.0`;
}

export function getMinorVersion(version: string): string {
  const parts = version.split('.');
  return `${parts[0]}.${parts[1]}.0`;
}

export function getPreviousMajorVersion(version: string): string {
  const parts = version.split('.');
  const major = parseInt(parts[0]);
  return `${major - 1}.0.0`;
}

export function getNextMajorVersion(version: string): string {
  const parts = version.split('.');
  const major = parseInt(parts[0]);
  return `${major + 1}.0.0`;
}

// buildVersionPredicate returns the predicate query corresponding to a given version filter selection.
export function buildVersionPredicate(
  filter: string,
  operator: string,
  value1: string,
  value2: string,
  currentVersion: string
): string {
  if (!filter) return '';

  const minorVersion = getMinorVersion(currentVersion);
  const prevMajor = getPreviousMajorVersion(currentVersion);
  const nextMajor = getNextMajorVersion(currentVersion);

  switch (filter) {
    case 'up-to-date':
      return `version == "${currentVersion}"`;
    case 'patch':
      return `between(version, "${minorVersion}", "${currentVersion}")`;
    case 'upgrade':
      return `between(version, "${prevMajor}", "${minorVersion}")`;
    case 'unsupported':
      return `older_than(version, "${prevMajor}") || newer_than(version, "${nextMajor}")`;
    case 'custom':
      switch (operator) {
        case 'equals':
          return value1 ? `version == "${value1}"` : '';
        case 'less-than':
          return value1 ? `older_than(version, "${value1}")` : '';
        case 'greater-than':
          return value1 ? `newer_than(version, "${value1}")` : '';
        case 'between':
          return value1 && value2
            ? `between(version, "${value1}", "${value2}")`
            : '';
        default:
          return '';
      }
    default:
      return '';
  }
}

/**
 * getFilterDescription returns the text on the right of the preset version filter options indicating what each option entails
 */
const getFilterDescription = (
  option: FilterOption,
  currentVersion: string
): string => {
  const minorVersion = getMinorVersion(currentVersion);
  const prevMajor = getPreviousMajorVersion(currentVersion);
  const nextMajor = getNextMajorVersion(currentVersion);

  switch (option) {
    case 'up-to-date':
      return currentVersion;
    case 'patch':
      return `between ${minorVersion} & ${currentVersion}`;
    case 'upgrade':
      return `between ${prevMajor} & ${minorVersion}`;
    case 'unsupported':
      return `<${prevMajor} or >${nextMajor}`;
    default:
      return '';
  }
};

const FilterMenuItem = styled(MenuItem)`
  &:hover {
    background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
  }
`;

const Divider = styled.div`
  height: 1px;
  background-color: ${p => p.theme.colors.interactive.tonal.neutral[1]};
`;

const CheckIconWrapper = styled.div`
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  color: ${p => p.theme.colors.text.main};
`;

const ClearButton = styled.button`
  background: transparent;
  border: none;
  cursor: pointer;
  padding: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: ${p => p.theme.colors.text.muted};
  border-radius: 4px;
  height: 32px;
  width: 32px;
  margin-left: ${p => p.theme.space[1]}px;

  &:hover:not(:disabled) {
    background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
    color: ${p => p.theme.colors.text.main};
  }

  &:disabled {
    opacity: 0.3;
  }
`;

const ActionButtonsContainer = styled(Flex)`
  position: sticky;
  bottom: 0;
  background-color: ${p => p.theme.colors.levels.elevated};
  z-index: 1;
`;
