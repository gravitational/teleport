import Logger from 'telebase-app/lib/logger';

import { ResourceEnum } from '../../services/enums';
import { saveClusterStatus, deleteResourceStatus } from '../../flux/status/actions';
import reactor from '../../reactor';
import * as backend from '../../services/backend';
import { closeDeleteDialog } from './../settingsDialogs/actions';
import { getClusterStore } from './store';
import * as AT from './actionTypes';

const logger = Logger.create('flux/settingsCluster/actions');
const TRUSTED_CLUSTER = ResourceEnum.TRUSTED_CLUSTER;

export function setCurCluster(item) {
  reactor.batch(() => {
    saveClusterStatus.clear();
    reactor.dispatch(AT.SET_CURRENT, item);
  });
}

export function saveCluster(cluster) {
  const handleError = err => {
    const msg = backend.getErrorText(err);
    logger.error('saveCluster()', err);
    saveClusterStatus.fail(msg);
  }

  const updateStore = items => reactor.batch(()=> {
    reactor.dispatch(AT.UPDATE_CLUSTERS, items);
    setCurCluster(items[0].id);
    saveClusterStatus.success();
  })

  try {
    const yaml = cluster.getContent();
    saveClusterStatus.start();
    return backend.upsert(TRUSTED_CLUSTER, yaml, cluster.getIsNew())
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
    deleteResourceStatus.success();
  });

  deleteResourceStatus.start();
  backend.remove(TRUSTED_CLUSTER, name)
    .done(updateStore)
    .fail(err => {
      const msg = backend.getErrorText(err);
      logger.error('deleteCluster()', err);
      deleteResourceStatus.fail(msg);
    });
}

export function fetchTrustedClusters() {
  return backend.getTrustedClusters().done(items => {
    reactor.dispatch(AT.RECEIVE_CLUSTERS, items);
  })
}
