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
import InputSearch from 'teleport/components/InputSearch';
import AjaxPoller from 'teleport/components/AjaxPoller';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { useStoreUser, useStoreNodes } from 'teleport/teleport';
import NodeList from './NodeList';

const POLLING_INTERVAL = 10000; // every 10 sec

export function Nodes({ nodes, logins, onFetch }) {
  const [searchValue, setSearchValue] = React.useState('');
  function onSearchChange(value) {
    setSearchValue(value);
  }

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle mr="5">Nodes</FeatureHeaderTitle>
        <InputSearch bg="primary.light" autoFocus onChange={onSearchChange} />
      </FeatureHeader>
      <NodeList logins={logins} nodes={nodes} search={searchValue} />
      <AjaxPoller time={POLLING_INTERVAL} onFetch={onFetch} />
    </FeatureBox>
  );
}

const mapState = () => {
  const store = useStoreNodes();
  const storeUser = useStoreUser();
  return {
    onFetch: () => store.fetchNodes(),
    nodes: store.state,
    logins: storeUser.getLogins(),
  };
};

export default withState(mapState)(Nodes);
