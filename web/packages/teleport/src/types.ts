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

import Ctx from 'teleport/teleportContext';

export type NavGroup = 'team' | 'activity' | 'clusters' | 'accessrequests';

type FeatureFlags = {
  audit: boolean;
  authConnector: boolean;
  roles: boolean;
  trustedClusters: boolean;
  users: boolean;
  applications: boolean;
};

export interface Context {
  init(features: Feature[]): Promise<void>;
  getFeatureFlags(): FeatureFlags;
}

export abstract class Feature {
  abstract topNavTitle: string;
  abstract route: FeatureRoute;
  abstract isAvailable(ctx: Ctx): boolean;
  abstract register(ctx: Ctx): void;
}

export type StickyCluster = {
  clusterId: string;
  hasClusterUrl: boolean;
  isLeafCluster: boolean;
};

type FeatureRoute = {
  title: string;
  path: string;
  exact?: boolean;
  component: React.FunctionComponent;
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
