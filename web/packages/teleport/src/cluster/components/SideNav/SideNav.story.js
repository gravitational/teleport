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
import { storiesOf } from '@storybook/react';
import * as Icons from 'design/Icon';
import { Box } from 'design';
import { ClusterSideNav } from './SideNav';

storiesOf('Teleport', module).add('SideNav', () => {
  const props = {
    ...defaultProps,
  };

  const inMemoryHistory = createMemoryHistory({});
  return (
    <Box
      mt={-3}
      height="100%"
      style={{ position: 'absolute', overflow: 'hidden' }}
    >
      <Router history={inMemoryHistory}>
        <ClusterSideNav {...props} />
      </Router>
    </Box>
  );
});

const clusterOptions = [
  { value: 'Kawupic', label: 'Kawupic' },
  { value: 'Ajiromil', label: 'Ajiromil' },
  { value: 'Wedolarav', label: 'Wedolarav' },
  { value: 'Urijorun', label: 'Urijorun' },
  { value: 'Irosutno', label: 'Irosutno' },
  { value: 'Wecdivrof', label: 'Wecdivrof' },
  { value: 'Nesbokti', label: 'Nesbokti' },
  { value: 'Nezfifku', label: 'Nezfifku' },
  { value: 'Cebfabca', label: 'Cebfabca' },
  { value: 'Cidbudud', label: 'Cidbudud' },
  { value: 'Jeakalu', label: 'Jeakalu' },
  { value: 'Luwcohba', label: 'Luwcohba' },
];

const defaultProps = {
  clusterOptions,
  clusterName: 'Wecdivrof',
  items: [
    {
      title: 'Apartment',
      Icon: Icons.Apartment,
      exact: true,
      to: '/web/site/apartment',
    },
    {
      title: 'Apple',
      Icon: Icons.Apple,
      exact: true,
      to: '/web/site/apple',
    },
    {
      title: 'Camera',
      Icon: Icons.Camera,
      exact: true,
      to: '/web/site/camera',
    },
  ],
  version: '6.0.0',
  homeUrl: '/localhost',
};
