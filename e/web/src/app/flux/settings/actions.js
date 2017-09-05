import reactor from 'app/reactor';
import getters from './getters';
import * as AT from './actionTypes';
import * as RAT from './../restApi/constants';
import apiActions from './../restApi/actions';

export function addNavItem(navItem){
  reactor.dispatch(AT.ADD_NAV_ITEM, navItem)
}

export function initSettings(featureActivator) {                    
  // init only once
  let store = reactor.evaluate(getters.store)
  if (store.isReady()){
    return;
  }
  
  featureActivator.onload();         
  reactor.dispatch(AT.INIT, {});            
  apiActions.success(RAT.TRYING_TO_INIT_SETTINGS);                  
}         

export function openDeleteDialog(item){
  reactor.dispatch(AT.SET_RES_TO_DELETE, item);
}

export function closeDeleteDialog(){
  apiActions.clear(RAT.TRYING_TO_DELETE_RESOURCE);
  reactor.dispatch(AT.SET_RES_TO_DELETE, null);
}