import { keyBy, values } from 'lodash';
import { Store } from 'nuclear-js';
import { Record } from 'immutable';
import { HUB_RECEIVE_CLUSTERS, HUB_UPDATE_CLUSTERS } from './actionTypes';

const StoreRec = Record({
  clusters: {
    /** a map of clusters */
  },
});

export default Store({
  getInitialState() {
    return new StoreRec();
  },

  initialize() {
    this.on(HUB_RECEIVE_CLUSTERS, setClusters);
    this.on(HUB_UPDATE_CLUSTERS, updateClusters);
  },
});

function setClusters(state, json) {
  const clusters = keyBy(json, 'id');
  return state.set('clusters', clusters);
}

function updateClusters(state, json) {
  const clusters = {};

  // shallow objects do not have logos and icons, so we need to update
  // existing objects without overriding these values
  json.forEach(item => {
    // assign application icon from previous record
    const { logo, apps: previousApps } = state.clusters[item.id] || {};
    const apps = {
      ...item.apps,
    };

    if (previousApps) {
      values(item.apps).forEach(a => {
        if (previousApps[a.id]) {
          apps[a.id].icon = previousApps[a.id].icon;
        }
      });
    }

    clusters[item.id] = {
      ...item,
      logo,
      apps,
    };
  });
  return state.set('clusters', clusters);
}
