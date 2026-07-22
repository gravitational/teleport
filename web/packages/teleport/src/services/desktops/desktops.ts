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

import cfg, { UrlResourcesParams } from 'teleport/config';
import { ResourcesResponse } from 'teleport/services/agents';
import api from 'teleport/services/api';

import { makeDesktop } from './makeDesktop';
import type { Desktop } from './types';

class DesktopService {
  fetchDesktops(
    clusterId: string,
    params: UrlResourcesParams,
    signal?: AbortSignal
  ): Promise<ResourcesResponse<Desktop>> {
    return api.get(cfg.getDesktopsUrl(clusterId, params), signal).then(json => {
      const items = json?.items || [];

      return {
        agents: items.map(makeDesktop),
        startKey: json?.startKey,
        totalCount: json?.totalCount,
      };
    });
  }

  fetchDesktop(clusterId: string, desktopPath: string) {
    return api
      .get(cfg.getDesktopUrl(clusterId, desktopPath))
      .then(json => makeDesktop(json));
  }

  checkDesktopIsActive(
    clusterId: string,
    desktopName: string
  ): Promise<boolean> {
    return api
      .get(cfg.getDesktopIsActiveUrl(clusterId, desktopName))
      .then(json => json.active);
  }
}

const desktopService = new DesktopService();

export default desktopService;
