import 'u2f-api-polyfill';
import api from 'teleport/services/api';
import auth, {
  makeWebauthnAssertionResponse,
  makeWebauthnCreationResponse,
} from 'teleport/services/auth';
import cfg from 'e-teleport/config';
import makeRecoveryToken from './makeRecoveryToken';
import {
  RecoveryToken,
  StartRecoveryRequest,
  VerifyUserRequest,
  NewCredentialRequest,
} from './types';

class RecoveryService {
  // startRecovery validates the recovery code and sends the email
  startRecovery(data: StartRecoveryRequest) {
    return api.post(cfg.api.recoveryStartPath, data);
  }

  fetchRecoveryToken(tokenId: string) {
    return api.get(cfg.getRecoveryTokenUrl(tokenId)).then(makeRecoveryToken);
  }

  // verifyUser authenticates the user defined in token with their password or totp token
  verifyUser(credentials: VerifyUserRequest) {
    return api
      .post(cfg.api.recoveryVerifyUserPath, credentials)
      .then(makeRecoveryToken);
  }

  // verifyUserWithU2f authenticates the user defined in token with their u2f creds
  verifyUserWithU2f(tokenId: string, username: string) {
    const err = auth.u2fBrowserSupported();
    if (err) {
      return Promise.reject(err);
    }

    return new Promise<RecoveryToken>((resolve, reject) => {
      auth.createMfaAuthnChallengeWithToken(tokenId).then(data => {
        window['u2f'].sign(null, null, data.u2fSignRequests, res => {
          if (res.errorCode) {
            const err = auth._getU2fErr(res.errorCode);
            reject(err);
            return;
          }

          api
            .post(cfg.api.recoveryVerifyUserPath, {
              tokenId,
              username,
              u2fSignResponse: res,
            })
            .then(res => resolve(makeRecoveryToken(res)))
            .catch(err => {
              reject(err);
            });
        });
      });
    });
  }

  verifyUserWithWebautn(tokenId: string, username: string) {
    return auth
      .checkWebauthnSupport()
      .then(() => auth.createMfaAuthnChallengeWithToken(tokenId))
      .then(res =>
        navigator.credentials.get({
          publicKey: res.webauthnPublicKey,
        })
      )
      .then(res => {
        const request = {
          tokenId,
          username,
          webauthnAssertionResponse: makeWebauthnAssertionResponse(res),
        };

        return api.post(cfg.api.recoveryVerifyUserPath, request);
      })
      .then(res => makeRecoveryToken(res));
  }

  setNewTotpDeviceOrPassword(data: NewCredentialRequest) {
    return api.post(cfg.api.recoveryNewCredentialsPath, data);
  }

  setNewU2fDevice(data: NewCredentialRequest) {
    const err = auth.u2fBrowserSupported();
    if (err) {
      return Promise.reject(err);
    }

    return auth._getU2FRegisterRes(data.tokenId).then(u2fRegisterResponse => {
      return api.post(cfg.api.recoveryNewCredentialsPath, {
        ...data,
        u2fRegisterResponse,
      });
    });
  }

  setNewWebauthnDevice(data: NewCredentialRequest) {
    return auth
      .checkWebauthnSupport()
      .then(() => auth.createMfaRegistrationChallenge(data.tokenId, 'webauthn'))
      .then(res =>
        navigator.credentials.create({
          publicKey: res.webauthnPublicKey,
        })
      )
      .then(res => {
        const request = {
          ...data,
          webauthnCreationResponse: makeWebauthnCreationResponse(res),
        };

        return api.post(cfg.api.recoveryNewCredentialsPath, request);
      });
  }

  generateRecoveryCodes(tokenId: string) {
    return api
      .post(cfg.api.recoveryNewCodesPath, { tokenId })
      .then(res => res || []);
  }
}

export default RecoveryService;
