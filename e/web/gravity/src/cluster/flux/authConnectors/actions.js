import reactor from 'gravity/reactor';
import { ResourceEnum } from 'gravity/services/enums';
import * as resApi from 'gravity/services/resources';
import Logger from 'shared/libs/logger';
import * as AT from './actionTypes';

const logger = Logger.create('flux/settingsAuth/actions');

export function fetchAuthProviders(){
  return resApi.getAuthProviders().done(json => {
    reactor.dispatch(AT.RECEIVE_CONNECTORS, json);
  })
}

export function saveAuthProvider(yaml, isNew) {
  return resApi.upsert(ResourceEnum.AUTH_CONNECTORS, yaml, isNew)
    .done(items => {
      reactor.dispatch(AT.UPDATE_CONNECTORS, items);
    })
    .fail(err => {
      logger.error('saveAuthProvider()', err);
    });
}

export function deleteAuthProvider(authRec) {
  const { name, id, kind } = authRec;
  return resApi.remove(kind, name)
    .done(()=>{
      reactor.dispatch(AT.DELETE_CONN, id);
    })
    .fail(err => {
      logger.error('deleteAuthProvider()', err);
    });
}
