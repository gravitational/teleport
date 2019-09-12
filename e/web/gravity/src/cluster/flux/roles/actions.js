import reactor from 'gravity/reactor';
import { ResourceEnum } from 'gravity/services/enums';
import * as resApi from 'gravity/services/resources';
import Logger from 'shared/libs/logger';
import * as AT from './actionTypes';

const logger = Logger.create('flux/roles/actions');

export function saveRole(yaml, isNew) {
  const handleError = err => {
    logger.error('saveRole()', err);
  };

  const updateStore = items => {
    reactor.dispatch(AT.UPSERT_ROLES, items);
  };

  return resApi
    .upsert(ResourceEnum.ROLE, yaml, isNew)
    .done(updateStore)
    .fail(handleError);
}

export function deleteRole(roleRec) {
  const { name, id } = roleRec;
  const updateStore = () => {
    reactor.dispatch(AT.DELETE_ROLE, id);
  };

  return resApi
    .remove(ResourceEnum.ROLE, name)
    .then(updateStore)
    .fail(err => {
      logger.error('deleteRole()', err);
    });
}

export function fetchRoles() {
  return resApi.getRoles().done(items => {
    reactor.dispatch(AT.RECEIVE_ROLES, items);
  });
}
