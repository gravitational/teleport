/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import api from 'teleport/services/api';
import cfg, { UrlAppParams } from 'teleport/config';
import makeApp from './makeApps';
import { AppsResponse } from './types';

const service = {
  fetchApps(clusterId: string): Promise<AppsResponse> {
    return api.get(cfg.getApplicationsUrl(clusterId)).then(json => {
      const items = json?.items || [];

      return {
        apps: items.map(makeApp),
        startKey: json?.startKey,
        totalCount: json?.totalCount,
      };
    });
  },

  createAppSession(params: UrlAppParams) {
    const { fqdn, clusterId = '', publicAddr = '', arn = '' } = params;
    return api
      .post(cfg.api.appSession, {
        fqdn,
        cluster_name: clusterId,
        public_addr: publicAddr,
        arn: arn,
      })
      .then(json => ({
        fqdn: json.fqdn as string,
        value: json.value as string,
      }));
  },

  getAppFqdn(params: UrlAppParams) {
    return api.get(cfg.getAppFqdnUrl(params)).then(json => ({
      fqdn: json.fqdn as string,
    }));
  },
};

export default service;
