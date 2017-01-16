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

var { sampleData, reactor, expect, Dfd, cfg, spyOn, api } = require('./../');
var {actions, getters} = require('app/modules/sessions');
var { setSiteId } = require('app/modules/app/actions');

describe('modules/sessions', function () {
  let siteid = 'siteid123';

  beforeEach(() => {
    setSiteId(siteid);  
    spyOn(api, 'get').andReturn(Dfd())
  });

  afterEach(function () {
    reactor.reset()
    expect.restoreSpies();
  })

  describe('actions', function () {
    describe('fetchActiveSessions', function () {

      it('should fetch based on the input params', function () {  
        spyOn(cfg.api, 'getFetchSessionsUrl');
        actions.fetchActiveSessions();

        let [actualSiteId] = cfg.api.getFetchSessionsUrl.calls[0].arguments;
        expect(api.get.calls.length).toBe(1);
        expect(actualSiteId).toBe(siteid);                
      })

    });
  });

  describe('getters', function () {
    beforeEach(function () {
      spyOn(api, 'get');
    });

    it('should get "activeSessionById"', function () {
      api.get.andReturn(Dfd().resolve(sampleData.sessions));
      actions.fetchActiveSessions();
      var sid = sampleData.ids.sids[1];
      var actual = reactor.evaluate(getters.activeSessionById(sid));
      var expected = {
        'id': '11d76502-0ed7-470c-9ae2-472f3873fa6e',
        'login': 'akontsevoy',
        'namespace': undefined,
        'server_id': undefined,
        'active': true,
        'created': '2016-03-15T19:55:49.251601013Z',
        'last_active': '2016-03-15T19:55:49.251601164Z',
        'siteId': 'siteid123',
        'parties': [
          {
            'user': 'user1',
            'serverIp': '127.0.0.1:60973',
            'serverId': 'ad2109a6-42ac-44e4-a570-5ce1b470f9b6'
          },
          {
            'user': 'user2',
            'serverIp': '127.0.0.1:60973',
            'serverId': 'ad2109a6-42ac-44e4-a570-5ce1b470f9b6'
          }
        ]
      }
      
      actual = actual.toJS();      
      expect(actual).toEqual(expected);
    });    
  });
})