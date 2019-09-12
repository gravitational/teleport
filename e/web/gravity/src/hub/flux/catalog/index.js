import reactor from 'gravity/reactor';
import store from './store';

const STORE_NAME = 'hub_catalog';

reactor.registerStores({ [STORE_NAME] : store });

export const getters = {
  catalogStore: [STORE_NAME]
}
