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

import FeatureBase from 'shared/libs/featureBase';

export interface Context {
  isAccountEnabled(): boolean;
  isAuditEnabled(): boolean;
  isAuthConnectorEnabled(): boolean;
  isRolesEnabled(): boolean;
  isTrustedClustersEnabled(): boolean;
}

export interface Feature extends FeatureBase {
  Component: (props: any) => JSX.Element;
  getRoute(): FeatureRouteParams;
  onload(context: Context): void;
}

type FeatureRouteParams = {
  title: string;
  path: string;
  exact?: boolean;
  component(props: any): JSX.Element;
};
