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

import { TestLayout } from 'teleport/Console/Console.story';
import ConsoleContext from 'teleport/Console/consoleContext';

import ClusterSelector from './ClusterSelector';

export default {
  title: 'Teleport/Console/DocumentNodes/ClusterSelector',
};

export const Component = () => {
  const ctx = mockContext();
  return renderlusterSelector(ctx, {
    defaultMenuIsOpen: true,
  });
};

export const Loading = () => {
  const ctx = mockContext();
  ctx.fetchClusters = () => {
    return new Promise<any>(() => null);
  };

  return renderlusterSelector(ctx, {
    defaultMenuIsOpen: true,
  });
};

export const Failed = () => {
  const ctx = mockContext();
  ctx.fetchClusters = () => {
    return Promise.reject(new Error('server error'));
  };

  return renderlusterSelector(ctx, {
    defaultMenuIsOpen: true,
  });
};

function renderlusterSelector(ctx, { ...props } = {}) {
  return (
    <TestLayout ctx={ctx}>
      <ClusterSelector
        mx="auto"
        open={true}
        value={'clusterId'}
        width="400px"
        maxMenuHeight={200}
        onChange={() => null}
        {...props}
      />
    </TestLayout>
  );
}

function mockContext() {
  const ctx = new ConsoleContext();
  ctx.fetchClusters = () => {
    return Promise.resolve<any>(clusters);
  };

  return ctx;
}

const clusters = [
  {
    clusterId: 'cluster-Cordelia-Lynch',
  },
  {
    clusterId: 'cluster-Cameron-Smith',
  },
  {
    clusterId: 'cluster-Agnes-Lee',
  },
  {
    clusterId: 'cluster-Victor-Nguyen',
  },
  {
    clusterId: 'cluster-Catherine-Dennis',
  },
  {
    clusterId: 'cluster-Bertha-Maldonado',
  },
  {
    clusterId: 'cluster-Hulda-Mullins',
  },
  {
    clusterId: 'cluster-Mary-Andrews',
  },
];
