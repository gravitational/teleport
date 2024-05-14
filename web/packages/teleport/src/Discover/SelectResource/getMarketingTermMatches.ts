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

import { Resource } from 'gen-proto-ts/teleport/userpreferences/v1/onboard_pb';

import { MarketingParams } from 'teleport/services/userPreferences/types';

/**
 * Returns a list of resource kinds that match provided marketing parameters.
 *
 * @param marketingParams - MarketingParams from cluster user preferences which are set at signup
 * @returns an array of Resource associated with the marketing params for resource discoverability
 *
 */
export const getMarketingTermMatches = (
  marketingParams: MarketingParams
): Resource[] => {
  const params = [];
  if (marketingParams) {
    marketingParams.campaign && params.push(marketingParams.campaign);
    marketingParams.medium && params.push(marketingParams.medium);
    marketingParams.source && params.push(marketingParams.source);
    marketingParams.intent && params.push(marketingParams.intent);
  }
  if (params.length === 0) {
    return [];
  }

  const matches = new Set<Resource>();
  params.forEach(p => {
    Object.values(TermMatch).forEach(m => {
      const clusterResource = matchTerm(m);
      if (p.includes(m) && clusterResource) {
        matches.add(clusterResource);
      }
    });
  });

  return Array.from(matches);
};

export enum TermMatch {
  App = 'app',
  Database = 'database',
  Desktop = 'desktop',
  K8s = 'k8s',
  Kube = 'kube',
  Kubernetes = 'kubernetes',
  Server = 'server',
  SSH = 'ssh',
  Windows = 'windows',
  AWS = 'aws',
}

const matchTerm = (m: string): Resource => {
  switch (m) {
    case TermMatch.App:
      return Resource.WEB_APPLICATIONS;
    case TermMatch.Database:
      return Resource.DATABASES;
    case TermMatch.Kube:
    case TermMatch.Kubernetes:
    case TermMatch.K8s:
      return Resource.KUBERNETES;
    case TermMatch.SSH:
    case TermMatch.Server:
      return Resource.SERVER_SSH;
    case TermMatch.Desktop:
    case TermMatch.Windows:
      return Resource.WINDOWS_DESKTOPS;
    // currently we have no resource kind nor cluster resource defined for AWS
    // in the future, we can search the resources based on this term.
    case TermMatch.AWS:
    default:
      return null;
  }
};
