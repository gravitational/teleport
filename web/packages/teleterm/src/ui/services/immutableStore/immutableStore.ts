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
