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
import { components } from 'react-select';
import styled from 'styled-components';
import { Box, Flex, Text } from 'design';
import { SelectAsync } from 'shared/components/Select';

const ValueContainer = ({ children, ...props }) => (
  <components.ValueContainer {...props}>
    <Flex alignItems="center" color="text.main">
      <Text typography="body2" mr="2">
        Cluster:
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
    <StyledBox
      {...styles}
      className="teleport-cluster-selector"
      data-testid="cluster-selector"
    >
      <StyledSelectAsync
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

const StyledSelectAsync = styled(SelectAsync)`
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

    .react-select__dropdown-indicator {
      color: ${props => props.theme.colors.text.slightlyMuted};
    }

    &:focus,
    &:active {
      background: ${props => props.theme.colors.levels.surface};
      border-color: ${props => props.theme.colors.text.main};
    }
    &:hover {
      background: ${props => props.theme.colors.levels.surface};
      border-color: ${props => props.theme.colors.text.main};

      .react-select__dropdown-indicator {
        color: ${props => props.theme.colors.text.main};
      }
    }
  }

  .react-select__indicator,
  .react-select__dropdown-indicator {
    padding: 4px 16px;
    color: ${props => props.theme.colors.text.slightlyMuted};
    &:hover {
      color: ${props => props.theme.colors.text.main};
    }
  }

  .react-select__control--menu-is-open {
    .react-select__indicator,
    .react-select__dropdown-indicator {
      color: ${props => props.theme.colors.text.main};
      &:hover {
        color: ${props => props.theme.colors.text.main};
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
