/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { ConfigureSSHClients } from './ConfigureSSHClients';

export default {
  title: 'Teleterm/Vnet/ConfigureSSHClients',
  argTypes: {
    simulateError: {
      name: 'simulate error',
      control: 'boolean',
      defaultValue: false,
    },
    host: {
      name: 'set specific SSH host',
      control: 'text',
    },
  },
};

export const Story = ({ simulateError, host }) => (
  <ConfigureSSHClients
    onClose={() => console.log('closed')}
    onConfirm={() =>
      new Promise<void>((resolve, reject) => {
        setTimeout(
          () =>
            simulateError ? reject(new Error('test error')) : resolve(null),
          1000
        );
      })
    }
    host={host}
    vnetSSHConfigPath="/Users/user/Library/Application Support/Teleport Connect/tsh/vnet_ssh_config"
  />
);
