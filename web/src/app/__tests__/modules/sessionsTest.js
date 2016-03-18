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

describe('modules/nodes', function () {

  beforeEach( ()=> spyOn(api, 'get').andReturn(Dfd()) );

  afterEach(function () {
    reactor.reset()
    expect.restoreSpies();
  })

  describe('actions', function () {
    describe('fetchSessions', function () {
      it('should fetch for last sessions when called w/o params', function () {
        spyOn(cfg.api, 'getFetchSessionsUrl');
        actions.fetchSessions();

        let expected = {
          limit: 50,
          order: -1,
          start: newDateMinusMin()
        }

        let [actual] = cfg.api.getFetchSessionsUrl.calls[0].arguments;

        expect(api.get.calls.length).toBe(1);
        expect(actual.limit).toBe(expected.limit);
        expect(actual.order).toBe(expected.order);
        expect(new Date(actual.start).getTime())
          .toBeGreaterThan(expected.start.getTime());
      });

      it('should fetch based on the input params', function () {
        let sid = 'xx';
        let before = new Date();
        let limit = 33;

        spyOn(cfg.api, 'getFetchSessionsUrl');
        actions.fetchSessions({sid, before, limit});

        let [actual] = cfg.api.getFetchSessionsUrl.calls[0].arguments;

        expect(api.get.calls.length).toBe(1);
        expect(actual.limit).toBe(limit);
        expect(actual.sid).toBe(sid);
        expect(new Date(actual.start).getTime()).toBe(before.getTime());
      })

    });
  });

  describe('getters', function () {
    beforeEach(function () {
      spyOn(api, 'get');
    });

    it('should get "partiesBySessionId"', function () {
      api.get.andReturn(Dfd().resolve(sampleData.sessions));
      actions.fetchSessions();
      var sid = sampleData.ids.sids[1];
      var expected = [{"user":"user1","serverIp":"127.0.0.1:60973","serverId":"ad2109a6-42ac-44e4-a570-5ce1b470f9b6","isActive":false},{"user":"user2","serverIp":"127.0.0.1:60973","serverId":"ad2109a6-42ac-44e4-a570-5ce1b470f9b6","isActive":true}];
      var actual = reactor.evaluate(getters.partiesBySessionId(sid));
      expect(actual).toEqual(expected);
    });

    it('should get "sessionsView"', function () {
      api.get.andReturn(Dfd().resolve(sampleData.sessions));
      actions.fetchSessions();
      var expected = '[{"sid":"f60c4f1e-aedd-4fa6-8fe5-8068b49b17b4","sessionUrl":"/web/sessions/f60c4f1e-aedd-4fa6-8fe5-8068b49b17b4","active":false,"created":"2016-03-12T20:25:02.748Z","lastActive":"2016-03-12T20:25:02.748Z","login":"akontsevoy","parties":[],"cols":115,"rows":34},{"sid":"11d76502-0ed7-470c-9ae2-472f3873fa6e","sessionUrl":"/web/sessions/11d76502-0ed7-470c-9ae2-472f3873fa6e","serverIp":"127.0.0.1:60973","serverId":"ad2109a6-42ac-44e4-a570-5ce1b470f9b6","active":true,"created":"2016-03-15T19:55:49.251Z","lastActive":"2016-03-15T19:55:49.251Z","login":"akontsevoy","parties":[{"user":"user1","serverIp":"127.0.0.1:60973","serverId":"ad2109a6-42ac-44e4-a570-5ce1b470f9b6","isActive":false},{"user":"user2","serverIp":"127.0.0.1:60973","serverId":"ad2109a6-42ac-44e4-a570-5ce1b470f9b6","isActive":true}],"cols":114,"rows":36}]';
      var actual = reactor.evaluate(getters.sessionsView);
      expect(JSON.stringify(actual)).toEqual(expected);
    });
  });
})

function newDateMinusMin(){
  let d = new Date();
  d.setMinutes(d.getMinutes() - 1);
  return d;
}
