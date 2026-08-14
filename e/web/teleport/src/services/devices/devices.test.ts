import api from 'teleport/services/api';

import { deviceService } from './devices';

test('fetch devices', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(deviceResponse);

  const response = await deviceService.fetchDevices();

  expect(response).toEqual(deviceResponse);
});

const deviceResponse = {
  items: [
    {
      id: 'goteleport.local',
      assetTag: 'CSXXXXXXXXX',
      osType: 'macOS',
      enrollStatus: 'enrolled',
    },
    {
      id: 'goteleport.local',
      assetTag: 'DSXXXXXXXXX',
      osType: 'Linux',
      enrollStatus: 'not enrolled',
    },
    {
      id: 'goteleport.local',
      assetTag: 'ESXXXXXXXXX',
      osType: 'Windows',
      enrollStatus: 'enrolled',
    },
  ],
  startKey: '',
};

test('check null response', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(nullResponse);

  const devices = deviceService;
  const response = await devices.fetchDevices();

  expect(response).toEqual({ items: [], startKey: '' });
});

const nullResponse = {
  items: null,
  startKey: '',
};
