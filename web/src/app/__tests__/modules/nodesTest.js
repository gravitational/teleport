var { sampleData, reactor, expect, Dfd, spyOn, api } = require('./../');
var getters = require('app/modules/nodes/getters');
var actions = require('app/modules/actions');

describe('modules/nodes', function () {
  beforeEach(function () {
    spyOn(api, 'get');
  });

  afterEach(function () {
    reactor.reset()
  })

  describe('getters and actions', function () {
    beforeEach(function () {
      api.get.andReturn(Dfd().resolve(sampleData.nodesAndSessions));
      actions.fetchNodesAndSessions();
    });

    it('should get "nodeInfos"', function () {
      var expected = [{"tags":[{"role":"role","value":"mysql"},{"role":"db_status","value":"master","tooltip":"mysql -c status"}],"addr":"0.0.0.0:3022","sessionCount":1}];
      expect(reactor.evaluateToJS(getters.nodeListView)).toEqual(expected);
    });

  });
})
