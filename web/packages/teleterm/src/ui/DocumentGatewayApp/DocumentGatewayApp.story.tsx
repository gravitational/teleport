/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { Flex } from 'design';
import { usePromiseRejectedOnUnmount, wait } from 'shared/utils/wait';

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import {
  makeApp,
  makeAppGateway,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';
import { DocumentGatewayApp } from 'teleterm/ui/DocumentGatewayApp/DocumentGatewayApp';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import * as types from 'teleterm/ui/services/workspacesService';

type StoryProps = {
  appType: 'web' | 'tcp' | 'tcp-multi-port';
  online: boolean;
  changeLocalPort: 'succeed' | 'throw-error';
  changeTargetPort: 'succeed' | 'throw-error';
  disconnect: 'succeed' | 'throw-error';
  getTcpPorts: 'succeed' | 'throw-error' | 'processing' | 'many-ports';
};

const meta: Meta<StoryProps> = {
  title: 'Teleterm/DocumentGatewayApp',
  component: Story,
  argTypes: {
    appType: {
      control: { type: 'radio' },
      options: ['web', 'tcp', 'tcp-multi-port'],
    },
    changeLocalPort: {
      if: { arg: 'online' },
      control: { type: 'radio' },
      options: ['succeed', 'throw-error'],
    },
    changeTargetPort: {
      if: { arg: 'online' },
      control: { type: 'radio' },
      options: ['succeed', 'throw-error'],
    },
    disconnect: {
      if: { arg: 'online' },
      control: { type: 'radio' },
      options: ['succeed', 'throw-error'],
    },
    getTcpPorts: {
      if: { arg: 'online' },
      control: { type: 'radio' },
      options: ['succeed', 'throw-error', 'processing', 'many-ports'],
      description: 'Used only for multi-port TCP apps.',
    },
  },
  args: {
    appType: 'web',
    online: true,
    changeLocalPort: 'succeed',
    changeTargetPort: 'succeed',
    disconnect: 'succeed',
    getTcpPorts: 'succeed',
  },
};
export default meta;

export function Story(props: StoryProps) {
  const rootCluster = makeRootCluster({ uri: '/clusters/bar' });
  const gateway = makeAppGateway();
  if (props.appType === 'tcp') {
    gateway.protocol = 'TCP';
  }
  if (props.appType === 'tcp-multi-port') {
    gateway.protocol = 'TCP';
    gateway.targetSubresourceName = '4242';
  }
  const documentGateway: types.DocumentGateway = {
    kind: 'doc.gateway',
    targetUri: '/clusters/bar/apps/quux',
    origin: 'resource_table',
    gatewayUri: gateway.uri,
    uri: '/docs/123',
    title: 'quux',
    targetUser: '',
    status: '',
    targetName: 'quux',
    targetSubresourceName: undefined,
  };
  if (!props.online) {
    documentGateway.gatewayUri = undefined;
  }
  if (props.appType === 'tcp-multi-port') {
    documentGateway.targetSubresourceName = '4242';
  }

  const infinitePromise = usePromiseRejectedOnUnmount();

  const appContext = new MockAppContext();
  appContext.addRootClusterWithDoc(rootCluster, documentGateway);
  if (props.online) {
    appContext.clustersService.createGateway = () => Promise.resolve(gateway);
    appContext.clustersService.setState(draftState => {
      draftState.gateways.set(gateway.uri, gateway);
    });

    appContext.tshd.setGatewayLocalPort = ({ localPort }) =>
      wait(1000).then(
        () =>
          new MockedUnaryCall(
            {
              ...appContext.clustersService.findGateway(gateway.uri),
              localPort,
            },
            props.changeLocalPort === 'throw-error'
              ? new Error('something went wrong')
              : undefined
          )
      );
    appContext.tshd.setGatewayTargetSubresourceName = ({
      targetSubresourceName,
    }) =>
      wait(1000).then(
        () =>
          new MockedUnaryCall(
            {
              ...appContext.clustersService.findGateway(gateway.uri),
              targetSubresourceName,
            },
            props.changeTargetPort === 'throw-error'
              ? new Error('something went wrong')
              : undefined
          )
      );
    appContext.tshd.removeGateway = () =>
      wait(50).then(
        () =>
          new MockedUnaryCall(
            {},
            props.disconnect === 'throw-error'
              ? new Error('something went wrong')
              : undefined
          )
      );

    if (props.getTcpPorts === 'processing') {
      appContext.tshd.getApp = () => infinitePromise;
    } else {
      let tcpPorts = [
        { port: 1337, endPort: 4242 },
        { port: 4242, endPort: 0 },
        { port: 17231, endPort: 0 },
        { port: 27381, endPort: 28400 },
      ];
      if (props.getTcpPorts === 'many-ports') {
        tcpPorts = new Array(9).fill(tcpPorts).flat();
      }

      appContext.tshd.getApp = () =>
        wait(500).then(
          () =>
            new MockedUnaryCall(
              {
                app: makeApp({
                  tcpPorts,
                }),
              },
              props.getTcpPorts === 'throw-error'
                ? new Error('something went wrong')
                : undefined
            )
        );
    }
  } else {
    appContext.clustersService.createGateway = () =>
      Promise.reject(new Error('failed to create gateway'));
  }

  return (
    <MockAppContextProvider
      appContext={appContext}
      // Completely re-mount everything when controls change.
      // This ensures that effects are fired again.
      key={JSON.stringify(props)}
    >
      <MockWorkspaceContextProvider>
        <Flex
          flexDirection="column"
          // Simulate TabHost from the real app.
          css={`
            position: absolute;
            top: 0;
            bottom: 0;
            left: 0;
            right: 0;
          `}
        >
          <DocumentGatewayApp doc={documentGateway} visible={true} />
        </Flex>
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );
}
