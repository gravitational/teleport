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

import {
  RequiredDiscoverProviders,
  resourceSpecServerLinuxUbuntu,
} from 'teleport/Discover/Fixtures/fixtures';
import { nodes } from 'teleport/Nodes/fixtures';

import { TestConnection } from './TestConnection';

export default {
  title: 'Teleport/Discover/Server/TestConnection',
};

const node = { ...nodes[0] };
node.sshLogins = [
  ...node.sshLogins,
  'george_washington_really_long_name_testing',
];
const agentStepProps = {
  prevStep: () => {},
  nextStep: () => {},
  agentMeta: { resourceName: node.hostname, node, agentMatcherLabels: [] },
};

export const Init = () => (
  <Provider>
    <TestConnection {...agentStepProps} />
  </Provider>
);

const Provider = ({ children }) => {
  return (
    <RequiredDiscoverProviders
      agentMeta={agentStepProps.agentMeta}
      resourceSpec={resourceSpecServerLinuxUbuntu}
    >
      {children}
    </RequiredDiscoverProviders>
  );
};
