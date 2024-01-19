import { majorDifference, removeIncompatibleVersions } from './useDownloads';

describe('removeIncompatibleVersions', () => {
  const cases: {
    authVersion: string;
    versions: string[];
    expected: string[];
  }[] = [
    {
      authVersion: '10.0.0',
      versions: ['9.0.0', '10.0.0', '10.0.1'],
      expected: ['9.0.0', '10.0.0', '10.0.1'],
    },
    {
      authVersion: '1.0.0',
      versions: ['1.0.0', '2.0.0', '9.0.0', '10.0.0', '10.0.1'],
      expected: ['1.0.0'],
    },
    {
      authVersion: '1.0.0',
      versions: ['0.0.0', '0.1.0', '1.1.1', '2.0.0'],
      expected: ['0.0.0', '0.1.0', '1.1.1'],
    },
    {
      authVersion: '15.0.0',
      versions: ['9.0.0', '10.0.0', '10.0.1'],
      expected: [],
    },
    { authVersion: '9.1.0', versions: ['10.0.0', '10.0.1'], expected: [] },
  ];
  test.each(cases)(
    'test removeNewerVersions: authVersion=$authVersion and versions=$versions should return $expected',
    ({ authVersion, versions, expected }) => {
      const result = removeIncompatibleVersions(authVersion, versions);
      expect(result).toEqual(expected);
    }
  );
});

test('majorDifference', () => {
  expect(majorDifference('10.9.9', '10.9.9')).toBe(0);
  expect(majorDifference('11.9.9', '10.9.9')).toBe(1);
  expect(majorDifference('9.9.9', '10.9.9')).toEqual(-1);
  expect(majorDifference('10.9.9', '100.9.9')).toEqual(-90);
  expect(majorDifference('invalidinput', '10.0.0')).toBeNull();
  expect(majorDifference('10.0.0', 'invalidinput')).toBeNull();
});
