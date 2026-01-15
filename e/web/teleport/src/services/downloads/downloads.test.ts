import api from 'teleport/services/api';

import { downloadsService } from './downloads';

test('correct formatting of license fetch response', async () => {
  const now = new Date();

  jest.spyOn(api, 'get').mockResolvedValue({
    pem: 'pem',
    expiry: now,
  });

  const response = await downloadsService.fetchLicense();
  expect(response).toEqual({
    pem: 'pem',
    expiry: now,
  });
});

test('null response from license fetch', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(null);

  const response = await downloadsService.fetchLicense();
  expect(response).toEqual({ pem: '', expiry: undefined });
});

test('empty response from license fetch', async () => {
  jest.spyOn(api, 'get').mockResolvedValue('');

  const response = await downloadsService.fetchLicense();
  expect(response).toEqual({ pem: '', expiry: undefined });
});

test('invalid fields in response from license fetch', async () => {
  jest.spyOn(api, 'get').mockResolvedValue({ key: 'val' });

  const response = await downloadsService.fetchLicense();
  expect(response).toEqual({ pem: '', expiry: undefined });
});
