import { ResourceEnum } from 'app/services/enums';
import Logger from 'telebase-app/lib/logger';
import apiActions from 'app/flux/restApi/actions';
import reactor from 'app/reactor';
import api from 'app/services/api';
import cfg from 'app/config';

import { closeDeleteDialog } from '../settings/actions';
import * as RAT from 'app/flux/restApi/constants';
import * as AT from './actionTypes';
import {getStore} from './store';

const logger = Logger.create('flux/settingsCluster/actions');

export function setCurCluster(item) {      
  reactor.batch(() => {  
    apiActions.clear(RAT.TRYING_TO_SAVE_CLUSTER);
    reactor.dispatch(AT.SET_CURRENT, item);
  });
}

export function saveCluster(cluster) {    
  apiActions.start(RAT.TRYING_TO_SAVE_CLUSTER);            
  return api.put(cfg.getResourcesUrl(ResourceEnum.TRUSTED_CLUSTER), cluster)      
    .then( res => res.items)
    .done( items => {
      reactor.dispatch(AT.UPDATE_CLUSTERS, items);
      setCurCluster(items[0].name);        
      apiActions.success(RAT.TRYING_TO_SAVE_CLUSTER);            
    })
    .fail(err => {
      const msg = api.getErrorText(err);
      logger.error('saveCluster()', err);        
      apiActions.fail(RAT.TRYING_TO_SAVE_CLUSTER, msg);            
  })
}

export function deleteCluster(id) {  
  apiActions.start(RAT.TRYING_TO_DELETE_RESOURCE);    
  const item = getStore().findItem(id);
  api.delete(cfg.getResourcesUrl(ResourceEnum.TRUSTED_CLUSTER, id), item)      
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
  return api.get(cfg.getResourcesUrl(ResourceEnum.TRUSTED_CLUSTER))
  .then(res => { return res.items || [] })
  .done(items => {
      reactor.dispatch(AT.RECEIVE_CLUSTERS, items);
  })    
}



