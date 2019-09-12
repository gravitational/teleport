import reactor from 'gravity/reactor';
import service from 'gravity/services/applications';
import * as actionTypes from './actionTypes';

export function fetchApps() {
  return service.fetchApplications().then(apps => {
    reactor.dispatch(actionTypes.CATALOG_RECEIVE_APPS, apps);
  })
}

