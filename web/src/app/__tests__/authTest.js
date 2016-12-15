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

var expect = require('expect');
var $ = require('jQuery');
var api = require('app/services/api');
var session = require('app/services/session');
var spyOn = expect.spyOn;
var auth = require('app/services/auth');

describe('auth', function () {
  var sample = { token: 'token', expires_in: -599, created: new Date().getTime() };

  beforeEach(function () {
    spyOn(session, 'setUserData');
    spyOn(session, 'getUserData');
    spyOn(session, 'clear');
    spyOn(api, 'post');
    spyOn(api, 'delete').andReturn($.Deferred().resolve());
    spyOn(auth, '_startTokenRefresher');
    spyOn(auth, '_stopTokenRefresher');
    spyOn(auth, '_redirect');
    spyOn(auth, '_shouldRefreshToken').andCallThrough();
  });

  afterEach(function () {
    expect.restoreSpies();
  });

  describe('login(username, password, token)', function () {
    it('should successfully login and put user data in the session', function () {
      var token = null;
      api.post.andReturn($.Deferred().resolve(sample));
      auth.login('user', 'password').done(()=>{ token = sample.token; });

      expect(token).toEqual(sample.token);
      expect(auth._startTokenRefresher.calls.length).toEqual(1);
      expect(getCallArgs(session.setUserData).token, sample.token);
    });

    it('should return rejected promise if failed to log in', function () {
      var wasCalled = false;
      api.post.andReturn($.Deferred().reject());
      auth.login('user', 'password').fail(()=> { wasCalled = true });
      expect(wasCalled).toEqual(true);
    });
  });

  describe('u2flogin(name, password)', function () {
    var u2fSample = { type: 2, signRequests: -2, timeoutSeconds: -599, requestId: 2 };

    it('should successfully login and put user data in the session', function () {
      window.u2f = {
	sign: function(appId, challenge, registeredKeys, callback) {
	  u2fSample.errorCode = 0;
	  callback(u2fSample);
	}
      };

      var token = null;
      api.post.andReturn($.Deferred().resolve(u2fSample));
      auth.u2fLogin('user', 'password').done(()=>{ token = u2fSample; });

      expect(token).toEqual(u2fSample);
      expect(auth._startTokenRefresher.calls.length).toEqual(1);
      expect(getCallArgs(session.setUserData).token, u2fSample);
    });

    it('should return rejected promise if failed to log in', function () {
      var wasCalled = false;
      api.post.andReturn($.Deferred().reject());
      auth.u2fLogin('user', 'password').fail(()=> { wasCalled = true });
      expect(wasCalled).toEqual(true);
    });

    it('should return rejected promise if u2f api throws an error', function() {
      window.u2f = {
	sign: function(appId, challenge, registeredKeys, callback) {
	  callback({errorCode: 1});
	}
      };

      var wasCalled = false;-
      api.post.andReturn($.Deferred().resolve(u2fSample));
      auth.u2fLogin('user', 'password').fail(()=> { wasCalled = true });
      expect(wasCalled).toEqual(true);
    });
  });

  describe('ensureUser()', function () {
    describe('when token is valid', function () {
      it('should be resolved', function () {
        var wasCalled = false;
        session.getUserData.andReturn(sample);
        auth.ensureUser('user', 'password').done(()=> { wasCalled = true });

        expect(wasCalled).toEqual(true);
        expect(auth._startTokenRefresher).toHaveBeenCalled();
        expect(auth._shouldRefreshToken).toHaveBeenCalled();
      });
    });

    describe('when token is about to be expired', function () {
      it('should renew the token', function () {
        var wasCalled = false;
        api.post.andReturn($.Deferred().resolve(sample));
        session.getUserData.andReturn({
           ...sample,
           created: new Date('12/12/2000').getTime()
         });

        auth.ensureUser('user', 'password').done(()=> { wasCalled = true });

        expect(wasCalled).toEqual(true);
        expect(auth._startTokenRefresher).toHaveBeenCalled();
        expect(auth._shouldRefreshToken).toHaveBeenCalled();
      });
    });

    describe('when token is missing', function () {
      it('should reject', function () {
        var wasCalled = false;
        session.getUserData.andReturn({});
        auth.ensureUser('user', 'password').fail(()=> { wasCalled = true });
        expect(wasCalled).toEqual(true);
      });
    });

  });

  describe('logout()', function () {
    it('should clear the session and stop refreshTimer', function () {
      auth.logout();
      expect(api.delete.calls.length).toEqual(1);
      expect(session.clear.calls.length).toEqual(1);
      expect(auth._stopTokenRefresher.calls.length).toEqual(1);
    });
  });

  function getCallArgs(spy){
    return spy.getLastCall().arguments[0];
  }
})
