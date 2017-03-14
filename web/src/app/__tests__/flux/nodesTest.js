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

var { sampleData, reactor, expect, Dfd, spyOn, api } = require('./../');
var { getters, actions } = require('app/flux/nodes');
var { setSiteId } = require('app/flux/app/actions');

describe('flux/nodes', function () {
  let siteid = 'siteid123';
  
  beforeEach(() => {
    setSiteId(siteid);  
    spyOn(api, 'get');
  });

  afterEach(function() {
    reactor.reset()
    expect.restoreSpies();
  })

  describe('getters and actions', function() {
    beforeEach(function() {
      api.get.andReturn(Dfd().resolve(sampleData.nodes));
      actions.fetchNodes();
    });

    it('should get "nodeListView"', function () {      
      var expected = [{"id":"ad2109a6-42ac-44e4-a570-5ce1b470f9b6","siteId": "siteid123", "hostname":"x220","tags":[{"role":"role","value":"mysql"},{"role":"db_status","value":"master","tooltip":"mysql -c status"}],"addr":"0.0.0.0:3022"}];
      expect(reactor.evaluateToJS(getters.nodeListView)).toEqual(expected);
    });

    it('should get "nodeHostNameByServerId"', function() {
      var id = sampleData.ids.serverIds[0];
      expect(reactor.evaluateToJS(getters.nodeHostNameByServerId(id))).toEqual('x220');
    });
  });
})
