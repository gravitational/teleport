import cfg from 'teleport/config';
import api from 'teleport/services/api';
import MfaService, {
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

export const auth = {};
export class AuthService {
  mfaService: MfaService;

  constructor(mfaService: MfaService) {
    this.mfaService = mfaService;
  }

  checkWebauthnSupport() {
    if (window.PublicKeyCredential) {
      return Promise.resolve();
    }

    return Promise.reject(
      new Error(
        'this browser does not support Webauthn required for hardware tokens, please try the latest version of Chrome, Firefox or Safari'
      )
    );
  }

  async checkMfaRequired(
    params: IsMfaRequiredRequest,
    abortSignal?: AbortSignal
  ): Promise<IsMfaRequiredResponse> {
    return api.post(cfg.getMfaRequiredUrl(), params, abortSignal);
  }

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
  }

  createNewWebAuthnDevice(
    req: CreateNewHardwareDeviceRequest
  ): Promise<Credential> {
    return this.checkWebauthnSupport()
      .then(() =>
        this.createMfaRegistrationChallenge(
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
  }

  mfaLoginBegin(creds?: UserCredentials) {
    return api
      .post(cfg.api.mfaLoginBegin, {
        passwordless: !creds,
        user: creds?.username,
        pass: creds?.password,
      })
      .then(parseMfaChallengeJson);
  }

  login(userId: string, password: string, otpCode: string) {
    const data = {
      user: userId,
      pass: password,
      second_factor_token: otpCode,
    };

    return api.post(cfg.api.webSessionPath, data);
  }

  async loginWithWebauthn(creds?: UserCredentials) {
    return this.checkWebauthnSupport()
      .then(() => this.mfaLoginBegin(creds))
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
  }

  fetchPasswordToken(tokenId: string) {
    const path = cfg.getPasswordTokenUrl(tokenId);
    return api.get(path).then(makePasswordToken);
  }

  async resetPasswordWithWebauthn(
    props: ResetPasswordWithWebauthnReqWithEvent
  ) {
    const { req, credential, eventMeta } = props;
    const deviceUsage: DeviceUsage = req.password ? 'mfa' : 'passwordless';

    return this.checkWebauthnSupport()
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
  }

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
  }

  changePassword({ oldPassword, newPassword, mfaResponse }: ChangePasswordReq) {
    const data = {
      old_password: base64EncodeUnicode(oldPassword),
      new_password: base64EncodeUnicode(newPassword),
      second_factor_token: mfaResponse.totp_code,
      webauthnAssertionResponse: mfaResponse.webauthn_response,
    };

    return api.put(cfg.api.changeUserPasswordPath, data);
  }

  async headlessSSOGet(transactionId: string) {
    return this.checkWebauthnSupport()
      .then(() => api.get(cfg.getHeadlessSsoPath(transactionId)))
      .then((json: any) => {
        json = json || {};

        return {
          clientIpAddress: json.client_ip_address,
        };
      });
  }

  async headlessSSOAccept(transactionId: string) {
    return this.getMfaChallenge({ scope: MfaChallengeScope.HEADLESS_LOGIN })
      .then(challenge => this.getMfaChallengeResponse(challenge, 'webauthn'))
      .then(res => {
        const request = {
          action: 'accept',
          webauthnAssertionResponse: res?.webauthn_response,
        };

        return api.put(cfg.getHeadlessSsoPath(transactionId), request);
      });
  }

  headlessSSOReject(transactionId: string) {
    const request = {
      action: 'denied',
    };

    return api.put(cfg.getHeadlessSsoPath(transactionId), request);
  }

  async getMfaChallenge(
    req: CreateAuthenticateChallengeRequest,
    abortSignal?: AbortSignal
  ) {
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
  }

  async getMfaChallengeResponse(
    challenge: MfaAuthenticateChallenge,
    mfaType?: DeviceType,
    totpCode?: string
  ): Promise<MfaChallengeResponse | undefined> {
    if (!challenge) return;

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
      return this.getWebAuthnChallengeResponse(challenge.webauthnPublicKey);
    }

    if (mfaType === 'sso') {
      return this.getSsoChallengeResponse(challenge.ssoChallenge);
    }

    if (mfaType === 'totp') {
      return {
        totp_code: totpCode,
      };
    }

    return;
  }

  async getWebAuthnChallengeResponse(
    webauthnPublicKey: PublicKeyCredentialRequestOptions
  ): Promise<MfaChallengeResponse> {
    return this.checkWebauthnSupport().then(() =>
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
  }

  createPrivilegeToken(existingMfaResponse?: MfaChallengeResponse) {
    return api.post(cfg.api.createPrivilegeTokenPath, {
      existingMfaResponse,
      secondFactorToken: existingMfaResponse?.totp_code,
      webauthnAssertionResponse: existingMfaResponse?.webauthn_response,
    });
  }

  async getSsoChallengeResponse(
    challenge: SsoChallenge
  ): Promise<MfaChallengeResponse> {
    const abortController = new AbortController();
    this.openSsoChallengeRedirect(challenge, abortController);
    return await this.waitForSsoChallengeResponse(
      challenge,
      abortController.signal
    );
  }

  openSsoChallengeRedirect(
    { redirectUrl }: SsoChallenge,
    abortController?: AbortController
  ) {
    const width = 1045;
    const height = 550;
    const left = (screen.width - width) / 2;
    const top = (screen.height - height) / 2;
    const params = `width=${width},height=${height},left=${left},top=${top}`;
    const w = window.open(redirectUrl, '_blank', params);
    w.onclose = abortController?.abort;
  }

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
  }

  createPrivilegeTokenWithWebauthn() {
    return this.getMfaChallenge({ scope: MfaChallengeScope.MANAGE_DEVICES })
      .then(this.getMfaChallengeResponse.bind(this))
      .then(mfaResp => this.createPrivilegeToken(mfaResp));
  }

  createPrivilegeTokenWithTotp(secondFactorToken: string) {
    return api.post(cfg.api.createPrivilegeTokenPath, { secondFactorToken });
  }

  createRestrictedPrivilegeToken() {
    return api.post(cfg.api.createPrivilegeTokenPath, {});
  }

  async getWebauthnResponse(
    scope: MfaChallengeScope,
    allowReuse?: boolean,
    isMfaRequiredRequest?: IsMfaRequiredRequest,
    abortSignal?: AbortSignal
  ) {
    if (isMfaRequiredRequest) {
      try {
        const isMFARequired = await this.checkMfaRequired(
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
          return;
        }
        throw err;
      }
    }

    return this.getMfaChallenge(
      { scope, allowReuse, isMfaRequiredRequest },
      abortSignal
    )
      .then(challenge => this.getMfaChallengeResponse(challenge, 'webauthn'))
      .then(res => res?.webauthn_response);
  }

  getMfaChallengeResponseForAdminAction(allowReuse?: boolean) {
    if (!cfg.isAdminActionMfaEnforced()) {
      return;
    }

    return this.getMfaChallenge({
      scope: MfaChallengeScope.ADMIN_ACTION,
      allowReuse: allowReuse,
      isMfaRequiredRequest: {
        admin_action: {},
      },
    }).then(this.getMfaChallengeResponse.bind(this));
  }

  getWebauthnResponseForAdminAction(allowReuse?: boolean) {
    return this.getMfaChallengeResponseForAdminAction(allowReuse);
  }
}

// Helper functions
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
    function eventHandler(e: MessageEvent) {
      channel.removeEventListener('message', eventHandler);
      resolve(e);
    }

    channel.addEventListener('message', eventHandler);

    abortSignal.onabort = e => {
      channel.removeEventListener('message', eventHandler);
      reject(e);
    };
  });
}

export type IsMfaRequiredResponse = {
  required: boolean;
};

export type IsMfaRequiredRequest =
  | IsMfaRequiredDatabase
  | IsMfaRequiredNode
  | IsMfaRequiredKube
  | IsMfaRequiredWindowsDesktop
  | IsMfaRequiredApp
  | IsMfaRequiredAdminAction;

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
