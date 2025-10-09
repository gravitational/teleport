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

import ReactSelect, {
  ClearIndicatorProps,
  components,
  DropdownIndicatorProps,
  GroupBase,
  MultiValueRemoveProps,
} from 'react-select';
import ReactSelectAsync from 'react-select/async';
import ReactSelectCreatableAsync from 'react-select/async-creatable';
import CreatableSelect from 'react-select/creatable';
import styled from 'styled-components';

import { ChevronDown, Cross } from 'design/Icon';
import { space, width } from 'design/system';
import { Theme } from 'design/theme/themes/types';

import {
  AsyncCreatableProps,
  AsyncProps,
  CreatableProps,
  Option,
  Props,
  SelectSize,
} from './types';

export default function Select<
  Opt = Option,
  IsMulti extends boolean = false,
  Group extends GroupBase<Opt> = GroupBase<Opt>,
>(props: Props<Opt, IsMulti, Group>) {
  const {
    size = 'medium',
    hasError = false,
    elevated = false,
    stylesConfig,
    closeMenuOnSelect = true,
    components,
    customProps,
    ...restOfProps
  } = props;
  return (
    <StyledSelect selectSize={size} hasError={hasError} elevated={elevated}>
      <ReactSelect<Opt, IsMulti, Group>
        components={{ ...defaultComponents, ...components }}
        menuPlacement="auto"
        className="react-select-container"
        classNamePrefix="react-select"
        isClearable={false}
        isSearchable={true}
        closeMenuOnSelect={closeMenuOnSelect}
        placeholder="Select..."
        styles={stylesConfig}
        customProps={{ size, ...customProps }}
        {...restOfProps}
      />
    </StyledSelect>
  );
}

export function SelectAsync<
  Opt = Option,
  IsMulti extends boolean = false,
  Group extends GroupBase<Opt> = GroupBase<Opt>,
>(props: AsyncProps<Opt, IsMulti, Group>) {
  const {
    size = 'medium',
    hasError = false,
    components,
    customProps,
    ...restOfProps
  } = props;
  return (
    <StyledSelect selectSize={size} hasError={hasError}>
      <ReactSelectAsync<Opt, IsMulti, Group>
        components={{ ...defaultComponents, ...components }}
        className="react-select-container"
        classNamePrefix="react-select"
        isClearable={false}
        isSearchable={true}
        defaultOptions={false}
        cacheOptions={false}
        defaultMenuIsOpen={false}
        placeholder="Select..."
        customProps={{ size, ...customProps }}
        {...restOfProps}
      />
    </StyledSelect>
  );
}

export function SelectCreatable<
  Opt = Option,
  IsMulti extends boolean = false,
  Group extends GroupBase<Opt> = GroupBase<Opt>,
>(props: CreatableProps<Opt, IsMulti, Group>) {
  const {
    size = 'medium',
    hasError = false,
    stylesConfig,
    components,
    customProps,
    ...restOfProps
  } = props;
  return (
    <StyledSelect selectSize={size} hasError={hasError}>
      <CreatableSelect<Opt, IsMulti, Group>
        components={{ ...defaultComponents, ...components }}
        className="react-select-container"
        classNamePrefix="react-select"
        styles={stylesConfig}
        customProps={{ size, ...customProps }}
        {...restOfProps}
      />
    </StyledSelect>
  );
}

export function SelectCreatableAsync<
  Opt = Option,
  IsMulti extends boolean = false,
  Group extends GroupBase<Opt> = GroupBase<Opt>,
>(props: AsyncCreatableProps<Opt, IsMulti, Group>) {
  const {
    size = 'medium',
    hasError = false,
    stylesConfig,
    components,
    customProps,
    ...restOfProps
  } = props;
  return (
    <StyledSelect selectSize={size} hasError={hasError}>
      <ReactSelectCreatableAsync<Opt, IsMulti, Group>
        components={{ ...defaultComponents, ...components }}
        className="react-select-container"
        classNamePrefix="react-select"
        styles={stylesConfig}
        isClearable={false}
        isSearchable={true}
        defaultOptions={false}
        cacheOptions={false}
        defaultMenuIsOpen={false}
        customProps={{ size, ...customProps }}
        {...restOfProps}
      />
    </StyledSelect>
  );
}

function DropdownIndicator(props: DropdownIndicatorProps) {
  return (
    <components.DropdownIndicator {...props}>
      <ChevronDown size={18} />
    </components.DropdownIndicator>
  );
}

function ClearIndicator(props: ClearIndicatorProps) {
  return (
    <components.ClearIndicator {...props}>
      <Cross size={18} />
    </components.ClearIndicator>
  );
}

function MultiValueRemove(props: MultiValueRemoveProps) {
  return (
    <components.MultiValueRemove {...props}>
      <Cross padding="0 8px 0 2px" size={14} />
    </components.MultiValueRemove>
  );
}

const defaultComponents = {
  DropdownIndicator,
  ClearIndicator,
  MultiValueRemove,
};

const selectGeometry: {
  [s in SelectSize]: {
    height: number;
    indicatorPadding: number;
    typography: keyof Theme['typography'];
    multiValueTypography: keyof Theme['typography'];
  };
} = {
  large: {
    height: 48,
    indicatorPadding: 12,
    typography: 'body1',
    multiValueTypography: 'body2',
  },
  medium: {
    height: 40,
    indicatorPadding: 10,
    typography: 'body2',
    multiValueTypography: 'body3',
  },
  small: {
    height: 32,
    indicatorPadding: 6,
    typography: 'body3',
    multiValueTypography: 'body4',
  },
};

