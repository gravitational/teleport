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

var api = require('./api');
var session = require('./session');
var cfg = require('app/config');
var $ = require('jQuery');

const PROVIDER_GOOGLE = 'google';

const REFRESH_RATE_COEFFICIENT = 0.66;

var refreshTokenTimerId = null;

var auth = {

  signUp(name, password, token, inviteToken){
    var data = {user: name, pass: password, second_factor_token: token, invite_token: inviteToken};
    return api.post(cfg.api.createUserPath, data)
      .then((user)=>{
        session.setUserData(user);
        auth._startTokenRefresher();
        return user;
      });
  },

  login(name, password, token){
    auth._stopTokenRefresher();
    session.clear();
    return auth._login(name, password, token).done(auth._startTokenRefresher);
  },

  ensureUser(){
    var userData = session.getUserData();
    if(userData.token){
      // refresh timer will not be set in case of browser refresh event
      if(auth._getRefreshTokenTimerId() === null){
        return auth._refreshToken().done(auth._startTokenRefresher);
      }

      return $.Deferred().resolve(userData);
    }

    return $.Deferred().reject();
  },

  logout(){
    auth._stopTokenRefresher();
    session.clear();
    auth._redirect();
  },

  _redirect(){
    window.location = cfg.routes.login;
  },

  _startTokenRefresher(){
    var {expires_in} = session.getUserData();
    if(expires_in < 0) {
      expires_in = expires_in * -1;
    }

    var refreshRate = (expires_in * 1000) * REFRESH_RATE_COEFFICIENT;

    refreshTokenTimerId = setInterval(auth._refreshToken, refreshRate);
  },

  _stopTokenRefresher(){
    clearInterval(refreshTokenTimerId);
    refreshTokenTimerId = null;
  },

  _getRefreshTokenTimerId(){
    return refreshTokenTimerId;
  },

  _refreshToken(){
    return api.post(cfg.api.renewTokenPath).then(data=>{
      session.setUserData(data);
      return data;
    }).fail(()=>{
      auth.logout();
    });
  },

  _login(name, password, token){
    var data = {
      user: name,
      pass: password,
      second_factor_token: token
    };

    return api.post(cfg.api.sessionPath, data, false).then(data=>{
      session.setUserData(data);
      return data;
    });
  }
}

module.exports = auth;
module.exports.PROVIDER_GOOGLE = PROVIDER_GOOGLE;
