/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import cfg from 'teleport/config';
import api from 'teleport/services/api';
import { DeviceType } from 'teleport/services/mfa';
import { CaptureEvent, userEventService } from 'teleport/services/userEvent';

import { makeChangedUserAuthn } from './make';
import {
  makeMfaAuthenticateChallenge,
  makeMfaRegistrationChallenge,
  makeWebauthnAssertionResponse,
  makeWebauthnCreationResponse,
} from './makeMfa';
import makePasswordToken from './makePasswordToken';
import {
  ChangePasswordReq,
  CreateAuthenticateChallengeRequest,
  CreateNewHardwareDeviceRequest,
  DeviceUsage,
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

  checkMfaRequired: checkMfaRequired,

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

  /**
   * Creates an MFA registration challenge and a corresponding client-side
   * WebAuthn credential.
   */
  createNewWebAuthnDevice(
    req: CreateNewHardwareDeviceRequest
  ): Promise<Credential> {
    return auth
      .checkWebauthnSupport()
      .then(() =>
        auth.createMfaRegistrationChallenge(
          req.tokenId,
          'webauthn',
          req.deviceUsage
        )
      )
      .then(res =>
        navigator.credentials.create({
          publicKey: res.webauthnPublicKey,
        })
      );
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
    const { req, credential, eventMeta } = props;
    const deviceUsage: DeviceUsage = req.password ? 'mfa' : 'passwordless';

    return auth
      .checkWebauthnSupport()
      .then(() => {
        const request = {
          token: req.tokenId,
          password: req.password ? base64EncodeUnicode(req.password) : null,
          webauthnCreationResponse: makeWebauthnCreationResponse(credential),
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

  changePassword({
    oldPassword,
    newPassword,
    secondFactorToken,
    credential,
  }: ChangePasswordReq) {
    const data = {
      old_password: base64EncodeUnicode(oldPassword),
      new_password: base64EncodeUnicode(newPassword),
      second_factor_token: secondFactorToken,
      webauthnAssertionResponse:
        credential && makeWebauthnAssertionResponse(credential),
    };

    return api.put(cfg.api.changeUserPasswordPath, data);
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
      .fetchWebAuthnChallenge({ scope: MfaChallengeScope.HEADLESS_LOGIN })
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

  async getChallenge(
    req: CreateAuthenticateChallengeRequest,
    abortSignal?: AbortSignal
  ) {
    return api
      .post(
        cfg.api.mfaAuthnChallengePath,
        {
          challenge_scope: req.scope,
        },
        abortSignal
      )
      .then(makeMfaAuthenticateChallenge);
  },

  async fetchWebAuthnChallenge(
    req: CreateAuthenticateChallengeRequest,
    abortSignal?: AbortSignal
  ) {
    return auth
      .checkWebauthnSupport()
      .then(() =>
        api
          .post(
            cfg.api.mfaAuthnChallengePath,
            {
              is_mfa_required_req: req.isMfaRequiredRequest,
              challenge_scope: req.scope,
              challenge_allow_reuse: req.allowReuse,
              user_verification_requirement: req.userVerificationRequirement,
            },
            abortSignal
          )
          .then(makeMfaAuthenticateChallenge)
      )
      .then(res =>
        navigator.credentials.get({
          publicKey: res.webauthnPublicKey,
        })
      );
  },

  createPrivilegeTokenWithWebauthn() {
    // Creating privilege tokens always expects the MANAGE_DEVICES webauthn scope.
    return auth
      .fetchWebAuthnChallenge({ scope: MfaChallengeScope.MANAGE_DEVICES })
      .then(res =>
        api.post(cfg.api.createPrivilegeTokenPath, {
          webauthnAssertionResponse: makeWebauthnAssertionResponse(res),
        })
      );
  },

  createRestrictedPrivilegeToken() {
    return api.post(cfg.api.createPrivilegeTokenPath, {});
  },

  async getWebauthnResponse(
    scope: MfaChallengeScope,
    allowReuse?: boolean,
    isMfaRequiredRequest?: IsMfaRequiredRequest,
    abortSignal?: AbortSignal
  ) {
    // TODO(Joerger): DELETE IN 16.0.0
    // the create mfa challenge endpoint below supports
    // MFARequired requests without the extra roundtrip.
    if (isMfaRequiredRequest) {
      try {
        const isMFARequired = await checkMfaRequired(
          isMfaRequiredRequest,
          abortSignal
        );
        if (!isMFARequired.required) {
          return;
        }
      } catch (err) {
        if (
          err?.response?.status === 400 &&
          err?.message.includes('missing target for MFA check')
        ) {
          // checking MFA requirement for admin actions is not supported by old
          // auth servers, we expect an error instead. In this case, assume MFA is
          // not required. Callers should fallback to retrying with MFA if needed.
          return;
        }

        throw err;
      }
    }

    return auth
      .fetchWebAuthnChallenge(
        { scope, allowReuse, isMfaRequiredRequest },
        abortSignal
      )
      .then(res => makeWebauthnAssertionResponse(res));
  },

  getWebauthnResponseForAdminAction(allowReuse?: boolean) {
    // If the client is checking if MFA is required for an admin action,
    // but we know admin action MFA is not enforced, return early.
    if (!cfg.isAdminActionMfaEnforced()) {
      return;
    }

    return auth.getWebauthnResponse(
      MfaChallengeScope.ADMIN_ACTION,
      allowReuse,
      {
        admin_action: {},
      }
    );
  },
};

function checkMfaRequired(
  params: IsMfaRequiredRequest,
  abortSignal?
): Promise<IsMfaRequiredResponse> {
  const appParams = params as IsMfaRequiredApp;
  if (appParams?.app?.cluster_name) {
    return api.post(
      cfg.getMfaRequiredUrl(appParams.app.cluster_name),
      params,
      abortSignal
    );
  }
  return api.post(cfg.getMfaRequiredUrl(), params, abortSignal);
}

function base64EncodeUnicode(str: string) {
  return window.btoa(
    encodeURIComponent(str).replace(/%([0-9A-F]{2})/g, function (match, p1) {
      const hexadecimalStr = '0x' + p1;
      return String.fromCharCode(Number(hexadecimalStr));
    })
  );
}

export default auth;

// TODO(Joerger): In order to check if mfa is required for a leaf host, the leaf
// clusterID must be included in the request. Currently, only IsMfaRequiredApp
// supports this functionality.
export type IsMfaRequiredRequest =
  | IsMfaRequiredDatabase
  | IsMfaRequiredNode
  | IsMfaRequiredKube
  | IsMfaRequiredWindowsDesktop
  | IsMfaRequiredApp
  | IsMfaRequiredAdminAction;

export type IsMfaRequiredResponse = {
  required: boolean;
};

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

export type IsMfaRequiredApp = {
  app: {
    // fqdn indicates (tentatively) the fully qualified domain name of the application.
    fqdn: string;
    // public_addr is the public address of the application.
    public_addr: string;
    // cluster_name is the cluster within which this application is running.
    cluster_name: string;
  };
};

export type IsMfaRequiredAdminAction = {
  // empty object.
  admin_action: Record<string, never>;
};

// MfaChallengeScope is an mfa challenge scope. Possible values are defined in mfa.proto
export enum MfaChallengeScope {
  UNSPECIFIED = 0,
  LOGIN = 1,
  PASSWORDLESS_LOGIN = 2,
  HEADLESS_LOGIN = 3,
  MANAGE_DEVICES = 4,
  ACCOUNT_RECOVERY = 5,
  USER_SESSION = 6,
  ADMIN_ACTION = 7,
  CHANGE_PASSWORD = 8,
}
