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
import {
  DeviceType,
  DeviceUsage,
  MfaAuthenticateChallenge,
  MfaChallengeResponse,
  SsoChallenge,
} from 'teleport/services/mfa';
import { CaptureEvent, userEventService } from 'teleport/services/userEvent';

import {
  makeWebauthnAssertionResponse,
  makeWebauthnCreationResponse,
  parseMfaChallengeJson,
  parseMfaRegistrationChallengeJson,
} from '../mfa/makeMfa';
import { makeChangedUserAuthn } from './make';
import makePasswordToken from './makePasswordToken';
import {
  ChangePasswordReq,
  CreateAuthenticateChallengeRequest,
  CreateNewHardwareDeviceRequest,
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
      .then(parseMfaRegistrationChallengeJson);
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
      .then(parseMfaChallengeJson);
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
      .then(parseMfaChallengeJson);
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

  changePassword({ oldPassword, newPassword, mfaResponse }: ChangePasswordReq) {
    const data = {
      old_password: base64EncodeUnicode(oldPassword),
      new_password: base64EncodeUnicode(newPassword),
      second_factor_token: mfaResponse.totp_code,
      webauthnAssertionResponse: mfaResponse.webauthn_response,
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
      .getMfaChallenge({ scope: MfaChallengeScope.HEADLESS_LOGIN })
      .then(challenge => auth.getMfaChallengeResponse(challenge, 'webauthn'))
      .then(res => {
        const request = {
          action: 'accept',
          webauthnAssertionResponse: res.webauthn_response,
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

  // getChallenge gets an MFA challenge for the provided parameters. If is_mfa_required_req
  // is provided and it is found that MFA is not required, returns undefined instead.
  async getMfaChallenge(
    req: CreateAuthenticateChallengeRequest,
    abortSignal?: AbortSignal
  ): Promise<MfaAuthenticateChallenge | undefined> {
    return api
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
      .then(parseMfaChallengeJson);
  },

  // getChallengeResponse gets an MFA challenge response for the provided parameters.
  // If challenge is undefined or has no viable challenge options, returns empty response.
  async getMfaChallengeResponse(
    challenge: MfaAuthenticateChallenge,
    mfaType?: DeviceType,
    totpCode?: string
  ): Promise<MfaChallengeResponse> {
    // No challenge, return empty response.
    if (!challenge) return {};

    // TODO(Joerger): If mfaType is not provided by a parent component, use some global context
    // to display a component, similar to the one used in useMfa. For now we just default to
    // whichever method we can succeed with first.
    if (!mfaType) {
      if (totpCode) {
        mfaType = 'totp';
      } else if (challenge.webauthnPublicKey) {
        mfaType = 'webauthn';
      } else if (challenge.ssoChallenge) {
        mfaType = 'sso';
      }
    }

    if (mfaType === 'webauthn') {
      return auth.getWebAuthnChallengeResponse(challenge.webauthnPublicKey);
    }

    if (mfaType === 'sso') {
      return auth.getSsoChallengeResponse(challenge.ssoChallenge);
    }

    if (mfaType === 'totp') {
      return {
        totp_code: totpCode,
      };
    }

    // No viable challenge, return empty response.
    return {};
  },

  async getWebAuthnChallengeResponse(
    webauthnPublicKey: PublicKeyCredentialRequestOptions
  ): Promise<MfaChallengeResponse> {
    return auth.checkWebauthnSupport().then(() =>
      navigator.credentials
        .get({
          publicKey: webauthnPublicKey,
        })
        .then(cred => {
          return makeWebauthnAssertionResponse(cred);
        })
        .then(resp => {
          return { webauthn_response: resp };
        })
    );
  },

  createPrivilegeToken(existingMfaResponse?: MfaChallengeResponse) {
    return api.post(cfg.api.createPrivilegeTokenPath, {
      existingMfaResponse,
      // TODO(Joerger): DELETE IN v19.0.0
      // Also provide totp/webauthn response in backwards compatible format.
      secondFactorToken: existingMfaResponse?.totp_code,
      webauthnAssertionResponse: existingMfaResponse?.webauthn_response,
    });
  },

  async getSsoChallengeResponse(
    challenge: SsoChallenge
  ): Promise<MfaChallengeResponse> {
    const abortController = new AbortController();

    auth.openSsoChallengeRedirect(challenge, abortController);
    return await auth.waitForSsoChallengeResponse(
      challenge,
      abortController.signal
    );
  },

  openSsoChallengeRedirect(
    { redirectUrl }: SsoChallenge,
    abortController?: AbortController
  ) {
    // try to center the screen
    const width = 1045;
    const height = 550;
    const left = (screen.width - width) / 2;
    const top = (screen.height - height) / 2;

    // these params will open a tiny window.
    const params = `width=${width},height=${height},left=${left},top=${top}`;
    const w = window.open(redirectUrl, '_blank', params);

    // If the redirect URL window is closed prematurely, abort.
    w.onclose = abortController?.abort;
  },

  async waitForSsoChallengeResponse(
    { channelId, requestId }: SsoChallenge,
    abortSignal: AbortSignal
  ): Promise<MfaChallengeResponse> {
    const channel = new BroadcastChannel(channelId);
    const msg = await waitForMessage(channel, abortSignal);
    return {
      sso_response: {
        requestId,
        token: msg.data.mfaToken,
      },
    };
  },

  createRestrictedPrivilegeToken() {
    return api.post(cfg.api.createPrivilegeTokenPath, {});
  },

  getMfaChallengeResponseForAdminAction(allowReuse?: boolean) {
    // If the client is checking if MFA is required for an admin action,
    // but we know admin action MFA is not enforced, return early.
    if (!cfg.isAdminActionMfaEnforced()) {
      return;
    }

    return auth
      .getMfaChallenge({
        scope: MfaChallengeScope.ADMIN_ACTION,
        allowReuse: allowReuse,
        isMfaRequiredRequest: {
          admin_action: {},
        },
      })
      .then(auth.getMfaChallengeResponse);
  },
};

function checkMfaRequired(
  params: IsMfaRequiredRequest,
  abortSignal?
): Promise<IsMfaRequiredResponse> {
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

function waitForMessage(
  channel: BroadcastChannel,
  abortSignal: AbortSignal
): Promise<MessageEvent> {
  return new Promise((resolve, reject) => {
    // Create the event listener
    function eventHandler(e: MessageEvent) {
      // Remove the event listener after it triggers
      channel.removeEventListener('message', eventHandler);
      // Resolve the promise with the event object
      resolve(e);
    }

    // Add the event listener
    channel.addEventListener('message', eventHandler);

    // Close the event listener early if aborted.
    abortSignal.onabort = e => {
      channel.removeEventListener('message', eventHandler);
      reject(e);
    };
  });
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
