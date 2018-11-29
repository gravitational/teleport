import reactor from '../../reactor';
import { Store } from 'nuclear-js';
import * as AT from './actionTypes';

import { StoreRec } from './../records';

export function getClusterStore() {
  return reactor.evaluate(['tlp_settings_cluster'])
}

export default Store({

  getInitialState() {
    return new StoreRec()
  },

  initialize() {
    this.on(AT.UPDATE_CLUSTERS, (state, items) => state.upsertItems(items) );
    this.on(AT.RECEIVE_CLUSTERS, (state, items) => state.setItems(items) );
    this.on(AT.SET_CURRENT, (state, item) => state.setCurItem(item));
    this.on(AT.DELETE_CLUSTER, (state, id) => state.remove(id));
  }
})

