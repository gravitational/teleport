/*
Copyright 2023 Gravitational, Inc.

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

export enum NavigationCategory {
  Resources = 'Resources',
  Management = 'Management',
}

export enum ManagementSection {
  Access = 'Access',
  Activity = 'Activity',
  Billing = 'Billing',
  Clusters = 'Clusters',
}

export const MANAGEMENT_NAVIGATION_SECTIONS = [
  ManagementSection.Access,
  ManagementSection.Activity,
  ManagementSection.Billing,
  ManagementSection.Clusters,
];

export const NAVIGATION_CATEGORIES = [
  NavigationCategory.Resources,
  NavigationCategory.Management,
];
