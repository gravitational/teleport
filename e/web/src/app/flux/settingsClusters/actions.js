import { ResourceEnum } from 'app/services/enums';
import Logger from 'telebase-app/lib/logger';
import apiActions from 'app/flux/restApi/actions';
import reactor from 'app/reactor';
import api from 'app/services/api';
import * as resApi from 'app/services/resources';
import { checkResourceKind } from './../utils';
import { closeDeleteDialog } from '../settings/actions';
import * as RAT from 'app/flux/restApi/constants';
import * as AT from './actionTypes';

const logger = Logger.create('flux/settingsCluster/actions');

export function setCurCluster(item) {      
  reactor.batch(() => {  
    apiActions.clear(RAT.TRYING_TO_SAVE_CLUSTER);
    reactor.dispatch(AT.SET_CURRENT, item);
  });
}
    
export function saveCluster(cluster) {    
  const handleError = err => {
    const msg = api.getErrorText(err);
    logger.error('saveCluster()', err);        
    apiActions.fail(RAT.TRYING_TO_SAVE_CLUSTER, msg);            
  }

  try {
    const yaml = cluster.getContent();
    apiActions.start(RAT.TRYING_TO_SAVE_CLUSTER);            
    checkResourceKind([ResourceEnum.TRUSTED_CLUSTER], yaml);  
    return resApi.upsert(yaml)            
      .done( items => {
        reactor.dispatch(AT.UPDATE_CLUSTERS, items);
        setCurCluster(items[0].name);        
        apiActions.success(RAT.TRYING_TO_SAVE_CLUSTER);            
      })
      .fail(handleError);
  }    
  catch(err){
    handleError(err);
  }
}

export function deleteCluster(id) {  
  apiActions.start(RAT.TRYING_TO_DELETE_RESOURCE);      
  resApi.remove(ResourceEnum.TRUSTED_CLUSTER, id)  
    .then(fetchTrustedClusters)
    .done(() => {      
      saveCluster(null)
      closeDeleteDialog();      
      apiActions.success(RAT.TRYING_TO_DELETE_RESOURCE);
    })
    .fail(err => {
      const msg = api.getErrorText(err);
      logger.error('deleteCluster()', err);
      apiActions.fail(RAT.TRYING_TO_DELETE_RESOURCE, msg);              
    });        
}

export function fetchTrustedClusters() {                    
  return resApi.getTrustedClusters().done(items => {
    reactor.dispatch(AT.RECEIVE_CLUSTERS, items);
  })    
}
