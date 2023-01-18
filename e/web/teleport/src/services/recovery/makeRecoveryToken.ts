import { RecoveryToken } from './types';

export default function makeRecoveryToken(json): RecoveryToken {
  const {
    username,
    isRecoverPassword,
    isApproved = false,
    qrCode,
    tokenId,
  } = json;

  return {
    username,
    isRecoverPassword,
    isApproved,
    qrCode,
    id: tokenId,
  };
}
