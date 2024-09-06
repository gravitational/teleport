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
  ctx.clustersService.fetchClusters = () => {
    return new Promise<any>(() => null);
  };

  return renderlusterSelector(ctx, {
    defaultMenuIsOpen: true,
  });
};

export const Failed = () => {
  const ctx = mockContext();
  ctx.clustersService.fetchClusters = () => {
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
  ctx.clustersService.fetchClusters = () => {
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
