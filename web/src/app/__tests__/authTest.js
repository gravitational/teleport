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
  var sample = { token: 'token' };

  beforeEach(function () {
    spyOn(session, 'setUserData');
    spyOn(session, 'getUserData');
    spyOn(session, 'clear');
    spyOn(api, 'post');
    spyOn(auth, '_startTokenRefresher');
    spyOn(auth, '_stopTokenRefresher');
    spyOn(auth, '_getRefreshTokenTimerId');
    spyOn(auth, '_redirect');
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
