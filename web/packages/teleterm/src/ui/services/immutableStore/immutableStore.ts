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

  /**
   * Adds a callback which gets called only when the part of the state returned by selector is
   * changed. selector must be pure.
   */
  subscribeWithSelector<SelectedState>(
    selector: (state: T) => SelectedState,
    callback: () => void
  ) {
    let selectedState = selector(this.state);

    this.subscribe(() => {
      const newSelectedState = selector(this.state);
      // It doesn't appear to be explicitly documented anywhere, but Immer preserves object
      // identity, so Object.is works as expected. This behavior is covered by our tests.
      const hasSelectedStateChanged = !Object.is(
        newSelectedState,
        selectedState
      );

      if (hasSelectedStateChanged) {
        callback();
      }

      selectedState = newSelectedState;
    });
  }
}
