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

import { Flex, Text, Box } from 'design';

import {
  makeApp,
  makeRootCluster,
  makeServer,
  makeDatabase,
  makeKube,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { VnetContextProvider } from 'teleterm/ui/Vnet';

import {
  ConnectAppActionButton,
  ConnectServerActionButton,
  ConnectDatabaseActionButton,
  ConnectKubeActionButton,
} from './ActionButtons';

export default {
  title: 'Teleterm/DocumentCluster/ActionButtons',
};

export function ActionButtons() {
  const appContext = new MockAppContext();
  appContext.configService.set('feature.vnet', true);
  prepareAppContext(appContext);

  return (
    <MockAppContextProvider appContext={appContext}>
      <VnetContextProvider>
        <Buttons />
      </VnetContextProvider>
    </MockAppContextProvider>
  );
}

export function WithoutVnet() {
  const appContext = new MockAppContext({ platform: 'win32' });
  prepareAppContext(appContext);

  return (
    <MockAppContextProvider appContext={appContext}>
      <VnetContextProvider>
        <Buttons />
      </VnetContextProvider>
    </MockAppContextProvider>
  );
}

function Buttons() {
  return (
    <Flex gap={4}>
      <Flex gap={3} flexDirection="column">
        <Box>
          <Text>TCP app</Text>
          <TcpApp />
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
          { arn: 'foo', display: 'foo', name: 'foo' },
          { arn: 'bar', display: 'bar', name: 'bar' },
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
