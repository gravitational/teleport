var { reactor, expect, Dfd, spyOn, api } = require('./../');
var {actions, getters} = require('app/modules/nodes');

describe('modules/nodes', function () {
  beforeEach(function () {
    spyOn(api, 'get');
  });

  afterEach(function () {
    reactor.reset()
  })

  describe('getters and actions', function () {
    beforeEach(function () {
      api.get.andReturn(Dfd().resolve({ "nodes": [ { "addr": "127.0.0.1:8080", "hostname": "a.example.com", "labels": {"role": "mysql"}, "cmd_labels": { "db_status": { "command": "mysql -c status", "result": "master", "period": 1000000000 }} } ] }) );
      actions.fetchNodes();
    });

    it('should handle "nodeInfos"', function () {
      var expected = [{"tags":[{"role":"role","value":"mysql"},{"role":"db_status","value":"master","tooltip":"mysql -c status"}],"ip":"127.0.0.1:8080"}]
      expect(reactor.evaluateToJS(getters.nodeListView)).toEqual(expected);
    });

  });
})
