import $ from 'jQuery';
import api from 'app/services/api';
import Logger from 'telebase-app/lib/logger';
import { ResourceEnum } from 'app/services/enums';
import cfg from 'app/config';
import { getStore } from './authStore';
import reactor from 'app/reactor';
import { closeDeleteDialog } from '../settings/actions';
import * as RAT from 'app/flux/restApi/constants';
import apiActions from 'telebase-app/flux/restApi/actions';
import * as AT from './actionTypes';
const logger = Logger.create('flux/settingsAuth/actions');
      
export function setCurProvider(item) {    
  reactor.batch(() => {      
    apiActions.clear(RAT.TRYING_TO_SAVE_AUTH_PROVIDER);
    reactor.dispatch(AT.SET_CURRENT, item)
  });
}

export function fetchAuthProviders(){
  return $.when(
    api.get(cfg.getResourcesUrl(ResourceEnum.OIDC)),
    api.get(cfg.getResourcesUrl(ResourceEnum.SAML)))
    .then((res1, res2) => {        
      return [...res1[0].items, ...res2[0].items]
    })
    .done(items => {
      reactor.dispatch(AT.RECEIVE_CONNECTORS, items);
    });      
}

export function saveAuthProvider(authProvider) {
  apiActions.start(RAT.TRYING_TO_SAVE_AUTH_PROVIDER);            
  return api.put(cfg.getResourcesUrl(ResourceEnum.OIDC), authProvider)          
    .then( res => res.items)
    .done( items =>{  
      reactor.dispatch(AT.UPDATE_CONNECTORS, items);
      setCurProvider(items[0].name);
      apiActions.success(RAT.TRYING_TO_SAVE_AUTH_PROVIDER);            
    })
    .fail(err => {
      const msg = api.getErrorText(err);
      logger.error('saveAuthProvider()', err);        
      apiActions.fail(RAT.TRYING_TO_SAVE_AUTH_PROVIDER, msg);            
  })
}

export function deleteAuthProvider(id) {  
  apiActions.start(RAT.TRYING_TO_DELETE_RESOURCE);    
  const item = getStore().findItem(id);
  api.delete(cfg.getResourcesUrl(ResourceEnum.OIDC, id), item)      
    .then(fetchAuthProviders)
    .done(() => {      
      setCurProvider(null)
      closeDeleteDialog();      
      apiActions.success(RAT.TRYING_TO_DELETE_RESOURCE);
    })
    .fail(err => {
      const msg = api.getErrorText(err);
      logger.error('deleteAuthProvider()', err);
      apiActions.fail(RAT.TRYING_TO_DELETE_RESOURCE, msg);              
    });        
}
