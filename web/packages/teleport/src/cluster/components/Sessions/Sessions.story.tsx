/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';
import { keyBy } from 'lodash';
import { Sessions } from './Sessions';

type PropTypes = Parameters<typeof Sessions>[0];

export default {
  title: 'Teleport/Sessions',
};

export function Loaded() {
  const props = {
    sessions,
    onRefresh,
    nodes,
    attempt: {
      isSuccess: true,
      isProcessing: false,
      isFailed: false,
      message: '',
    },
  };

  return (
    <Router history={createMemoryHistory()}>
      <Sessions {...props} />
    </Router>
  );
}

const onRefresh: PropTypes['onRefresh'] = () => {
  return Promise.resolve();
};

const sessions = [
  {
    id: 'BZ',
    namespace: 'AG',
    login: 'root',
    active: 'AZ',
    created: new Date('2019-04-22T00:00:51.543Z'),
    durationText: '12 min',
    serverId: '10_128_0_6.demo.gravitational.io',
    clusterId: '',
    hostname: 'localhost',
    sid: 'sid0',
    parties: [
      {
        user: 'hehwawe@aw.sg',
        remoteAddr: '129.232.123.132',
      },
      {
        user: 'ma@pewu.tz',
        remoteAddr: '129.232.123.132',
      },
    ],
  },
];

const nodes = keyBy(
  [
    {
      clusterId: 'localhost',
      tunnel: false,
      tags: [],
      addr: '232.232.323.232',
      advertiseIp: '10.128.0.6',
      hostname: 'demo.gravitational.io',
      id: '10_128_0_6.demo.gravitational.io',
    },
    {
      clusterId: 'localhost',
      tunnel: false,
      tags: [],
      addr: '232.232.323.232',
      advertiseIp: '10.128.0.6',
      hostname: 'demo.gravitational.io',
      id: '10_128_0_6.demo.gravitational.io',
    },
  ],
  'id'
);
