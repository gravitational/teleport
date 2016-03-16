var { sampleData, reactor, expect, Dfd, spyOn, api } = require('./../');
var {actions, getters} = require('app/modules/sessions');

describe('modules/nodes', function () {
  afterEach(function () {
    reactor.reset()
  })

  describe('getters', function () {
    beforeEach(function () {
      spyOn(api, 'get');
    });

    it('should get "partiesBySessionId"', function () {
      api.get.andReturn(Dfd().resolve(sampleData.sessions));
      actions.fetchSessions(new Date(), new Date());
      var sid = sampleData.ids.sids[1];
      var expected = [{"user":"user1","serverIp":"127.0.0.1:60973","serverId":"ad2109a6-42ac-44e4-a570-5ce1b470f9b6","isActive":false},{"user":"user2","serverIp":"127.0.0.1:60973","serverId":"ad2109a6-42ac-44e4-a570-5ce1b470f9b6","isActive":true}];            
      var actual = reactor.evaluate(getters.partiesBySessionId(sid));
      expect(actual).toEqual(expected);
    });

  });
})
