import $ from 'jQuery';
import api from 'app/services/api';
import cfg from 'app/config';
import reactor from 'app/reactor';
import { TRYING_TO_DELETE_AUTH_PROVIDER } from 'app/flux/restApi/constants';
import restApiActions from 'telebase-app/flux/restApi/actions';
import { showError, showSuccess } from 'telebase-app/flux/notifications/actions';

import {
  SETTINGS_AUTH_CONN_NEW,
  SETTINGS_AUTH_CONN_SET_TO_DELETE,
  SETTINGS_AUTH_CONN_CANCEL_NEW,
  SETTINGS_AUTH_CONN_RECEIVE,
  SETTINGS_AUTH_CONN_CLEAR
} from './actionTypes';

const actions = {

  clear() {
    reactor.dispatch(SETTINGS_AUTH_CONN_CLEAR);
  },

  addNew() {
    reactor.dispatch(SETTINGS_AUTH_CONN_NEW);    
  },   

  cancelNew() {
    reactor.dispatch(SETTINGS_AUTH_CONN_CANCEL_NEW);
  },

  fetchConnectors() {
    return api.get(cfg.getOicdConnectorsPath()).done(json => {      
      reactor.dispatch(SETTINGS_AUTH_CONN_RECEIVE, json);
    })    
  },

  save(connector) {
    let dfd = $.Deferred();
    let url = cfg.getOicdConnectorsPath();
    if (connector.isNew) {
      dfd = api.post(url, connector); 
    } else {
      dfd = api.put(url, connector);
    }

    let { id } = connector;    
    return dfd
      .then(() => actions.fetchConnectors())
      .done(() => {
        showSuccess(`Connector ${id} has been saved`, '');
      })
      .fail(err => {
        let msg = api.getErrorText(err);                
        showError(msg, 'Failed to save a connector');        
      })      
  },

  deleteConnector(connectorId) {
    restApiActions.start(TRYING_TO_DELETE_AUTH_PROVIDER);    
    api.delete(cfg.getOicdConnectorsPath(connectorId))
      .then(() =>  actions.fetchConnectors() )
      .done(() => {
        restApiActions.success(TRYING_TO_DELETE_AUTH_PROVIDER);
        actions.closeDeleteConnectorDialog();
      })
      .fail(err => {
        let msg = api.getErrorText(err);
        restApiActions.fail(TRYING_TO_DELETE_AUTH_PROVIDER);        
        showError(msg, 'Failed to delete a connector');
      });        
  },
  
  openDeleteConnectorDialog(connectorId){
    reactor.dispatch(SETTINGS_AUTH_CONN_SET_TO_DELETE, connectorId);
  },

  closeDeleteConnectorDialog(){
    reactor.dispatch(SETTINGS_AUTH_CONN_SET_TO_DELETE, null);
  }
}

export default actions;