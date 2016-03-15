import { Store, Immutable } from 'nuclear-js';
import {TLPT_NOTIFICATIONS_ADD} from './actionTypes';

export default Store({
  getInitialState() {
    return new Immutable.OrderedMap();
  },

  initialize() {
    this.on(TLPT_NOTIFICATIONS_ADD, addNotification);
  },
});

function addNotification(state, message) {
  return state.set(state.size, message);
}
