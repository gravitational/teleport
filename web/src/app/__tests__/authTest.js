var expect = require('expect');
var $ = require('jQuery');
var api = require('app/services/api');
var session = require('app/session');
var spyOn = expect.spyOn;
var auth = require('app/auth');

describe('auth', function () {
  var sample = { token: 'token' };

  beforeEach(function () {
    spyOn(session, 'setUserData');
    spyOn(session, 'getUserData');
    spyOn(session, 'clear');
    spyOn(api, 'post');
    spyOn(auth, '_startTokenRefresher');
    spyOn(auth, '_stopTokenRefresher');
    spyOn(auth, '_getRefreshTokenTimerId');
  });

  afterEach(function () {
    expect.restoreSpies();
  })

  describe('login(username, password, token)', function () {
    it('should successfully login and put user data in the session', function () {
      var token = null;
      api.post.andReturn($.Deferred().resolve(sample));
      auth.login('user', 'password').done(user=>{ token = sample.token; });

      expect(token).toEqual(sample.token);
      expect(auth._startTokenRefresher.calls.length).toEqual(1);
      expect(getCallArgs(session.setUserData).token, sample.token);
    });

    it('should return rejected promise if failed to log in', function () {
      var token = null;
      var wasCalled = false;
      api.post.andReturn($.Deferred().reject());
      auth.login('user', 'password').fail(()=> { wasCalled = true });
      expect(wasCalled).toEqual(true);
    });
  });

  describe('ensureUser()', function () {
    describe('when session has a token and refreshTimer is active', function () {
      it('should be resolved', function () {
        var wasCalled = false;
        auth._getRefreshTokenTimerId.andReturn(11);
        session.getUserData.andReturn(sample);
        auth.ensureUser('user', 'password').done(()=> { wasCalled = true });

        expect(wasCalled).toEqual(true);
      });
    });

    describe('when session has a token but refreshTimer is not active (browser refresh case)', function () {
      it('should be resolved succesfully if token is valid', function () {
        var wasCalled = false;
        api.post.andReturn($.Deferred().resolve(sample));
        auth._getRefreshTokenTimerId.andReturn(null);
        session.getUserData.andReturn(sample);
        auth.ensureUser('user', 'password').done(()=> { wasCalled = true });

        expect(api.post.calls.length).toEqual(1);
        expect(wasCalled).toEqual(true);
      });

      it('should be rejected if token is invalid', function () {
        var wasCalled = false;
        api.post.andReturn($.Deferred().reject());
        auth._getRefreshTokenTimerId.andReturn(null);
        session.getUserData.andReturn(sample);
        auth.ensureUser('user', 'password').fail(()=> { wasCalled = true });

        expect(api.post.calls.length).toEqual(1);
        expect(wasCalled).toEqual(true);
      });
    });
  });

  describe('logout()', function () {
    it('should clear the session and stop refreshTimer', function () {
      auth.logout();
      expect(session.clear.calls.length).toEqual(1);
      expect(auth._stopTokenRefresher.calls.length).toEqual(1);
    });
  });

  function getCallArgs(spy){
    return spy.getLastCall().arguments[0];
  }
})
