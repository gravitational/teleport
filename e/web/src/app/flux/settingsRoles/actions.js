import reactor from 'app/reactor';
import api from 'app/services/api';
import { TRYING_TO_SAVE_ROLE, TRYING_TO_DELETE_RESOURCE } from 'app/flux/restApi/constants';
import { ResourceEnum } from 'app/services/enums';
import * as resApi from 'app/services/resources';
import Logger from 'telebase-app/lib/logger';
import apiActions from 'app/flux/restApi/actions';
import * as AT from './actionTypes';
import { closeDeleteDialog } from '../settings/actions';
import { checkResourceKind } from './../utils';

const logger = Logger.create('flux/settingsCluster/actions');

export function setCurRole(item) {    
  reactor.batch(() => {  
    apiActions.clear(TRYING_TO_SAVE_ROLE);
    reactor.dispatch(AT.SET_CURRENT, item)
  });
}

export function saveRole(rolRec) {      
  const handleError = err => {
    const msg = api.getErrorText(err);
    logger.error('saveRole()', err);        
    apiActions.fail(TRYING_TO_SAVE_ROLE, msg);            
  }

  try {
    const yaml = rolRec.getContent();  
    checkResourceKind([ResourceEnum.ROLE], yaml);  
    apiActions.start(TRYING_TO_SAVE_ROLE);                          
    return resApi.upsert(yaml)      
      .done( items => {            
        reactor.dispatch(AT.UPSERT_ROLES, items);
        setCurRole(items[0].name);        
        apiActions.success(TRYING_TO_SAVE_ROLE);      
      })
      .fail(handleError)
  }    
  catch(err){
    handleError(err);
  }
}
    
export function deleteRole(id) {  
  apiActions.start(TRYING_TO_DELETE_RESOURCE);      
  resApi.remove(ResourceEnum.ROLE, id)  
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
  return resApi.getRoles().done(items => {
    reactor.dispatch(AT.RECEIVE_ROLES, items);
  })    
}
