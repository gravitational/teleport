import { Store, toImmutable } from 'nuclear-js';
import {  Record, List } from 'immutable';
import { SETTINGS_CLUSTER_RECEIVE } from './actionTypes';

export class ClusterRec extends Record({
  key: null,
  name: '',
  proxyAddress: '',
  reverseTunnelAddress: '',
  enabled: false,
  roles: new List()
}) {
  constructor(props) {    
    let key = Math.random();
    super({ key, ...props });                
  }  
}

export default Store({
  getInitialState() {
    return toImmutable({      
      clusterToDelete: null,        
      clusters: []
    });
  },

  initialize() {     
    this.on(SETTINGS_CLUSTER_RECEIVE, receiveClusters);                
  }
})

function receiveClusters(state, json) {      
  json = json || [];  
  let recList = json.reduce(
    (list, item) => list.push(new ClusterRec(item)),
    new List()
  );

  recList = recList.sortBy(item => !item.enabled);

  return state.set('clusters', recList);
}