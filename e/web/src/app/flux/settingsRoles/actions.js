import reactor from 'app/reactor';
import { showError, showSuccess } from 'telebase-app/flux/notifications/actions';
import { fetchAcl } from 'telebase-app/flux/userAcl/actions';

import { TRYING_TO_DELETE_ROLE } from './../restApi/constants';
import restApiActions from './../restApi/actions';
import api from 'app/services/api';
import cfg from 'app/config';

import {
  SETTINGS_ROLES_RECEIVE,
  SETTINGS_ROLES_SET_CURRENT,  
  SETTINGS_ROLES_NEW,
  SETTINGS_ROLES_CLEAR,  
  SETTINGS_ROLES_SET_ROLE_TO_DELETE
} from './actionTypes';

const actions =  {

  setSelectedRole(roleName) {
    reactor.batch(() => {      
      reactor.dispatch(SETTINGS_ROLES_SET_CURRENT, roleName)
    });        
  },   
      
  clear(){
    reactor.dispatch(SETTINGS_ROLES_CLEAR);
  },

  openDeleteRoleDialog(roleName){
    reactor.dispatch(SETTINGS_ROLES_SET_ROLE_TO_DELETE, roleName);
  },

  closeDeleteRoleDialog(){
    reactor.dispatch(SETTINGS_ROLES_SET_ROLE_TO_DELETE, null);
  },

  cancelNewRole() {
    reactor.dispatch(SETTINGS_ROLES_NEW, false);
  },

  newRole() {
    reactor.dispatch(SETTINGS_ROLES_NEW, true);  
  },
    
  fetchRoles() {                    
    return api.get(cfg.getRolesUrl()).done(json => {
      reactor.dispatch(SETTINGS_ROLES_RECEIVE, json)
    });      
  },
    
  deleteRole(roleName) {
    console.log(roleName);
    restApiActions.start(TRYING_TO_DELETE_ROLE);
    api.delete(cfg.getRolesUrl(roleName))
      .then(() => actions.fetchRoles())
      .done(() => {
        fetchAcl();
        actions.closeDeleteRoleDialog();
        actions.setSelectedRole();
        restApiActions.success(TRYING_TO_DELETE_ROLE);
        showSuccess(`role ${roleName} has been deleted`, '');
      })
      .fail(err => {
        let msg = api.getErrorText(err);                
        showError(msg, '');
        restApiActions.fail(TRYING_TO_DELETE_ROLE);
      })      
  },

  saveRole(role) {            
    let dfd = null;
    if(role.isNew){
      dfd = api.post(cfg.getRolesUrl(), role);
    }else{
      dfd = api.put(cfg.getRolesUrl(), role);
    }

    return dfd
      .then(() =>  actions.fetchRoles())
      .done(() => {        
        fetchAcl();        
        actions.setSelectedRole(role.name);                        
        showSuccess(`role ${role.name} has been saved`, '');
      })
      .fail(err => {
        let msg = api.getErrorText(err);                
        showError(msg, '');        
      });      
  }    
}

export default actions;