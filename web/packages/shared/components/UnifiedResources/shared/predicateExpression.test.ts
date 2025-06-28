/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { ResourceHealthStatus } from '../types';
import { buildPredicateExpression } from './predicateExpression';

type ResourceStatus = ResourceHealthStatus | '';

describe('getPredicateExpression', () => {
  const testCases: {
    name: string;
    requestedStatuses?: ResourceStatus[];
    requestedQuery?: string;
    expected: string | undefined;
  }[] = [
    {
      name: 'undefined fields',
      requestedStatuses: undefined,
      requestedQuery: undefined,
      expected: '',
    },
    {
      name: 'empty status array',
      requestedStatuses: [],
      expected: '',
    },
    {
      name: 'empty value in status array are ignored',
      requestedStatuses: ['', ''],
      expected: '',
    },
    {
      name: 'with only status',
      requestedStatuses: ['healthy'],
      expected: 'health.status == "healthy"',
    },
    {
      name: 'with multiple statuses',
      requestedStatuses: ['healthy', '', 'unhealthy'],
      expected: 'health.status == "healthy" || health.status == "unhealthy"',
    },
    {
      name: 'with only query',
      requestedQuery: 'name == "mysql"',
      expected: 'name == "mysql"',
    },
    {
      name: 'with status and query',
      requestedStatuses: ['healthy'],
      requestedQuery: 'name == "mysql"',
      expected: '(name == "mysql") && (health.status == "healthy")',
    },
    {
      name: 'with multi status and query',
      requestedStatuses: ['healthy', '', 'unhealthy'],
      requestedQuery: 'name == "mysql" || name == "postgres"',
      expected:
        '(name == "mysql" || name == "postgres") && (health.status == "healthy" || health.status == "unhealthy")',
    },
  ];

  test.each(testCases)(
    '$name',
    ({ requestedStatuses, requestedQuery, expected }) => {
      expect(
        buildPredicateExpression(requestedStatuses, requestedQuery)
      ).toEqual(expected);
    }
  );
});
