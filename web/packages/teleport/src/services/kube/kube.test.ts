/*
Copyright 2021 Gravitational, Inc.

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

import KubeService from './kube';
import api from 'teleport/services/api';

test('correct processed fetch response formatting', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(mockApiResponse);

  const kubeService = new KubeService();
  const response = await kubeService.fetchKubernetes('clusterId');

  expect(response).toEqual([
    {
      name: 'tele.logicoma.dev-prod',
      tags: ['kernal: 4.15.0-51-generic', 'env: prod'],
    },
  ]);
});

test('handling of null fetch response', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(null);

  const kubeService = new KubeService();
  const response = await kubeService.fetchKubernetes('clusterId');

  expect(response).toEqual([]);
});

test('handling of null labels', async () => {
  jest.spyOn(api, 'get').mockResolvedValue([{ name: 'test', labels: null }]);

  const kubeService = new KubeService();
  const response = await kubeService.fetchKubernetes('clusterId');

  expect(response).toEqual([{ name: 'test', tags: [] }]);
});

const mockApiResponse = [
  {
    name: 'tele.logicoma.dev-prod',
    labels: [
      { name: 'kernal', value: '4.15.0-51-generic' },
      { name: 'env', value: 'prod' },
    ],
  },
];
