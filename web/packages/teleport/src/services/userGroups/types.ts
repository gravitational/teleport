/*
Copyright 2021-2022 Gravitational, Inc.

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

import { AgentLabel } from 'teleport/services/agents';

// UserGroup is a remote user group.
export type UserGroup = {
  // Name is name of the user group.
  name: string;
  // Description is the description of the user group.
  description: string;
  // Labels for the user group.
  labels: AgentLabel[];
  // FriendlyName for the user group.
  friendlyName?: string;
  // Applications is a list of associated applications.
  applications: string[];
};

export type UserGroupsResponse = {
  userGroups: UserGroup[];
  startKey?: string;
  totalCount?: number;
};
