import { Store } from 'nuclear-js';
import { Record } from 'immutable';
import * as actionTypes from './actionTypes';

const NavStoreRec = Record({
  settings: [],
  topNav: [],
  userRole: []
});

export default Store({
  getInitialState() {
    return new NavStoreRec();
  },

  initialize() {
    this.on(actionTypes.NAV_ADD_TOP_ITEM, addTopItem);
    this.on(actionTypes.NAV_ADD_SETTING_ITEM, addSettingItem);
    this.on(actionTypes.NAV_ADD_USERROLE_ITEM, addUserRoleItem);
  }
})

function addTopItem(state, item) {
  const items = [...state.topNav, item];
  return state.set('topNav', items);
}

function addSettingItem(state, item) {
  const items = [...state.settings, item];
  return state.set('settings', items);
}


function addUserRoleItem(state, item) {
  const items = [...state.userRole, item];
  return state.set('userRole', items);
}