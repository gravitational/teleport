import { Store } from 'nuclear-js';
import { Record } from 'immutable';
import { keyBy } from 'lodash';
import * as actionTypes from './actionTypes';

const CatalogStoreRec = Record({
  apps: {},
});

export default Store({
  getInitialState() {
    return new CatalogStoreRec();
  },

  initialize() {
    this.on(actionTypes.CATALOG_RECEIVE_APPS, receiveApps);
  }
})

function receiveApps(state, json) {
  const apps = keyBy(json, 'id');
  return state.set('apps', apps);
}