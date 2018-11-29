import Logger from 'telebase-app/lib/logger';
import reactor from '../../reactor';
import { saveRoleStatus, deleteResourceStatus } from '../../flux/status/actions';

import { ResourceEnum } from '../../services/enums';
import * as backend from '../../services/backend';
import * as AT from './actionTypes';
import { closeDeleteDialog } from './../settingsDialogs/actions';
import { getRoleStore } from './store';

const logger = Logger.create('flux/settingsCluster/actions');

export function setCurRole(item) {
  reactor.batch(() => {
    saveRoleStatus.clear();
    reactor.dispatch(AT.SET_CURRENT, item)
  });
}

export function saveRole(rolRec) {
  const handleError = err => {
    const msg = backend.getErrorText(err);
    logger.error('saveRole()', err);
    saveRoleStatus.fail(msg);
  }

  const updateStore = items => reactor.batch(()=> {
    reactor.dispatch(AT.UPSERT_ROLES, items);
    setCurRole(items[0].id);
    saveRoleStatus.success();
  })

  try {
    const yaml = rolRec.getContent();
    saveRoleStatus.start();
    return backend.upsert(ResourceEnum.ROLE, yaml, rolRec.getIsNew())
      .done(updateStore)
      .fail(handleError)
  }
  catch(err){
    handleError(err);
  }
}

export function deleteRole(rolRec) {
  const { name, id } = rolRec;

  const updateStore = () => {
    reactor.batch(()=>{
      const next = getRoleStore().getNext(id);
      closeDeleteDialog();
      reactor.dispatch(AT.DELETE_ROLE, id);
      deleteResourceStatus.success();
      setCurRole(next);
    })
  }

  deleteResourceStatus.start();
  backend.remove(ResourceEnum.ROLE, name)
    .done(updateStore)
    .fail(err => {
      const msg = backend.getErrorText(err);
      logger.error('deleteRole()', err);
      deleteResourceStatus.fail(msg);
    });
}

export function fetchRoles() {
  return backend.getRoles().done(items => {
    reactor.dispatch(AT.RECEIVE_ROLES, items);
  })
}
