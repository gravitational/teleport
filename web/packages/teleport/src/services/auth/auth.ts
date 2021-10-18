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
import {
  makeMfaAuthenticateChallenge,
  makeMfaRegistrationChallenge,
} from './makeMfa';
import { DeviceType } from './types';

const auth = {
  u2fBrowserSupported() {
    if (window['u2f']) {
      return null;
    }

    return new Error(
      'this browser does not support U2F required for hardware tokens, please try Chrome or Firefox instead'
    );
  },

  createMfaRegistrationChallenge(tokenId: string, deviceType: DeviceType) {
    return api
      .post(cfg.getMfaCreateRegistrationChallengeUrl(tokenId), {
        deviceType,
      })
      .then(makeMfaRegistrationChallenge);
  },

  createMfaAuthnChallengeWithToken(tokenId: string) {
    return api
      .post(cfg.getAuthnChallengeWithTokenUrl(tokenId))
      .then(makeMfaAuthenticateChallenge);
  },

  // mfaLoginBegin retrieves users mfa challenges for their
  // registered devices after verifying given username and password
  // at login.
  mfaLoginBegin(user: string, pass: string) {
    return api
      .post(cfg.api.mfaLoginBegin, { user, pass })
      .then(makeMfaAuthenticateChallenge);
  },

  // changePasswordBegin retrieves users mfa challenges for their
  // registered devices after verifying given password from an
  // authenticated user.
  mfaChangePasswordBegin(oldPass: string) {
    return api
      .post(cfg.api.mfaChangePasswordBegin, { pass: oldPass })
      .then(makeMfaAuthenticateChallenge);
  },

  login(userId: string, password: string, token: string) {
    const data = {
      user: userId,
      pass: password,
      second_factor_token: token,
    };

    return api.post(cfg.api.sessionPath, data);
  },

  loginWithU2f(name: string, password: string) {
    const err = this.u2fBrowserSupported();
    if (err) {
      return Promise.reject(err);
    }

    return auth.mfaLoginBegin(name, password).then(data => {
      const promise = new Promise((resolve, reject) => {
        window['u2f'].sign(null, null, data.u2fSignRequests, function(res) {
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
            .post(cfg.api.mfaLoginFinish, response)
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

  fetchPasswordToken(tokenId: string) {
    const path = cfg.getPasswordTokenUrl(tokenId);
    return api.get(path).then(makePasswordToken);
  },

  resetPasswordWithU2f(tokenId: string, password: string) {
    const err = this.u2fBrowserSupported();
    if (err) {
      return Promise.reject(err);
    }

    return auth._getU2FRegisterRes(tokenId).then(u2fRes => {
      return auth._resetPassword(tokenId, password, null, u2fRes);
    });
  },

  resetPassword(tokenId: string, password: string, hotpToken: string) {
    return this._resetPassword(tokenId, password, hotpToken);
  },

  changePassword(oldPass: string, newPass: string, token: string) {
    const data = {
      old_password: base64EncodeUnicode(oldPass),
      new_password: base64EncodeUnicode(newPass),
      second_factor_token: token,
    };

    return api.put(cfg.api.changeUserPasswordPath, data);
  },

  changePasswordWithU2f(oldPass: string, newPass: string) {
    const err = this.u2fBrowserSupported();
    if (err) {
      return Promise.reject(err);
    }

    return auth.mfaChangePasswordBegin(oldPass).then(data => {
      return new Promise((resolve, reject) => {
        window['u2f'].sign(null, null, data.u2fSignRequests, function(res) {
          if (res.errorCode) {
            const err = auth._getU2fErr(res.errorCode);
            reject(err);
            return;
          }

          const data = {
            new_password: base64EncodeUnicode(newPass),
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

  _resetPassword(tokenId: string, psw: string, hotpToken: string, u2fResponse) {
    const request = {
      password: base64EncodeUnicode(psw),
      second_factor_token: hotpToken,
      token: tokenId,
      u2f_register_response: u2fResponse,
    };

    return api.put(cfg.getPasswordTokenUrl(), request);
  },

  _getU2FRegisterRes(tokenId: string) {
    return auth.createMfaRegistrationChallenge(tokenId, 'u2f').then(data => {
      const challenge = data.u2fRegisterRequest;
      return new Promise((resolve, reject) => {
        window['u2f'].register(challenge.appId, [challenge], [], function(res) {
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

  _getU2fErr(errorCode: number) {
    let errorMsg = `error code ${errorCode}`;
    // lookup error message...
    for (var msg in window['u2f'].ErrorCodes) {
      if (window['u2f'].ErrorCodes[msg] == errorCode) {
        errorMsg = msg;
      }
    }

    let message = `Please check your U2F settings, make sure it is plugged in and you are using the supported browser.\nU2F error: ${errorMsg}`;

    return new Error(message);
  },
};

function base64EncodeUnicode(str: string) {
  return window.btoa(
    encodeURIComponent(str).replace(/%([0-9A-F]{2})/g, function(match, p1) {
      const hexadecimalStr = '0x' + p1;
      return String.fromCharCode(Number(hexadecimalStr));
    })
  );
}

export default auth;
