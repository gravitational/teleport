import Logger from 'shared/libs/logger';
import { Activator } from 'gravity/lib/featureBase';
import { fetchUserContext } from 'gravity/flux/user/actions';
import * as featureFlags from 'gravity/cluster/featureFlags';
import service, { applyConfig } from 'gravity/services/clusters';
import cfg from 'gravity/config';
import { setClusters, updateClusters } from './clusters/actions';
import { setCluster } from 'gravity/flux/cluster/actions';

const logger = Logger.create('hub/flux/actions');

export function initHub(features) {
  const siteId = cfg.getLocalSiteId();
  cfg.setDefaultSiteId(siteId);
  return fetchUserContext()
    .then(() => init(features))
    .fail(err => {
      logger.error('initHub()', err);
    });
}

function init(features){
  return service.fetchCluster({ shallow: false })
    .then(cluster => {
      setCluster(cluster);
      // Apply cluster web config settings
      applyConfig(cluster);
      // Init features
      const activator = new Activator(features);
      activator.onload({ featureFlags });
    })
}

export function fetchClusters(){
  return service.fetchClusters({shallow: false}).then(clusters => {
    setClusters(clusters);
  })
}

export function refreshClusters(){
  return service.fetchClusters().then(clusters => {
    updateClusters(clusters);
  })
}

export function unlinkCluster(siteId){
  // unlink the cluster and then re-fetch to update
  return service.unlink(siteId).then(() => fetchClusters());
}