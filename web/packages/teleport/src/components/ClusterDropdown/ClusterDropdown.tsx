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

import React, { useEffect, useState } from 'react';
import { useHistory } from 'react-router';
import styled from 'styled-components';

import { Box, ButtonSecondary, Flex, Menu, MenuItem, Text } from 'design';
import { ChevronDown } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';

import cfg from 'teleport/config';
import { Cluster } from 'teleport/services/clusters';

export interface ClusterDropdownProps {
  clusterLoader: ClusterLoader;
  clusterId: string;
  /*
   * onChange is an optional prop. If onChange is not passed, it will use the built in "changeCluster" function
   */
  onChange?: (newValue: string) => void;
  /*
   * onError is required because this dropdown can be placed on any page, it does not display its own error
   * messages. Even if using the internal "loadClusters", we will pass the error back to be consumed by the parent.
   */
  onError: (errorMessage: string) => void;
  mb?: number;
}

interface ClusterLoader {
  fetchClusters: (
    signal?: AbortSignal,
    fromCache?: boolean
  ) => Promise<Cluster[]>;
  clusters: Cluster[];
}

function createOptions(clusters: Cluster[]) {
  return clusters.map(cluster => ({
    value: cluster.clusterId,
    label: cluster.clusterId,
  }));
}

export function ClusterDropdown({
  clusterLoader,
  clusterId,
  onChange,
  onError,
  mb = 0,
}: ClusterDropdownProps) {
  const initialClusters = clusterLoader.clusters;
  const [options, setOptions] = React.useState<Option[]>(
    createOptions(initialClusters)
  );
  const showInput = options.length > 5 ? true : false;
  const [clusterFilter, setClusterFilter] = useState('');
  const history = useHistory();
  const [anchorEl, setAnchorEl] = useState(null);

  const selectedOption = {
    value: clusterId,
    label: clusterId,
  };

  function loadClusters(signal: AbortSignal) {
    onError('');
    try {
      return clusterLoader.fetchClusters(signal);
    } catch (err) {
      onError(err.message);
    }
  }

  function changeCluster(clusterId: string) {
    const newPathName = cfg.getClusterRoute(clusterId);

    const oldPathName = cfg.getClusterRoute(selectedOption.value);

    const newPath = history.location.pathname.replace(oldPathName, newPathName);

    // keep current view just change the clusterId
    history.push(newPath);
  }

  function onChangeOption(clusterId: string) {
    if (onChange) {
      onChange(clusterId);
    } else {
      changeCluster(clusterId);
    }
    handleClose();
  }

  useEffect(() => {
    const signal = new AbortController();
    async function getOptions() {
      try {
        const res = await loadClusters(signal.signal);
        setOptions(createOptions(res));
      } catch (err) {
        onError(err.message);
      }
    }

    getOptions();
    return () => {
      signal.abort();
    };
  }, []);

  const handleOpen = event => {
    setAnchorEl(event.currentTarget);
  };

  const handleClose = () => {
    setAnchorEl(null);
  };

  // If only a single cluster is available, hide the dropdown
  if (options.length < 2) {
    return null;
  }

  const onClusterFilterChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setClusterFilter(e.target.value);
  };

  let filteredOptions = options;
  if (clusterFilter) {
    filteredOptions = options.filter(cluster =>
      cluster.label.toLowerCase().includes(clusterFilter.toLowerCase())
    );
  }

  return (
    <Flex textAlign="center" alignItems="center" mb={mb}>
      <HoverTooltip tipContent={'Select cluster'}>
        <ButtonSecondary size="small" onClick={handleOpen}>
          Cluster: {selectedOption.label}
          <ChevronDown ml={2} size="small" color="text.slightlyMuted" />
        </ButtonSecondary>
      </HoverTooltip>
      <Menu
        popoverCss={() => `
          margin-top: ${showInput ? '40px' : '4px'}; 
          max-height: 265px; 
          overflow: hidden; 
        `}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'left',
        }}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'left',
        }}
        anchorEl={anchorEl}
        open={Boolean(anchorEl)}
        onClose={handleClose}
      >
        {showInput ? (
          <Box
            css={`
              padding: ${p => p.theme.space[2]}px;
            `}
          >
            <ClusterFilter
              type="text"
              autoFocus
              value={clusterFilter}
              autoComplete="off"
              onChange={onClusterFilterChange}
              placeholder={'Search clusters…'}
            />
          </Box>
        ) : (
          // without this empty box, the entire positioning is way out of whack
          // TODO (avatus): find out why during menu/popover rework
          <Box />
        )}
        <Box
          css={`
            max-height: 220px;
            overflow: auto;
          `}
        >
          {filteredOptions.map(cluster => (
            <MenuItem
              px={2}
              key={cluster.value}
              onClick={() => onChangeOption(cluster.value)}
            >
              <Text
                ml={2}
                fontWeight={cluster.value === clusterId ? 500 : 300}
                fontSize={2}
              >
                {cluster.label}
              </Text>
            </MenuItem>
          ))}
        </Box>
      </Menu>
    </Flex>
  );
}

type Option = { value: string; label: string };

const ClusterFilter = styled.input(
  ({ theme }) => `
  background-color: ${theme.colors.spotBackground[0]};
  padding-left: ${theme.space[3]}px;
  width: 100%;
  border-radius: 29px;
  box-sizing: border-box;
  color: ${theme.colors.text.main};
  height: 32px;
  font-size: ${theme.fontSizes[1]}px;
  outline: none;
  border: none;
  &:focus {
    border: none;
  }

  &::placeholder {
    color: ${theme.colors.text.muted};
    opacity: 1;
  }
`
);
