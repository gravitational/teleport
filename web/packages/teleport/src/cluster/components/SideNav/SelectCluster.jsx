/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import ReactSelect from 'react-select';
import styled from 'styled-components';
import { Box } from 'design';

export default function SelectCluster({ py, px, ...props }) {
  return (
    <StyledSelect py={py} px={px}>
      <ReactSelect
        className="react-select-container"
        classNamePrefix="react-select"
        clearable={false}
        placeholder="Select..."
        issimplevalue={true}
        issearchable={false}
        {...props}
      />
    </StyledSelect>
  );
}

const StyledSelect = styled(Box)(
  ({ theme }) => `
  .react-select__control,
  .react-select__control--is-focused {
    min-height: 40px;
  }

  .react-select-container {
    border-radius: 4px;
    border: none;
    box-shadow: inset 0 2px 4px rgba(0, 0, 0, 0.24);
    box-sizing: border-box;
    color: rgba(0, 0, 0, 0.87);
    display: block;
    font-size: 12px;
    font-weight: bold;
    margin-bottom: 0px;
    outline: noe;
    text-transform: uppercase;
    width: 100%;
  }

  .react-select__menu {
    margin-top: 0px;
  }

  react-select__menu-list {
  }

  .react-select__indicator-separator {
    display: none;
  }

  .react-select__control {
    &:hover {
      border-color: transparent;
    }
  }

  .react-select__control--is-focused {
    background-color: transparent;
    border-color: transparent;
    border-radius: 4px;
    border-style: solid;
    border-width: 1px;
    box-shadow: none;
  }

  .react-select__option--is-selected {
    background-color: #cfd8dc;
    color: inherit;
  }

  .react-select__single-value{
    color: inherit;
  }

  .react-select__dropdown-indicator{
    color: ${theme.colors.primary.dark};
  }

  .react-select__control {
    border-color: ${theme.colors.primary.dark};
    background-color: ${theme.colors.primary.main};
    color: ${theme.colors.text.secondary};
    &:hover {
      border-color: ${theme.colors.text.primary};
      .react-select__dropdown-indicator{
        color: ${theme.colors.text.primary};
      }
    }
  }

  .react-select__control--is-focused {
    border-color: ${theme.colors.text.primary};
    .react-select__dropdown-indicator{
      color: ${theme.colors.text.primary};
    }
  }

  .react-select__option--is-selected {
    background-color: #cfd8dc;
    color: inherit;
  }
`
);
