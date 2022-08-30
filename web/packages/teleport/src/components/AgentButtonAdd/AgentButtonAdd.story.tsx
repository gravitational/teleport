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

import AgentButtocnAdd, { Props } from './AgentButtonAdd';

export default {
  title: 'Teleport/AgentButtonAdd',
};

export const CanCreate = () => <AgentButtocnAdd {...props} />;

export const CannotCreate = () => (
  <AgentButtocnAdd {...props} canCreate={false} />
);

export const CannotCreateVowel = () => (
  <AgentButtocnAdd
    {...props}
    agent="application"
    beginsWithVowel={true}
    canCreate={false}
  />
);

export const OnLeaf = () => <AgentButtocnAdd {...props} isLeafCluster={true} />;

export const OnLeafVowel = () => (
  <AgentButtocnAdd {...props} isLeafCluster={true} beginsWithVowel={true} />
);

const props: Props = {
  agent: 'server',
  beginsWithVowel: false,
  canCreate: true,
  isLeafCluster: false,
  onClick: () => null,
};
