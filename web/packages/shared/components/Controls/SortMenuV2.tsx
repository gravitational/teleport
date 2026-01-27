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

import { useState } from 'react';
import styled, { useTheme } from 'styled-components';

import { ButtonBorder } from 'design/Button/Button';
import { CheckThick } from 'design/Icon/Icons/CheckThick';
import { ChevronDown } from 'design/Icon/Icons/ChevronDown';
import { SortAscending } from 'design/Icon/Icons/SortAscending';
import { SortDescending } from 'design/Icon/Icons/SortDescending';
import Menu from 'design/Menu/Menu';
import MenuItem from 'design/Menu/MenuItem';
import Text from 'design/Text/Text';

export type SortOrder = 'ASC' | 'DESC';

export type SortItem = {
  /** A unique key for the item */
  key: string;
  /** The label that appears for the item in the options menu, as well as the sort button when selected */
  label: string;
  /** An optional override for the label used in the sort button when this item is selected and the selected order is ascending  */
  ascendingLabel?: string;
  /** An optional override for the label used in the sort button when this item is selected and the selected order is descending  */
  descendingLabel?: string;
  /** An optional label for the ascending label used in the options menu */
  ascendingOptionLabel?: string;
  /** An optional label for the descending label used in the options menu */
  descendingOptionLabel?: string;
  /** An optional default sort order used when the item is initially selected */
  defaultOrder?: SortOrder;
  /** Disable the sort options when this item is selected. The default sort order is still honored when the item is selected. */
  disableSort?: boolean;
};

export function SortMenu(props: {
  /** Items for the "sort by" section of the options menu */
  items: SortItem[];
  /** The key of the currently selected item. */
  selectedKey: string;
  /** The order currently selected. */
  selectedOrder: SortOrder;
  /** Function to use as a callback when the sort item or order changes */
  onChange: (key: string, order: SortOrder) => void;
}) {
  const { items, selectedKey, selectedOrder, onChange } = props;

  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);

  const theme = useTheme();

  const {
    key,
    label = key ?? 'No label',
    ascendingLabel,
    descendingLabel,
    ascendingOptionLabel = 'Ascending',
    descendingOptionLabel = 'Descending',
    disableSort = false,
  } = items.find(f => f.key === selectedKey) ?? {};

  const handleOpen = (event: React.MouseEvent<HTMLButtonElement, MouseEvent>) =>
    setAnchorEl(event.currentTarget);

  const handleClose = () => setAnchorEl(null);

  const handleItemSelected = (key: string) => {
    handleClose();
    onChange(
      key,
      items.find(f => f.key === key)?.defaultOrder ?? selectedOrder ?? 'ASC'
    );
  };

  const handleOrderSelected = (order: 'ASC' | 'DESC') => {
    if (disableSort) {
      return;
    }

    handleClose();
    onChange(selectedKey, order);
  };

  return (
    <>
      <StyledButtonBorder
        textTransform="none"
        size="small"
        px={2}
        onClick={handleOpen}
        aria-label="Sort by"
        aria-haspopup="true"
        aria-expanded={!!anchorEl}
      >
        {selectedOrder === 'ASC' ? (
          <SortDescending
            size={'small'}
            color={theme.colors.text.muted}
            data-testid="sort-asc-icon"
          />
        ) : (
          <SortAscending
            size={'small'}
            color={theme.colors.text.muted}
            data-testid="sort-desc-icon"
          />
        )}

        {selectedOrder === 'ASC'
          ? (ascendingLabel ?? label)
          : (descendingLabel ?? label)}

        <ChevronDown size={'small'} />
      </StyledButtonBorder>
      <Menu
        anchorEl={anchorEl}
        open={Boolean(anchorEl)}
        onClose={handleClose}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'right',
        }}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'right',
        }}
        popoverCss={() => `margin-top: ${theme.space[5]}px; margin-left: 0;`}
        menuListCss={() => `padding-bottom: 8px;`}
      >
        <MenuTitle>Sort by</MenuTitle>
        {items.map(({ key, label: optionLabel }) => (
          <StyledMenuItem key={key} onClick={() => handleItemSelected(key)}>
            <Tick
              size={'small'}
              checked={selectedKey == key}
              color={theme.colors.text.muted}
            />
            {optionLabel}
          </StyledMenuItem>
        ))}
        <MenuTitle>Order</MenuTitle>
        <StyledMenuItem
          onClick={() => handleOrderSelected('ASC')}
          disabled={disableSort}
        >
          <Tick
            size={'small'}
            checked={selectedOrder == 'ASC'}
            color={theme.colors.text.muted}
          />
          <SortDescending size={'small'} color={theme.colors.text.muted} />
          {ascendingOptionLabel}
        </StyledMenuItem>
        <StyledMenuItem
          onClick={() => handleOrderSelected('DESC')}
          disabled={disableSort}
        >
          <Tick
            size={'small'}
            checked={selectedOrder == 'DESC'}
            color={theme.colors.text.muted}
          />
          <SortAscending size={'small'} color={theme.colors.text.muted} />
          {descendingOptionLabel}
        </StyledMenuItem>
      </Menu>
    </>
  );
}

const StyledButtonBorder = styled(ButtonBorder)`
  border-color: ${props => props.theme.colors.interactive.tonal.neutral[2]};
  gap: ${({ theme }) => theme.space[2]}px;
`;

const Tick = styled(CheckThick)<{ checked: boolean }>`
  opacity: ${({ checked }) => (checked ? 1 : 0)};
`;

const MenuTitle = styled(Text)`
  height: ${({ theme }) => theme.space[4]}px;
  padding: 0 ${({ theme }) => theme.space[3]}px;
  padding-top: ${({ theme }) => theme.space[2]}px;
  font-size: ${({ theme }) => theme.fontSizes[1]}px;
  font-weight: ${({ theme }) => theme.fontWeights.bold};
  color: ${({ theme }) => theme.colors.text.muted};
`;

const StyledMenuItem = styled(MenuItem)`
  min-height: 0;
  gap: ${({ theme }) => theme.space[2]}px;
  font-size: ${({ theme }) => theme.fontSizes[2]}px;
  height: ${({ theme }) => theme.space[5]}px;
  padding-left: ${({ theme }) => theme.space[3]}px;
  padding-right: ${({ theme }) => theme.space[4]}px;
`;
