/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import {
  ClusterResource,
  MarketingParams,
} from 'teleport/services/userPreferences/types';

/**
 * Returns a list of resource kinds that match provided marketing parameters.
 *
 * @param marketingParams - MarketingParams from cluster user preferences which are set at signup
 * @returns an array of ClusterResource associated with the marketing params for resource discoverability
 *
 */
export const getMarketingTermMatches = (
  marketingParams: MarketingParams
): ClusterResource[] => {
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

  const matches = new Set<ClusterResource>();
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

const matchTerm = (m: string): ClusterResource => {
  switch (m) {
    case TermMatch.App:
      return ClusterResource.RESOURCE_WEB_APPLICATIONS;
    case TermMatch.Database:
      return ClusterResource.RESOURCE_DATABASES;
    case TermMatch.Kube:
    case TermMatch.Kubernetes:
    case TermMatch.K8s:
      return ClusterResource.RESOURCE_KUBERNETES;
    case TermMatch.SSH:
    case TermMatch.Server:
      return ClusterResource.RESOURCE_SERVER_SSH;
    case TermMatch.Desktop:
    case TermMatch.Windows:
      return ClusterResource.RESOURCE_WINDOWS_DESKTOPS;
    // currently we have no resource kind nor cluster resource defined for AWS
    // in the future, we can search the resources based on this term.
    case TermMatch.AWS:
    default:
      return null;
  }
};
