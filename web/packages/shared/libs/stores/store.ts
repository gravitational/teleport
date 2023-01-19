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

import logger from './logger';

type callback = () => any;

// Store is the base class for all stores.
export default class Store<T> {
  protected _subs: callback[] = [];

  state: T;

  // adds a callback to the list of subscribers
  subscribe(cb: callback) {
    const storeName = this.constructor.name;
    logger.info(`subscribe to store ${storeName}`, this.state);
    this._subs.push(cb);
  }

  // removes a callback from the list of subscribers
  unsubscribe(cb: callback) {
    const index = this._subs.indexOf(cb);
    if (index > -1) {
      const storeName = this.constructor.name;
      logger.info(`unsubscribe from store ${storeName}`);
      this._subs.splice(index, 1);
    }
  }

  // this is the primary method you use to update the store state,
  // it changes the store state and notifies subscribers.
  public setState(nextState: Partial<T>) {
    this.state = mergeStates(nextState, this.state);
    logger.logState(this.constructor.name, this.state, 'with', nextState);

    this._subs.forEach(cb => {
      try {
        cb();
      } catch (err) {
        logger.error(
          `Store ${this.constructor.name} failed to notify subscriber`,
          err
        );
      }
    });
  }
}

function mergeStates<T>(nextState: Partial<T>, prevState: T): T {
  if (isObject(prevState) && isObject(nextState)) {
    return {
      ...prevState,
      ...nextState,
    };
  }

  return nextState as T;
}

function isObject(obj: any) {
  return !Array.isArray(obj) && typeof obj === 'object' && obj !== null;
}
