import reactor from 'app/reactor';
import { Store } from 'nuclear-js';
import * as AT from './actionTypes';

import { StoreRec } from './../records';

export function getStore() {
  return reactor.evaluate(['tlp_settings_role'])
}

export default Store({

  getInitialState() {
    return new StoreRec()
  },

  initialize() {      
    this.on(AT.UPSERT_ROLES, (state, items) => state.upsertItems(items) );            
    this.on(AT.RECEIVE_ROLES, (state, items) => state.setItems(items) );            
    this.on(AT.SET_TO_DELETE, (state, id) => state.setItemToDelete(id) );            
    this.on(AT.SET_CURRENT, (state, item) => state.setCurItem(item))
  }
})

