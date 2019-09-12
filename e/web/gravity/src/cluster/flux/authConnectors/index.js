import reactor from 'gravity/reactor';
import store from './authStore';

const STORE_NAME = 'settings_auth';

reactor.registerStores({ [STORE_NAME] : store });

export function getAuthSettings(){
  return reactor.evaluate([STORE_NAME]);
}

export const getters = {
  store: [STORE_NAME]
}