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

import { Meta } from '@storybook/react';
import { ComponentProps } from 'react';

import {
  makeEmptyAttempt,
  makeErrorAttempt,
  makeProcessingAttempt,
} from 'shared/hooks/useAsync';

import { makeDatabaseGateway } from 'teleterm/services/tshd/testHelpers';

import { OfflineGateway } from '../components/OfflineGateway';
import {
  formSchema,
  makeRenderFormControlsFromDefaultPort,
} from './DocumentGateway';
import { OnlineDocumentGateway } from './OnlineDocumentGateway';

type StoryProps = {
  online: boolean;
  // Online props.
  longValues: boolean;
  dbNameAttempt: 'not-started' | 'processing' | 'error';
  portAttempt: 'not-started' | 'processing' | 'error';
  disconnectAttempt: 'not-started' | 'error';
  // Offline props.
  connectAttempt: 'not-started' | 'processing' | 'error';
};

const meta: Meta<StoryProps> = {
  title: 'Teleterm/DocumentGateway',
  component: Story,
  argTypes: {
    // Online props.
    longValues: { if: { arg: 'online' } },
    dbNameAttempt: {
      if: { arg: 'online' },
      control: { type: 'radio' },
      options: ['not-started', 'processing', 'error'],
    },
    portAttempt: {
      if: { arg: 'online' },
      control: { type: 'radio' },
      options: ['not-started', 'processing', 'error'],
    },
    disconnectAttempt: {
      if: { arg: 'online' },
      control: { type: 'radio' },
      options: ['not-started', 'error'],
    },
    // Offline props.
    connectAttempt: {
      if: { arg: 'online', truthy: false },
      control: { type: 'radio' },
      options: ['not-started', 'processing', 'error'],
    },
  },
  args: {
    online: true,
    // Online props.
    longValues: false,
    dbNameAttempt: 'not-started',
    portAttempt: 'not-started',
    disconnectAttempt: 'not-started',
    // Offline props.
    connectAttempt: 'not-started',
  },
};
export default meta;

export function Story(props: StoryProps) {
  let gateway = makeDatabaseGateway({
    uri: '/gateways/bar',
    targetName: 'sales-production',
    targetUri: '/clusters/bar/dbs/foo',
    targetUser: 'alice',
    localAddress: 'localhost',
    localPort: '1337',
    protocol: 'postgres',
    targetSubresourceName: 'bar',
  });
  gateway.gatewayCliCommand.preview = 'connect-me-to-db-please';

  if (!props.online) {
    const offlineGatewayProps: ComponentProps<typeof OfflineGateway> = {
      connectAttempt: makeEmptyAttempt(),
      reconnect: () => {},
      targetName: gateway.targetName,
      gatewayKind: 'database',
      formSchema,
      renderFormControls: makeRenderFormControlsFromDefaultPort('1337'),
    };

    if (props.connectAttempt === 'error') {
      offlineGatewayProps.connectAttempt = makeErrorAttempt(
        new Error('listen tcp 127.0.0.1:62414: bind: address already in use')
      );
    }

    if (props.connectAttempt === 'processing') {
      offlineGatewayProps.connectAttempt = makeProcessingAttempt();
    }

    return (
      <OfflineGateway
        // Completely re-mount all components on props change.
        key={JSON.stringify(props)}
        {...offlineGatewayProps}
      />
    );
  }

  if (props.longValues) {
    gateway = makeDatabaseGateway({
      uri: '/gateways/bar',
      targetName: 'sales-production',
      targetUri: '/clusters/bar/dbs/foo',
      targetUser:
        'quux-quuz-foo-bar-quux-quuz-foo-bar-quux-quuz-foo-bar-quux-quuz-foo-bar',
      localAddress: 'localhost',
      localPort: '13337',
      protocol: 'postgres',
      targetSubresourceName:
        'foo-bar-baz-quux-quuz-foo-bar-baz-quux-quuz-foo-bar-baz-quux-quuz',
    });
    gateway.gatewayCliCommand.preview =
      'connect-me-to-db-please-baz-quux-quuz-foo-baz-quux-quuz-foo-baz-quux-quuz-foo';
  }

  const onlineDocumentGatewayProps: ComponentProps<
    typeof OnlineDocumentGateway
  > = {
    gateway: gateway,
    disconnect: async () => [undefined, null],
    runCliCommand: () => {},
    changeDbName: async () => [undefined, null],
    changeDbNameAttempt: makeEmptyAttempt(),
    changePort: async () => [undefined, null],
    changePortAttempt: makeEmptyAttempt(),
    disconnectAttempt: makeEmptyAttempt(),
  };

  if (props.dbNameAttempt === 'error') {
    onlineDocumentGatewayProps.changeDbNameAttempt = makeErrorAttempt(
      new Error('Something went wrong with setting database name.')
    );
  }
  if (props.dbNameAttempt === 'processing') {
    onlineDocumentGatewayProps.changeDbNameAttempt = makeProcessingAttempt();
  }

  if (props.portAttempt === 'error') {
    onlineDocumentGatewayProps.changePortAttempt = makeErrorAttempt(
      new Error('Something went wrong with setting port.')
    );
  }
  if (props.portAttempt === 'processing') {
    onlineDocumentGatewayProps.changePortAttempt = makeProcessingAttempt();
  }

  if (props.disconnectAttempt === 'error') {
    onlineDocumentGatewayProps.disconnectAttempt = makeErrorAttempt(
      new Error('Something went wrong with closing connection.')
    );
  }

  return (
    <OnlineDocumentGateway
      // Completely re-mount all components on props change. This impacts the default values of
      // inputs.
      key={JSON.stringify(props)}
      {...onlineDocumentGatewayProps}
    />
  );
}
