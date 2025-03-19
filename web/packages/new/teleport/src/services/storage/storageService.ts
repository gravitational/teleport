import type { BearerToken } from '../session/sessionService';
import { KeysEnum } from './types';

export const StorageService = {
  getLicenseAcknowledged(): boolean {
    return (
      window.localStorage.getItem(KeysEnum.LICENSE_ACKNOWLEDGED) === 'true'
    );
  },

  setLicenseAcknowledged() {
    window.localStorage.setItem(KeysEnum.LICENSE_ACKNOWLEDGED, 'true');
  },

  getBearerToken(): BearerToken | null {
    const item = window.localStorage.getItem(KeysEnum.TOKEN);

    if (item) {
      return JSON.parse(item) as BearerToken;
    }

    return null;
  },

  setBearerToken(token: BearerToken) {
    window.localStorage.setItem(KeysEnum.TOKEN, JSON.stringify(token));
  },
};
