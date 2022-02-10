/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { Server } from 'teleterm/services/tshd/types';
import { ServerConnect } from './ServerConnect';
import { State } from './useServerConnect';

export default {
  title: 'Teleterm/ServerConnect',
};

export const Story = () => {
  const server: Server = {
    uri: 'clusters/localhost/servers/hostname3',
    tunnel: false,
    name: 'server1',
    clusterId: 'localhost',
    hostname: 'hostname3',
    addr: '123.12.12.12',
    labelsList: [{ name: 'os', value: 'linux' }],
  };

  const props: State = {
    server,
    logins: ['a', 'test', 'test2', 'test3', 'test4', 'test5'],
    connect: () => null,
    onClose: () => null,
  };

  return <ServerConnect {...props} />;
};
