/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { Box, Flex, Text } from 'design';

import {
  makeApp,
  makeDatabase,
  makeKube,
  makeRootCluster,
  makeServer,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { ConnectionsContextProvider } from 'teleterm/ui/TopBar/Connections/connectionsContext';
import { VnetContextProvider } from 'teleterm/ui/Vnet';

import {
  AccessRequestButton,
  ConnectAppActionButton,
  ConnectDatabaseActionButton,
  ConnectKubeActionButton,
  ConnectServerActionButton,
} from './ActionButtons';

type StoryProps = {
  vnet: boolean;
  lotsOfMenuItems: boolean;
};

const meta: Meta<StoryProps> = {
  title: 'Teleterm/DocumentCluster/ActionButtons',
  component: Buttons,
  argTypes: {
    vnet: { control: { type: 'boolean' } },
    lotsOfMenuItems: {
      control: { type: 'boolean' },
      description:
        // TODO(ravicious): Support this prop in more places than just TCP ports.
        'Renders long lists of options in menus. Currently works only with ports for multi-port TCP apps.',
    },
  },
  args: {
    vnet: true,
    lotsOfMenuItems: false,
  },
};

export default meta;

export function Story(props: StoryProps) {
  const platform = props.vnet ? 'darwin' : 'win32';
  const appContext = new MockAppContext({ platform });
  prepareAppContext(appContext);

  return (
    <MockAppContextProvider appContext={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <Buttons {...props} />
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </MockAppContextProvider>
  );
}

function Buttons(props: StoryProps) {
  return (
    <Flex gap={4}>
      <Flex gap={3} flexDirection="column">
        <Box>
          <Text>TCP app</Text>
          <TcpApp />
        </Box>
        <Box>
          <Text>multi-port TCP app</Text>
          <TcpMultiPortApp lotsOfMenuItems={props.lotsOfMenuItems} />
        </Box>
        <Box>
          <Text>Web app</Text>
          <HttpApp />
        </Box>
        <Box>
          <Text>AWS console</Text>
          <AwsConsole />
        </Box>
        <Box>
          <Text>SAML app</Text>
          <SamlApp />
        </Box>
      </Flex>
      <Box>
        <Text>Server</Text>
        <Server />
      </Box>
      <Box>
        <Text>Database</Text>
        <Database />
      </Box>
      <Box>
        <Text>Kube</Text>
        <Kube />
      </Box>
      <Flex gap={3} flexDirection="column">
        <Box>
          <Text>Request not started</Text>
          <RequestNotStarted />
        </Box>
        <Box>
          <Text>Request started</Text>
          <RequestStarted />
        </Box>
        <Box>
          <Text>Resource added</Text>
          <ResourceAdded />
        </Box>
      </Flex>
    </Flex>
  );
}

const testCluster = makeRootCluster();
testCluster.loggedInUser.sshLogins = ['ec2-user'];

function prepareAppContext(appContext: MockAppContext): void {
  appContext.workspacesService.setState(d => {
    d.rootClusterUri = testCluster.uri;
  });
  appContext.clustersService.setState(d => {
    d.clusters.set(testCluster.uri, testCluster);
  });
  appContext.resourcesService.getDbUsers = async () => ['postgres-user'];
}

function TcpApp() {
  return (
    <ConnectAppActionButton
      app={makeApp({
        uri: `${testCluster.uri}/apps/bar`,
      })}
    />
  );
}

function TcpMultiPortApp(props: { lotsOfMenuItems: boolean }) {
  let tcpPorts = [
    { port: 1337, endPort: 0 },
    { port: 4242, endPort: 0 },
    { port: 54221, endPort: 61879 },
  ];

  if (props.lotsOfMenuItems) {
    tcpPorts = new Array(50).fill(tcpPorts).flat();
  }

  return (
    <ConnectAppActionButton
      app={makeApp({
        uri: `${testCluster.uri}/apps/bar`,
        endpointUri: 'tcp://localhost',
        tcpPorts,
      })}
    />
  );
}

function HttpApp() {
  return (
    <ConnectAppActionButton
      app={makeApp({
        endpointUri: 'http://localhost:3000',
        uri: `${testCluster.uri}/apps/bar`,
      })}
    />
  );
}

function AwsConsole() {
  return (
    <ConnectAppActionButton
      app={makeApp({
        endpointUri: 'https://localhost:3000',
        awsConsole: true,
        awsRoles: [
          {
            arn: 'foo',
            display: 'foo',
            name: 'foo',
            accountId: '123456789012',
          },
          {
            arn: 'bar',
            display: 'bar',
            name: 'bar',
            accountId: '123456789012',
          },
        ],
        uri: `${testCluster.uri}/apps/bar`,
      })}
    />
  );
}

function SamlApp() {
  return (
    <ConnectAppActionButton
      app={makeApp({
        endpointUri: 'https://localhost:3000',
        samlApp: true,
        uri: `${testCluster.uri}/apps/bar`,
      })}
    />
  );
}

function Server() {
  return (
    <ConnectServerActionButton
      server={makeServer({
        uri: `${testCluster.uri}/servers/bar`,
      })}
    />
  );
}

function Database() {
  return (
    <ConnectDatabaseActionButton
      database={makeDatabase({
        uri: `${testCluster.uri}/dbs/bar`,
      })}
    />
  );
}

function Kube() {
  return (
    <ConnectKubeActionButton
      kube={makeKube({
        uri: `${testCluster.uri}/kubes/bar`,
      })}
    />
  );
}

function RequestNotStarted() {
  return (
    <AccessRequestButton
      requestStarted={false}
      onClick={() => {}}
      isResourceAdded={false}
    />
  );
}

function RequestStarted() {
  return (
    <AccessRequestButton
      requestStarted={true}
      onClick={() => {}}
      isResourceAdded={false}
    />
  );
}

function ResourceAdded() {
  return (
    <AccessRequestButton
      requestStarted={true}
      onClick={() => {}}
      isResourceAdded={true}
    />
  );
}
