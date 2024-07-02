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

import React, { useState } from 'react';
import { Box, LabelInput } from 'design';
import { SelectAsync } from 'shared/components/Select';

import { useConsoleContext } from 'teleport/Console/consoleContextProvider';

export default function ClusterSelector({
  value,
  onChange,
  defaultMenuIsOpen = false,
  ...styles
}) {
  const consoleCtx = useConsoleContext();
  const [errorMessage, setError] = useState(null);
  const [options, setOptions] = useState<Option[]>([]);

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
      promise = consoleCtx.clustersService
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
      <SelectAsync
        noOptionsMessage={getNoOptionsMessage}
        value={selectedOption}
        onChange={onChangeOption}
        loadOptions={onLoadOptions}
        defaultMenuIsOpen={defaultMenuIsOpen}
        hasError={false}
        maxMenuHeight={400}
        isSearchable
        isSimpleValue={false}
        isClearable={false}
        defaultOptions
        cacheOptions
      />
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
