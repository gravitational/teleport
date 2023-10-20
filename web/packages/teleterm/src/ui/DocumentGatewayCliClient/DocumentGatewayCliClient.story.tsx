/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { DocumentGatewayCliClient } from 'teleterm/ui/services/workspacesService';

import { WaitingForGatewayContent } from './DocumentGatewayCliClient';

export default {
  title: 'Teleterm/DocumentGatewayCliClient',
};

const doc: DocumentGatewayCliClient = {
  uri: '/docs/1234',
  title: 'psql',
  kind: 'doc.gateway_cli_client',
  rootClusterId: 'foo',
  leafClusterId: 'bar',
  status: '',
  targetName: 'postgres',
  targetProtocol: 'postgres',
  targetUri: '/clusters/foo/dbs/elo',
  targetUser: 'alice',
};

const docWithLongValues = {
  ...doc,
  targetName: 'sales-quarterly-fq1-2024-production',
  targetUser:
    'quux-quuz-foo-bar-quux-quuz-foo-bar-quux-quuz-foo-bar-quux-quuz-foo-bar',
};

const noop = () => {};

export const Waiting = (props: { doc?: DocumentGatewayCliClient }) => (
  <WaitingForGatewayContent
    hasTimedOut={false}
    doc={props.doc || doc}
    openConnection={noop}
  />
);

export const WaitingTimedOut = (props: { doc?: DocumentGatewayCliClient }) => (
  <WaitingForGatewayContent
    hasTimedOut={true}
    doc={props.doc || doc}
    openConnection={noop}
  />
);

export const WaitingWithLongValues = () => <Waiting doc={docWithLongValues} />;

export const WaitingTimedOutWithLongValues = () => (
  <WaitingTimedOut doc={docWithLongValues} />
);
