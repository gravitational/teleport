import reactor from 'gravity/reactor';
import * as actionTypes from './actionTypes';

export function addTopNavItem(item) {
  reactor.dispatch(actionTypes.NAV_ADD_TOP_ITEM, item);
}

export function addSettingNavItem(item) {
  reactor.dispatch(actionTypes.NAV_ADD_SETTING_ITEM, item);
}

export function addUserRoleNavItem(item) {
  reactor.dispatch(actionTypes.NAV_ADD_USERROLE_ITEM, item);
}

