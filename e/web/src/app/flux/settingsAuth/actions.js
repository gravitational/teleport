import $ from 'jQuery';
import reactor from 'app/reactor';
import { ResourceEnum } from 'app/services/enums';
import * as resApi from 'app/services/resources';
import api from 'app/services/api';
import * as RAT from 'app/flux/restApi/constants';
import Logger from 'telebase-app/lib/logger';
import apiActions from 'telebase-app/flux/restApi/actions';
import { closeDeleteDialog } from '../settings/actions';
import { checkResourceKind } from './../utils';
import * as AT from './actionTypes';

const logger = Logger.create('flux/settingsAuth/actions');
      
export function setCurProvider(item) {    
  reactor.batch(() => {      
    apiActions.clear(RAT.TRYING_TO_SAVE_AUTH_PROVIDER);
    reactor.dispatch(AT.SET_CURRENT, item)
  });
}

export function fetchAuthProviders(){  
  const dfdOidc = $.Deferred();
  const dfdSaml = $.Deferred();    

  let receivedItems = [];
  let errorMessages = [];

  const addToErrors = err => {
    const text = api.getErrorText(err);
    errorMessages.push(text);     
  }

  const addToReceived = items => {
    receivedItems = receivedItems.concat(items);
  }

  resApi.getOidc()
    .done(addToReceived)
    .fail(addToErrors)
    .always( () => { dfdOidc.resolve(); });

  resApi.getSaml()
    .done(addToReceived)
    .fail(addToErrors)
    .always( () => { dfdSaml.resolve(); })

  return $.when(dfdOidc, dfdSaml).done(()=> {        
    reactor.dispatch(AT.ADD_ERROR, errorMessages)
    reactor.dispatch(AT.RECEIVE_CONNECTORS, receivedItems);
  })
}

export function saveAuthProvider(authProvider) {      
  const handleError = err => {
    const msg = api.getErrorText(err);
    logger.error('saveAuthProvider()', err);        
    apiActions.fail(RAT.TRYING_TO_SAVE_AUTH_PROVIDER, msg);            
  }

  try {
    const yaml = authProvider.getContent();
    apiActions.start(RAT.TRYING_TO_SAVE_AUTH_PROVIDER);            
    checkResourceKind([ResourceEnum.OIDC, ResourceEnum.SAML], yaml);    
    return resApi.upsert(yaml)
      .done( items => {  
        reactor.dispatch(AT.UPDATE_CONNECTORS, items);
        setCurProvider(items[0].name);
        apiActions.success(RAT.TRYING_TO_SAVE_AUTH_PROVIDER);            
      })
      .fail(handleError);
  }catch(err){
    handleError(err)    
  }  
}

export function deleteAuthProvider(id) {  
  apiActions.start(RAT.TRYING_TO_DELETE_RESOURCE);      
  resApi.remove(ResourceEnum.OIDC, id )  
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
