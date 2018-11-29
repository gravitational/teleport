import Logger from 'telebase-app/lib/logger';
import reactor from './../../reactor';
import * as resApi from '../../services/backend';
import * as AT from './actionTypes';
import $ from 'jQuery';

const logger = Logger.create('flux/license/actions');

export function fetchLicenseStatus(){
  const fetchPromise = $.Deferred();
  resApi.fetchLicenseStatus()
    .done(json => {
      if(json){
        reactor.dispatch(AT.RECEIVE_STATUS, json);
      }
    })
    .fail(err => {
      logger.error('fetchLicenseStatus()', err);
    })
    .always(()=> {
      fetchPromise.resolve();
    });

  return fetchPromise;
}
