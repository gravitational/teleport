var { sampleData, reactor, expect, Dfd, spyOn, api } = require('./../');
var { getters, actions } = require('app/modules/nodes');

describe('modules/nodes', function() {
  beforeEach(function() {
    spyOn(api, 'get');
  });

  afterEach(function() {
    reactor.reset()
  })

  describe('getters and actions', function() {
    beforeEach(function() {
      api.get.andReturn(Dfd().resolve(sampleData.nodes));
      actions.fetchNodes();
    });

    it('should get "nodeListView"', function() {            
      var expected = [{"id":"ad2109a6-42ac-44e4-a570-5ce1b470f9b6","hostname":"x220","tags":[{"role":"role","value":"mysql"},{"role":"db_status","value":"master","tooltip":"mysql -c status"}],"addr":"0.0.0.0:3022"}];
      expect(reactor.evaluateToJS(getters.nodeListView)).toEqual(expected);
    });

    it('should get "nodeHostNameByServerId"', function() {
      var id = sampleData.ids.serverIds[0];
      expect(reactor.evaluateToJS(getters.nodeHostNameByServerId(id))).toEqual('x220');
    });
  });
})
