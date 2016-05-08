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

const CHECK_TOKEN_REFRESH_RATE = 10*1000; // 10 sec

var refreshTokenTimerId = null;

var UserData = function(json){
  $.extend(this, json);
  this.created = new Date().getTime();
}

var auth = {

  signUp(name, password, token, inviteToken){
    var data = {user: name, pass: password, second_factor_token: token, invite_token: inviteToken};
    return api.post(cfg.api.createUserPath, data)
      .then((user)=>{
        session.setUserData(new UserData(user));
        auth._startTokenRefresher();
        return user;
      });
  },

  login(name, password, token){
    auth._stopTokenRefresher();
    session.clear();

    var data = {
      user: name,
      pass: password,
      second_factor_token: token
    };

    return api.post(cfg.api.sessionPath, data, false).then(data=>{
      session.setUserData(new UserData(data));
      this._startTokenRefresher();
      return data;
    });
  },

  ensureUser(){
    this._stopTokenRefresher();

    var userData = session.getUserData();

    if(!userData.token){
      return $.Deferred().reject();
    }

    if(this._shouldRefreshToken(userData)){
      return this._refreshToken().then(this._startTokenRefresher);
    }

    this._startTokenRefresher();
    return $.Deferred().resolve(userData);
  },

  logout(){
    auth._stopTokenRefresher();
    session.clear();
    auth._redirect();
  },

  _redirect(){
    window.location = cfg.routes.login;
  },

  _shouldRefreshToken({ expires_in, created } ){
    if(!created || !expires_in){
      return true;
    }

    if(expires_in < 0) {
      expires_in = expires_in * -1;
    }

    expires_in = expires_in * 1000;

    var delta = created + expires_in - new Date().getTime();

    return delta < expires_in * 0.33;
  },

  _startTokenRefresher(){
    refreshTokenTimerId = setInterval(auth.ensureUser.bind(auth), CHECK_TOKEN_REFRESH_RATE);
  },

  _stopTokenRefresher(){
    clearInterval(refreshTokenTimerId);
    refreshTokenTimerId = null;
  },

  _refreshToken(){
    return api.post(cfg.api.renewTokenPath).then(data=>{      
      session.setUserData(new UserData(data));
      return data;
    }).fail(()=>{
      auth.logout();
    });
  }

}

module.exports = auth;
module.exports.PROVIDER_GOOGLE = PROVIDER_GOOGLE;
