import reactor from '../../reactor';
import { Store } from 'nuclear-js';
import * as AT from './actionTypes';

import { StoreRec } from './../records';

export function getAuthStore() {
  return reactor.evaluate(['tlp_settings_auth'])
}

export default Store({

  getInitialState() {
    return new StoreRec()
  },

  initialize() {
    this.on(AT.UPDATE_CONNECTORS, (state, items) => state.upsertItems(items) );
    this.on(AT.RECEIVE_CONNECTORS, (state, items) => state.setItems(items) );
    this.on(AT.SET_CURRENT, (state, item) => state.setCurItem(item))
    this.on(AT.DELETE_CONN, (state, id) => state.remove(id));
  }
})

