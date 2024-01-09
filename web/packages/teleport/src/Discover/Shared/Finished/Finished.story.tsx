/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { Finished as Component } from './Finished';

import type { AgentStepProps } from '../../types';

export default {
  title: 'Teleport/Discover/Shared',
};

export const Finished = () => <Component {...props} />;

export const FinishedWithAutoEnroll = () => (
  <Component
    {...props}
    agentMeta={
      {
        autoDiscoveryConfig: {
          name: 'some-name',
          discoveryGroup: 'some-group',
          aws: [
            {
              types: ['rds'],
              regions: ['us-east-1'],
              tags: {},
              integration: 'some-integration',
            },
          ],
        },
      } as any
    }
  />
);

const props: AgentStepProps = {
  agentMeta: { resourceName: 'some-resource-name', agentMatcherLabels: [] },
  updateAgentMeta: () => null,
  nextStep: () => null,
};
