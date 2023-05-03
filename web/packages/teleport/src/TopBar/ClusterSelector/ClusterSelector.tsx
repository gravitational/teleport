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
import { components } from 'react-select';
import styled from 'styled-components';
import { Box, Flex, Text } from 'design';
import { SelectAsync, DarkStyledSelect } from 'shared/components/Select';

const ValueContainer = ({ children, ...props }) => (
  <components.ValueContainer {...props}>
    <Flex alignItems="center" color="text.main">
      <Text typography="h6" fontWeight="regular" mr="2">
        CLUSTER:
      </Text>
      {children}
    </Flex>
  </components.ValueContainer>
);

export default function ClusterSelector({
  value,
  onChange,
  onLoad,
  defaultMenuIsOpen = false,
  ...styles
}) {
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
      promise = onLoad()
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
    <StyledBox {...styles} className="teleport-cluster-selector">
      <StyledSelect>
        <SelectAsync
          components={{ ValueContainer }}
          noOptionsMessage={getNoOptionsMessage}
          value={selectedOption}
          onChange={onChangeOption}
          loadOptions={onLoadOptions}
          defaultMenuIsOpen={defaultMenuIsOpen}
          hasError={false}
          maxMenuHeight={600}
          menuPosition="fixed"
          isSearchable
          isSimpleValue={false}
          isClearable={false}
          defaultOptions
          cacheOptions
        />
      </StyledSelect>
    </StyledBox>
  );
}

function filterOptions(value = '', options: Option[] = []) {
  value = value.toLocaleLowerCase();
  return options.filter(o => {
    return o.value.toLocaleLowerCase().indexOf(value) !== -1;
  });
}

type Option = { value: string; label: string };

const StyledSelect = styled(DarkStyledSelect)`
  .react-select__value-container {
    padding: 0 16px;
  }

  .react-select__single-value {
    transform: none;
    position: absolute;
    left: 86px;
    top: 4px;
    width: 270px;
    text-overflow: ellipsis;
  }

  .react-select__control {
    min-height: 42px;
    height: 42px;
    border-color: #1f264b;

    &:focus,
    &:active {
      border-color: #1f264b;
      background: #1f264b;
    }
    &:hover {
      border-color: #1f264b;
      background: #1f264b;

      .react-select__dropdown-indicator {
        color: white;
      }
    }
  }

  .react-select__indicator,
  .react-select__dropdown-indicator {
    padding: 4px 16px;
    color: white;
    &:hover {
      color: white;
    }
  }

  .react-select__control--menu-is-open {
    .react-select__indicator,
    .react-select__dropdown-indicator {
      color: white !important;
      &:hover {
        color: white !important;
      }
    }
  }
`;

const StyledBox = styled(Box)`
  &.mute {
    opacity: 0.5;
    pointer-events: none;
  }
`;
