import reactor from 'gravity/reactor';
import store from './store';
const STORE_NAME = 'hub_clusters';

reactor.registerStores({ [STORE_NAME] : store });

export const getters = {
  clusterStore: [STORE_NAME]
}