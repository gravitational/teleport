import reactor from 'gravity/reactor';
import store from './store';

const STORE_NAME = 'cluster_roles';

reactor.registerStores({ [STORE_NAME] : store });

export function getRoles() {
  return reactor.evaluate([STORE_NAME]);
}

export const getters = {
  store: [STORE_NAME]
}