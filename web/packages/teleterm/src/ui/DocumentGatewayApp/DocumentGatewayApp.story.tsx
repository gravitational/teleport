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
import { wait } from 'shared/utils/wait';

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import { makeAppGateway } from 'teleterm/services/tshd/testHelpers';
import { DocumentGatewayApp } from 'teleterm/ui/DocumentGatewayApp/DocumentGatewayApp';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import * as types from 'teleterm/ui/services/workspacesService';

type StoryProps = {
  appType: 'web' | 'tcp';
  online: boolean;
  changePort: 'succeed' | 'throw-error';
  disconnect: 'succeed' | 'throw-error';
};

const meta: Meta<StoryProps> = {
  title: 'Teleterm/DocumentGatewayApp',
  component: Story,
  argTypes: {
    appType: {
      control: { type: 'radio' },
      options: ['web', 'tcp'],
    },
    changePort: {
      if: { arg: 'online' },
      control: { type: 'radio' },
      options: ['succeed', 'throw-error'],
    },
    disconnect: {
      if: { arg: 'online' },
      control: { type: 'radio' },
      options: ['succeed', 'throw-error'],
    },
  },
  args: {
    appType: 'web',
    online: true,
    changePort: 'succeed',
    disconnect: 'succeed',
  },
};
export default meta;

export function Story(props: StoryProps) {
  const rootClusterUri = '/clusters/bar';
  const gateway = makeAppGateway();
  if (props.appType === 'tcp') {
    gateway.protocol = 'TCP';
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
  };
  if (!props.online) {
    documentGateway.gatewayUri = undefined;
  }

  const appContext = new MockAppContext();
  appContext.workspacesService.setState(draftState => {
    draftState.rootClusterUri = rootClusterUri;
    draftState.workspaces[rootClusterUri] = {
      localClusterUri: rootClusterUri,
      documents: [documentGateway],
      location: documentGateway.uri,
      accessRequests: undefined,
    };
  });
  if (props.online) {
    appContext.clustersService.createGateway = () => Promise.resolve(gateway);
    appContext.clustersService.setState(draftState => {
      draftState.gateways.set(gateway.uri, gateway);
    });

    appContext.tshd.setGatewayLocalPort = ({ localPort }) =>
      wait(1000).then(
        () =>
          new MockedUnaryCall(
            { ...gateway, localPort },
            props.changePort === 'throw-error'
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
