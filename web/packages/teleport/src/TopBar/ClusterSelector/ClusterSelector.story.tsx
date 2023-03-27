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

import ClusterSelector from './ClusterSelector';

export default {
  title: 'Teleport/TopBar/ClusterSelector',
};

export const Component = () => {
  return renderlusterSelector({
    defaultMenuIsOpen: true,
    onLoad: () => Promise.resolve(clusters),
  });
};

export const Loading = () => {
  return renderlusterSelector({
    defaultMenuIsOpen: true,
    onLoad: () => new Promise<any>(() => null),
  });
};

export const Failed = () => {
  return renderlusterSelector({
    defaultMenuIsOpen: true,
    onLoad: () => Promise.reject(new Error('server error')),
  });
};

function renderlusterSelector(props) {
  return (
    <Flex style={{ position: 'absolute' }} width="100%" height="100%">
      <ClusterSelector
        mx="auto"
        open={true}
        value={'clusterIdfsdfsdfsdfsdfsdfsdfsdfsdfsdfsdfsdfsdff'}
        width="384px"
        maxMenuHeight={200}
        onChange={() => null}
        {...props}
      />
    </Flex>
  );
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
    clusterId: 'cluster-Mary-250-Andrews',
  },
  {
    clusterId: 'cluster-Mary-158-Andrews',
  },
  {
    clusterId: 'cluster-Mary-4-Andrews',
  },
  {
    clusterId: 'cluster-80-Mary-228-Andrews',
  },
  {
    clusterId: 'cluster-189-Mary-228-Andrews',
  },
  {
    clusterId: 'cluster-145-Mary-228-Andrews',
  },
  {
    clusterId: 'cluster-163-Mary-228-Andrews',
  },
  {
    clusterId: 'cluster-132-Mary-228-Andrews',
  },
  {
    clusterId: 'cluster-218-Mary-228-Andrews',
  },
  {
    clusterId: 'cluster-58-Mary-228-Andrews',
  },
  {
    clusterId: 'cluster-227-Mary-228-Andrews',
  },
  {
    clusterId: 'cluster-67-Mary-228-Andrews',
  },
  {
    clusterId: 'cluster-77-Mary-228-Andrews',
  },
  {
    clusterId: 'cluster-221-Mary-228-Andrews',
  },
  {
    clusterId: 'cluster-103-Mary-228-Andrews',
  },
  {
    clusterId: 'cluster-146-Mary-228-Andrews',
  },
  {
    clusterId: 'cluster-187-Mary-228-Andrews',
  },
  {
    clusterId: 'cluster-202-Mary-228-Andrews',
  },
  {
    clusterId: 'cluster-210-Mary-228-Andrews',
  },
  {
    clusterId: 'cluster-5-Mary-228-Andrews',
  },
  {
    clusterId: 'cluster-188-Mary-228-Andrews',
  },
  {
    clusterId: 'cluster-197-Mary-228-Andrews',
  },
  {
    clusterId: 'cluster-122-Mary-228-Andrews',
  },
];
