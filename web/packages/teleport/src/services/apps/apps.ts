/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import api from 'teleport/services/api';
import cfg, { UrlAppParams, UrlResourcesParams } from 'teleport/config';
import { ResourcesResponse } from 'teleport/services/agents';

import makeApp from './makeApps';
import { App } from './types';

const service = {
  fetchApps(
    clusterId: string,
    params: UrlResourcesParams
  ): Promise<ResourcesResponse<App>> {
    return api.get(cfg.getApplicationsUrl(clusterId, params)).then(json => {
      const items = json?.items || [];

      return {
        agents: items.map(makeApp),
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
        cookieValue: json.cookie_value as string,
        subjectCookieValue: json.subject_cookie_value as string,
      }));
  },

  getAppFqdn(params: UrlAppParams) {
    return api.get(cfg.getAppFqdnUrl(params)).then(json => ({
      fqdn: json.fqdn as string,
    }));
  },
};

export default service;
