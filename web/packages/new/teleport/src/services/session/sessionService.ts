import type { TrustedDeviceRequirement } from 'gen-proto-ts/teleport/legacy/types/trusted_device_requirement_pb';
import { Logger } from 'shared-new/logger';

import { StorageService } from '../storage/storageService';

export interface BearerToken {
  accessToken: string;
  created: number;
  expiresIn: number;
  sessionExpires: Date;
  sessionInactiveTimeout: number;
  trustedDeviceRequirement: TrustedDeviceRequirement;
}

interface BackendBearerToken {
  token: string;
  expires_in: number;
  sessionExpires: Date;
  sessionInactiveTimeout: number;
  trustedDeviceRequirement: number;
}

const logger = new Logger('services/session');

export class SessionService {
  static getBearerToken() {
    try {
      const token = SessionService.extractBearerTokenFromHtml();

      if (token) {
        StorageService.setBearerToken(token);

        return token;
      }

      const storedToken = StorageService.getBearerToken();

      if (storedToken) {
        return storedToken;
      }
    } catch (err) {
      logger.error('No bearer token found in HTML or local storage', err);
    }
  }

  static extractBearerTokenFromHtml(): BearerToken | null {
    const el = document.querySelector<HTMLMetaElement>(
      '[name=grv_bearer_token]'
    );

    if (!el?.content) {
      return null;
    }

    el.parentNode?.removeChild(el);

    const decoded = window.atob(el.content);

    const token = JSON.parse(decoded) as BackendBearerToken;

    return convertBackendBearerToken(token);
  }
}

function convertBackendBearerToken(token: BackendBearerToken): BearerToken {
  return {
    accessToken: token.token,
    created: Date.now(),
    expiresIn: token.expires_in,
    sessionExpires: token.sessionExpires,
    sessionInactiveTimeout: token.sessionInactiveTimeout,
    trustedDeviceRequirement:
      token.trustedDeviceRequirement as TrustedDeviceRequirement,
  };
}
