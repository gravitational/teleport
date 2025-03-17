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
import { Meta } from '@storybook/react';
import { MemoryRouter } from 'react-router';

import {
  OverrideUserAgent,
  UserAgent,
} from 'shared/components/OverrideUserAgent';

import { ContextProvider } from 'teleport';
import { createTeleportContext } from 'teleport/mocks/contexts';

import { tcpApp } from './fixtures';
import { TcpAppConnectDialog as Component } from './TcpAppConnectDialog';

type StoryProps = {
  doesPlatformSupportVnet: boolean;
};

const meta: Meta<StoryProps> = {
  title: 'Teleport/Apps/TcpAppConnectDialog',
  decorators: (Story, { args }) => {
    const ctx = createTeleportContext();
    return (
      <MemoryRouter>
        <ContextProvider ctx={ctx}>
          <OverrideUserAgent
            userAgent={
              args.doesPlatformSupportVnet ? UserAgent.macOS : UserAgent.Linux
            }
          >
            <Story />
          </OverrideUserAgent>
        </ContextProvider>
      </MemoryRouter>
    );
  },
  argTypes: {
    doesPlatformSupportVnet: {
      control: 'boolean',
    },
  },
  args: {
    doesPlatformSupportVnet: true,
  },
};
export default meta;

export function TcpAppConnectDialog() {
  return (
    <Component app={tcpApp} clusterId={tcpApp.clusterId} onClose={() => {}} />
  );
}
