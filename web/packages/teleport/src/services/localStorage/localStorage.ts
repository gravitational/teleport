/*
Copyright 2019 Gravitational, Inc.

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

import { BearerToken, KeysEnum } from './types';

const storage = {
  clear() {
    storage.setAccessRequestResult(null);
    storage.setBearerToken(null);
    window.localStorage.clear();
    window.sessionStorage.clear();
  },

  subscribe(fn) {
    window.addEventListener('storage', fn);
  },

  unsubscribe(fn) {
    window.removeEventListener('storage', fn);
  },

  setBearerToken(token: BearerToken) {
    window.localStorage.setItem(KeysEnum.TOKEN, JSON.stringify(token));
  },

  getBearerToken(): BearerToken {
    const item = window.localStorage.getItem(KeysEnum.TOKEN);
    if (item) {
      return JSON.parse(item);
    }

    return null;
  },

  getAccessToken() {
    const bearerToken = this.getBearerToken();
    return bearerToken ? bearerToken.accessToken : null;
  },

  setAccessRequestResult(request) {
    window.sessionStorage.setItem(
      KeysEnum.ACCESS_REQUEST,
      JSON.stringify(request)
    );
  },

  getAccessRequestResult() {
    const item = window.sessionStorage.getItem(KeysEnum.ACCESS_REQUEST);
    return item ? JSON.parse(item) : null;
  },

  broadcast(messageType, messageBody) {
    window.localStorage.setItem(messageType, messageBody);
    window.localStorage.removeItem(messageType);
  },
};

export default storage;
