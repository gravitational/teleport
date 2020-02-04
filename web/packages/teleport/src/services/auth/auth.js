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

// This puts it in window.u2f
import 'u2f-api-polyfill';
import api from 'teleport/services/api';
import cfg from 'teleport/config';
import makePasswordToken from './makePasswordToken';

const auth = {
  login(userId, password, token) {
    const data = {
      user: userId,
      pass: password,
      second_factor_token: token,
    };

    return api.post(cfg.api.sessionPath, data, false);
  },

  loginWithU2f(name, password) {
    const data = {
      user: name,
      pass: password,
    };

    return api.post(cfg.api.u2fSessionChallengePath, data, false).then(data => {
      const promise = new Promise((resolve, reject) => {
        window.u2f.sign(data.appId, data.challenge, [data], function(res) {
          if (res.errorCode) {
            const err = auth._getU2fErr(res.errorCode);
            reject(err);
            return;
          }

          const response = {
            user: name,
            u2f_sign_response: res,
          };

          api
            .post(cfg.api.u2fSessionPath, response, false)
            .then(data => {
              resolve(data);
            })
            .catch(data => {
              reject(data);
            });
        });
      });

      return promise;
    });
  },

  fetchPasswordToken(tokenId) {
    const path = cfg.getPasswordTokenUrl(tokenId);
    return api.get(path).then(makePasswordToken);
  },

  resetPasswordWithU2f(tokenId, password) {
    return auth._getU2FRegisterRes(tokenId).then(u2fRes => {
      return auth._resetPassword(tokenId, password, null, u2fRes);
    });
  },

  resetPassword(tokenId, password, hotpToken) {
    return this._resetPassword(tokenId, password, hotpToken);
  },

  changePassword(oldPass, newPass, token) {
    const data = {
      old_password: window.btoa(oldPass),
      new_password: window.btoa(newPass),
      second_factor_token: token,
    };

    return api.put(cfg.api.changeUserPasswordPath, data);
  },

  changePasswordWithU2f(oldPass, newPass) {
    const data = {
      user: name,
      pass: oldPass,
    };

    return api.post(cfg.api.u2fChangePassChallengePath, data).then(data => {
      return new Promise((resolve, reject) => {
        window.u2f.sign(data.appId, data.challenge, [data], function(res) {
          if (res.errorCode) {
            const err = auth._getU2fErr(res.errorCode);
            reject(err);
            return;
          }

          const data = {
            new_password: window.btoa(newPass),
            u2f_sign_response: res,
          };

          api
            .put(cfg.api.changeUserPasswordPath, data)
            .then(data => {
              resolve(data);
            })
            .catch(data => {
              reject(data);
            });
        });
      });
    });
  },

  _resetPassword(tokenId, psw, hotpToken, u2fResponse) {
    const request = {
      password: window.btoa(psw),
      second_factor_token: hotpToken,
      token: tokenId,
      u2f_register_response: u2fResponse,
    };

    return api.put(cfg.getPasswordTokenUrl(), request, false);
  },

  _getU2FRegisterRes(tokenId) {
    return api.get(cfg.getU2fCreateUserChallengeUrl(tokenId)).then(data => {
      return new Promise((resolve, reject) => {
        window.u2f.register(data.appId, [data], [], function(res) {
          if (res.errorCode) {
            const err = auth._getU2fErr(res.errorCode);
            reject(err);
            return;
          }
          resolve(res);
        });
      });
    });
  },

  _getU2fErr(errorCode) {
    let errorMsg = '';
    // lookup error message...
    for (var msg in window.u2f.ErrorCodes) {
      if (window.u2f.ErrorCodes[msg] == errorCode) {
        errorMsg = msg;
      }
    }

    let message = `Please check your U2F settings, make sure it is plugged in and you are using the supported browser.\nU2F error: ${errorMsg}`;

    return {
      responseJSON: {
        message,
      },
    };
  },
};

export default auth;
