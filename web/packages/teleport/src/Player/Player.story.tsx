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

import { Flex } from 'design';

import { createMemoryRouter, RouterProvider } from 'react-router-dom';

import PlayerComponent from './Player';

export default {
  title: 'Teleport/Player',
};

const routes = [
  {
    path: '/web/cluster/:clusterId/session/:sid',
    element: (
      <Flex m={-3}>
        <PlayerComponent />
      </Flex>
    ),
  },
];

export const SSH = () => {
  const router = createMemoryRouter(routes, {
    initialEntries: ['/web/cluster/localhost/session/123?recordingType=ssh'],
    initialIndex: 0,
  });

  return <RouterProvider router={router} />;
};

export const Desktop = () => {
  const router = createMemoryRouter(routes, {
    initialEntries: [
      '/web/cluster/localhost/session/123?recordingType=desktop&durationMs=1234',
    ],
    initialIndex: 0,
  });

  return <RouterProvider router={router} />;
};

export const RecordingTypeError = () => {
  const router = createMemoryRouter(routes, {
    initialEntries: ['/web/cluster/localhost/session/123?recordingType=bla'],
    initialIndex: 0,
  });

  return <RouterProvider router={router} />;
};

export const DurationMsError = () => {
  const router = createMemoryRouter(routes, {
    initialEntries: [
      '/web/cluster/localhost/session/123?recordingType=desktop&durationMs=blabla',
    ],
    initialIndex: 0,
  });

  return <RouterProvider router={router} />;
};
