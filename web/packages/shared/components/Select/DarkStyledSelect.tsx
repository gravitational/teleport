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

import styled from 'styled-components';

import { StyledSelect } from './Select';

const StyledDarkSelect = styled(StyledSelect)(
  ({ theme }) => `
  .react-select-container {
    background: transparent;
  }
  
  .react-select__value-container {
    padding: 0 8px;
  }
  .react-select__single-value {
    color: ${theme.colors.text.main}
  }
  
  .react-select__control {
    min-height: 34px;
    height: 34px;
    border-color: rgba(255, 255, 255, 0.24);
    color: ${theme.colors.text.slightlyMuted};
    &:focus, &:active {
      background-color: ${theme.colors.levels.elevated};
    }
    &:hover {
      border-color: rgba(255, 255, 255, 0.24);
      background-color: ${theme.colors.levels.elevated};
      .react-select__dropdown-indicator {
        color: ${theme.colors.text.main};
      }
    }
    .react-select__indicator,
    .react-select__dropdown-indicator {
      padding: 4px 8px;
      color: ${theme.colors.text.main};
      &:hover {
        color: ${theme.colors.text.main};
      }
    }
  }
  .react-select__control--menu-is-open {
    background-color: ${theme.colors.levels.elevated};
    border-color: transparent;
    border-radius: 4px;
    border-style: solid;
    border-width: 1px;
    box-shadow: none;
    border-color: rgba(255, 255, 255, 0.24);
    .react-select__indicator,
    .react-select__dropdown-indicator {
      color: ${theme.colors.spotBackground[0]}
      &:hover {
        color: ${theme.colors.spotBackground[1]}
      }
    }
  }
  .react-select__input {
    color: ${theme.colors.text.main}
  }
  .react-select__placeholder {
    color: ${theme.colors.text.slightlyMuted}
  }
  .react-select__option {
    padding: 4px 12px;
  } 
  .react-select__menu {
    border-top-left-radius: 0;
    border-top-right-radius: 0;
  }
  .react-select__multi-value {
    background-color: ${theme.colors.levels.sunkenSecondary};
    border: 1px solid ${theme.colors.text.muted};
  }
  .react-select__multi-value__label {
    color: ${theme.colors.text.main};
    padding: 0 6px;
  }
  .react-select--is-disabled {
    .react-select__single-value,
    .react-select__placeholder,
    .react-select__indicator {
      color: ${theme.colors.text.muted};
    }
  }
`
);

export default StyledDarkSelect;
