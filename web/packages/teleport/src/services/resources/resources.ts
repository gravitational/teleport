import { map, at } from 'lodash';
import api from 'teleport/services/api';
import makeResource from './makeResource';
import { ResourceKind } from './types';
import cfg from 'teleport/config';

const service = {
  fetchAuthConnectors() {
    return api.get(cfg.getResourcesUrl('auth_connector')).then(makeResources);
  },

  fetchRoles() {
    return api.get(cfg.getResourcesUrl('role')).then(makeResources);
  },

  fetchTrustedClusters() {
    return api.get(cfg.getResourcesUrl('trusted_cluster')).then(makeResources);
  },

  upsertTrustedCluster(yaml: string, isNew = false) {
    return service.upsert('trusted_cluster', yaml, isNew);
  },

  upsertAuthConnector(yaml: string, isNew = false) {
    return service.upsert('auth_connector', yaml, isNew);
  },

  upsertRole(yaml: string, isNew = false) {
    return service.upsert('role', yaml, isNew);
  },

  upsert(kind: ResourceKind, yaml: string, isNew = false) {
    const req = { kind, content: yaml };
    if (isNew) {
      return api.post(cfg.getResourcesUrl(), req).then(makeResources);
    }

    return api.put(cfg.getResourcesUrl(), req).then(makeResources);
  },

  deleteRole(name: string) {
    return service.delete('role', name);
  },

  deleteAuthConnector(name: string) {
    return service.delete('auth_connector', name);
  },

  deleteTrustedCluster(name: string) {
    return service.delete('trusted_cluster', name);
  },

  delete(kind: ResourceKind, name: string) {
    return api.delete(cfg.getRemoveResourceUrl(kind, name));
  },
};

function makeResources(json: any) {
  const [items] = at(json, 'items');
  return map(items, makeResource);
}

export default service;
