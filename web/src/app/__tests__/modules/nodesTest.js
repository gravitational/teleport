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
      api.get.andReturn(Dfd().resolve([{id: 1, ip: '127.3.4.5:1000', count: 1, name: 'alex'}, {id: 2, ip: '227.1.1.5:1000', count: 4, name: 'martha'}]));
      actions.fetchNodes();
    });

    it('should handle "nodeInfos"', function () {
      var expected = [{"count":1,"ip":"127.3.4.5:1000","tags":["tag1","tag2","tag3"],"roles":["r1","r2","r3"]},{"count":4,"ip":"227.1.1.5:1000","tags":["tag1","tag2","tag3"],"roles":["r1","r2","r3"]}];
      expect(reactor.evaluateToJS(getters.nodeListView)).toEqual(expected);
    });

  });
})
