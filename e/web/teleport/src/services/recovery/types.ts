import { NewCredentialRequest } from 'teleport/services/auth';
import { WebauthnAssertionResponse } from 'teleport/services/mfa';

export type StartRecoveryRequest = {
  username: string;
  recoveryCode: string;
  isRecoverPassword: boolean;
};

export type VerifyUserRequest = {
  tokenId: string;
  username: string;
  password?: string;
  secondFactorToken?: string;
  webauthnAssertionResponse?: WebauthnAssertionResponse;
};

export type RecoveryToken = {
  id: string;
  username: string;
  isRecoverPassword: boolean;
  isApproved: boolean;
  qrCode: string;
};

export type NewWebAuthnDeviceRequest = {
  credentialRequest: NewCredentialRequest;
  credential: Credential;
};
