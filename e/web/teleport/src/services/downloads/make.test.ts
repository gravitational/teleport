import { makeReleases } from './make';

test('makeReleases with null', () => {
  expect(makeReleases(null)).toEqual([]);
});

test('makeReleases with empty response', () => {
  expect(makeReleases('')).toEqual([]);
  expect(makeReleases([])).toEqual([]);
});

test('makeReleases with version and no assets', () => {
  expect(
    makeReleases([
      {
        version: 'v10.1.3',
        assets: [],
      },
    ])
  ).toEqual([{ version: 'v10.1.3', assets: [] }]);
});

test("makeReleases with missing fields on assets won't throw exceptions", () => {
  expect(
    makeReleases([
      {
        version: 'v10.1.3',
        assets: [{}],
      },
    ])
  ).toEqual([
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

test('makeReleases with version assets', () => {
  expect(
    makeReleases([
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
    ])
  ).toEqual([
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
