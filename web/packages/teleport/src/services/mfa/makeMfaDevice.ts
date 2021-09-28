import { MfaDevice } from './types';

export default function makeMfaDevice(json): MfaDevice {
  const { id, name, lastUsed, addedAt } = json;

  let description = '';
  if (json.type === 'TOTP') {
    description = 'Authenticator App';
  } else if (json.type === 'U2F') {
    description = 'Hardware Key';
  } else {
    description = 'unknown device';
  }

  return {
    id,
    name,
    description,
    registeredDate: new Date(addedAt),
    lastUsedDate: new Date(lastUsed),
  };
}
