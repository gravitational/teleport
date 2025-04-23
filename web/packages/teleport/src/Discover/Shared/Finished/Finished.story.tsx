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

import type { AgentStepProps } from '../../types';
import { Finished as Component } from './Finished';

export default {
  title: 'Teleport/Discover/Shared',
};

export const Finished = () => <Component {...props} />;

export const FinishedWithoutAgentMeta = () => (
  <Component {...props} agentMeta={undefined} />
);

export const FinishedWithAutoEnroll = () => (
  <Component
    {...props}
    agentMeta={{
      autoDiscovery: {
        config: {
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
        requiredVpcsAndSubnets: undefined,
      },
    }}
  />
);

const props: AgentStepProps = {
  agentMeta: { resourceName: 'some-resource-name', agentMatcherLabels: [] },
  updateAgentMeta: () => null,
  nextStep: () => null,
};
