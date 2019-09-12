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

import { Store } from 'nuclear-js';
import { Record } from 'immutable';
import { SITE_SERVERS_RECEIVE } from './actionTypes';

const StoreRec = Record({
  nodes: {},
})

export default Store({
  getInitialState() {
    return new StoreRec();
  },

  initialize() {
    this.on(SITE_SERVERS_RECEIVE, receiveNodes);
  }
})

function receiveNodes(state, { gravityNodes, k8sNodes, canSsh, sshLogins }) {
  k8sNodes = k8sNodes || {};
  const nodes = gravityNodes.map(node => {
    const k8s = k8sNodes[node.advertiseIp] || {};
    return {
      ...node,
      canSsh,
      sshLogins,
      k8s
    }
  });

  return state.set('nodes', nodes);
}