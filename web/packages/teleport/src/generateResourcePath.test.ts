/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import cfg, { UrlResourcesParams } from './config';
import generateResourcePath from './generateResourcePath';

test('undefined params are set to empty string', () => {
  expect(
    generateResourcePath(cfg.api.unifiedResourcesPath, { clusterId: 'cluster' })
  ).toStrictEqual(
    '/v1/webapi/sites/cluster/resources?searchAsRoles=&limit=&startKey=&kinds=&query=&search=&sort=&pinnedOnly=&includedResourceMode='
  );
});

test('defined params are set', () => {
  const unifiedParams: UrlResourcesParams = {
    query: 'query',
    search: 'search',
    sort: { fieldName: 'field', dir: 'DESC' },
    limit: 100,
    startKey: 'startkey',
    searchAsRoles: 'yes',
    pinnedOnly: true,
    includedResourceMode: 'all',
    kinds: ['app'],
  };
  expect(
    generateResourcePath(cfg.api.unifiedResourcesPath, {
      clusterId: 'cluster',
      ...unifiedParams,
    })
  ).toStrictEqual(
    '/v1/webapi/sites/cluster/resources?searchAsRoles=yes&limit=100&startKey=startkey&kinds=app&query=query&search=search&sort=field:desc&pinnedOnly=true&includedResourceMode=all'
  );
});

test('defined params but set to empty values are set to empty string', () => {
  const unifiedParams: UrlResourcesParams = {
    query: '',
    search: null,
    limit: 0,
    pinnedOnly: false,
    kinds: [],
  };
  expect(
    generateResourcePath(cfg.api.unifiedResourcesPath, {
      clusterId: 'cluster',
      ...unifiedParams,
    })
  ).toStrictEqual(
    '/v1/webapi/sites/cluster/resources?searchAsRoles=&limit=&startKey=&kinds=&query=&search=&sort=&pinnedOnly=&includedResourceMode='
  );
});
