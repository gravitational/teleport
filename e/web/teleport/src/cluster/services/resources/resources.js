import { map, at } from 'lodash';
import api from 'teleport/services/api';
import makeResource, { ResourceEnum } from 'e-shared/services/makeResource';
import cfg from 'e-teleport/config';

const service = {
  fetchAuthConnectors() {
    return api
      .get(cfg.getResourcesUrl(ResourceEnum.AUTH_CONNECTORS))
      .then(makeResources);
  },

  fetchRoles() {
    return api.get(cfg.getResourcesUrl(ResourceEnum.ROLE)).then(makeResources);
  },

  fetchTrustedClusters() {
    return api
      .get(cfg.getResourcesUrl(ResourceEnum.TRUSTED_CLUSTER))
      .then(makeResources);
  },

  upsertTrustedCluster(yaml, isNew = false) {
    return this.upsert(ResourceEnum.TRUSTED_CLUSTER, yaml, isNew);
  },

  upsertAuthConnector(yaml, isNew = false) {
    return this.upsert(ResourceEnum.AUTH_CONNECTORS, yaml, isNew);
  },

  upsertRole(yaml, isNew = false) {
    return this.upsert(ResourceEnum.ROLE, yaml, isNew);
  },

  upsert(kind, yaml, isNew = false) {
    const req = { kind, content: yaml };
    if (isNew) {
      return api.post(cfg.getResourcesUrl(), req).then(makeResources);
    }

    return api.put(cfg.getResourcesUrl(), req).then(makeResources);
  },

  deleteRole(name) {
    return this.delete(ResourceEnum.ROLE, name);
  },

  deleteAuthConnector(name) {
    return this.delete(ResourceEnum.AUTH_CONNECTORS, name);
  },

  deleteTrustedCluster(name) {
    return this.delete(ResourceEnum.TRUSTED_CLUSTER, name);
  },

  delete(kind, name) {
    return api.delete(cfg.getRemoveResourceUrl(kind, name));
  },
};

function makeResources(json) {
  const [items] = at(json, 'items');
  return map(items, makeResource);
}

export default service;
