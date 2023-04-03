/*
Copyright 2019-2022 Gravitational, Inc.

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

import api from 'teleport/services/api';
import cfg from 'teleport/config';
import { DeviceType, DeviceUsage } from 'teleport/services/mfa';

import { CaptureEvent, userEventService } from 'teleport/services/userEvent';

import makePasswordToken from './makePasswordToken';
import { makeChangedUserAuthn } from './make';
import {
  makeMfaAuthenticateChallenge,
  makeMfaRegistrationChallenge,
  makeWebauthnAssertionResponse,
  makeWebauthnCreationResponse,
} from './makeMfa';
import {
  ResetPasswordReqWithEvent,
  ResetPasswordWithWebauthnReqWithEvent,
  UserCredentials,
} from './types';

const auth = {
  checkWebauthnSupport() {
    if (window.PublicKeyCredential) {
      return Promise.resolve();
    }

    return Promise.reject(
      new Error(
        'this browser does not support Webauthn required for hardware tokens, please try the latest version of Chrome, Firefox or Safari'
      )
    );
  },
  checkMfaRequired(
    params: IsMfaRequiredRequest
  ): Promise<{ required: boolean }> {
    return api.post(cfg.getMfaRequiredUrl(), params);
  },
  createMfaRegistrationChallenge(
    tokenId: string,
    deviceType: DeviceType,
    deviceUsage: DeviceUsage = 'mfa'
  ) {
    return api
      .post(cfg.getMfaCreateRegistrationChallengeUrl(tokenId), {
        deviceType,
        deviceUsage,
      })
      .then(makeMfaRegistrationChallenge);
  },

  createMfaAuthnChallengeWithToken(tokenId: string) {
    return api
      .post(cfg.getAuthnChallengeWithTokenUrl(tokenId))
      .then(makeMfaAuthenticateChallenge);
  },

  // mfaLoginBegin retrieves users mfa challenges for their
  // registered devices. Empty creds indicates request for passwordless challenges.
  // Otherwise, non-passwordless challenges requires creds to be verified.
  mfaLoginBegin(creds?: UserCredentials) {
    return api
      .post(cfg.api.mfaLoginBegin, {
        passwordless: !creds,
        user: creds?.username,
        pass: creds?.password,
      })
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

  login(userId: string, password: string, otpCode: string) {
    const data = {
      user: userId,
      pass: password,
      second_factor_token: otpCode,
    };

    return api.post(cfg.api.webSessionPath, data);
  },

  loginWithWebauthn(creds?: UserCredentials) {
    return auth
      .checkWebauthnSupport()
      .then(() => auth.mfaLoginBegin(creds))
      .then(res =>
        navigator.credentials.get({
          publicKey: res.webauthnPublicKey,
          mediation: 'silent',
        })
      )
      .then(res => {
        const request = {
          user: creds?.username,
          webauthnAssertionResponse: makeWebauthnAssertionResponse(res),
        };

        return api.post(cfg.api.mfaLoginFinish, request);
      });
  },

  fetchPasswordToken(tokenId: string) {
    const path = cfg.getPasswordTokenUrl(tokenId);
    return api.get(path).then(makePasswordToken);
  },

  // resetPasswordWithWebauthn either sets a new password and a new webauthn device,
  // or if passwordless is requested (indicated by empty password param),
  // skips setting a new password and only sets a passwordless device.
  resetPasswordWithWebauthn(props: ResetPasswordWithWebauthnReqWithEvent) {
    const { req, eventMeta } = props;
    const deviceUsage: DeviceUsage = req.password ? 'mfa' : 'passwordless';

    return auth
      .checkWebauthnSupport()
      .then(() =>
        auth.createMfaRegistrationChallenge(
          req.tokenId,
          'webauthn',
          deviceUsage
        )
      )
      .then(res =>
        navigator.credentials.create({
          publicKey: res.webauthnPublicKey,
        })
      )
      .then(res => {
        const request = {
          token: req.tokenId,
          password: req.password ? base64EncodeUnicode(req.password) : null,
          webauthnCreationResponse: makeWebauthnCreationResponse(res),
          deviceName: req.deviceName,
        };

        return api.put(cfg.getPasswordTokenUrl(), request);
      })
      .then(j => {
        if (eventMeta) {
          userEventService.capturePreUserEvent({
            event: CaptureEvent.PreUserOnboardSetCredentialSubmitEvent,
            username: eventMeta.username,
          });

          userEventService.capturePreUserEvent({
            event: CaptureEvent.PreUserOnboardRegisterChallengeSubmitEvent,
            username: eventMeta.username,
            mfaType: eventMeta.mfaType,
            loginFlow: deviceUsage,
          });
        }
        return makeChangedUserAuthn(j);
      });
  },

  resetPassword(props: ResetPasswordReqWithEvent) {
    const { req, eventMeta } = props;

    const request = {
      password: base64EncodeUnicode(req.password),
      second_factor_token: req.otpCode,
      token: req.tokenId,
      deviceName: req.deviceName,
    };

    return api.put(cfg.getPasswordTokenUrl(), request).then(j => {
      if (eventMeta) {
        userEventService.capturePreUserEvent({
          event: CaptureEvent.PreUserOnboardSetCredentialSubmitEvent,
          username: eventMeta.username,
        });
      }
      return makeChangedUserAuthn(j);
    });
  },

  changePassword(oldPass: string, newPass: string, token: string) {
    const data = {
      old_password: base64EncodeUnicode(oldPass),
      new_password: base64EncodeUnicode(newPass),
      second_factor_token: token,
    };

    return api.put(cfg.api.changeUserPasswordPath, data);
  },

  changePasswordWithWebauthn(oldPass: string, newPass: string) {
    return auth
      .checkWebauthnSupport()
      .then(() => api.post(cfg.api.mfaChangePasswordBegin, { pass: oldPass }))
      .then(res =>
        navigator.credentials.get({
          publicKey: makeMfaAuthenticateChallenge(res).webauthnPublicKey,
        })
      )
      .then(res => {
        const request = {
          old_password: base64EncodeUnicode(oldPass),
          new_password: base64EncodeUnicode(newPass),
          webauthnAssertionResponse: makeWebauthnAssertionResponse(res),
        };

        return api.put(cfg.api.changeUserPasswordPath, request);
      });
  },

  headlessSSOGet(transactionId: string) {
    return auth
      .checkWebauthnSupport()
      .then(() => api.get(cfg.getHeadlessSsoPath(transactionId)))
      .then((json: any) => {
        json = json || {};

        return {
          clientIpAddress: json.client_ip_address,
        };
      });
  },

  headlessSSOAccept(transactionId: string) {
    return auth
      .checkWebauthnSupport()
      .then(() => api.post(cfg.api.mfaAuthnChallengePath))
      .then(res =>
        navigator.credentials.get({
          publicKey: makeMfaAuthenticateChallenge(res).webauthnPublicKey,
        })
      )
      .then(res => {
        const request = {
          action: 'accept',
          webauthnAssertionResponse: makeWebauthnAssertionResponse(res),
        };

        return api.put(cfg.getHeadlessSsoPath(transactionId), request);
      });
  },

  headlessSSOReject(transactionId: string) {
    const request = {
      action: 'denied',
    };

    return api.put(cfg.getHeadlessSsoPath(transactionId), request);
  },

  createPrivilegeTokenWithTotp(secondFactorToken: string) {
    return api.post(cfg.api.createPrivilegeTokenPath, { secondFactorToken });
  },

  fetchWebauthnChallenge() {
    return auth
      .checkWebauthnSupport()
      .then(() =>
        api
          .post(cfg.api.mfaAuthnChallengePath)
          .then(makeMfaAuthenticateChallenge)
      )
      .then(res =>
        navigator.credentials.get({
          publicKey: res.webauthnPublicKey,
        })
      );
  },

  createPrivilegeTokenWithWebauthn() {
    return auth.fetchWebauthnChallenge().then(res =>
      api.post(cfg.api.createPrivilegeTokenPath, {
        webauthnAssertionResponse: makeWebauthnAssertionResponse(res),
      })
    );
  },

  createRestrictedPrivilegeToken() {
    return api.post(cfg.api.createPrivilegeTokenPath, {});
  },

  getWebauthnResponse() {
    return auth
      .fetchWebauthnChallenge()
      .then(res => makeWebauthnAssertionResponse(res));
  },
};

function base64EncodeUnicode(str: string) {
  return window.btoa(
    encodeURIComponent(str).replace(/%([0-9A-F]{2})/g, function (match, p1) {
      const hexadecimalStr = '0x' + p1;
      return String.fromCharCode(Number(hexadecimalStr));
    })
  );
}

export default auth;

export type IsMfaRequiredRequest =
  | IsMfaRequiredDatabase
  | IsMfaRequiredNode
  | IsMfaRequiredKube
  | IsMfaRequiredWindowsDesktop;

export type IsMfaRequiredDatabase = {
  database: {
    // service_name is the database service name.
    service_name: string;
    // protocol is the type of the database protocol.
    protocol: string;
    // username is an optional database username.
    username?: string;
    // database_name is an optional database name.
    database_name?: string;
  };
};

export type IsMfaRequiredNode = {
  node: {
    // node_name can be node's hostname or UUID.
    node_name: string;
    // login is the OS login name.
    login: string;
  };
};

export type IsMfaRequiredWindowsDesktop = {
  windows_desktop: {
    // desktop_name is the Windows Desktop server name.
    desktop_name: string;
    // login is the Windows desktop user login.
    login: string;
  };
};

export type IsMfaRequiredKube = {
  kube: {
    // cluster_name is the name of the kube cluster.
    cluster_name: string;
  };
};
