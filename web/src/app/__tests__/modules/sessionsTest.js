var { sampleData, reactor, expect, Dfd, spyOn, api } = require('./../');
var {actions, getters} = require('app/modules/sessions');
var {fetchNodesAndSessions} = require('app/modules/actions');

describe('modules/nodes', function () {
  afterEach(function () {
    reactor.reset()
  })

  describe('getters', function () {
    beforeEach(function () {
      spyOn(api, 'get');
    });

    it('should get "partiesBySessionId"', function () {
      api.get.andReturn(Dfd().resolve(sampleData.nodesAndSessions));
      fetchNodesAndSessions();
      var sid = sampleData.nodesAndSessions.nodes[0].sessions[0].id;
      var expected = [{"user":"user1","isActive":true},{"user":"user2","isActive":false}];
      var actual = reactor.evaluate(getters.partiesBySessionId(sid));
      expect(actual).toEqual(expected);
    });

  });
})
