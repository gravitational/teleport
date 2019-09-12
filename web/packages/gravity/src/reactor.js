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

import { Reactor } from 'nuclear-js'

const isDebug = process.env.NODE_ENV === 'development' && !process.env.NODE_ENV_TYPE === 'test';
const CSS = 'color: blue';

// reactor options
const options = {
  debug: isDebug
}

const logger = {
  dispatchStart(reactorState, actionType, payload) {
    window.console.log(`%creactor.dispatch("${actionType}", `, CSS, payload, `)`);
  },

  dispatchError: function (reactorState, error) {
    window.console.debug('Dispatch error: ' + error)
  },

  dispatchEnd(reactorState, state, dirtyStores) {
    const stateChanges = state.filter((val, key) => dirtyStores.contains(key));
    window.console.log('%cupdated store -> ',
      CSS,
      stateChanges.toJS())
  }
}

if (isDebug) {
  options.logger = logger
}

const reactor = new Reactor(options)
window.reactor = reactor;

export default reactor