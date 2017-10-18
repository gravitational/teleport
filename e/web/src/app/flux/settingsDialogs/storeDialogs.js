import { Store } from 'nuclear-js';
import { Record } from 'immutable';
import * as AT from './actionTypes';

class DialogsRec extends Record({  
  resourceToDelete: null
}){
    
  setResourceToDelete(item){
    return this.set('resourceToDelete', item);
  }

  getResourceToDelete(){
    return this.get('resourceToDelete');
  }  
}

export default Store({
  getInitialState() {
    return new DialogsRec();
  },

  initialize() {    
    this.on(AT.SET_RES_TO_DELETE, (state, item) => state.setResourceToDelete(item))        
  }
});
