import cfg from 'e-teleport/config';
import api from 'teleport/services/api';
import {
  makeRecoveryCodes,
  makeWebauthnCreationResponse,
  NewCredentialRequest,
} from 'teleport/services/auth';

import makeRecoveryToken from './makeRecoveryToken';
import {
  NewWebAuthnDeviceRequest,
  StartRecoveryRequest,
  VerifyUserRequest,
} from './types';

class RecoveryService {
  // startRecovery validates the recovery code and sends the email
  startRecovery(data: StartRecoveryRequest) {
    return api.post(cfg.api.recoveryStartPath, data);
  }

  fetchRecoveryToken(tokenId: string) {
    return api.get(cfg.getRecoveryTokenUrl(tokenId)).then(makeRecoveryToken);
  }

  // verifyUser authenticates the recovery token holder with either a password
  // or an MFA challenge response.
  verifyUser(req: VerifyUserRequest) {
    return api
      .post(cfg.api.recoveryVerifyUserPath, req)
      .then(makeRecoveryToken);
  }

  setNewTotpDeviceOrPassword(req: NewCredentialRequest) {
    return api.post(cfg.api.recoveryNewCredentialsPath, {
      ...req,
      secondFactorToken: req.otpCode,
    });
  }

  setNewWebauthnDevice(req: NewWebAuthnDeviceRequest) {
    const request = {
      ...req.credentialRequest,
      webauthnCreationResponse: makeWebauthnCreationResponse(req.credential),
    };
    return api.post(cfg.api.recoveryNewCredentialsPath, request);
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
