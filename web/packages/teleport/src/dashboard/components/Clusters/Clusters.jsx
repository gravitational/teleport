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
import { withState } from 'shared/hooks';
import { Flex } from 'design';
import AjaxPoller from 'teleport/components/AjaxPoller';
import { useStoreClusters } from 'teleport/teleportContextProvider';
import InputSearch from 'teleport/components/InputSearch';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/dashboard/components/Layout';
import ClusterList from './ClusterList';

const POLL_INTERVAL = 4000; // every 4 sec

export function Clusters(props) {
  const { clusters, onRefresh } = props;

  const [searchValue, setSearchValue] = React.useState('');
  function onSearchChange(value) {
    setSearchValue(value);
  }

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <Flex alignItems="center" flex="1">
          <FeatureHeaderTitle mr="5">Clusters</FeatureHeaderTitle>
          <InputSearch
            ml="auto"
            height="30px"
            bg="primary.light"
            onChange={onSearchChange}
          />
        </Flex>
      </FeatureHeader>
      <ClusterList clusters={clusters} filter={searchValue} />
      <AjaxPoller time={POLL_INTERVAL} onFetch={onRefresh} />
    </FeatureBox>
  );
}

function mapState() {
  const store = useStoreClusters();
  const clusters = store.getClusters();

  function onRefresh() {
    return store.fetchClusters();
  }

  return {
    clusters,
    onRefresh,
  };
}

export default withState(mapState)(Clusters);
