import reactor from 'app/reactor';
import { ResourceEnum } from 'app/services/enums';
import * as resApi from 'app/services/resources';
import api from 'app/services/api';
import * as RAT from 'app/flux/restApi/constants';
import Logger from 'telebase-app/lib/logger';
import apiActions from 'telebase-app/flux/restApi/actions';
import { closeDeleteDialog } from '../settings/actions';
import { getAuthStore } from './store';
import * as AT from './actionTypes';

const logger = Logger.create('flux/settingsAuth/actions');
      
export function setCurProvider(item) {    
  reactor.batch(() => {      
    apiActions.clear(RAT.TRYING_TO_SAVE_AUTH_PROVIDER);
    reactor.dispatch(AT.SET_CURRENT, item)
  });
}

export function fetchAuthProviders(){    
  return resApi.getAuthProviders().done(items => {            
    reactor.dispatch(AT.RECEIVE_CONNECTORS, items);
  })
}

export function saveAuthProvider(authProvider) {      
  const handleError = err => {
    const msg = api.getErrorText(err);
    logger.error('saveAuthProvider()', err);        
    apiActions.fail(RAT.TRYING_TO_SAVE_AUTH_PROVIDER, msg);            
  }

  const updateStore = items => reactor.batch( ()=> {
    reactor.dispatch(AT.UPDATE_CONNECTORS, items);
    setCurProvider(items[0].name);
    apiActions.success(RAT.TRYING_TO_SAVE_AUTH_PROVIDER);            
  })

  try {
    const yaml = authProvider.getContent();
    apiActions.start(RAT.TRYING_TO_SAVE_AUTH_PROVIDER);                
    return resApi.upsert(ResourceEnum.AUTH_CONNECTORS, yaml, authProvider.getIsNew())
      .done(updateStore)
      .fail(handleError);
  }catch(err){
    handleError(err)    
  }  
}

export function deleteAuthProvider(id) {  
  const updateStore = () => reactor.batch(()=> {
    const next = getAuthStore().getNext(id);      
    closeDeleteDialog();      
    reactor.dispatch(AT.DELETE_CONN, id);
    setCurProvider(next);
    apiActions.success(RAT.TRYING_TO_DELETE_RESOURCE);
  });

  apiActions.start(RAT.TRYING_TO_DELETE_RESOURCE);      
  resApi.remove(ResourceEnum.OIDC, id )      
    .done(updateStore)
    .fail(err => {
      const msg = api.getErrorText(err);
      logger.error('deleteAuthProvider()', err);
      apiActions.fail(RAT.TRYING_TO_DELETE_RESOURCE, msg);              
    });        
}
