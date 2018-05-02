/*
Copyright 2015 Gravitational, Inc.

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

import { Store, Immutable } from 'nuclear-js';
import reactor from 'app/reactor';

const SET = 'MISC_SET';
const STORE_NAME = 'tlpt_misc';

// stores any temporary data
const store = Store({
  getInitialState() {
    return new Immutable.Map();
  },

  initialize() {
    this.on(SET, (state, { key, payload } ) => state.set(key, payload));
  }
});

export const register = reactor => {
  reactor.registerStores({  
    [STORE_NAME]: store
  });
}

export const storage = {
  
  save(key, payload) {
    reactor.dispatch(SET, { key, payload });
  },

  findByKey(key) {
    return reactor.evaluate([STORE_NAME, key]);
  }
}

