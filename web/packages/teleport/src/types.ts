/*
Copyright 2020 Gravitational, Inc.

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

import React from 'react';

import {
  ManagementSection,
  NavigationCategory,
} from 'teleport/Navigation/categories';

export type NavGroup = 'team' | 'activity' | 'clusters' | 'accessrequests';

export interface Context {
  init(): Promise<void>;
  getFeatureFlags(): FeatureFlags;
}

interface TeleportFeatureNavigationItem {
  title: string;
  icon: React.ReactNode;
  exact?: boolean;
  getLink?(clusterId: string): string;
  isExternalLink?: boolean;
}

interface TeleportFeatureRoute {
  title: string;
  path: string;
  exact?: boolean;
  component: React.FunctionComponent;
}

export interface TeleportFeature {
  parent?: new () => TeleportFeature | null;
  category?: NavigationCategory;
  section?: ManagementSection;
  hasAccess(flags: FeatureFlags): boolean;
  route?: TeleportFeatureRoute;
  navigationItem?: TeleportFeatureNavigationItem;
  topMenuItem?: TeleportFeatureNavigationItem;
}

export type StickyCluster = {
  clusterId: string;
  hasClusterUrl: boolean;
  isLeafCluster: boolean;
};

export type Label = {
  name: string;
  value: string;
};

// TODO: create a better abscraction for a filter, right now it's just a label
export type Filter = {
  value: string;
  name: string;
  kind: 'label';
};

export interface FeatureFlags {
  audit: boolean;
  recordings: boolean;
  authConnector: boolean;
  roles: boolean;
  trustedClusters: boolean;
  users: boolean;
  applications: boolean;
  kubernetes: boolean;
  billing: boolean;
  databases: boolean;
  desktops: boolean;
  nodes: boolean;
  activeSessions: boolean;
  accessRequests: boolean;
  newAccessRequest: boolean;
  downloadCenter: boolean;
  discover: boolean;
  plugins: boolean;
  deviceTrust: boolean;
}
