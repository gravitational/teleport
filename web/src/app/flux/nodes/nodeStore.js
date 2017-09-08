/*
Copyright 2015 Gravitational, Inc.

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

import reactor from 'app/reactor';
import { Store, toImmutable } from 'nuclear-js';
import { Record, List } from 'immutable';
import { TLPT_NODES_RECEIVE } from './actionTypes';

export class ServerRec extends Record({
  id: '',
  siteId: '',
  hostname: '',
  tags: new List(),
  addr: ''
}) {
  constructor(props) {
    const tags = new List(toImmutable(props.tags));
    super({
      ...props,
      tags
    })
  }
}

class NodeStoreRec extends Record({
  servers: new List()
}) {
   
  findServer(serverId) {    
    return this.servers.find(s => s.id === serverId);      
  }

  getSiteServers(siteId) {
    return this.servers.filter(s => s.siteId === siteId);    
  }

  addSiteServers(jsonItems) {      
    const list = new List().withMutations(state => {
      jsonItems.forEach(item => state.push(new ServerRec(item)));
      return state;
    });
    
    return list.equals(this.servers) ? this : this.set('servers', list);    
  }
}

export function getNodeStore() {
  return reactor.evaluate(['tlpt_nodes']);
}

export default Store({
  getInitialState() {
    return new NodeStoreRec();
  },

  initialize() {
    this.on(TLPT_NODES_RECEIVE, (state, items) => state.addSiteServers(items))
  }
})
