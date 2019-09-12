import reactor from 'gravity/reactor';
import { HUB_RECEIVE_CLUSTERS, HUB_UPDATE_CLUSTERS } from './actionTypes';

export function setClusters(clusters){
  reactor.dispatch(HUB_RECEIVE_CLUSTERS, clusters);
}

export function updateClusters(clusters){
  reactor.dispatch(HUB_UPDATE_CLUSTERS, clusters);
}