function error({ hasError, theme }: { hasError?: boolean; theme: Theme }) {
  if (!hasError) {
    return;
  }

  return {
    borderRadius: 'inherit !important',
    borderWidth: '1px !important',
    borderColor: theme.colors.interactive.solid.danger.default,
    '&:hover': {
      borderColor: `${theme.colors.interactive.solid.danger.default} !important`,
    },
  };
}

/**
 * Don't use directly. If you need to apply a custom style to a dropdown, just
 * apply it to a regular Select component.
 */
const StyledSelect = styled.div<{
  selectSize: SelectSize;
  hasError?: boolean;
  elevated?: boolean;
  isDisabled?: boolean;
}>`
  .react-select-container {
    box-sizing: border-box;
    display: block;
    outline: none;
    width: 100%;
    color: ${props => props.theme.colors.text.main};
    background-color: transparent;
    margin-bottom: 0px;
    border-radius: 4px;

    ${props =>
      props.theme.typography[selectGeometry[props.selectSize].typography]}
  }

  .react-select__control {
    outline: none;
    min-height: ${props => selectGeometry[props.selectSize].height}px;
    height: fit-content;
    border: 1px solid;
    border-color: ${props => props.theme.colors.interactive.tonal.neutral[2]};
    border-radius: 4px;
    background-color: transparent;
    box-shadow: none;

    ${error}

    .react-select__dropdown-indicator {
      padding: ${props => selectGeometry[props.selectSize].indicatorPadding}px;
      color: ${props =>
        props.isDisabled
          ? props.theme.colors.text.disabled
          : props.theme.colors.text.slightlyMuted};
    }
    &:hover {
      border: 1px solid ${props => props.theme.colors.text.muted};
      cursor: pointer;
    }
  }

  .react-select__control--is-focused {
    border-color: ${props =>
      props.theme.colors.interactive.solid.primary.default};
    cursor: pointer;

    .react-select__dropdown-indicator {
      color: ${props => props.theme.colors.text.main};
    }

    &:hover {
      border-color: ${props =>
        props.theme.colors.interactive.solid.primary.default};
    }
  }

  .react-select__value-container {
    padding: 0 0 0 12px;
  }

  .react-select__single-value {
    color: ${props => props.theme.colors.text.main};
  }

  .react-select__placeholder {
    color: ${props => props.theme.colors.text.muted};
  }

  .react-select__multi-value {
    background-color: ${props =>
      props.theme.colors.interactive.tonal.neutral[0]};
    border-radius: 1000px;
    padding: 0 0 0 12px;
    overflow: hidden;

    /* 
     * These margins keep the height of item rows consistent when the select
     * goes multiline. They do so by keeping flex line height consistent between
     * the lines containing only value pills and those with the input container.
     */
    margin-top: 6px;
    margin-bottom: 6px;

    .react-select__multi-value__label {
      color: ${props => props.theme.colors.text.main};
      padding: 0 2px 0 0;
      ${props =>
        props.theme.typography[
          selectGeometry[props.selectSize].multiValueTypography
        ]}
    }
    .react-select__multi-value__remove {
      color: ${props => props.theme.colors.text.slightlyMuted};
      &:hover {
        background-color: ${props =>
          props.theme.colors.interactive.tonal.neutral[0]};
        color: ${props => props.theme.colors.interactive.solid.danger.default};
      }
    }
  }

  .react-select__multi-value--is-disabled {
    .react-select__multi-value__label,
    .react-select__multi-value__remove {
      color: ${props => props.theme.colors.text.disabled};
    }
  }

  .react-select__option {
    cursor: pointer;
    &:hover {
      background-color: ${props =>
        props.theme.colors.interactive.tonal.neutral[0]};
    }
  }

  .react-select__option--is-focused {
    background-color: ${props =>
      props.theme.colors.interactive.tonal.neutral[0]};
    &:hover {
      background-color: ${props =>
        props.theme.colors.interactive.tonal.neutral[0]};
    }
  }

  .react-select__option--is-selected {
    background-color: ${props =>
      props.theme.colors.interactive.tonal.neutral[1]};
    color: inherit;
    font-weight: 500;

    &:hover {
      background-color: ${props =>
        props.theme.colors.interactive.tonal.neutral[1]};
    }
  }

  .react-select__clear-indicator {
    color: ${props => props.theme.colors.text.slightlyMuted};
    padding: ${props => selectGeometry[props.selectSize].indicatorPadding}px;
    &:hover,
    &:focus {
      background-color: ${props =>
        props.theme.colors.interactive.tonal.neutral[0]};
      color: ${props => props.theme.colors.interactive.solid.danger.default};
    }
  }
  .react-select__menu {
    z-index: 10;
    margin-top: 0px;
    // If the component is on an elevated platform (such as a dialog), use a lighter background.
    background-color: ${props =>
      props.elevated
        ? props.theme.colors.levels.popout
        : props.theme.colors.levels.surface};
    box-shadow: ${props => props.theme.boxShadow[1]};

    ${props =>
      props.selectSize === 'small'
        ? props.theme.typography.body3
        : props.theme.typography.body2}

    .react-select__menu-list::-webkit-scrollbar-thumb {
      background: ${props => props.theme.colors.interactive.tonal.neutral[1]};
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
    background-color: ${props =>
      props.theme.colors.interactive.tonal.neutral[0]};
    color: ${props => props.theme.colors.text.disabled};
    border: 1px solid transparent;
    .react-select__single-value,
    .react-select__placeholder {
      color: ${props => props.theme.colors.text.disabled};
    }

    .react-select__indicator {
      color: ${props => props.theme.colors.text.disabled};
    }
  }

  .react-select__input-container {
    color: ${props => props.theme.colors.text.main};
  }

  ${width}
  ${space}
`;
