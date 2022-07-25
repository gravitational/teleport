import api from 'teleport/services/api';

import clusterService from './clusters';

test('correct formatting of clusters fetch response', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(mockResponse);

  const response = await clusterService.fetchClusters();

  expect(response).toEqual([
    {
      clusterId: 'im-a-cluster-name',
      lastConnected: new Date('2022-02-02T14:03:00.355597-05:00'),
      connectedText: '2022-02-02 19:03:00',
      status: 'online',
      url: '/web/cluster/im-a-cluster-name/',
      authVersion: '8.0.0-alpha.1',
      nodeCount: 1,
      publicURL: 'mockurl:3080',
      proxyVersion: '8.0.0-alpha.1',
    },
  ]);
});

const mockResponse = [
  {
    name: 'im-a-cluster-name',
    lastConnected: '2022-02-02T14:03:00.355597-05:00',
    status: 'online',
    nodeCount: 1,
    publicURL: 'mockurl:3080',
    authVersion: '8.0.0-alpha.1',
    proxyVersion: '8.0.0-alpha.1',
  },
];
