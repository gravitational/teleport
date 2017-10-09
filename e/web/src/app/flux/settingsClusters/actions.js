import { ResourceEnum } from 'app/services/enums';
import Logger from 'telebase-app/lib/logger';
import apiActions from 'app/flux/restApi/actions';
import reactor from 'app/reactor';
import api from 'app/services/api';
import * as resApi from 'app/services/resources';
import { closeDeleteDialog } from '../settings/actions';
import { getClusterStore } from './store';
import * as RAT from 'app/flux/restApi/constants';
import * as AT from './actionTypes';

const logger = Logger.create('flux/settingsCluster/actions');
const TRUSTED_CLUSTER = ResourceEnum.TRUSTED_CLUSTER;

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

  const updateStore = items => reactor.batch(()=> {
    reactor.dispatch(AT.UPDATE_CLUSTERS, items);
    setCurCluster(items[0].id);        
    apiActions.success(RAT.TRYING_TO_SAVE_CLUSTER);            
  })

  try {
    const yaml = cluster.getContent();
    apiActions.start(RAT.TRYING_TO_SAVE_CLUSTER);                  
    return resApi.upsert(TRUSTED_CLUSTER, yaml, cluster.getIsNew())            
      .done(updateStore)
      .fail(handleError);
  }    
  catch(err){
    handleError(err);
  }
}

export function deleteCluster(clusterRec) { 
  const { name, id } = clusterRec;
  
  const updateStore = () => reactor.batch(()=> {
    const next = getClusterStore().getNext(id);      
    closeDeleteDialog();      
    reactor.dispatch(AT.DELETE_CLUSTER, id);
    setCurCluster(next);
    apiActions.success(RAT.TRYING_TO_DELETE_RESOURCE);
  });
  
  apiActions.start(RAT.TRYING_TO_DELETE_RESOURCE);      
  resApi.remove(TRUSTED_CLUSTER, name)      
    .done(updateStore)
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
