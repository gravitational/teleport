import api from 'teleport/services/api';

import { downloadsService } from './downloads';

test('correct formatting of release fetch response', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(mockReleasesResponse);

  const response = await downloadsService.fetchReleases();
  expect(response).toEqual([
    {
      version: 'v10.1.3',
      assets: [
        {
          description: 'tsh',
          displaySize: '100mb',
          kind: 'tsh client',
          name: 'tsh-asset-name',
          os: 'Linux',
          sha256: 'sha256',
          url: 'https://example.com/tsh',
        },
        {
          description: 'tsh',
          displaySize: '100mb',
          kind: 'tsh client',
          name: 'tsh-asset-name',
          os: 'Windows',
          sha256: 'sha256',
          url: 'https://example.com/tsh',
        },
        {
          description: 'tsh',
          displaySize: '100mb',
          kind: 'tsh client',
          name: 'tsh-asset-name',
          os: 'macOS',
          sha256: 'sha256',
          url: 'https://example.com/tsh',
        },
      ],
    },
  ]);
});

test('correct formatting of release fetch response without assets', async () => {
  jest.spyOn(api, 'get').mockResolvedValue([
    {
      version: 'v10.1.3',
      assets: [],
    },
  ]);

  const response = await downloadsService.fetchReleases();
  expect(response).toEqual([{ version: 'v10.1.3', assets: [] }]);
});

test('no error thrown of release fetch response with missing fields', async () => {
  jest.spyOn(api, 'get').mockResolvedValue([
    {
      version: 'v10.1.3',
      assets: [{}],
    },
  ]);

  const response = await downloadsService.fetchReleases();
  expect(response).toEqual([
    {
      version: 'v10.1.3',
      assets: [
        {
          description: '',
          displaySize: '',
          kind: 'Teleport',
          name: '',
          os: 'Linux',
          sha256: '',
          url: '',
        },
      ],
    },
  ]);
});

test('null response from release fetch', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(null);

  const response = await downloadsService.fetchReleases();
  expect(response).toEqual([]);
});

test('empty response from release fetch', async () => {
  jest.spyOn(api, 'get').mockResolvedValue('');

  const response = await downloadsService.fetchReleases();
  expect(response).toEqual([]);
});

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

const mockReleasesResponse = [
  {
    version: 'v10.1.3',
    assets: [
      {
        name: 'tsh-asset-name',
        os: 'linux',
        display_size: '100mb',
        description: 'tsh',
        sha256: 'sha256',
        public_url: 'https://example.com/tsh',
      },
      {
        name: 'tsh-asset-name',
        os: 'windows',
        display_size: '100mb',
        description: 'tsh',
        sha256: 'sha256',
        public_url: 'https://example.com/tsh',
      },
      {
        name: 'tsh-asset-name',
        os: 'darwin',
        display_size: '100mb',
        description: 'tsh',
        sha256: 'sha256',
        public_url: 'https://example.com/tsh',
      },
    ],
  },
];
