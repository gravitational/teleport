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

import React, { useState, useEffect } from 'react';
import { useHistory } from 'react-router';
import { ButtonSecondary, Flex, Menu, MenuItem, Text } from 'design';
import { ChevronDown } from 'design/Icon';
import cfg from 'teleport/config';
import { Cluster } from 'teleport/services/clusters';

import { HoverTooltip } from 'shared/components/ToolTip';

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
}: ClusterDropdownProps) {
  const initialClusters = clusterLoader.clusters;
  const [options, setOptions] = React.useState<Option[]>(
    createOptions(initialClusters)
  );
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

  return (
    <Flex textAlign="center" alignItems="center">
      <HoverTooltip tipContent={'Select cluster'}>
        <ButtonSecondary
          px={2}
          css={`
            border-color: ${props => props.theme.colors.spotBackground[0]};
          `}
          textTransform="none"
          size="small"
          onClick={handleOpen}
        >
          {selectedOption.label}
          <ChevronDown ml={2} size="small" color="text.slightlyMuted" />
        </ButtonSecondary>
      </HoverTooltip>
      <Menu
        popoverCss={() => `margin-top: 36px;`}
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
        {options.map(cluster => (
          <MenuItem
            px={2}
            key={cluster.value}
            onClick={() => onChangeOption(cluster.value)}
          >
            <Text ml={2} fontWeight={300} fontSize={2}>
              {cluster.label}
            </Text>
          </MenuItem>
        ))}
      </Menu>
    </Flex>
  );
}

type Option = { value: string; label: string };
