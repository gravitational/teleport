import Logger from 'telebase-app/lib/logger';
import reactor from '../../reactor';
import { ResourceEnum } from '../../services/enums';
import * as backend from '../../services/backend';
import { closeDeleteDialog } from './../settingsDialogs/actions';
import { getAuthStore } from './store';
import * as AT from './actionTypes';
import  { saveAuthProviderStatus, deleteResourceStatus } from '../../flux/status/actions';

const logger = Logger.create('flux/settingsAuth/actions');

export function setCurProvider(item) {
  reactor.batch(() => {
    saveAuthProviderStatus.clear();
    reactor.dispatch(AT.SET_CURRENT, item)
  });
}

export function fetchAuthProviders(){
  return backend.getAuthProviders().done(items => {
    reactor.dispatch(AT.RECEIVE_CONNECTORS, items);
  })
}

export function saveAuthProvider(authProvider) {
  const handleError = err => {
    const msg = backend.getErrorText(err);
    logger.error('saveAuthProvider()', err);
    saveAuthProviderStatus.fail(msg);
  }

  const updateStore = items => reactor.batch( ()=> {
    reactor.dispatch(AT.UPDATE_CONNECTORS, items);
    setCurProvider(items[0].id);
    saveAuthProviderStatus.success();
  })

  try {
    const yaml = authProvider.getContent();
    saveAuthProviderStatus.start();
    return backend.upsert(ResourceEnum.AUTH_CONNECTORS, yaml, authProvider.getIsNew())
      .done(updateStore)
      .fail(handleError);
  }catch(err){
    handleError(err)
  }
}

export function deleteAuthProvider(authRec) {
  const { name, id, kind } = authRec;

  const updateStore = () => reactor.batch(()=> {
    const next = getAuthStore().getNext(id);
    closeDeleteDialog();
    reactor.dispatch(AT.DELETE_CONN, id);
    setCurProvider(next);
    deleteResourceStatus.success();
  });

  deleteResourceStatus.start();
  backend.remove(kind, name)
    .done(updateStore)
    .fail(err => {
      const msg = backend.getErrorText(err);
      logger.error('deleteAuthProvider()', err);
      deleteResourceStatus.fail(msg);
    });
}
