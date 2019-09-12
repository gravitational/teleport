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
import { keyBy } from 'lodash';
import { withState } from 'shared/hooks';
import AjaxPoller from 'teleport/components/AjaxPoller';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { useStoreSessions, useStoreNodes } from 'teleport/teleport';
import SessionList from './SessionList';

const POLLING_INTERVAL = 3000; // every 3 sec

export function Sessions({ nodes, sessions, onRefresh }) {
  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle>Sessions</FeatureHeaderTitle>
      </FeatureHeader>
      <SessionList sessions={sessions} nodes={nodes} />
      <AjaxPoller time={POLLING_INTERVAL} onFetch={onRefresh} />
    </FeatureBox>
  );
}

function mapState() {
  const sessionStore = useStoreSessions();
  const nodeStore = useStoreNodes();
  function onRefresh() {
    return sessionStore.fetchSessions();
  }

  const nodes = keyBy(nodeStore.getNodes(), 'id');
  const sessions = sessionStore.getSessions();

  return {
    sessions,
    nodes,
    onRefresh,
  };
}

export default withState(mapState)(Sessions);
