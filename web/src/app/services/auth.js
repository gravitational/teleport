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

import api from './api';
import session from './session';
import cfg from 'app/config';
import $ from 'jQuery';
import Logger from 'app/lib/logger';

// This puts it in window.u2f
import 'u2f-api-polyfill'; 

const logger = Logger.create('services/auth');

const AUTH_IS_RENEWING = 'GRV_AUTH_IS_RENEWING';

const CHECK_TOKEN_REFRESH_RATE = 10 * 1000; // 10 sec

let refreshTokenTimerId = null;

const auth = {

  signUp(name, password, token, inviteToken){
    var data = {user: name, pass: password, second_factor_token: token, invite_token: inviteToken};
    return api.post(cfg.api.createUserPath, data)
      .then((data)=>{
        session.setUserData(data);
        auth._startTokenRefresher();
        return data;
      });
  },

  signUpWithU2f(name, password, inviteToken){
    return api.get(cfg.api.getU2fCreateUserChallengeUrl(inviteToken))
      .then(data => {
        let deferred = $.Deferred();

        window.u2f.register(data.appId, [data], [], function(res){        
          if (res.errorCode) {
            let err = auth._getU2fErr(res.errorCode);
            deferred.reject(err);
            return;
        }

        let response = {
          user: name,
          pass: password,
          u2f_register_response: res,
          invite_token: inviteToken
        };

        api.post(cfg.api.u2fCreateUserPath, response, false)
          .then(data => {
            session.setUserData(data);
            auth._startTokenRefresher();
            deferred.resolve(data);
          })
          .fail(err => {
            deferred.reject(err);
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

  loginWithU2f(name, password){
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

    let userData = session.getUserData();
    
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
      auth.redirect();
    });
    session.clear();
    auth._stopTokenRefresher();
  },

  redirect(url) {
    // default URL to redirect
    url = url || cfg.routes.login;
    window.location = url;
  },

  _shouldRefreshToken({ expires_in, created } ){
    if(!created || !expires_in){
      return true;
    }
    
    expires_in = expires_in * 1000;

    var delta = created + expires_in - new Date().getTime();

    // give some extra time for slow connection  
    return delta < CHECK_TOKEN_REFRESH_RATE * 3;
  },

  _startTokenRefresher(){
    refreshTokenTimerId = setInterval(() => {      
      // check if barer-token needs to be renewed
      auth.ensureUser();
      // extra ping to a server to see of logout was triggered from another tab
      auth._checkStatus();
    }, CHECK_TOKEN_REFRESH_RATE);        
  },

  _stopTokenRefresher(){
    clearInterval(refreshTokenTimerId);
    refreshTokenTimerId = null;
  },

  _checkStatus(){
    // do not attemp to fetch the status with potentially invalid token
    // as it will trigger logout action.
    if(localStorage.getItem(AUTH_IS_RENEWING) !== null){
      return;
    }
    
    api.get(cfg.api.userStatus)          
      .fail(err => {              
        // indicates that user session is no longer valid
        if(err.status == 403){
          auth.logout();
        }
      });
  },

  _refreshToken() {
    localStorage.setItem(AUTH_IS_RENEWING, true);
    return api.post(cfg.api.renewTokenPath)
      .then(data => {
        session.setUserData(data);        
        return data;
      })
      .fail(() => {
        auth.logout();
      })
      .always(() => {
        localStorage.removeItem(AUTH_IS_RENEWING);
      });
  },

  _getU2fErr(errorCode){
    let errorMsg = "";
    // lookup error message...
    for(var msg in window.u2f.ErrorCodes){
      if(window.u2f.ErrorCodes[msg] == errorCode){
        errorMsg = msg;
      }
    }

    let message = `Please check your U2F settings, make sure it is plugged in and you are using the supported browser.\nU2F error: ${errorMsg}`

    return {
      responseJSON: {
        message
      }
    };
  }
}

export default auth;

