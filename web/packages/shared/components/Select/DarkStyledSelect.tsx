// Copyright 2021 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import styled from 'styled-components';

import { StyledSelect } from './Select';

const StyledDarkSelect = styled(StyledSelect)(
  ({ theme }) => `
  .react-select-container {
    background: transparent;
  }
  .react-select__option--is-focused:active {
    background-color: ${theme.colors.grey[50]};
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
        color: #666;
      }
    }
    .react-select__indicator,
    .react-select__dropdown-indicator {
      padding: 4px 8px;
      color: #666;
      &:hover {
        color: #999;
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
      color: #999 !important;
      &:hover {
        color: #ccc !important;
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
