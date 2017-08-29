import reactor from 'app/reactor';
import { Store, toImmutable } from 'nuclear-js';
import * as AT from './actionTypes';

export function getStore() {
  return reactor.evaluate(['tlp_settings_role'])
}

export default Store({

  getInitialState() {
    return toImmutable([]);
  },

  initialize() {      
    this.on(AT.ADD_ERROR, (state, messages) => state.push(...messages))    
  }
})

