var { reactor, expect, Dfd, spyOn } = require('./../');
var {actions, getters} = require('app/modules/user');
var {TLPT_RECEIVE_USER} =  require('app/modules/user/actionTypes');

describe('modules/nodes', function () {

  afterEach(function () {
    reactor.reset()
  })

  describe('getters', function () {
    beforeEach(function () {
      var sample = {"type": "bearer", "token": "bearer token", "user": {"name": "alex", "allowed_logins": ["admin", "bob"]}, "expires_in": 20};
      reactor.dispatch(TLPT_RECEIVE_USER, sample.user);
    });

    it('should return "user"', function () {
      var expected = {"name":"alex","logins":["admin","bob"]};
      expect(reactor.evaluateToJS(getters.user)).toEqual(expected);
    });

  });
})
