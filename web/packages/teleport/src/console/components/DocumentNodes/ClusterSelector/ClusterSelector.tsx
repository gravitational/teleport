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

import styled from 'styled-components';
import React from 'react';
import { Box, LabelInput } from 'design';
import { SelectAsync } from 'shared/components/Select';
import { useConsoleContext } from 'teleport/console/consoleContextProvider';

export default function ClusterSelector({
  value,
  onChange,
  defaultMenuIsOpen = false,
  ...styles
}) {
  const consoleCtx = useConsoleContext();
  const [errorMessage, setError] = React.useState(null);
  const [options, setOptions] = React.useState<Option[]>([]);

  const selectedOption = {
    value,
    label: value,
  };

  function onChangeOption(option) {
    onChange(option.value);
  }

  function onLoadOptions(inputValue: string) {
    let promise = Promise.resolve(options);
    if (options.length === 0) {
      promise = consoleCtx
        .fetchClusters()
        .then(clusters =>
          clusters.map(o => ({
            value: o.clusterId,
            label: o.clusterId,
          }))
        )
        .then(options => {
          setOptions(options);
          return options;
        });
    }

    return promise
      .then(options => filterOptions(inputValue, options))
      .catch((err: Error) => {
        setError(err.message);
      });
  }

  function getNoOptionsMessage() {
    if (errorMessage) {
      return `Error: ${errorMessage}`;
    }

    return 'No leaf clusters found';
  }

  return (
    <Box {...styles}>
      <LabelInput> Clusters </LabelInput>
      <StyledSelect>
        <SelectAsync
          noOptionsMessage={getNoOptionsMessage}
          value={selectedOption}
          onChange={onChangeOption}
          loadOptions={onLoadOptions}
          defaultMenuIsOpen={defaultMenuIsOpen}
          hasError={false}
          width={400}
          maxMenuHeight={300}
          isSearchable
          isSimpleValue={false}
          clearable={false}
          defaultOptions
          cacheOptions
        />
      </StyledSelect>
    </Box>
  );
}

function filterOptions(value = '', options: Option[] = []) {
  value = value.toLocaleLowerCase();
  return options.filter(o => {
    return o.value.toLocaleLowerCase().indexOf(value) !== -1;
  });
}

type Option = { value: string; label: string };

const StyledSelect = styled(Box)(
  ({ theme }) => `
  .react-select__control,
  .react-select__control--is-focused {
    border-color: #FFF;
    height: 34px;
    min-height: 34px;
  }

  .react-select__option {
    padding: 4px 12px;
  }
  .react-select__option--is-focused,
  .react-select__option--is-focused:active {
    background-color: ${theme.colors.grey[50]};
  }

  .react-select__menu {
    margin-top: 0px;
    font-size: 14px;
  }

  react-select__menu-list {
  }

  .react-select__indicator-separator {
    display: none;
  }

  .react-select__value-container{
    height: 30px;
    padding: 0 8px;
  }

  .react-select__option--is-selected {
    background-color: inherit;
    color: inherit;
  }

  .react-select__option--is-focused {
    background-color: #cfd8dc;
    color: inherit;
  }

  .react-select__single-value{
    padding: 0 4px !important;
    margin: 0 !important;
    color: white;
    font-size: 14px;
  }

  .react-select__dropdown-indicator{
    padding: 4px 8px;
    color: ${theme.colors.text.secondary};
  }

  input {
    font-family: ${theme.font};
    font-size: 14px;
    height: 26px;
  }

  .react-select__input {
    color: white;
    height: 20px;
    font-size: 14px;
    font-family: ${theme.font};
  }

  .react-select__control {
    border-radius: 4px;
    border-color: rgba(255, 255, 255, 0.24);
    background-color: ${theme.colors.primary.light};
    color: ${theme.colors.text.secondary};

    &:focus, &:active {
      background-color: ${theme.colors.primary.lighter};
    }

    &:hover {
      border-color: rgba(255, 255, 255, 0.24);
      background-color: ${theme.colors.primary.lighter};
      .react-select__dropdown-indicator{
        color: ${theme.colors.text.primary};
      }
    }
  }

  .react-select__control--is-focused {
    background-color: ${theme.colors.primary.lighter};
    border-color: transparent;
    border-radius: 4px;
    border-style: solid;
    border-width: 1px;
    box-shadow: none;
    border-color: rgba(255, 255, 255, 0.24);

    .react-select__dropdown-indicator{
      color: ${theme.colors.text.secondary};
    }
  }

  .react-select__menu {
    border-top-left-radius: 0;
    border-top-right-radius: 0;
  }
`
);
