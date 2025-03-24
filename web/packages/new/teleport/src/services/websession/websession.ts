import { Logger } from 'shared-new/logger';

import type { TrustedDeviceRequirement } from 'gen-proto-ts/teleport/legacy/types/trusted_device_requirement_pb';

import { cfg } from '../../config';
import { api } from '../api';
import { HistoryService } from '../history/historyService';
import { StorageService } from '../storage/storageService';
import { KeysEnum } from '../storage/types';
import type {
  BackendBearerToken,
  BearerToken,
  DeleteWebSessionResponse,
  RenewSessionRequest,
} from './types';

/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

// Time to determine when to renew session which is
// when expiry time of token is less than 3 minutes.
const RENEW_TOKEN_TIME = 180 * 1000;
const TOKEN_CHECKER_INTERVAL = 15 * 1000; //  every 15 sec

const logger = new Logger('services/session');

let sessionCheckerTimerId: number | null = null;

export const session = {
  async logout(rememberLocation = false) {
    const res = await api.delete<DeleteWebSessionResponse>(
      cfg.api.webSessionPath
    );

    this.clear();

    if (res.samlSloUrl) {
      window.open(res.samlSloUrl, '_self');
    } else {
      HistoryService.goToLogin({ rememberLocation });
    }
  },

  async logoutWithoutSlo({
    rememberLocation = false,
    withAccessChangedMessage = false,
  } = {}) {
    try {
      await api.delete(cfg.api.webSessionPath);
    } finally {
      this.clear();

      HistoryService.goToLogin({ rememberLocation, withAccessChangedMessage });
    }
  },

  clearBrowserSession(rememberLocation = false) {
    this.clear();

    HistoryService.goToLogin({ rememberLocation });
  },

  clear() {
    this._stopTokenChecker();

    StorageService.unsubscribe(receiveMessage);
    StorageService.clear();
  },

  // ensureSession verifies that token is valid and starts
  // periodically checking and refreshing the token.
  ensureSession() {
    this._stopTokenChecker();
    this._ensureLocalStorageSubscription();

    if (!this.isValid()) {
      void this.logout();

      return;
    }

    if (this._shouldRenewToken()) {
      this._renewToken()
        .then(() => {
          this._startTokenChecker();
        })
        .catch(this.logout.bind(this));
    } else {
      this._startTokenChecker();
    }
  },

  // renewSession renews session and returns the
  // absolute time the new session expires.
  renewSession(req: RenewSessionRequest, signal?: AbortSignal): Promise<Date> {
    return this._renewToken(req, signal).then(token => token.sessionExpires);
  },

  /**
   * isValid first extracts bearer token from HTML if
   * not already extracted and sets in the local storage.
   * Then checks if token is not expired.
   */
  isValid() {
    return this._timeLeft() > 0;
  },

  getInactivityTimeout() {
    const bearerToken = this._getBearerToken();

    if (!bearerToken) {
      return 0;
    }

    const time = Number(bearerToken.sessionInactiveTimeout);

    return time ? time : 0;
  },

  _getBearerToken() {
    try {
      const token = this._extractBearerTokenFromHtml();

      if (token) {
        StorageService.setBearerToken(token);

        return token;
      }

      const storedToken = StorageService.getBearerToken();

      if (storedToken) {
        return storedToken;
      }
    } catch (err) {
      logger.error('Cannot find bearer token', err);
    }

    return;
  },

  _extractBearerTokenFromHtml() {
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
  },

  _shouldRenewToken() {
    if (this._getIsRenewing()) {
      return false;
    }

    const token = this._getBearerToken();

    if (!token) {
      return false;
    }

    // Convert seconds to millis.
    const expiresIn = token.expiresIn * 1000;
    const sessionExpiresIn = token.sessionExpiresIn * 1000;

    // Session TTL decreases on every renewal, up to a point where it doesn't
    // make sense to renew anymore, as we won't gain any extra time from it.
    // Once values are low enough both expiresIn (token expiration) and
    // sessionExpiresIn are set in lockstep.

    if (
      expiresIn > 0 &&
      sessionExpiresIn > 0 &&
      expiresIn >= sessionExpiresIn &&
      sessionExpiresIn <= RENEW_TOKEN_TIME
    ) {
      logger.warn(
        `Session TTL is only ${sessionExpiresIn}ms, the session will expire soon.`
      );
      return false;
    }

    // Browsers have js timer throttling behavior in inactive tabs that can go
    // up to 100s between timer calls from testing. 3 minutes seems to be a safe number
    // with extra padding.
    return this._timeLeft() < RENEW_TOKEN_TIME;
  },

  async _renewToken(req: RenewSessionRequest = {}, signal?: AbortSignal) {
    this._setAndBroadcastIsRenewing(true);

    try {
      const res = await api.post<BackendBearerToken>(
        cfg.getRenewTokenUrl(),
        req,
        signal
      );

      const token = convertBackendBearerToken(res);

      StorageService.setBearerToken(token);

      return token;
    } finally {
      this._setAndBroadcastIsRenewing(false);
    }
  },

  _setAndBroadcastIsRenewing(value: boolean) {
    this._setIsRenewing(value);

    StorageService.broadcast(KeysEnum.TOKEN_RENEW, value.toString());
  },

  _isRenewing: false,
  _isDeviceTrustRequired: false,
  _isDeviceTrusted: false,

  _setIsRenewing(value: boolean) {
    this._isRenewing = value;
  },

  _getIsRenewing() {
    return this._isRenewing;
  },

  setDeviceTrustRequired() {
    this._isDeviceTrustRequired = true;
  },

  getDeviceTrustRequired() {
    return this._isDeviceTrustRequired;
  },

  getIsDeviceTrusted() {
    return this._isDeviceTrusted;
  },

  // a session will never be "downgraded" so we can just set to true
  // if this method is called.
  setIsDeviceTrusted() {
    this._isDeviceTrusted = true;
  },

  _timeLeft() {
    const token = this._getBearerToken();

    if (!token) {
      return 0;
    }

    const { expiresIn, created } = token;

    if (!created || !expiresIn) {
      return 0;
    }

    return created + expiresIn * 1000 - new Date().getTime();
  },

  _shouldCheckStatus() {
    if (this._getIsRenewing()) {
      return false;
    }

    /*
     * double the threshold value for slow connections to avoid
     * access-denied response due to concurrent renew token request
     * made from other tab
     */
    return this._timeLeft() > TOKEN_CHECKER_INTERVAL * 2;
  },

  // subsribes to StorageService changes (triggered from other browser tabs)
  _ensureLocalStorageSubscription() {
    StorageService.subscribe(receiveMessage);
  },

  _fetchStatus() {
    this.validateCookieAndSession().catch(err => {
      // this indicates that session is no longer valid (caused by server restarts or updates)
      if (err.response.status == 403) {
        this.clearBrowserSession();
      }
    });
  },

  /**
   * validateCookieAndSessionFromBackend makes an authenticated request
   * which checks if the cookie and the user session are still valid.
   */
  validateCookieAndSession() {
    return api.get(cfg.api.userStatusPath);
  },

  _startTokenChecker() {
    this._stopTokenChecker();

    sessionCheckerTimerId = window.setInterval(() => {
      // calling ensureSession() will again invoke _startTokenChecker
      this.ensureSession();

      // handle server restarts when session may become invalid
      if (this._shouldCheckStatus()) {
        this._fetchStatus();
      }
    }, TOKEN_CHECKER_INTERVAL);
  },

  _stopTokenChecker() {
    if (sessionCheckerTimerId) {
      window.clearInterval(sessionCheckerTimerId);
    }

    sessionCheckerTimerId = null;
  },
};

function receiveMessage(event: StorageEvent) {
  const { key, newValue } = event;

  // check if logout was triggered from other tabs
  if (StorageService.getBearerToken() === null) {
    session.clearBrowserSession();
  }

  // check if token is being renewed from another tab
  if (key === KeysEnum.TOKEN_RENEW && !!newValue) {
    session._setIsRenewing(JSON.parse(newValue) as boolean);
  }
}

export function convertBackendBearerToken(
  token: BackendBearerToken
): BearerToken {
  return {
    accessToken: token.token,
    created: Date.now(),
    expiresIn: token.expires_in,
    sessionExpires: token.sessionExpires,
    sessionExpiresIn: token.sessionExpiresIn,
    sessionInactiveTimeout: token.sessionInactiveTimeout,
    trustedDeviceRequirement:
      token.trustedDeviceRequirement as TrustedDeviceRequirement,
  };
}
