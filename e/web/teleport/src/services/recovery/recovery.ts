import api from 'teleport/services/api';
import auth, {
  makeWebauthnAssertionResponse,
  makeWebauthnCreationResponse,
  makeRecoveryCodes,
  NewCredentialRequest,
} from 'teleport/services/auth';

import cfg from 'e-teleport/config';

import makeRecoveryToken from './makeRecoveryToken';
import { StartRecoveryRequest, VerifyUserRequest } from './types';

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

  verifyUserWithWebauthn(tokenId: string, username: string) {
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

  setNewTotpDeviceOrPassword(req: NewCredentialRequest) {
    return api.post(cfg.api.recoveryNewCredentialsPath, {
      ...req,
      secondFactorToken: req.otpCode,
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
      .post(cfg.api.recoveryCodesPath, { tokenId })
      .then(makeRecoveryCodes);
  }

  fetchRecoveryCodesMetadata() {
    return api.get(cfg.api.recoveryCodesPath).then(makeRecoveryCodes);
  }
}

export default RecoveryService;
