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

import { Flex, Stack, Text } from 'design';

import { ContextProvider } from 'teleport';
import {
  awsConsoleApp,
  awsIamIcAccountApp,
  gcpCloudApp,
  mcpApp,
} from 'teleport/Apps/fixtures';
import { databases } from 'teleport/Databases/fixtures';
import { desktops } from 'teleport/Desktops/fixtures';
import { kubes } from 'teleport/Kubes/fixtures';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { nodes } from 'teleport/Nodes/fixtures';
import { SamlAppActionProvider } from 'teleport/SamlApplications/useSamlAppActions';
import makeApp from 'teleport/services/apps/makeApps';

import { ResourceActionButton as Component } from './ResourceActionButton';

const meta: Meta = {
  title: 'Teleport/UnifiedResources/ResourceActionButton',
  decorators: Story => {
    const ctx = createTeleportContext();

    return (
      <MemoryRouter>
        <ContextProvider ctx={ctx}>
          <SamlAppActionProvider>
            <Story />
          </SamlAppActionProvider>
        </ContextProvider>
      </MemoryRouter>
    );
  },
};
export default meta;

export function ResourceActionButton() {
  return (
    <Flex gap={4} flexWrap="wrap">
      <Stack gap={3}>
        <Stack>
          <Text>TCP app</Text>
          <Component
            resource={makeApp({
              uri: 'tcp://localhost:1234',
              publicAddr: 'tcp-app.teleport.example.com',
              name: 'tcp-app',
            })}
          />
        </Stack>

        <Stack>
          <Text>Web app</Text>
          <Component
            resource={makeApp({
              uri: 'http://localhost:1234',
              publicAddr: 'web-app.teleport.example.com',
              name: 'web-app',
            })}
          />
        </Stack>

        <Stack>
          <Text>AWS console</Text>
          <Component resource={awsConsoleApp} />
        </Stack>

        <Stack>
          <Text>AWS IAM IC account</Text>
          <Component resource={awsIamIcAccountApp} />
        </Stack>

        <Stack>
          <Text>Cloud app (GCP)</Text>
          <Component resource={gcpCloudApp} />
        </Stack>

        <Stack>
          <Text>SAML app</Text>
          <Component
            resource={makeApp({
              uri: 'http://localhost:300',
              publicAddr: 'saml-app.teleport.example.com',
              fqdn: 'saml-app.teleport.example.com',
              name: 'saml-app',
              samlApp: true,
            })}
          />
        </Stack>
        <Stack>
          <Text>SAML app with launch URLs</Text>
          <Component
            resource={makeApp({
              uri: 'http://localhost:300',
              publicAddr: 'saml-app.teleport.example.com',
              fqdn: 'saml-app.teleport.example.com',
              name: 'saml-app',
              samlApp: true,
              samlAppLaunchUrls: [
                { url: 'https://example.com' },
                { url: 'https://example.com/1' },
              ],
            })}
          />
        </Stack>

        <Stack>
          <Text>MCP app</Text>
          <Component resource={mcpApp} />
        </Stack>
      </Stack>

      <Stack>
        <Text>Server</Text>
        <Component resource={nodes[0]} />
      </Stack>

      <Stack>
        <Text>Database</Text>
        <Component resource={databases[0]} />
      </Stack>

      <Stack>
        <Text>Kube</Text>
        <Component resource={kubes[0]} />
      </Stack>

      <Stack>
        <Text>Windows desktop</Text>
        <Component resource={desktops[0]} />
      </Stack>
    </Flex>
  );
}
