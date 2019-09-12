/*
Copyright 2019 Gravitational, Inc.

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

import api from './../api';
import cfg from 'gravity/config';
import $ from 'jQuery';
import makeUserToken from './makeUserToken';
// This puts it in window.u2f
import 'u2f-api-polyfill';

const auth = {

  fetchToken(tokenId){
    const path = cfg.getUserRequestInfo(tokenId);
    return api.get(path).then(json => {
      return makeUserToken(json)
    })
  },

  login(userId, password, token){
    const data = {
      user: userId,
      pass: password,
      second_factor_token: token
    };

    return api.post(cfg.api.sessionPath, data, false);
  },

  loginWithU2f(name, password){
    const data = {
      user: name,
      pass: password
    };

    return api.post(cfg.api.u2fSessionChallengePath, data, false).then(data=>{
      const deferred = $.Deferred();
      window.u2f.sign(data.appId, data.challenge, [data], function(res){
        if(res.errorCode){
          let err = auth._getU2fErr(res.errorCode);
          deferred.reject(err);
          return;
        }

        const response = {
          user: name,
          u2f_sign_response: res
        };

        api.post(cfg.api.u2fSessionPath, response, false).then(data=>{
          deferred.resolve(data);
        }).fail(data=>{
          deferred.reject(data);
        });

      });

      return deferred.promise();
    });
  },

  registerWithU2F(password, tokenId){
    return auth._getU2FRegisterRes(tokenId).then(u2fRes => {
      return auth._finishRegistration(cfg.api.userTokenInviteDonePath, tokenId, password, null, u2fRes)
    })
  },

  registerWith2FA(password, hotpToken, tokenId){
    return auth._finishRegistration(
      cfg.api.userTokenInviteDonePath,
      tokenId,
      password,
      hotpToken);
  },

  resetPasswordWithU2F(password, tokenId){
    return auth._getU2FRegisterRes(tokenId).then(u2fRes => {
      return auth._finishRegistration(
        cfg.api.userTokenResetDonePath,
        tokenId,
        password,
        null,
        u2fRes
      );
    })
  },

  resetPasswordWith2FA(password, hotpToken, tokenId) {
    return this._finishRegistration(
      cfg.api.userTokenResetDonePath,
      tokenId,
      password,
      hotpToken,
    );
  },

  _finishRegistration(url, tokenId, psw, hotpToken, u2fResponse) {
    const request = {
      'password': window.btoa(psw),
      'second_factor_token': hotpToken,
      'token': tokenId,
      'u2f_register_response': u2fResponse
    }

    return api.post(url, request, false);
  },

  _getU2FRegisterRes(tokenID){
    return api.get(cfg.getU2fCreateUserChallengeUrl(tokenID))
      .then(data => {
        const deferred = $.Deferred();
        window.u2f.register(data.appId, [data], [], function(res){
          if (res.errorCode) {
            let err = auth._getU2fErr(res.errorCode);
            deferred.reject(err);
            return;
          }

          deferred.resolve(res)
        });

        return deferred.promise();
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