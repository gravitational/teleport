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

import { reactor, expect, Dfd, spyOn, api } from './../';
import actions from 'app/flux/nodes/actions';
import getters from 'app/flux/nodes/getters';
import { getNodeStore } from 'app/flux/nodes/nodeStore';
import { setSiteId } from 'app/flux/app/actions';
import { nodes } from 'app/__tests__/apiData'

describe('flux/nodes', () => {
  const siteid = 'siteid123';
  const serverId = 'ad2109a6-42ac-44e4-a570-5ce1b470f9b6';
  
  beforeEach(() => {
    setSiteId(siteid);  
    spyOn(api, 'get');
  });

  afterEach(() => {
    reactor.reset()
    expect.restoreSpies();
  })

  describe('getters and actions', () => {
    beforeEach(() => {
      api.get.andReturn(Dfd().resolve(nodes));
      actions.fetchNodes();
    });

    it('should get cluster nodes"', () => {                           
      expect(reactor.evaluateToJS(getters.siteNodes)).toEqual(nodes.items);
    });

    it('should findServer', () => {            
      const server = getNodeStore().findServer(serverId);
      expect(server.hostname).toEqual('x220');
    });
  });
})
