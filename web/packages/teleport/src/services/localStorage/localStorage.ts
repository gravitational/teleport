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

import { OnboardDiscover } from 'teleport/services/user';

import { KeysEnum } from './types';

const storage = {
  clear() {
    window.localStorage.clear();
  },

  // setOnboardDiscover persists states used to determine if a user should
  // be onboarded to use the discovery wizard or not. User should only
  // be onboarded once upon login.
  setOnboardDiscover(d: OnboardDiscover) {
    window.localStorage.setItem(KeysEnum.DISCOVER, JSON.stringify(d));
  },

  getOnboardDiscover(): OnboardDiscover {
    const item = window.localStorage.getItem(KeysEnum.DISCOVER);
    if (item) {
      return JSON.parse(item);
    }
    return null;
  },
};

export default storage;
