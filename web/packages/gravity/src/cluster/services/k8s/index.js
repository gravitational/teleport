/*
Copyright 2019-2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import api from 'gravity/services/api';
import cfg from 'gravity/config';
import { map, sortBy } from 'lodash';
import { generatePath } from 'react-router';
import { makeNodes } from './makeK8sNode';
import makeK8sSecret from './makeK8sSecret';

const k8s = {
  getNamespaces() {
    const siteId = cfg.defaultSiteId;
    const url = generatePath(cfg.api.k8sNamespacePath, { siteId });
    return api.get(url).then(onlyItems);
  },

  saveConfigMap(namespace, name, data) {
    const siteId = cfg.defaultSiteId;
    const url = generatePath(cfg.api.k8sConfigMapsByNamespacePath, {
      siteId,
      namespace,
      name,
    });

    return api.patch(url, data);
  },

  createSecret(namespace, data) {
    const siteId = cfg.defaultSiteId;
    const url = generatePath(cfg.api.k8sSecretsPath, { siteId, namespace });

    return api.post(url, data);
  },

  saveSecret(namespace, name, data) {
    const siteId = cfg.defaultSiteId;
    const url = generatePath(cfg.api.k8sSecretsPath, {
      siteId,
      namespace,
      name,
    });

    return api.patch(url, data);
  },

  getConfigMaps() {
    const siteId = cfg.defaultSiteId;
    const url = generatePath(cfg.api.k8sConfigMapsPath, { siteId });
    return api.get(url).then(onlyItems);
  },

  getNodes() {
    const siteId = cfg.defaultSiteId;
    const url = generatePath(cfg.api.k8sNodesPath, { siteId });
    return api.get(url).then(json => makeNodes(json.items));
  },

  getJobs(namespace) {
    const siteId = cfg.defaultSiteId;
    const url = generatePath(cfg.api.k8sJobsPath, { siteId, namespace });
    return api.get(url).then(onlyItems);
  },

  getSecrets(namespace) {
    const siteId = cfg.defaultSiteId;
    const url = generatePath(cfg.api.k8sSecretsPath, { siteId, namespace });
    return api
      .get(url)
      .then(json => map(json.items, makeK8sSecret))
      .then(secrets => sortBy(secrets, ['created']).reverse());
  },

  getPods(namespace) {
    const siteId = cfg.defaultSiteId;
    let url = cfg.api.k8sPodsPath;
    if (namespace) {
      url = cfg.api.k8sPodsByNamespacePath;
    }

    api.get(generatePath(cfg.api.k8sSecretsPath, { siteId, namespace }));

    url = generatePath(url, { siteId, namespace });
    return api.get(url).then(onlyItems);
  },

  getServices() {
    const siteId = cfg.defaultSiteId;
    const url = generatePath(cfg.api.k8sServicesPath, { siteId });
    return api.get(url).then(onlyItems);
  },

  getDeployments() {
    const siteId = cfg.defaultSiteId;
    const url = generatePath(cfg.api.k8sDelploymentsPath, { siteId });
    return api.get(url).then(onlyItems);
  },

  getDaemonSets() {
    const siteId = cfg.defaultSiteId;
    const url = generatePath(cfg.api.k8sDaemonSetsPath, { siteId });
    return api.get(url).then(onlyItems);
  },
};

function onlyItems(json) {
  return json.items || [];
}

export default k8s;
