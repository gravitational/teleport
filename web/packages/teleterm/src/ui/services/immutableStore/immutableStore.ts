/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

/* eslint-disable @typescript-eslint/ban-ts-comment*/

import { enableMapSet, produce } from 'immer';
import Store from 'shared/libs/stores/store';
import stateLogger from 'shared/libs/stores/logger';

import Logger from 'teleterm/logger';

enableMapSet();

export class ImmutableStore<T> extends Store<T> {
  protected logger = new Logger(this.constructor.name);

  // @ts-ignore
  setState(nextState: (draftState: T) => T | void): void {
    const prevState = this.state;
    this.state = produce(this.state, nextState);
    stateLogger.logState(this.constructor.name, prevState, 'with', this.state);

    this._subs.forEach(cb => {
      try {
        cb();
      } catch (error) {
        this.logger.error(`Store failed to notify subscriber`, error);
      }
    });
  }
}
