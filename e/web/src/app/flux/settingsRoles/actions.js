import { ResourceEnum } from 'app/services/enums';
import Logger from 'telebase-app/lib/logger';
import reactor from 'app/reactor';
import api from 'app/services/api';
import cfg from 'app/config';
import apiActions from 'app/flux/restApi/actions';
import { TRYING_TO_SAVE_ROLE, TRYING_TO_DELETE_RESOURCE } from 'app/flux/restApi/constants';
import * as AT from './actionTypes';
import { closeDeleteDialog } from '../settings/actions';
import {getStore} from './store';

const logger = Logger.create('flux/settingsCluster/actions');

export function setCurRole(item) {    
  reactor.batch(() => {  
    apiActions.clear(TRYING_TO_SAVE_ROLE);
    reactor.dispatch(AT.SET_CURRENT, item)
  });
}

export function saveRole(item) {    
  apiActions.start(TRYING_TO_SAVE_ROLE);            
  return api.put(cfg.getResourcesUrl(ResourceEnum.ROLE), item)      
    .then( res => res.items)
    .done( items => {            
      reactor.dispatch(AT.UPSERT_ROLES, items);
      setCurRole(items[0].name);        
      apiActions.success(TRYING_TO_SAVE_ROLE);      
    })
    .fail(err => {
      logger.error('saveRole()', err);
      const msg = api.getErrorText(err);       
      apiActions.fail(TRYING_TO_SAVE_ROLE, msg);
  })
}
    
export function deleteRole(id) {  
  apiActions.start(TRYING_TO_DELETE_RESOURCE);    
  const item = getStore().findItem(id);
  api.delete(cfg.getResourcesUrl(ResourceEnum.ROLE, id), item)      
    .then(fetchRoles)
    .done(() => {      
      setCurRole(null)
      closeDeleteDialog();      
      apiActions.success(TRYING_TO_DELETE_RESOURCE);
    })
    .fail(err => {
      const msg = api.getErrorText(err);
      logger.error('deleteRole()', err);
      apiActions.fail(TRYING_TO_DELETE_RESOURCE, msg);              
    });        
}

export function fetchRoles() {                    
  return api.get(cfg.getResourcesUrl(ResourceEnum.ROLE))
  .then(res => { return res.items || [] })
  .done(items => {
      reactor.dispatch(AT.RECEIVE_ROLES, items);
  })    
}

