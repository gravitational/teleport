/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { KeysEnum, EventKeys } from './types';

import type { BroadcastChannelMessage, BearerToken } from './types';

export class StoreWebSession {
  bcBroadcaster: BroadcastChannel;
  bcReceiver: BroadcastChannel;

  constructor(bcBroadcaster?: BroadcastChannel, bcReceiver?: BroadcastChannel) {
    this.initBroadcastChannel(bcBroadcaster, bcReceiver);
  }

  clear() {
    this.bcBroadcaster.postMessage({ key: EventKeys.CLEAR });
  }

  setBearerToken(token: BearerToken) {
    this.bcBroadcaster.postMessage({ key: EventKeys.UPSERT_TOKEN, token });
  }

  setIsRenewing(isTokenRenewing: boolean) {
    this.bcBroadcaster.postMessage({
      key: EventKeys.UPSERT_TOKEN_IS_RENEWING,
      isTokenRenewing,
    });
  }

  setLastActive(lastActive: number) {
    this.bcBroadcaster.postMessage({
      key: EventKeys.UPSERT_LAST_ACTIVE,
      lastActive,
    });
  }

  getIsRenewing(): boolean {
    return window.sessionStorage.getItem(KeysEnum.TOKEN_RENEW) === 'true';
  }

  getLastActive() {
    const time = Number(window.sessionStorage.getItem(KeysEnum.LAST_ACTIVE));
    return time ? time : 0;
  }

  getAccessToken() {
    const bearerToken = getBearerToken();
    return bearerToken ? bearerToken.accessToken : null;
  }

  getSessionInactivityTimeout() {
    const bearerToken = getBearerToken();
    const time = Number(bearerToken.sessionInactiveTimeout);
    return time ? time : 0;
  }

  initBroadcastChannel(
    bcBroadcaster?: BroadcastChannel,
    bcReceiver?: BroadcastChannel
  ) {
    this.bcBroadcaster =
      bcBroadcaster || new BroadcastChannel('websession_store');
    this.bcReceiver = bcReceiver || new BroadcastChannel('websession_store');

    this.bcReceiver.onmessage = ({
      data,
    }: MessageEvent<BroadcastChannelMessage>) => {
      // UPSERT_TOKEN indicates that a token has been upserted from a tab, this
      // causes all tabs to update their own sessionStorage with that token.
      if (data.key === EventKeys.UPSERT_TOKEN) {
        window.sessionStorage.setItem(
          KeysEnum.TOKEN,
          JSON.stringify(data.token)
        );
      }

      // GET_TOKEN indicates that a tab is asking for the token, if a tab
      // has a token in its sessionStorage, it upserts it so that other tabs
      // receive it and update their own sessionStorage with it.
      if (data.key === EventKeys.GET_TOKEN) {
        const token = getBearerToken();
        if (token) {
          this.bcBroadcaster.postMessage({
            key: EventKeys.UPSERT_TOKEN,
            token,
          });
        }
      }

      if (data.key === EventKeys.UPSERT_TOKEN_IS_RENEWING) {
        window.sessionStorage.setItem(
          KeysEnum.TOKEN_RENEW,
          data.isTokenRenewing.toString()
        );
      }

      if (data.key === EventKeys.GET_TOKEN_IS_RENEWING) {
        const isRenewing = this.getIsRenewing();
        if (isRenewing) {
          this.bcBroadcaster.postMessage({
            key: EventKeys.UPSERT_TOKEN_IS_RENEWING,
            isRenewing,
          });
        }
      }

      if (data.key === EventKeys.UPSERT_LAST_ACTIVE) {
        window.sessionStorage.setItem(
          KeysEnum.LAST_ACTIVE,
          data.lastActive.toString()
        );
      }

      if (data.key === EventKeys.GET_LAST_ACTIVE) {
        const lastActive = this.getLastActive();
        if (lastActive) {
          this.bcBroadcaster.postMessage({
            key: EventKeys.UPSERT_LAST_ACTIVE,
            lastActive,
          });
        }
      }

      if (data.key === EventKeys.CLEAR) {
        window.sessionStorage.clear();
        window.localStorage.clear();
      }
    };

    // On page load, call out to other tabs to receive values
    this.bcBroadcaster.postMessage({
      key: EventKeys.GET_TOKEN,
    });
    this.bcBroadcaster.postMessage({
      key: EventKeys.GET_TOKEN_IS_RENEWING,
    });
    this.bcBroadcaster.postMessage({
      key: EventKeys.GET_LAST_ACTIVE,
    });
  }
}

export function getBearerToken(): BearerToken {
  try {
    return JSON.parse(window.sessionStorage.getItem(KeysEnum.TOKEN));
  } catch (err) {
    return err;
  }
}
