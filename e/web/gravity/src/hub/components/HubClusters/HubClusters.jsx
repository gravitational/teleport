import React from 'react';
import { values } from 'lodash';
import { useFluxStore } from 'gravity/components/nuclear';
import CardEmpty from 'gravity/components/CardEmpty';
import AjaxPoller from 'gravity/components/AjaxPoller';
import { getters } from 'e-gravity/hub/flux/clusters';
import { refreshClusters } from 'e-gravity/hub/flux/actions';
import { withState } from 'shared/hooks';
import { FeatureBox, FeatureHeader, FeatureHeaderTitle } from '../components/Layout';
import ClusterTileList from './ClusterTileList';
import HubClusterStore, { Provider, useStore } from './hubClusterStore';
import DisconnectDialog from './ClusterDisconnectDialog';

const POLL_INTERVAL = 4000; // every 4 sec

export function HubClusters(props){
  const { clusters, store, onRefresh } = props;
  const { setClusterToDisconnect } = store;
  const { clusterToDisconnect } = store.state;

  // ignore HUB cluster
  const joinedClusters = clusters.filter(c => !c.local);
  const isEmpty = joinedClusters.length === 0;

  useStore(store);

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>
          Clusters
        </FeatureHeaderTitle>
      </FeatureHeader>
      <Provider value={store}>
        { isEmpty && <CardEmpty title="There are no joined clusters"/> }
        { !isEmpty && <ClusterTileList clusters={joinedClusters}/> }
        { clusterToDisconnect && (
            <DisconnectDialog
              cluster={clusterToDisconnect}
              onClose={ () => setClusterToDisconnect(null) }
            />
        )}
      </Provider>
      <AjaxPoller time={POLL_INTERVAL} onFetch={onRefresh} />
    </FeatureBox>
  )
}

function mapState(){
  const [ store ] = React.useState(() => {
    return new HubClusterStore()
  });

  const clusterStore = useFluxStore(getters.clusterStore);
  return {
    store,
    clusters: values(clusterStore.clusters),
    onRefresh:  refreshClusters
  }
}

export default withState(mapState)(HubClusters)