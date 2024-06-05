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

import React from 'react';
import ReactSelect from 'react-select';
import ReactSelectAsync from 'react-select/async';
import CreatableSelect from 'react-select/creatable';
import ReactSelectCreatableAsync from 'react-select/async-creatable';
import styled from 'styled-components';
import { width, space } from 'design/system';

import { Props, AsyncProps, CreatableProps } from './types';

export default function Select(props: Props) {
  const {
    hasError = false,
    elevated = false,
    stylesConfig,
    closeMenuOnSelect = true,
    ...restOfProps
  } = props;
  return (
    <StyledSelect hasError={hasError} elevated={elevated}>
      <ReactSelect
        menuPlacement="auto"
        className="react-select-container"
        classNamePrefix="react-select"
        clearable={false}
        isMulti={false}
        isSearchable={true}
        closeMenuOnSelect={closeMenuOnSelect}
        placeholder="Select..."
        styles={stylesConfig}
        {...restOfProps}
      />
    </StyledSelect>
  );
}

export function SelectAsync(props: AsyncProps) {
  const { hasError = false, ...restOfProps } = props;
  return (
    <StyledSelect hasError={hasError}>
      <ReactSelectAsync
        className="react-select-container"
        classNamePrefix="react-select"
        clearable={false}
        isSearchable={true}
        defaultOptions={false}
        cacheOptions={false}
        defaultMenuIsOpen={false}
        placeholder="Select..."
        {...restOfProps}
      />
    </StyledSelect>
  );
}

export function SelectCreatable(props: CreatableProps) {
  const { hasError = false, stylesConfig, ...restOfProps } = props;
  return (
    <StyledSelect hasError={hasError}>
      <CreatableSelect
        className="react-select-container"
        classNamePrefix="react-select"
        styles={stylesConfig}
        {...restOfProps}
      />
    </StyledSelect>
  );
}

export function SelectCreatableAsync(props: AsyncProps & CreatableProps) {
  const { hasError = false, stylesConfig, ...restOfProps } = props;
  return (
    <StyledSelect hasError={hasError}>
      <ReactSelectCreatableAsync
        className="react-select-container"
        classNamePrefix="react-select"
        styles={stylesConfig}
        clearable={false}
        isSearchable={true}
        defaultOptions={false}
        cacheOptions={false}
        defaultMenuIsOpen={false}
        {...restOfProps}
      />
    </StyledSelect>
  );
}

export const StyledSelect = styled.div`
  .react-select-container {
    box-sizing: border-box;
    display: block;
    font-size: 14px;
    outline: none;
    width: 100%;
    color: ${props => props.theme.colors.text.main};
    background-color: transparent;
    margin-bottom: 0px;
    border-radius: 4px;
  }

  .react-select__control {
    outline: none;
    min-height: 40px;
    height: fit-content;
    border: 1px solid ${props => props.theme.colors.text.muted};
    border-radius: 4px;
    background-color: transparent;
    box-shadow: none;
    ${({ hasError, theme }) => {
      if (hasError) {
        return {
          borderRadius: 'inherit !important',
          borderWidth: '2px !important',
          border: `2px solid ${theme.colors.error.main} !important`,
        };
      }
    }}

    .react-select__dropdown-indicator {
      color: ${props => props.theme.colors.text.muted};
    }

    &:hover,
    &:focus,
    &:active {
      border: 1px solid ${props => props.theme.colors.text.slightlyMuted};
      background-color: ${props => props.theme.colors.spotBackground[0]};
      cursor: pointer;

      .react-select__dropdown-indicator {
        color: ${props => props.theme.colors.text.main};
      }
    }

    .react-select__indicator,
    .react-select__dropdown-indicator {
      &:hover,
      &:focus,
      &:active {
        color: ${props => props.theme.colors.text.main};
      }
    }
  }

  .react-select__control--is-focused {
    border-color: ${props => props.theme.colors.text.slightlyMuted};
    background-color: ${props => props.theme.colors.spotBackground[0]};
    cursor: pointer;

    .react-select__dropdown-indicator {
      color: ${props => props.theme.colors.text.main};
    }
  }

  .react-select__single-value {
    color: ${props => props.theme.colors.text.main};
  }

  .react-select__placeholder {
    color: ${props => props.theme.colors.text.muted};
  }

  .react-select__multi-value {
    background-color: ${props => props.theme.colors.spotBackground[1]};
    .react-select__multi-value__label {
      color: ${props => props.theme.colors.text.main};
      padding: 0 6px;
    }
    .react-select__multi-value__remove {
      color: ${props => props.theme.colors.text.main};
      &:hover {
        background-color: ${props => props.theme.colors.spotBackground[0]};
        color: ${props => props.theme.colors.error.main};
      }
    }
  }

  .react-select__option {
    &:hover {
      cursor: pointer;
      background-color: ${props => props.theme.colors.spotBackground[0]};
    }
  }

  .react-select__option--is-focused {
    background-color: ${props => props.theme.colors.spotBackground[0]};
    &:hover {
      cursor: pointer;
      background-color: ${props => props.theme.colors.spotBackground[0]};
    }
  }

  .react-select__option--is-selected {
    background-color: ${props => props.theme.colors.spotBackground[1]};
    color: inherit;
    font-weight: 500;

    &:hover {
      background-color: ${props => props.theme.colors.spotBackground[1]};
    }
  }

  .react-select__clear-indicator {
    color: ${props => props.theme.colors.text.slightlyMuted};
    &:hover,
    &:focus {
      background-color: ${props => props.theme.colors.spotBackground[0]};
      svg {
        color: ${props => props.theme.colors.error.main};
      }
    }
  }

  .react-select__menu {
    margin-top: 0px;
    // If the component is on an elevated platform (such as a dialog), use a lighter background.
    background-color: ${props =>
      props.elevated
        ? props.theme.colors.levels.popout
        : props.theme.colors.levels.elevated};
    box-shadow: ${props => props.theme.boxShadow[1]};

    .react-select__menu-list::-webkit-scrollbar-thumb {
      background: ${props => props.theme.colors.spotBackground[1]};
      border-radius: 4px;
    }
  }

  .react-select__indicator-separator {
    display: none;
  }

  .react-select__loading-indicator {
    display: none;
  }

  .react-select__control--is-disabled {
    color: ${props => props.theme.colors.text.disabled};
    border: 1px solid ${props => props.theme.colors.text.disabled};
    .react-select__single-value,
    .react-select__placeholder {
      color: ${props => props.theme.colors.text.disabled};
    }

    .react-select__indicator {
      color: ${props => props.theme.colors.text.disabled};
    }
  }

  .react-select__input {
    color: ${props => props.theme.colors.text.main};
  }

  ${width}
  ${space}
`;
