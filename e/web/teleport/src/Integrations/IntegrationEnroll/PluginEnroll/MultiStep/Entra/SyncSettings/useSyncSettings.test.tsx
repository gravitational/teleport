import type { PluginEntraIdSyncIntervals } from 'teleport/services/integrations';

import { hasZeroFilters } from './useSyncSettings';
import { makeSyncInterval } from './useSyncSettings';

describe('makeDefaultSyncInterval', () => {
  test.each<{
    name: string;
    input: Partial<PluginEntraIdSyncIntervals>;
    expected: PluginEntraIdSyncIntervals;
  }>([
    {
      name: 'missing intervals',
      input: undefined,
      expected: { full: '0s', delta: '0s' },
    },
    {
      name: 'both intervals empty',
      input: { full: '', delta: '' },
      expected: { full: '0s', delta: '0s' },
    },
    {
      name: 'only full configured',
      input: { full: '1h' },
      expected: { full: '1h', delta: '0s' },
    },
    {
      name: 'only delta configured',
      input: { delta: '2m' },
      expected: { full: '0s', delta: '2m' },
    },
    {
      name: 'both intervals configured',
      input: { full: '1h', delta: '2m' },
      expected: { full: '1h', delta: '2m' },
    },
    {
      name: 'explicit zero values are preserved',
      input: { full: '0', delta: '0s' },
      expected: { full: '0', delta: '0s' },
    },
  ])('$name', ({ input, expected }) => {
    expect(makeSyncInterval(input)).toEqual(expected);
  });
});

describe('hasZeroFilters', () => {
  const predicates: {
    name: string;
    filters: any;
    expected: boolean;
  }[] = [
    {
      name: 'with all filters',
      filters: {
        id: ['g1', 'g2'],
        nameRegex: ['admin-*'],
        excludeId: ['g2'],
        excludeNameRegex: ['hr*'],
      },
      expected: false,
    },
    {
      name: 'partial filters',
      filters: {
        id: ['g1', 'g2'],
        nameRegex: [],
        excludeId: ['g2'],
        excludeNameRegex: ['hr*'],
      },
      expected: false,
    },
    {
      name: 'empty filters',
      filters: {
        id: [],
        nameRegex: [],
        excludeId: [],
        excludeNameRegex: [],
      },
      expected: true,
    },
    {
      name: 'empty',
      filters: {},
      expected: true,
    },
  ];
  test.each(predicates)('$name', ({ filters, expected }) => {
    expect(hasZeroFilters(filters)).toBe(expected);
  });
});
