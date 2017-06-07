import { Store, toImmutable } from 'nuclear-js';
import { Record, List } from 'immutable';
import { sortBy } from 'lodash';
import { UserRoleSystemNameEnum } from 'app/services/enums';
import { isHiddenRoleName } from './../utils';
import {  
  SETTINGS_ROLES_CLEAR,
  SETTINGS_ROLES_RECEIVE,
  SETTINGS_ROLES_NEW,    
  SETTINGS_ROLES_SET_CURRENT,  
  SETTINGS_ROLES_SET_ROLE_TO_DELETE
} from './actionTypes';

const ROLE_DEF_NAME = 'Untitled';
const ADMIN_ROLE_DISPLAY_NAME = "admin";

class UserRoleRec extends Record({
  key: null,
  system: false,
  isNew: false,
  isSaving: false,
  name: '',
  displayName: '',
  access: toImmutable({
    admin: {
      enabled: false
    },            
    ssh: {
      nodeLabels: {'*':'*'},      
      maxTtl: 108000000000000, /**30h */
      logins: []
    }
  })
}) {
  constructor(props) {         
    super(props);
    let displayName = this.name;    
    if (displayName === UserRoleSystemNameEnum.ADMIN) {
      displayName = ADMIN_ROLE_DISPLAY_NAME;
    }

    return this.set('displayName', displayName)
               .set('key', Math.random().toString());
  }
}

export default Store({
  getInitialState() {
    return toImmutable({
      siteId: null,
      roleToDelete: null,
      selectedRole: null,
      allRoles: []
    }); 
  },

  initialize() {          
    this.on(SETTINGS_ROLES_CLEAR, state => state.set('selectedRole', null));
    this.on(SETTINGS_ROLES_RECEIVE, receiveRoles);  
    this.on(SETTINGS_ROLES_SET_CURRENT, setCurrentRole);    
    this.on(SETTINGS_ROLES_NEW, handleNewRole);      
    this.on(SETTINGS_ROLES_SET_ROLE_TO_DELETE, setRoleToDelete);            
  }
})

function receiveRoles(state, json) {  
  json = json || [];
  json = sortBy(json, r => r.name.toLowerCase());
  json = json.filter(r => !isHiddenRoleName(r.name));
  let recList = json.reduce((recList, item) => {      
    return recList.push(new UserRoleRec(toImmutable(item)));
  }, new List());
        
  return state.set('allRoles', recList);    
}

function setRoleToDelete(state, roleName) {
  return state.set('roleToDelete', roleName);
}

function handleNewRole(state, enable) {
  if (enable) {
    let roleRec = new UserRoleRec({
      name: ROLE_DEF_NAME,
      isNew: true
    });
        
    return state.set('selectedRole', roleRec);          
  }
    
  // cancel new role
  return setCurrentRole(state);   
}

function setCurrentRole(state, roleName) {  
  // if no name is provided, set the first available 
  if (!roleName) {
    let firstRole = state.get('allRoles').first();
    if (firstRole) {
      return setCurrentRole(state, firstRole.name);  
    }else{
      return state.set('selectedRole', null);
    }            
  }
  
  let roleRec = state.get('allRoles').find( r => r.get('name') === roleName);    
  return state.set('selectedRole', roleRec);      
}
