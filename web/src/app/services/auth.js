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
var logger = require('app/common/logger').create('services/auth');
require('u2f-api-polyfill'); // This puts it in window.u2f

const PROVIDER_GOOGLE = 'google';

const SECOND_FACTOR_TYPE_HOTP = 'hotp';
const SECOND_FACTOR_TYPE_OIDC = 'oidc';
const SECOND_FACTOR_TYPE_U2F = 'u2f';

const CHECK_TOKEN_REFRESH_RATE = 10*1000; // 10 sec

var refreshTokenTimerId = null;

var auth = {

  signUp(name, password, token, inviteToken){
    var data = {user: name, pass: password, second_factor_token: token, invite_token: inviteToken};
    return api.post(cfg.api.createUserPath, data)
      .then((data)=>{
        session.setUserData(data);
        auth._startTokenRefresher();
        return data;
      });
  },

  u2fSignUp(name, password, inviteToken){
    return api.get(cfg.api.getU2fCreateUserChallengeUrl(inviteToken)).then(data=>{
      var deferred = $.Deferred();

      window.u2f.register(data.appId, [data], [], function(res){
        if(res.errorCode){
          var err = auth._getU2fErr(res.errorCode);
          deferred.reject(err);
          return;
        }

        var response = {
          user:                  name,
          pass:                  password,
          u2f_register_response: res,
          invite_token:          inviteToken
        };
        api.post(cfg.api.u2fCreateUserPath, response, false).then(data=>{
          session.setUserData(data);
          auth._startTokenRefresher();
          deferred.resolve(data);
        }).fail(data=>{
          deferred.reject(data);
        })
      });

      return deferred.promise();
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
      session.setUserData(data);
      this._startTokenRefresher();
      return data;
    });
  },

  u2fLogin(name, password){
    auth._stopTokenRefresher();
    session.clear();

    var data = {
      user: name,
      pass: password
    };

    return api.post(cfg.api.u2fSessionChallengePath, data, false).then(data=>{
      var deferred = $.Deferred();

      window.u2f.sign(data.appId, data.challenge, [data], function(res){
        if(res.errorCode){
          var err = auth._getU2fErr(res.errorCode);
          deferred.reject(err);
          return;
        }

        var response = {
          user:              name,
          u2f_sign_response: res
        };
        api.post(cfg.api.u2fSessionPath, response, false).then(data=>{
          session.setUserData(data);
          auth._startTokenRefresher();
          deferred.resolve(data);
        }).fail(data=>{
          deferred.reject(data);
        });
      });

      return deferred.promise();
    });
  },

  ensureUser(){
    this._stopTokenRefresher();

    var userData = session.getUserData();

    if(!userData.token){
      return $.Deferred().reject();
    }

    if(this._shouldRefreshToken(userData)){
      return this._refreshToken().done(this._startTokenRefresher);
    }

    this._startTokenRefresher();
    return $.Deferred().resolve(userData);
  },

  logout(){
    logger.info('logout()');
    api.delete(cfg.api.sessionPath).always(()=>{
      auth._redirect();
    });
    session.clear();
    auth._stopTokenRefresher();
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
      session.setUserData(data);
      return data;
    }).fail(()=>{
      auth.logout();
    });
  },

  _getU2fErr(errorCode){
    var errorMsg = "";
    // lookup error message...
    for(var msg in window.u2f.ErrorCodes){
      if(window.u2f.ErrorCodes[msg] == errorCode){
        errorMsg = msg;
      }
    }
    return {responseJSON:{message:"U2F Error: " + errorMsg}};
  }

}

module.exports = auth;
module.exports.PROVIDER_GOOGLE = PROVIDER_GOOGLE;
module.exports.SECOND_FACTOR_TYPE_HOTP = SECOND_FACTOR_TYPE_HOTP;
module.exports.SECOND_FACTOR_TYPE_OIDC = SECOND_FACTOR_TYPE_OIDC;
module.exports.SECOND_FACTOR_TYPE_U2F = SECOND_FACTOR_TYPE_U2F;
