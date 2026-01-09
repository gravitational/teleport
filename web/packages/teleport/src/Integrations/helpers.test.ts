/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import {
  compareByTags,
  getStatus,
  getStatusAndLabel,
  withSortedTags,
  wrapLazyPresortCache,
} from 'teleport/Integrations/helpers';
import {
  Integration,
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { IntegrationLike, IntegrationTag, Status } from './types';

test.each`
  type                        | code                                       | expected
  ${'integration'}            | ${IntegrationStatusCode.Draft}             | ${Status.Success}
  ${'integration'}            | ${IntegrationStatusCode.Running}           | ${Status.Success}
  ${'integration'}            | ${IntegrationStatusCode.Unauthorized}      | ${Status.Success}
  ${'integration'}            | ${IntegrationStatusCode.SlackNotInChannel} | ${Status.Success}
  ${'integration'}            | ${IntegrationStatusCode.Unknown}           | ${Status.Success}
  ${'integration'}            | ${IntegrationStatusCode.OtherError}        | ${Status.Success}
  ${'external-audit-storage'} | ${IntegrationStatusCode.Draft}             | ${Status.Warning}
  ${'external-audit-storage'} | ${IntegrationStatusCode.Running}           | ${Status.Success}
  ${'external-audit-storage'} | ${IntegrationStatusCode.Unauthorized}      | ${Status.Success}
  ${'external-audit-storage'} | ${IntegrationStatusCode.SlackNotInChannel} | ${Status.Success}
  ${'external-audit-storage'} | ${IntegrationStatusCode.Unknown}           | ${Status.Success}
  ${'external-audit-storage'} | ${IntegrationStatusCode.OtherError}        | ${Status.Success}
  ${'any'}                    | ${IntegrationStatusCode.Draft}             | ${Status.Warning}
  ${'any'}                    | ${IntegrationStatusCode.Running}           | ${Status.Success}
  ${'any'}                    | ${IntegrationStatusCode.Unauthorized}      | ${Status.Error}
  ${'any'}                    | ${IntegrationStatusCode.SlackNotInChannel} | ${Status.Warning}
  ${'any'}                    | ${IntegrationStatusCode.Unknown}           | ${null}
  ${'any'}                    | ${IntegrationStatusCode.OtherError}        | ${Status.Error}
`(
  'getStatus type $type with code $code returns $expected',
  async ({ type, code, expected }) => {
    const item: IntegrationLike = {
      name: '',
      kind: undefined,
      resourceType: type,
      statusCode: code,
      tags: [],
    };
    const status = getStatus(item);
    expect(status).toBe(expected);
  }
);

test.each`
  type                        | code                                       | expectedLabelKind | expectedTitle
  ${'integration'}            | ${IntegrationStatusCode.Draft}             | ${'success'}      | ${'Draft'}
  ${'integration'}            | ${IntegrationStatusCode.Running}           | ${'success'}      | ${'Running'}
  ${'integration'}            | ${IntegrationStatusCode.Unauthorized}      | ${'success'}      | ${'Unauthorized'}
  ${'integration'}            | ${IntegrationStatusCode.SlackNotInChannel} | ${'success'}      | ${'Bot not invited to channel'}
  ${'integration'}            | ${IntegrationStatusCode.Unknown}           | ${'success'}      | ${'Unknown'}
  ${'integration'}            | ${IntegrationStatusCode.OtherError}        | ${'success'}      | ${'Unknown error'}
  ${'external-audit-storage'} | ${IntegrationStatusCode.Draft}             | ${'warning'}      | ${'Draft'}
  ${'external-audit-storage'} | ${IntegrationStatusCode.Running}           | ${'success'}      | ${'Running'}
  ${'external-audit-storage'} | ${IntegrationStatusCode.Unauthorized}      | ${'success'}      | ${'Unauthorized'}
  ${'external-audit-storage'} | ${IntegrationStatusCode.SlackNotInChannel} | ${'success'}      | ${'Bot not invited to channel'}
  ${'external-audit-storage'} | ${IntegrationStatusCode.Unknown}           | ${'success'}      | ${'Unknown'}
  ${'external-audit-storage'} | ${IntegrationStatusCode.OtherError}        | ${'success'}      | ${'Unknown error'}
  ${'any'}                    | ${IntegrationStatusCode.Draft}             | ${'warning'}      | ${'Draft'}
  ${'any'}                    | ${IntegrationStatusCode.Running}           | ${'success'}      | ${'Running'}
  ${'any'}                    | ${IntegrationStatusCode.Unauthorized}      | ${'danger'}       | ${'Unauthorized'}
  ${'any'}                    | ${IntegrationStatusCode.SlackNotInChannel} | ${'warning'}      | ${'Bot not invited to channel'}
  ${'any'}                    | ${IntegrationStatusCode.Unknown}           | ${'secondary'}    | ${'Unknown'}
  ${'any'}                    | ${IntegrationStatusCode.OtherError}        | ${'danger'}       | ${'Unknown error'}
`(
  'getStatusAndLabel type $type with code $code returns expected',
  async ({ type, code, expectedLabelKind, expectedTitle }) => {
    const item: Integration = {
      name: '',
      kind: undefined,
      resourceType: type,
      statusCode: code,
      tags: [],
    };
    const status = getStatusAndLabel(item);
    expect(status.status).toBe(expectedTitle);
    expect(status.labelKind).toBe(expectedLabelKind);
  }
);

const make = (name: string, tags: IntegrationTag[]): IntegrationLike => ({
  name,
  tags,
  resourceType: 'integration',
  kind: IntegrationKind.AwsOidc,
  statusCode: IntegrationStatusCode.Running,
});

describe('withSortedTags', () => {
  test('sorts an IntegrationLike tags alphabetically', () => {
    expect(
      withSortedTags(make('test', ['bot', 'scim', 'notifications']))
        .alphabeticalTags
    ).toEqual(['bot', 'notifications', 'scim']);
  });
});

describe('compareByTags', () => {
  test('comparing tag-by-tag alphabetically', () => {
    const a = withSortedTags(make('a', ['notifications', 'scim']));
    const b = withSortedTags(make('b', ['bot', 'scim']));

    expect(compareByTags(a, b)).toBeGreaterThan(0);
  });

  test('ties are broken by name', () => {
    const z = withSortedTags(make('ztest', ['bot', 'scim']));
    const b = withSortedTags(make('btest', ['bot', 'scim']));

    expect(compareByTags(z, b)).toBeGreaterThan(0);
    expect(compareByTags(b, z)).toBeLessThan(0);
  });
});

describe('wrapLazyPresortCache', () => {
  test('computes derived values lazily and caches by object', () => {
    const items = [{ id: 1 }, { id: 2 }, { id: 3 }];
    const compute = jest.fn((i: { id: number }) => i.id);
    const compare = (a: number, b: number) => a - b;

    const cmp = wrapLazyPresortCache(items, compute, compare);

    expect(cmp(items[2], items[0])).toBe(2);
    expect(compute).toHaveBeenCalledTimes(2);

    expect(cmp(items[2], items[0])).toBe(2);
    expect(compute).toHaveBeenCalledTimes(2);

    expect(cmp(items[1], items[0])).toBe(1);
    expect(compute).toHaveBeenCalledTimes(3);
  });
});
