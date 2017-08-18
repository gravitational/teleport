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
import cfg from 'app/config';
import $ from 'jQuery';

// This puts it in window.u2f
import 'u2f-api-polyfill'; 

const auth = {      

  login(email, password, token){
    const data = {
      user: email,
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
          const err = auth._getU2fErr(res.errorCode);
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

  acceptInvite(name, password, token, inviteToken){
    const data = {
      invite_token: inviteToken,      
      pass: password,      
      second_factor_token: token,
      user: name,      
    };

    return api.post(cfg.api.createUserPath, data, false);      
  },

  acceptInviteWithU2f(name, password, inviteToken){
    return api.get(cfg.api.getU2fCreateUserChallengeUrl(inviteToken))
      .then(data => {
        const deferred = $.Deferred();        
        window.u2f.register(data.appId, [data], [], function(res){        
          if (res.errorCode) {
            const err = auth._getU2fErr(res.errorCode);
            deferred.reject(err);
            return;
          }

          const response = {
            user: name,
            pass: password,
            u2f_register_response: res,
            invite_token: inviteToken
          };

          api.post(cfg.api.u2fCreateUserPath, response, false)
            .then(data => {                        
              deferred.resolve(data);
            })
            .fail(err => {
              deferred.reject(err);
            })
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

    const message = `Please check your U2F settings, make sure it is plugged in and you are using the supported browser.\nU2F error: ${errorMsg}`

    return {
      responseJSON: {
        message
      }
    };
  }
}

export default auth;