import { Store, toImmutable } from 'nuclear-js';
import { Record, List } from 'immutable';

import {
  SETTINGS_AUTH_CONN_NEW,
  SETTINGS_AUTH_CONN_CANCEL_NEW,
  SETTINGS_AUTH_CONN_SET_TO_DELETE,
  SETTINGS_AUTH_CONN_RECEIVE,
  SETTINGS_AUTH_CONN_CLEAR
} from './actionTypes';

const mappingSample = [
  {
    "claim": "claim",
    "value": "claim_value",
    "roles": [
      "role_name"
    ]
  }
]

class OIDConnectorRec extends Record({
  key: null,
  id: '',
  displayName: '',
  isNew: false,
  isSaving: false,
  clientId: '',
  clientSecret: '',
  issuerUrl: '',
  redirectUrl: '',
  scope: [],
  roleMapping: null
}) {
  constructor({ scope, ...props }) {
    let key = Math.random().toString();
    scope = scope || [];
    super({
      key,
      scope, 
      ...props
    });    
  }
}

export default Store({

  getInitialState() {
    return toImmutable({      
      connectors: [],
      connectorToDelete: null
    });
  },

  initialize() {
    this.on(SETTINGS_AUTH_CONN_CLEAR, clear);    
    this.on(SETTINGS_AUTH_CONN_RECEIVE, receive);
    this.on(SETTINGS_AUTH_CONN_NEW, createNew);
    this.on(SETTINGS_AUTH_CONN_CANCEL_NEW, cancelNew);
    this.on(SETTINGS_AUTH_CONN_SET_TO_DELETE, setToDelete);    
  }
})

function receive(state, json) {
  json = json || [];
  let connectorList = new List(json.map(
    i => new OIDConnectorRec(i)));
  
  return state.setIn(['connectors'], connectorList); 
}

function setToDelete(state, connectorId) {
  return state.set('connectorToDelete', connectorId);
}

function createNew(state) {
  let newRec = new OIDConnectorRec({
    isNew: true,  
    scope: ['scope1', 'scope2'],
    roleMapping: mappingSample
  });  

  return state.updateIn(['connectors'], provList => provList.unshift(newRec))
}

function cancelNew(state) {  
  let allConnectors = state.get('connectors').filter( item => !item.get('isNew'))
  return state.set('connectors', allConnectors);
}

function clear(state) {
  return cancelNew(state);
}