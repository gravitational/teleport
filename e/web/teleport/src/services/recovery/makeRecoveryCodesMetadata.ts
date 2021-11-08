import { RecoveryCodesMetadata } from './types';

export default function makeRecoveryCodesMetadata(json): RecoveryCodesMetadata {
  const { created } = json;

  return {
    createdDate: created ? new Date(created) : null,
  };
}
