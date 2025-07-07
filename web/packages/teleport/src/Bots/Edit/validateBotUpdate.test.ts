/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { validateBotUpdate } from './validateBotUpdate';

describe('validateBotUpdate', () => {
  it.each`
    name                                                    | prev                                              | req                                               | next                                              | fields
    ${'pass consistent roles'}                              | ${{ roles: [] }}                                  | ${{ roles: ['foo'] }}                             | ${{ roles: ['foo'] }}                             | ${[]}
    ${'detect inconsistent roles (added)'}                  | ${{ roles: [] }}                                  | ${{ roles: ['foo'] }}                             | ${{ roles: [] }}                                  | ${['roles']}
    ${'detect inconsistent roles (removed)'}                | ${{ roles: ['foo'] }}                             | ${{ roles: [] }}                                  | ${{ roles: ['foo'] }}                             | ${['roles']}
    ${'pass consistent traits'}                             | ${{ traits: [] }}                                 | ${{ traits: [{ name: 'bar', values: [] }] }}      | ${{ traits: [{ name: 'bar', values: [] }] }}      | ${[]}
    ${'detect inconsistent traits (added)'}                 | ${{ traits: [] }}                                 | ${{ traits: [{ name: 'bar', values: [] }] }}      | ${{ traits: [] }}                                 | ${['traits']}
    ${'detect inconsistent traits (removed)'}               | ${{ traits: [{ name: 'bar', values: [] }] }}      | ${{ traits: [] }}                                 | ${{ traits: [{ name: 'bar', values: [] }] }}      | ${['traits']}
    ${'pass consistent trait values'}                       | ${{ traits: [{ name: 'bar', values: [] }] }}      | ${{ traits: [{ name: 'bar', values: ['baz'] }] }} | ${{ traits: [{ name: 'bar', values: ['baz'] }] }} | ${[]}
    ${'detect inconsistent trait values (added)'}           | ${{ traits: [{ name: 'bar', values: [] }] }}      | ${{ traits: [{ name: 'bar', values: ['baz'] }] }} | ${{ traits: [{ name: 'bar', values: [] }] }}      | ${['traits']}
    ${'detect inconsistent trait values (removed)'}         | ${{ traits: [{ name: 'bar', values: ['baz'] }] }} | ${{ traits: [{ name: 'bar', values: [] }] }}      | ${{ traits: [{ name: 'bar', values: ['baz'] }] }} | ${['traits']}
    ${'pass consistent max_session_ttl'}                    | ${{ max_session_ttl: { seconds: 1 } }}            | ${{ max_session_ttl: '2m' }}                      | ${{ max_session_ttl: { seconds: 120 } }}          | ${[]}
    ${'pass consistent max_session_ttl (difference units)'} | ${{ max_session_ttl: { seconds: 1 } }}            | ${{ max_session_ttl: '120s' }}                    | ${{ max_session_ttl: { seconds: 120 } }}          | ${[]}
    ${'detect inconsistent max_session_ttl'}                | ${{ max_session_ttl: { seconds: 1 } }}            | ${{ max_session_ttl: '2m' }}                      | ${{ max_session_ttl: { seconds: 1 } }}            | ${['max_session_ttl']}
  `('should $name', ({ prev, req, next, fields }) => {
    expect(validateBotUpdate(prev, req, next)).toEqual(fields);
  });
});
