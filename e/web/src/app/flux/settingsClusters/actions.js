import Logger from 'telebase-app/lib/logger';
import { showError, showSuccess } from 'telebase-app/flux/notifications/actions';
import reactor from 'app/reactor';
import api from 'app/services/api';
import cfg from 'app/config';
import restApiActions from 'app/flux/restApi/actions';
import { TRYING_TO_SAVE_CLUSTER } from 'app/flux/restApi/constants';
import {  SETTINGS_CLUSTER_RECEIVE }  from './actionTypes';
const logger = Logger.create('flux/settingsCluster/actions');

const actions = {
  
  saveCluster(cluster) {    
    restApiActions.start(TRYING_TO_SAVE_CLUSTER);            
    return api.put(cfg.getClusterUrl(), cluster)      
      .done(()=>{
        actions.fetchTrustedClusters();
        showSuccess(`cluster ${cluster.name} has been saved`, '');
      })
      .fail(err => {
        let msg = api.getErrorText(err);
        logger.error('saveCluster()', err);
        showError(msg, '');      
    })
  },
    
  fetchTrustedClusters() {                    
    return api.get(cfg.getClusterUrl())
      .done(json => {
        reactor.dispatch(SETTINGS_CLUSTER_RECEIVE, json);
      })
      .fail(err => {
        let msg = api.getErrorText(err);
        logger.error('fetchClusters()', err);
        showError('Failed to fetch account users', msg);
      });
  }  
}

export default actions;
