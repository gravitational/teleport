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
  u2fSignResponse?: string;
};

export type NewCredentialRequest = {
  tokenId: string;
  password?: string;
  secondFactorToken?: string;
  u2fRegisterResponse?: string;
  deviceName?: string;
};

export type RecoveryToken = {
  id: string;
  username: string;
  isRecoverPassword: boolean;
  isApproved: boolean;
  qrCode: string;
};
