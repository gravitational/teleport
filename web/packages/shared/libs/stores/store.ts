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
