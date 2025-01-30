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

import Logger from 'shared/libs/logger';

import cfg from 'teleport/config';
import api from 'teleport/services/api';
import history from 'teleport/services/history';
import { KeysEnum, storageService } from 'teleport/services/storageService';

import makeBearerToken from './makeBearerToken';
import { RenewSessionRequest } from './types';

// Time to determine when to renew session which is
// when expiry time of token is less than 3 minutes.
const RENEW_TOKEN_TIME = 180 * 1000;
const TOKEN_CHECKER_INTERVAL = 15 * 1000; //  every 15 sec
const logger = Logger.create('services/session');

let sesstionCheckerTimerId = null;

const session = {
  logout(rememberLocation = false) {
    api.delete(cfg.api.webSessionPath).then(response => {
      this.clear();
      if (response.samlSloUrl) {
        window.open(response.samlSloUrl, '_self');
      } else {
        history.goToLogin({ rememberLocation });
      }
    });
  },

  logoutWithoutSlo({
    rememberLocation = false,
    withAccessChangedMessage = false,
  } = {}) {
    api.delete(cfg.api.webSessionPath).finally(() => {
      this.clear();
      history.goToLogin({ rememberLocation, withAccessChangedMessage });
    });
  },

  clearBrowserSession(rememberLocation = false) {
    this.clear();
    history.goToLogin({ rememberLocation });
  },

  clear() {
    this._stopTokenChecker();
    storageService.unsubscribe(receiveMessage);
    storageService.clear();
  },

  // ensureSession verifies that token is valid and starts
  // periodically checking and refreshing the token.
  ensureSession() {
    this._stopTokenChecker();
    this._ensureLocalStorageSubscription();

    if (!this.isValid()) {
      this.logout();
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
    const time = Number(bearerToken.sessionInactiveTimeout);
    return time ? time : 0;
  },

  _getBearerToken() {
    let token = null;
    try {
      token = this._extractBearerTokenFromHtml();
      if (token) {
        storageService.setBearerToken(token);
      } else {
        token = storageService.getBearerToken();
      }
    } catch (err) {
      logger.error('Cannot find bearer token', err);
    }

    return token;
  },

  _extractBearerTokenFromHtml() {
    const el = document.querySelector<HTMLMetaElement>(
      '[name=grv_bearer_token]'
    );
    if (!el || !el.content) {
      return null;
    }
    // remove token from HTML as it will be renewed with a time
    // and stored in the storageService
    el.parentNode.removeChild(el);
    const decoded = window.atob(el.content);
    const json = JSON.parse(decoded);
    return makeBearerToken(json);
  },

  _shouldRenewToken() {
    if (this._getIsRenewing()) {
      return false;
    }

    // Renew session if token expiry time is less than 3 minutes.
    // Browsers have js timer throttling behavior in inactive tabs that can go
    // up to 100s between timer calls from testing. 3 minutes seems to be a safe number
    // with extra padding.
    return this._timeLeft() < RENEW_TOKEN_TIME;
  },

  _renewToken(req: RenewSessionRequest = {}, signal?: AbortSignal) {
    this._setAndBroadcastIsRenewing(true);
    return api
      .post(cfg.getRenewTokenUrl(), req, signal)
      .then(json => {
        const token = makeBearerToken(json);
        storageService.setBearerToken(token);
        return token;
      })
      .finally(() => {
        this._setAndBroadcastIsRenewing(false);
      });
  },

  _setAndBroadcastIsRenewing(value) {
    this._setIsRenewing(value);
    storageService.broadcast(KeysEnum.TOKEN_RENEW, value);
  },

  _setIsRenewing(value) {
    this._isRenewing = value;
  },

  _getIsRenewing() {
    return !!this._isRenewing;
  },

  setDeviceTrustRequired() {
    this._isDeviceTrustRequired = true;
  },

  getDeviceTrustRequired() {
    return !!this._isDeviceTrustRequired;
  },

  getIsDeviceTrusted() {
    return !!this._isDeviceTrusted;
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

    let { expiresIn, created } = token;
    if (!created || !expiresIn) {
      return 0;
    }

    expiresIn = expiresIn * 1000;
    const delta = created + expiresIn - new Date().getTime();
    return delta;
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

  // subsribes to storageService changes (triggered from other browser tabs)
  _ensureLocalStorageSubscription() {
    storageService.subscribe(receiveMessage);
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

    sesstionCheckerTimerId = setInterval(() => {
      // calling ensureSession() will again invoke _startTokenChecker
      this.ensureSession();

      // handle server restarts when session may become invalid
      if (this._shouldCheckStatus()) {
        this._fetchStatus();
      }
    }, TOKEN_CHECKER_INTERVAL);
  },

  _stopTokenChecker() {
    clearInterval(sesstionCheckerTimerId);
    sesstionCheckerTimerId = null;
  },
};

function receiveMessage(event: StorageEvent) {
  const { key, newValue } = event;

  // check if logout was triggered from other tabs
  if (storageService.getBearerToken() === null) {
    session.clearBrowserSession();
  }

  // check if token is being renewed from another tab
  if (key === KeysEnum.TOKEN_RENEW && !!newValue) {
    session._setIsRenewing(JSON.parse(newValue));
  }
}

export default session;
