/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { ButtonBorder, Flex, Menu, MenuItem } from 'design';
import { ArrowDown, ArrowUp } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';

type SortMenuSort<T extends object> = {
  fieldName: Exclude<keyof T, symbol | number>;
  dir: 'ASC' | 'DESC';
};

export const SortMenu = <T extends object>({
  current,
  fields,
  onChange,
}: {
  current: SortMenuSort<T>;
  fields: { value: SortMenuSort<T>['fieldName']; label: string }[];
  onChange: (value: SortMenuSort<T>) => void;
}) => {
  const [anchorEl, setAnchorEl] = useState<HTMLElement>(null);

  const handleOpen = (event: React.MouseEvent<HTMLButtonElement, MouseEvent>) =>
    setAnchorEl(event.currentTarget);

  const handleClose = () => setAnchorEl(null);

  const handleSelect = (value: (typeof fields)[number]['value']) => {
    handleClose();
    onChange({
      fieldName: value,
      dir: current.dir,
    });
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
          aria-label="Sort by"
          aria-haspopup="true"
          aria-expanded={!!anchorEl}
        >
          {fields.find(f => f.value === current.fieldName)?.label}
        </ButtonBorder>
      </HoverTooltip>
      <Menu
        popoverCss={() => `margin-top: 36px; margin-left: 28px;`}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'right',
        }}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'right',
        }}
        anchorEl={anchorEl}
        open={Boolean(anchorEl)}
        onClose={handleClose}
      >
        {fields.map(({ value, label }) => (
          <MenuItem key={value} onClick={() => handleSelect(value)}>
            {label}
          </MenuItem>
        ))}
      </Menu>
      <HoverTooltip tipContent={'Sort direction'}>
        <ButtonBorder
          onClick={() =>
            onChange({
              fieldName: current.fieldName,
              dir: current.dir === 'ASC' ? 'DESC' : 'ASC',
            })
          }
          textTransform="none"
          css={`
            border-top-left-radius: 0;
            border-bottom-left-radius: 0;
            border-color: ${props => props.theme.colors.spotBackground[2]};
          `}
          size="small"
        >
          {current.dir === 'ASC' ? (
            <ArrowUp size={12} />
          ) : (
            <ArrowDown size={12} />
          )}
        </ButtonBorder>
      </HoverTooltip>
    </Flex>
  );
};
